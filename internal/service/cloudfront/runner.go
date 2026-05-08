package cloudfront

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dop251/goja"
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

// runFunction evaluates code with the given event JSON and returns the
// captured output. The contract matches CloudFront Functions: the
// script must define `function handler(event)`, the event is the
// parsed eventJSON, and the handler's return value is JSON-serialised
// into FunctionOutput. console.log calls accumulate into LogLines.
func runFunction(code []byte, eventJSON string) runFunctionResult {
	vm := goja.New()
	vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))

	logs := installConsole(vm)

	if err := installEvent(vm, eventJSON); err != nil {
		return runFunctionResult{LogLines: logs.lines(), Error: err.Error()}
	}

	if _, err := vm.RunString(string(code)); err != nil {
		return runFunctionResult{LogLines: logs.lines(), Error: err.Error()}
	}

	handler, ok := goja.AssertFunction(vm.Get("handler"))
	if !ok {
		return runFunctionResult{
			LogLines: logs.lines(),
			Error:    "function did not define a global `handler(event)`",
		}
	}

	timer := time.AfterFunc(defaultExecutionTimeout, func() {
		vm.Interrupt(errors.New("execution timeout"))
	})
	defer timer.Stop()

	ev := vm.Get("event")

	out, err := handler(goja.Undefined(), ev)
	if err != nil {
		return runFunctionResult{LogLines: logs.lines(), Error: err.Error()}
	}

	encoded, err := encodeReturn(out)
	if err != nil {
		return runFunctionResult{LogLines: logs.lines(), Error: err.Error()}
	}

	return runFunctionResult{LogLines: logs.lines(), Output: encoded}
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

func (c *console) emit(args []goja.Value) {
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
func installConsole(vm *goja.Runtime) *console {
	c := &console{}

	emit := func(call goja.FunctionCall) goja.Value {
		c.emit(call.Arguments)

		return goja.Undefined()
	}

	cons := vm.NewObject()
	for _, name := range []string{"log", "info", "warn", "error"} {
		_ = cons.Set(name, emit)
	}

	_ = vm.Set("console", cons)

	return c
}

// installEvent decodes the JSON event into a JS object and binds it
// to the global `event` name.
func installEvent(vm *goja.Runtime, eventJSON string) error {
	if strings.TrimSpace(eventJSON) == "" {
		if err := vm.Set("event", goja.Null()); err != nil {
			return fmt.Errorf("failed to install event: %w", err)
		}

		return nil
	}

	var parsed any
	if err := json.Unmarshal([]byte(eventJSON), &parsed); err != nil {
		return fmt.Errorf("invalid event JSON: %w", err)
	}

	if err := vm.Set("event", parsed); err != nil {
		return fmt.Errorf("failed to install event: %w", err)
	}

	return nil
}

// encodeReturn serialises the handler's return value back into JSON
// the way CloudFront returns FunctionOutput.
func encodeReturn(v goja.Value) (string, error) {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return "", nil
	}

	exported := v.Export()

	out, err := json.Marshal(exported)
	if err != nil {
		return "", fmt.Errorf("failed to encode handler return: %w", err)
	}

	return string(out), nil
}
