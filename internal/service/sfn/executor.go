package sfn

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// stateMachineDefinition represents a parsed Step Functions state machine definition.
//
//nolint:tagliatelle // AWS Step Functions definition uses PascalCase
type stateMachineDefinition struct {
	Comment string                     `json:"Comment"`
	StartAt string                     `json:"StartAt"`
	States  map[string]stateDefinition `json:"States"`

	// ProcessorConfig selects a Map state's ItemProcessor processing mode
	// (see mapstate.go). It is decoded on every stateMachineDefinition,
	// since Parallel Branches reuse this same struct, but it is only
	// meaningful when this stateMachineDefinition is itself a Map state's
	// ItemProcessor.
	ProcessorConfig *processorConfig `json:"ProcessorConfig"`

	// TimeoutSeconds bounds the whole execution's wall-clock time. It is only
	// meaningful on the top-level definition; MemoryStorage.runExecution is
	// the sole reader of this field, since sub-state-machines (Parallel
	// Branches / Map ItemProcessor) reuse this same struct but must not
	// inherit an outer execution's timeout as their own.
	TimeoutSeconds *int `json:"TimeoutSeconds"`
}

// stateDefinition represents a single state in a state machine definition.
//
//nolint:tagliatelle // AWS Step Functions definition uses PascalCase
type stateDefinition struct {
	Type       string         `json:"Type"`
	Resource   string         `json:"Resource"`
	Parameters map[string]any `json:"Parameters"`
	Next       string         `json:"Next"`
	End        bool           `json:"End"`
	Result     any            `json:"Result"`
	Comment    string         `json:"Comment"`

	// ResultSelector, ResultPath, InputPath, and OutputPath are the
	// remaining fields of the standard data-flow pipeline (see iodata.go).
	// ResultPath, InputPath, and OutputPath are left as raw JSON so an
	// explicit "null" (which, per the ASL spec, discards data) can be told
	// apart from the field being absent (which defaults to "$", no
	// filtering/replacement); both would otherwise unmarshal to the same
	// value through a plain *string. Per the spec's per-state field table,
	// not every state type honors every field -- see resolveEffectiveInput,
	// runRetryCatchResultPipeline in retry.go, and validateStateFields in
	// validate.go for which types actually apply which fields.
	ResultSelector map[string]any  `json:"ResultSelector"`
	ResultPath     json.RawMessage `json:"ResultPath"`
	InputPath      json.RawMessage `json:"InputPath"`
	OutputPath     json.RawMessage `json:"OutputPath"`

	// Choice state fields.
	Choices []choiceRule `json:"Choices"`
	Default string       `json:"Default"`

	// Wait state fields. Exactly one of these is expected to be set.
	Seconds       *int   `json:"Seconds"`
	SecondsPath   string `json:"SecondsPath"`
	Timestamp     string `json:"Timestamp"`
	TimestampPath string `json:"TimestampPath"`

	// Fail state fields.
	Error string `json:"Error"`
	Cause string `json:"Cause"`

	// Retry/Catch fields, valid on Task, Parallel, and Map states.
	Retry []retrier `json:"Retry"`
	Catch []catcher `json:"Catch"`

	// Parallel state fields.
	Branches []stateMachineDefinition `json:"Branches"`

	// Map state fields. ItemProcessor is the current field name; Iterator is
	// the legacy alias for the same sub-state-machine. If both are present,
	// ItemProcessor takes precedence. See mapstate.go, itemselector.go,
	// itemreader.go, itembatcher.go, and resultwriter.go for how
	// ItemSelector/ItemReader/ItemBatcher/ResultWriter are implemented, and
	// requireDistributedModeForFields in mapstate.go for which of these
	// fields require ItemProcessor.ProcessorConfig.Mode "DISTRIBUTED".
	// MaxItemsPerBatchPath/MaxInputBytesPerBatchPath/MaxConcurrencyPath/
	// ToleratedFailurePercentagePath/ToleratedFailureCountPath (the dynamic,
	// reference-path forms of these fields) are not implemented.
	ItemsPath                  string                  `json:"ItemsPath"`
	ItemProcessor              *stateMachineDefinition `json:"ItemProcessor"`
	Iterator                   *stateMachineDefinition `json:"Iterator"`
	MaxConcurrency             int                     `json:"MaxConcurrency"`
	ItemReader                 json.RawMessage         `json:"ItemReader"`
	ItemSelector               json.RawMessage         `json:"ItemSelector"`
	ItemBatcher                json.RawMessage         `json:"ItemBatcher"`
	ResultWriter               json.RawMessage         `json:"ResultWriter"`
	ToleratedFailurePercentage *float64                `json:"ToleratedFailurePercentage"`
	ToleratedFailureCount      *int                    `json:"ToleratedFailureCount"`

	// ToleratedFailurePercentagePath/ToleratedFailureCountPath are decoded
	// only so validate.go can flag them as unimplemented; the engine never
	// reads them (see resolveMapTolerance in maptolerance.go, which only
	// reads the static ToleratedFailurePercentage/ToleratedFailureCount).
	ToleratedFailurePercentagePath string `json:"ToleratedFailurePercentagePath"`
	ToleratedFailureCountPath      string `json:"ToleratedFailureCountPath"`

	// Task state timeout fields. TimeoutSeconds/TimeoutSecondsPath bound a
	// single execution attempt of the task (see timeout.go); per the ASL
	// spec's default of 99999999 seconds (~3.17 years) when absent, kumo
	// treats "unset" as no timeout at all rather than wrapping every task in
	// a multi-year context.
	TimeoutSeconds     *int   `json:"TimeoutSeconds"`
	TimeoutSecondsPath string `json:"TimeoutSecondsPath"`

	// HeartbeatSeconds/HeartbeatSecondsPath are decoded but intentionally
	// never enforced: kumo has no activity/callback (.waitForTaskToken) task
	// token support, so a task can never send a heartbeat. Enforcing this
	// field would fail every task that sets it, for a reason kumo can never
	// satisfy.
	HeartbeatSeconds     *int   `json:"HeartbeatSeconds"`
	HeartbeatSecondsPath string `json:"HeartbeatSecondsPath"`
}

