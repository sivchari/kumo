package lambda

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// bootstrapEntry is the executable file name in a provided.* runtime zip.
	bootstrapEntry = "bootstrap"
	// processStartGrace is how long ensureRunning waits after launching a
	// bootstrap to catch an immediate exit (bad binary / arch mismatch) before
	// returning. The invocation rendezvous itself is handled by the broker.
	processStartGrace = 250 * time.Millisecond
	// maxBootstrapBytes bounds how much is extracted from a function zip,
	// guarding against a decompression bomb (AWS caps unzipped code at 250 MB).
	maxBootstrapBytes = 512 << 20
)

// processExecutor runs provided.* Lambda bootstraps as child processes inside
// the kumo process, connected to kumo's own Runtime API. It is the in-process
// equivalent of LocalStack's docker lambda executor: no external sidecar, the
// emulator launches the function itself. Enabled via KUMO_LAMBDA_EXECUTOR=process.
type processExecutor struct {
	broker         *runtimeBroker
	runtimeAPIHost string        // host:port of kumo's Runtime API, e.g. "127.0.0.1:4566"
	workDir        string        // base dir for extracted bootstraps
	startGrace     time.Duration // window after launch to catch an immediate exit

	// ctx bounds the lifetime of every launched bootstrap; cancel (on close)
	// tears them all down.
	ctx    context.Context
	cancel context.CancelFunc

	mu    sync.Mutex
	procs map[string]*managedProc
}

// managedProc tracks a launched bootstrap so warm invocations skip relaunch and
// a dead process is detected for relaunch on the next invoke.
type managedProc struct {
	cmd   *exec.Cmd
	alive atomic.Bool
}

func newProcessExecutor(broker *runtimeBroker, runtimeAPIHost string) *processExecutor {
	ctx, cancel := context.WithCancel(context.Background())

	return &processExecutor{
		broker:         broker,
		runtimeAPIHost: runtimeAPIHost,
		workDir:        filepath.Join(os.TempDir(), "kumo-lambda"),
		startGrace:     processStartGrace,
		ctx:            ctx,
		cancel:         cancel,
		procs:          make(map[string]*managedProc),
	}
}

// supports reports whether a function can run via the process executor. Only
// custom runtimes whose zip carries a directly-executable bootstrap qualify;
// managed runtimes (python/node/...) need an interpreter the image lacks.
func (e *processExecutor) supports(fn *Function) bool {
	if fn == nil || fn.Code == nil || len(fn.Code.ZipFile) == 0 {
		return false
	}

	return strings.HasPrefix(fn.Runtime, "provided") || strings.HasPrefix(fn.Runtime, "go")
}

// ensureRunning launches the function's bootstrap if it is not already running,
// pointed at kumo's Runtime API. A warm process is reused. It returns once the
// process is started (or fails fast if it exits immediately).
func (e *processExecutor) ensureRunning(fn *Function) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if mp := e.procs[fn.FunctionName]; mp != nil && mp.alive.Load() {
		return nil
	}

	bootstrap, err := e.extractBootstrap(fn)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(e.ctx, bootstrap) //nolint:gosec // launching the function's own bootstrap is the opt-in executor's purpose
	cmd.Env = e.buildEnv(fn)
	// Surface the function's stdout/stderr in kumo's container logs for debugging.
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start bootstrap for %s: %w", fn.FunctionName, err)
	}

	mp := &managedProc{cmd: cmd}
	mp.alive.Store(true)
	e.procs[fn.FunctionName] = mp

	name := fn.FunctionName
	done := make(chan struct{})

	go func() {
		_ = cmd.Wait()

		mp.alive.Store(false)
		// Drop the broker registration so the next invoke relaunches instead of
		// waiting on a dead handler.
		e.broker.deregister(name)
		close(done)
	}()

	select {
	case <-done:
		return fmt.Errorf("bootstrap for %s exited immediately", fn.FunctionName)
	case <-time.After(e.startGrace):
		return nil
	}
}

// extractBootstrap writes the zip's bootstrap entry to a per-function work dir
// and returns its path.
func (e *processExecutor) extractBootstrap(fn *Function) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(fn.Code.ZipFile), int64(len(fn.Code.ZipFile)))
	if err != nil {
		return "", fmt.Errorf("read function zip for %s: %w", fn.FunctionName, err)
	}

	dir := filepath.Join(e.workDir, fn.FunctionName)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create work dir: %w", err)
	}

	for _, f := range zr.File {
		if filepath.Base(f.Name) == bootstrapEntry {
			return writeBootstrap(dir, f)
		}
	}

	return "", fmt.Errorf("no %q entry in zip for %s", bootstrapEntry, fn.FunctionName)
}

// writeBootstrap copies one zip entry to dir/bootstrap with the executable bit.
func writeBootstrap(dir string, f *zip.File) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", fmt.Errorf("open bootstrap in zip: %w", err)
	}

	defer func() { _ = rc.Close() }()

	out := filepath.Join(dir, bootstrapEntry)

	dst, err := os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755) //nolint:gosec // function bootstrap is executable by design
	if err != nil {
		return "", fmt.Errorf("create bootstrap file: %w", err)
	}

	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, io.LimitReader(rc, maxBootstrapBytes)); err != nil {
		return "", fmt.Errorf("write bootstrap: %w", err)
	}

	return out, nil
}

// buildEnv composes the child process environment: kumo's environment as a base,
// then the function's configured variables, then the Runtime API wiring and
// Lambda identity (which take precedence). Dummy AWS credentials and region are
// injected when absent so the function's aws-sdk LoadDefaultConfig succeeds.
func (e *processExecutor) buildEnv(fn *Function) []string {
	env := map[string]string{}

	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok {
			env[k] = v
		}
	}

	for k, v := range map[string]string{
		"AWS_REGION":            "us-east-1",
		"AWS_DEFAULT_REGION":    "us-east-1",
		"AWS_ACCESS_KEY_ID":     "test",
		"AWS_SECRET_ACCESS_KEY": "test",
	} {
		if env[k] == "" {
			env[k] = v
		}
	}

	if fn.Environment != nil {
		maps.Copy(env, fn.Environment.Variables)
	}

	env["AWS_LAMBDA_RUNTIME_API"] = e.runtimeAPIHost + "/_runtime/" + fn.FunctionName
	env["AWS_LAMBDA_FUNCTION_NAME"] = fn.FunctionName
	env["AWS_LAMBDA_FUNCTION_VERSION"] = "$LATEST"
	env["_HANDLER"] = fn.Handler

	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}

	return out
}

// stop kills a function's process (if any) and drops its broker registration.
// Called on DeleteFunction.
func (e *processExecutor) stop(fnName string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if mp := e.procs[fnName]; mp != nil {
		if mp.cmd.Process != nil {
			_ = mp.cmd.Process.Kill()
		}

		delete(e.procs, fnName)
	}

	e.broker.deregister(fnName)
}

// close kills every running bootstrap. Called on service shutdown.
func (e *processExecutor) close() {
	e.cancel()

	e.mu.Lock()
	defer e.mu.Unlock()

	for name, mp := range e.procs {
		if mp.cmd.Process != nil {
			_ = mp.cmd.Process.Kill()
		}

		delete(e.procs, name)
	}
}
