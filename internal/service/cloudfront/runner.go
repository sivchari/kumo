package cloudfront

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fastschema/qjs"
)

// runFunctionResult mirrors the AWS TestFunction response: stdout-like
// log lines from console.log calls, the JSON-serialised return value
// of the handler (FunctionOutput), and any error message from the JS
// runtime.
type runFunctionResult struct {
	LogLines []string
	Output   string
	Error    string
}

// defaultExecutionTimeout caps how long a TestFunction invocation can
// run. Real CloudFront enforces 1ms; kumo gives orders of magnitude
// more so test events with deliberate work still complete, while
// runaway loops still terminate.
const defaultExecutionTimeout = 5 * time.Second

// maxConsoleLogLines and maxConsoleLogBytes bound the console buffer.
// Without these a handler can do `for (;;) console.log(big_string)` and
// balloon kumo's heap before the execution timeout fires (timeouts only
// stop the VM, they don't reclaim what the buffer already accumulated).
const (
	maxConsoleLogLines = 1024
	maxConsoleLogBytes = 64 * 1024
)

var (
	errExecutionTimeout      = errors.New("execution timeout")
	errMissingFunctionHandle = errors.New("function did not define a global `handler(event)`")
)

// runFunction evaluates code with the given event JSON and returns the
// captured output. The contract matches CloudFront Functions: the
// script must define `function handler(event)`, the event is the
// parsed eventJSON, and the handler's return value is JSON-serialised
// into FunctionOutput. console.log calls accumulate into LogLines.
func runFunction(code []byte, eventJSON string) (result runFunctionResult) {
	logs := &console{}

	var timedOut atomic.Bool

	defer recoverRunFunction(&result, &logs, &timedOut)

	execCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt, ctx, err := newFunctionRuntime(execCtx)
	if err != nil {
		return runFunctionResult{LogLines: logs.lines(), Error: err.Error()}
	}
	defer rt.Close()

	logs = installConsole(ctx)

	if err := installEvent(ctx, eventJSON); err != nil {
		return runFunctionResult{LogLines: logs.lines(), Error: err.Error()}
	}

	value, handler, err := loadFunctionHandler(ctx, code)
	if err != nil {
		return runFunctionResult{LogLines: logs.lines(), Error: err.Error()}
	}

	if value != nil {
		defer value.Free()
	}

	defer handler.Free()

	ev := ctx.Global().GetPropertyStr("event")
	defer ev.Free()

	out, err := invokeFunctionHandler(ctx, handler, ev, cancel, &timedOut)
	if err != nil {
		return runFunctionResult{LogLines: logs.lines(), Error: err.Error()}
	}

	defer out.Free()

	encoded, err := encodeReturn(out)
	if err != nil {
		return runFunctionResult{LogLines: logs.lines(), Error: err.Error()}
	}

	return runFunctionResult{LogLines: logs.lines(), Output: encoded}
}

func newFunctionRuntime(execCtx context.Context) (*qjs.Runtime, *qjs.Context, error) {
	rt, err := qjs.New(qjs.Option{
		Context:            execCtx,
		CloseOnContextDone: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create qjs runtime: %w", err)
	}

	ctx := rt.Context()

	return rt, ctx, nil
}

func loadFunctionHandler(ctx *qjs.Context, code []byte) (*qjs.Value, *qjs.Value, error) {
	value, err := ctx.Eval("cloudfront-function.js", qjs.Code(string(code)))
	if err != nil {
		return nil, nil, fmt.Errorf("evaluate function source: %w", err)
	}

	handler := ctx.Global().GetPropertyStr("handler")
	if !handler.IsFunction() {
		handler.Free()

		if value != nil {
			value.Free()
		}

		return nil, nil, errMissingFunctionHandle
	}

	return value, handler, nil
}

func invokeFunctionHandler(
	ctx *qjs.Context,
	handler *qjs.Value,
	ev *qjs.Value,
	cancel context.CancelFunc,
	timedOut *atomic.Bool,
) (*qjs.Value, error) {
	timer := time.AfterFunc(defaultExecutionTimeout, func() {
		timedOut.Store(true)
		cancel()
	})
	defer timer.Stop()

	out, err := ctx.Invoke(handler, ctx.Global(), ev)

	if timedOut.Load() {
		if out != nil {
			out.Free()
		}

		return nil, errExecutionTimeout
	}

	if err != nil {
		return nil, fmt.Errorf("invoke function handler: %w", err)
	}

	return out, nil
}

func recoverRunFunction(result *runFunctionResult, logs **console, timedOut *atomic.Bool) {
	if recovered := recover(); recovered != nil {
		if timedOut.Load() {
			*result = runFunctionResult{LogLines: (*logs).lines(), Error: errExecutionTimeout.Error()}

			return
		}

		*result = runFunctionResult{LogLines: (*logs).lines(), Error: fmt.Sprint(recovered)}
	}
}

// console accumulates log/info/warn/error lines, with both line-count
// and byte-total caps so a misbehaving (or hostile) handler can't fill
// kumo's heap by spamming console output before the execution timeout
// fires. Once either cap is hit, further calls become no-ops.
type console struct {
	buf       []string
	totalSize int
}

func (c *console) lines() []string { return append([]string(nil), c.buf...) }

func (c *console) emit(args []*qjs.Value) {
	if len(c.buf) >= maxConsoleLogLines || c.totalSize >= maxConsoleLogBytes {
		return
	}

	parts := make([]string, 0, len(args))
	for _, a := range args {
		parts = append(parts, a.String())
	}

	line := strings.Join(parts, " ")

	// Truncate an oversize single line rather than dropping it: makes
	// debugging an over-logging script easier than a silent miss.
	if remaining := maxConsoleLogBytes - c.totalSize; len(line) > remaining {
		line = line[:remaining]
	}

	c.buf = append(c.buf, line)
	c.totalSize += len(line)
}

// installConsole exposes a console.log/info/warn/error global. The four
// share the same accumulator — real CloudFront Functions only have
// console.log, but accepting the others avoids spurious failures from
// scripts that lifted from a Node.js codebase.
func installConsole(ctx *qjs.Context) *console {
	c := &console{}

	emit := func(this *qjs.This) (*qjs.Value, error) {
		c.emit(this.Args())

		return this.Context().NewUndefined(), nil
	}

	cons := ctx.NewObject()
	for _, name := range []string{"log", "info", "warn", "error"} {
		cons.SetPropertyStr(name, ctx.Function(emit))
	}

	ctx.Global().SetPropertyStr("console", cons)

	return c
}

// installEvent decodes the JSON event into a JS object and binds it
// to the global `event` name.
func installEvent(ctx *qjs.Context, eventJSON string) error {
	if strings.TrimSpace(eventJSON) == "" {
		ctx.Global().SetPropertyStr("event", ctx.NewNull())

		return nil
	}

	var parsed any
	if err := json.Unmarshal([]byte(eventJSON), &parsed); err != nil {
		return fmt.Errorf("invalid event JSON: %w", err)
	}

	ctx.Global().SetPropertyStr("event", ctx.ParseJSON(eventJSON))

	return nil
}

// encodeReturn serialises the handler's return value back into JSON
// the way CloudFront returns FunctionOutput.
func encodeReturn(v *qjs.Value) (string, error) {
	if v == nil || v.IsUndefined() || v.IsNull() {
		return "", nil
	}

	out, err := v.JSONStringify()
	if err != nil {
		return "", fmt.Errorf("failed to encode handler return: %w", err)
	}

	return out, nil
}