// Execution error codes surfaced via DescribeExecution. States.TaskFailed is
// reserved for failures of a Task state's work itself; States.NoChoiceMatched
// is reserved for a Choice state with no matching rule and no Default;
// States.Timeout is reported when a Task or the whole execution exceeds its
// TimeoutSeconds; States.ALL is the Retry/Catch wildcard that matches any
// other Error Name; every other engine-level failure (unsupported state, bad
// definition wiring) is States.Runtime.
const (
	errorStatesRuntime         = "States.Runtime"
	errorStatesTaskFailed      = "States.TaskFailed"
	errorStatesNoChoiceMatched = "States.NoChoiceMatched"
	errorStatesTimeout         = "States.Timeout"
	errorStatesAll             = "States.ALL"
	errorStatesBranchFailed    = "States.BranchFailed"

	// errorStatesExceedToleratedFailureThreshold is reported when a
	// distributed-mode Map state's ToleratedFailurePercentage/
	// ToleratedFailureCount is exceeded (see mapstate.go).
	errorStatesExceedToleratedFailureThreshold = "States.ExceedToleratedFailureThreshold"
)

// taskFailedError marks a failure of the task invocation itself so the
// execution reports States.TaskFailed instead of States.Runtime.
type taskFailedError struct {
	err error
}

func (e *taskFailedError) Error() string { return e.err.Error() }

func (e *taskFailedError) Unwrap() error { return e.err }

// noChoiceMatchedError marks a Choice state that failed to match any Choice
// Rule and had no Default. Per the Amazon States Language spec, the
// interpreter throws a runtime States.NoChoiceMatched error in this case.
type noChoiceMatchedError struct {
	state string
}

func (e *noChoiceMatchedError) Error() string {
	return fmt.Sprintf("state %q: no Choice Rule matched and no Default was specified", e.state)
}

// failStateError carries the Error and Cause declared on a Fail state so the
// execution reports those exact values instead of a generic engine failure.
type failStateError struct {
	errorName string
	cause     string
}

func (e *failStateError) Error() string {
	if e.cause != "" {
		return fmt.Sprintf("%s: %s", e.errorName, e.cause)
	}

	return e.errorName
}

// executionErrorCode maps an engine error to the Step Functions error code
// reported on the failed execution.
func executionErrorCode(err error) string {
	var taskErr *taskFailedError
	if errors.As(err, &taskErr) {
		return errorStatesTaskFailed
	}

	var choiceErr *noChoiceMatchedError
	if errors.As(err, &choiceErr) {
		return errorStatesNoChoiceMatched
	}

	var timeoutErr *taskTimeoutError
	if errors.As(err, &timeoutErr) {
		return errorStatesTimeout
	}

	var failErr *failStateError
	if errors.As(err, &failErr) {
		return failErr.errorName
	}

	return errorStatesRuntime
}

// executionErrorCause returns the cause message reported on the failed
// execution. A Fail state reports its own declared Cause verbatim instead of
// the wrapped "execute state ...: ..." error chain every other failure uses.
func executionErrorCause(err error) string {
	var failErr *failStateError
	if errors.As(err, &failErr) {
		return failErr.cause
	}

	return err.Error()
}

// executionEngine executes a state machine definition.
type executionEngine struct {
	baseURL string
	client  *http.Client
}

// newExecutionEngine creates a new execution engine.
func newExecutionEngine(baseURL string) *executionEngine {
	return &executionEngine{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// parseDefinition parses a state machine definition JSON string.
func parseDefinition(definition string) (*stateMachineDefinition, error) {
	var def stateMachineDefinition
	if err := json.Unmarshal([]byte(definition), &def); err != nil {
		return nil, fmt.Errorf("parse definition: %w", err)
	}

	if def.StartAt == "" {
		return nil, fmt.Errorf("parse definition: StartAt is required")
	}

	if len(def.States) == 0 {
		return nil, fmt.Errorf("parse definition: States is required")
	}

	return &def, nil
}

// execute runs the state machine and returns the output JSON string.
func (e *executionEngine) execute(ctx context.Context, def *stateMachineDefinition, input string) (string, error) {
	currentState := def.StartAt
	currentInput := input

	for {
		state, ok := def.States[currentState]
		if !ok {
			return "", fmt.Errorf("state %q not found in definition", currentState)
		}

		output, nextOverride, err := e.executeState(ctx, currentState, &state, currentInput)
		if err != nil {
			return "", fmt.Errorf("execute state %q: %w", currentState, err)
		}

		// Succeed states are always terminal, regardless of End/Next.
		if state.Type == "Succeed" {
			return output, nil
		}

		// nextOverride is set by states that pick their own successor
		// dynamically: Choice, and any Task/Parallel/Map whose Catch routed
		// to a fallback state. It always takes priority over End/Next, since
		// a Catch must be able to redirect even a state marked End: true.
		if nextOverride != "" {
			currentInput = output
			currentState = nextOverride

			continue
		}

		if state.End {
			return output, nil
		}

		if state.Next == "" {
			return "", fmt.Errorf("state %q has no End or Next", currentState)
		}

		currentInput = output
		currentState = state.Next
	}
}

// executeState executes a single state and returns its output and, for
// states that determine the next state dynamically (Choice), the name of
// that next state. nextOverride is empty for states that use the state's own
// Next/End fields.
func (e *executionEngine) executeState(ctx context.Context, name string, state *stateDefinition, input string) (string, string, error) {
	switch state.Type {
	case "Pass":
		output, err := e.executePassState(name, state, input)

		return output, "", err
	case "Task":
		return e.executeTaskStateWithPolicy(ctx, name, state, input)
	case "Choice":
		return e.executeChoiceState(name, state, input)
	case "Wait":
		output, err := e.executeWaitState(ctx, state, input)

		return output, "", err
	case "Succeed":
		output, err := e.executeSucceedState(state, input)

		return output, "", err
	case "Fail":
		return "", "", e.executeFailState(state)
	case "Parallel":
		return e.executeParallelStateWithPolicy(ctx, name, state, input)
	case "Map":
		return e.executeMapStateWithPolicy(ctx, name, state, input)
	default:
		return "", "", fmt.Errorf("unsupported state type %q", state.Type)
	}
}

// executeTaskStateWithPolicy executes a Task state's full standard field
// pipeline: InputPath narrows the raw input to the effective input, Retry
// and Catch govern the work itself (each attempt individually bounded by
// TimeoutSeconds/TimeoutSecondsPath; see executeTaskStateWithTimeout in
// timeout.go), and on success ResultSelector/ResultPath/OutputPath shape
// the effective output. Parameters is resolved inside executeTaskState,
// against the effective input, since its resolved value doubles as the
// resource invocation's request payload rather than a generic JSON blob.
func (e *executionEngine) executeTaskStateWithPolicy(ctx context.Context, name string, state *stateDefinition, input string) (string, string, error) {
	effectiveInput, err := applyInputPath(state.InputPath, input)
	if err != nil {
		return "", "", fmt.Errorf("state %q: %w", name, err)
	}

	return e.runRetryCatchResultPipeline(ctx, name, state, input, effectiveInput, func(ctx context.Context) (string, error) {
		return e.executeTaskStateWithTimeout(ctx, name, state, effectiveInput)
	})
}

// executePassState executes a Pass state's full standard field pipeline:
// InputPath and Parameters build the effective input, Result (if present)
// -- else the effective input itself -- is the state's result, and
// ResultPath/OutputPath shape the effective output. Pass has no
// ResultSelector; see the spec's per-state field table.
func (e *executionEngine) executePassState(name string, state *stateDefinition, input string) (string, error) {
	effectiveInput, err := resolveEffectiveInput(state.InputPath, state.Parameters, input)
	if err != nil {
		return "", fmt.Errorf("pass state %q: %w", name, err)
	}

	result := effectiveInput

	if state.Result != nil {
		marshaled, err := json.Marshal(state.Result)
		if err != nil {
			return "", fmt.Errorf("pass state %q: marshal Result: %w", name, err)
		}

		result = string(marshaled)
	}

	merged, err := applyResultPath(state.ResultPath, effectiveInput, result)
	if err != nil {
		return "", fmt.Errorf("pass state %q: %w", name, err)
	}

	output, err := applyOutputPath(state.OutputPath, merged)
	if err != nil {
		return "", fmt.Errorf("pass state %q: %w", name, err)
	}

	return output, nil
}

// executeChoiceState executes a Choice state: InputPath narrows the raw
// input to the effective input, which the Choice Rules -- including the
// "*Path" comparators (see choice.go) -- are evaluated against; the first
// matching rule's Next is chosen, falling back to Default. Choice has no
// Parameters/ResultSelector/ResultPath (it produces no result of its own),
// so the state output is simply the effective input, narrowed by
// OutputPath.
func (e *executionEngine) executeChoiceState(name string, state *stateDefinition, input string) (string, string, error) {
	effectiveInput, err := applyInputPath(state.InputPath, input)
	if err != nil {
		return "", "", fmt.Errorf("choice state %q: %w", name, err)
	}

	var inputData map[string]any
	if err := json.Unmarshal([]byte(effectiveInput), &inputData); err != nil {
		return "", "", fmt.Errorf("choice state %q: parse input: %w", name, err)
	}

	next, err := chooseNextState(name, state, inputData)
	if err != nil {
		return "", "", err
	}

	output, err := applyOutputPath(state.OutputPath, effectiveInput)
	if err != nil {
		return "", "", fmt.Errorf("choice state %q: %w", name, err)
	}

	return output, next, nil
}

// chooseNextState scans a Choice state's Choices in order for the first
// matching rule's Next, falling back to Default, or reporting
// noChoiceMatchedError if neither exists.
func chooseNextState(name string, state *stateDefinition, inputData map[string]any) (string, error) {
	for i := range state.Choices {
		matched, err := evaluateChoiceRule(&state.Choices[i], inputData)
		if err != nil {
			return "", fmt.Errorf("choice state %q: %w", name, err)
		}

		if matched {
			if state.Choices[i].Next == "" {
				return "", fmt.Errorf("choice state %q: matched rule has no Next", name)
			}

			return state.Choices[i].Next, nil
		}
	}

	if state.Default == "" {
		return "", &noChoiceMatchedError{state: name}
	}

	return state.Default, nil
}

// executeSucceedState executes a Succeed state, narrowing the output through
// InputPath/OutputPath if present. Succeed has neither Parameters, Result,
// nor ResultPath: its output is always its (possibly filtered) input.
func (e *executionEngine) executeSucceedState(state *stateDefinition, input string) (string, error) {
	narrowed, err := applyInputPath(state.InputPath, input)
	if err != nil {
		return "", fmt.Errorf("succeed state: %w", err)
	}

	output, err := applyOutputPath(state.OutputPath, narrowed)
	if err != nil {
		return "", fmt.Errorf("succeed state: %w", err)
	}

	return output, nil
}

// executeFailState executes a Fail state, always returning a failStateError
// carrying the state's declared Error and Cause. Fail has neither
// InputPath, OutputPath, nor any other data-flow field.
func (e *executionEngine) executeFailState(state *stateDefinition) error {
	return &failStateError{errorName: state.Error, cause: state.Cause}
}

// executeTaskState executes a Task state by calling the appropriate service.
func (e *executionEngine) executeTaskState(ctx context.Context, name string, state *stateDefinition, input string) (string, error) {
	resource := state.Resource
	if resource == "" {
		return "", fmt.Errorf("task state %q has no Resource", name)
	}

	// Resolve parameters with JSONPath references from input.
	params, err := resolveParameters(state.Parameters, input)
	if err != nil {
		return "", fmt.Errorf("resolve parameters for state %q: %w", name, err)
	}

	switch {
	case resource == "arn:aws:states:::sqs:sendMessage":
		return wrapTaskResult(e.executeSQSSendMessage(ctx, params))
	case resource == "arn:aws:states:::lambda:invoke":
		return wrapTaskResult(e.executeLambdaInvoke(ctx, params))
	case strings.HasPrefix(resource, "arn:aws:lambda:"):
		return wrapTaskResult(e.executeLambdaFunctionTask(ctx, name, resource, params, input))
	default:
		return "", fmt.Errorf("unsupported task resource %q", resource)
	}
}

// wrapTaskResult wraps a task invocation failure in taskFailedError so it is
// reported as States.TaskFailed.
func wrapTaskResult(output string, err error) (string, error) {
	if err != nil {
		return "", &taskFailedError{err: err}
	}

	return output, nil
}

// resolveParameters resolves parameter values, handling JSONPath references
// (keys ending with ".$" whose values are JSONPath expressions like "$.field").
func resolveParameters(params map[string]any, input string) (map[string]any, error) {
	return resolveParametersWithContext(params, input, nil)
}

// resolveParametersWithContext extends resolveParameters with an optional
// Context object ($$.) lookup: contextData is nil everywhere except
// ItemSelector (see itemselector.go), which is the only place the Amazon
// States Language allows "$$." references.
func resolveParametersWithContext(params map[string]any, input string, contextData map[string]any) (map[string]any, error) {
	if params == nil {
		return map[string]any{}, nil
	}

	// inputData is parsed lazily on the first JSONPath reference and reused.
	// It is typed any, not map[string]any, since a Payload Template's input
	// is not always a JSON object -- ResultSelector, in particular, resolves
	// against a Parallel/Map state's result, which is a JSON array.
	var inputData any

	resolved := make(map[string]any, len(params))

	for key, value := range params {
		// A ".$" suffix marks a JSONPath reference; otherwise it is a static value.
		if strings.HasSuffix(key, ".$") {
			resolvedValue, parsed, err := resolveJSONPathRef(key, value, input, inputData, contextData)
			if err != nil {
				return nil, err
			}

			inputData = parsed
			resolved[strings.TrimSuffix(key, ".$")] = resolvedValue

			continue
		}

		resolvedValue, err := resolveStaticValue(value, input, contextData)
		if err != nil {
			return nil, err
		}

		resolved[key] = resolvedValue
	}

	return resolved, nil
}

// resolveJSONPathRef resolves a "key.$" JSONPath reference against the
// input, or, for a "$$."-prefixed path, against contextData. inputData is
// the lazily-parsed input (nil until first use); the possibly newly parsed
// value is returned so the caller can reuse it for later references,
// keeping the input JSON unmarshaled at most once per resolveParameters
// call. inputData -- and therefore the path resolution against it -- uses
// resolveAnyJSONPath rather than resolveJSONPath, since the input is not
// always a JSON object (see resolveParametersWithContext).
func resolveJSONPathRef(key string, value any, input string, inputData any, contextData map[string]any) (any, any, error) {
	pathStr, ok := value.(string)
	if !ok {
		return nil, inputData, fmt.Errorf("jsonPath reference for key %q must be a string", key)
	}

	if strings.HasPrefix(pathStr, "$$.") {
		if contextData == nil {
			return nil, inputData, fmt.Errorf("jsonPath reference for key %q uses the Context object (%q), which is only available in ItemSelector", key, pathStr)
		}

		resolvedValue, err := resolveJSONPath(contextData, "$"+strings.TrimPrefix(pathStr, "$$"))
		if err != nil {
			return nil, inputData, fmt.Errorf("resolve Context object path %q for key %q: %w", pathStr, key, err)
		}

		return resolvedValue, inputData, nil
	}

	if inputData == nil {
		if err := json.Unmarshal([]byte(input), &inputData); err != nil {
			return nil, inputData, fmt.Errorf("parse input for JSONPath: %w", err)
		}
	}

	resolvedValue, err := resolveAnyJSONPath(pathStr, inputData)
	if err != nil {
		return nil, inputData, fmt.Errorf("resolve JSONPath %q for key %q: %w", pathStr, key, err)
	}

	return resolvedValue, inputData, nil
}

// resolveStaticValue returns a non-reference parameter value, recursing into
// nested maps and leaving scalars unchanged.
func resolveStaticValue(value any, input string, contextData map[string]any) (any, error) {
	subMap, ok := value.(map[string]any)
	if !ok {
		return value, nil
	}

	return resolveParametersWithContext(subMap, input, contextData)
}

// resolveJSONPath resolves a simple JSONPath expression ("$" or "$.field") against the input data.
// Only single-level field access is supported (e.g., "$.message"), plus "$" for the whole input.
func resolveJSONPath(data map[string]any, path string) (any, error) {
	if path == "$" {
		return data, nil
	}

	if !strings.HasPrefix(path, "$.") {
		return nil, fmt.Errorf("unsupported JSONPath %q: must start with $", path)
	}

	field := strings.TrimPrefix(path, "$.")

	// Support nested field access like "$.a.b.c".
	parts := strings.Split(field, ".")

	var current any = data

	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("jsonPath %q: cannot access field %q on non-object", path, part)
		}

		val, exists := m[part]
		if !exists {
			return nil, fmt.Errorf("jsonPath %q: field %q not found", path, part)
		}

		current = val
	}

	return current, nil
}

// executeSQSSendMessage sends a message to SQS via HTTP.
func (e *executionEngine) executeSQSSendMessage(ctx context.Context, params map[string]any) (string, error) {
	queueURL, _ := params["QueueUrl"].(string)
	if queueURL == "" {
		return "", fmt.Errorf("sqs sendMessage: QueueUrl is required")
	}

	messageBody, err := formatMessageBody(params["MessageBody"])
	if err != nil {
		return "", fmt.Errorf("sqs sendMessage: %w", err)
	}

	// Build SQS SendMessage request payload (JSON protocol).
	sqsReq := map[string]any{
		"QueueUrl":    queueURL,
		"MessageBody": messageBody,
	}

	body, err := json.Marshal(sqsReq)
	if err != nil {
		return "", fmt.Errorf("sqs sendMessage: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("sqs sendMessage: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessage")

	resp, err := e.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("sqs sendMessage: send request: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("sqs sendMessage: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("sqs sendMessage: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	slog.Debug("SFN executor: SQS sendMessage succeeded", "queueUrl", queueURL)

	return string(respBody), nil
}

// formatMessageBody converts a message body value to a string suitable for SQS.
func formatMessageBody(v any) (string, error) {
	if v == nil {
		return "", fmt.Errorf("messageBody is required")
	}

	switch val := v.(type) {
	case string:
		return val, nil
	default:
		encoded, err := json.Marshal(val)
		if err != nil {
			return "", fmt.Errorf("marshal MessageBody: %w", err)
		}

		return string(encoded), nil
	}
}

// executeLambdaInvoke invokes a Lambda function via HTTP.
func (e *executionEngine) executeLambdaInvoke(ctx context.Context, params map[string]any) (string, error) {
	functionName, _ := params["FunctionName"].(string)
	if functionName == "" {
		return "", fmt.Errorf("lambda invoke: FunctionName is required")
	}

	// Extract just the function name from ARN if provided.
	functionName = extractLambdaFunctionName(functionName)

	var payload []byte

	if p, ok := params["Payload"]; ok {
		var err error

		payload, err = json.Marshal(p)
		if err != nil {
			return "", fmt.Errorf("lambda invoke: marshal payload: %w", err)
		}
	} else {
		payload = []byte("{}")
	}

	respBody, err := e.callLambda(ctx, functionName, payload)
	if err != nil {
		return "", err
	}

	slog.Debug("SFN executor: Lambda invoke succeeded", "function", functionName)

	// Wrap Lambda response in the Step Functions format.
	lambdaResult := map[string]any{
		"StatusCode": http.StatusOK,
	}

	// Try to parse the response body as JSON for the Payload field.
	var payloadValue any
	if err := json.Unmarshal(respBody, &payloadValue); err == nil {
		lambdaResult["Payload"] = payloadValue
	} else {
		lambdaResult["Payload"] = string(respBody)
	}

	result, err := json.Marshal(lambdaResult)
	if err != nil {
		return "", fmt.Errorf("lambda invoke: marshal result: %w", err)
	}

	return string(result), nil
}

// executeLambdaFunctionTask invokes a Lambda function specified directly by
// ARN in a Task state's Resource field (the "directly specified function
// resource" integration, as opposed to the optimized
// arn:aws:states:::lambda:invoke integration). With this integration the
// state input becomes the invocation payload verbatim, or resolved
// Parameters if given, and the task output is the function's response
// payload directly with no ExecutedVersion/Payload/StatusCode wrapping.
func (e *executionEngine) executeLambdaFunctionTask(ctx context.Context, name, resourceARN string, params map[string]any, input string) (string, error) {
	functionName := extractLambdaFunctionName(resourceARN)

	payload := []byte(input)

	if len(params) > 0 {
		var err error

		payload, err = json.Marshal(params)
		if err != nil {
			return "", fmt.Errorf("marshal parameters for state %q: %w", name, err)
		}
	}

	respBody, err := e.callLambda(ctx, functionName, payload)
	if err != nil {
		return "", err
	}

	slog.Debug("SFN executor: Lambda function invoke succeeded", "function", functionName)

	return string(respBody), nil
}

// callLambda performs the HTTP call to invoke a Lambda function and returns
// the raw response body.
func (e *executionEngine) callLambda(ctx context.Context, functionName string, payload []byte) ([]byte, error) {
	invokeURL := fmt.Sprintf("%s/lambda/2015-03-31/functions/%s/invocations", e.baseURL, functionName)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, invokeURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("lambda invoke: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lambda invoke: send request: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("lambda invoke: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lambda invoke: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// extractLambdaFunctionName extracts the function name from an ARN or returns the input as-is.
func extractLambdaFunctionName(nameOrARN string) string {
	if !strings.HasPrefix(nameOrARN, "arn:") {
		return nameOrARN
	}

	// arn:aws:lambda:<region>:<account>:function:<name>
	parts := strings.Split(nameOrARN, ":")
	if len(parts) >= 7 {
		return parts[6]
	}

	return nameOrARN
}
