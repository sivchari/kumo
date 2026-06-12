package sfn

import (
	"bytes"
	"context"
	"encoding/json"
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
	ResultPath *string        `json:"ResultPath"`
	InputPath  *string        `json:"InputPath"`
	OutputPath *string        `json:"OutputPath"`
	Comment    string         `json:"Comment"`
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

		output, err := e.executeState(ctx, currentState, &state, currentInput)
		if err != nil {
			return "", fmt.Errorf("execute state %q: %w", currentState, err)
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

// executeState executes a single state and returns the output.
func (e *executionEngine) executeState(ctx context.Context, name string, state *stateDefinition, input string) (string, error) {
	switch state.Type {
	case "Pass":
		return e.executePassState(state, input)
	case "Task":
		return e.executeTaskState(ctx, name, state, input)
	default:
		return "", fmt.Errorf("unsupported state type %q", state.Type)
	}
}

// executePassState executes a Pass state.
func (e *executionEngine) executePassState(state *stateDefinition, input string) (string, error) {
	if state.Result != nil {
		result, err := json.Marshal(state.Result)
		if err != nil {
			return "", fmt.Errorf("marshal pass result: %w", err)
		}

		return string(result), nil
	}

	return input, nil
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

	switch resource {
	case "arn:aws:states:::sqs:sendMessage":
		return e.executeSQSSendMessage(ctx, params)
	case "arn:aws:states:::lambda:invoke":
		return e.executeLambdaInvoke(ctx, params)
	default:
		return "", fmt.Errorf("unsupported task resource %q", resource)
	}
}

// resolveParameters resolves parameter values, handling JSONPath references
// (keys ending with ".$" whose values are JSONPath expressions like "$.field").
func resolveParameters(params map[string]any, input string) (map[string]any, error) {
	if params == nil {
		return map[string]any{}, nil
	}

	// inputData is parsed lazily on the first JSONPath reference and reused.
	var inputData map[string]any

	resolved := make(map[string]any, len(params))

	for key, value := range params {
		// A ".$" suffix marks a JSONPath reference; otherwise it is a static value.
		if strings.HasSuffix(key, ".$") {
			resolvedValue, parsed, err := resolveJSONPathRef(key, value, input, inputData)
			if err != nil {
				return nil, err
			}

			inputData = parsed
			resolved[strings.TrimSuffix(key, ".$")] = resolvedValue

			continue
		}

		resolvedValue, err := resolveStaticValue(value, input)
		if err != nil {
			return nil, err
		}

		resolved[key] = resolvedValue
	}

	return resolved, nil
}

// resolveJSONPathRef resolves a "key.$" JSONPath reference against the input.
// inputData is the lazily-parsed input (nil until first use); the possibly newly
// parsed map is returned so the caller can reuse it for later references,
// keeping the input JSON unmarshaled at most once per resolveParameters call.
func resolveJSONPathRef(key string, value any, input string, inputData map[string]any) (any, map[string]any, error) {
	pathStr, ok := value.(string)
	if !ok {
		return nil, inputData, fmt.Errorf("jsonPath reference for key %q must be a string", key)
	}

	if inputData == nil {
		if err := json.Unmarshal([]byte(input), &inputData); err != nil {
			return nil, inputData, fmt.Errorf("parse input for JSONPath: %w", err)
		}
	}

	resolvedValue, err := resolveJSONPath(inputData, pathStr)
	if err != nil {
		return nil, inputData, fmt.Errorf("resolve JSONPath %q for key %q: %w", pathStr, key, err)
	}

	return resolvedValue, inputData, nil
}

// resolveStaticValue returns a non-reference parameter value, recursing into
// nested maps and leaving scalars unchanged.
func resolveStaticValue(value any, input string) (any, error) {
	subMap, ok := value.(map[string]any)
	if !ok {
		return value, nil
	}

	return resolveParameters(subMap, input)
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
//
//nolint:funlen // Straightforward HTTP call with error handling.
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

	invokeURL := fmt.Sprintf("%s/lambda/2015-03-31/functions/%s/invocations", e.baseURL, functionName)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, invokeURL, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("lambda invoke: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("lambda invoke: send request: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("lambda invoke: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("lambda invoke: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	slog.Debug("SFN executor: Lambda invoke succeeded", "function", functionName)

	// Wrap Lambda response in the Step Functions format.
	lambdaResult := map[string]any{
		"StatusCode": resp.StatusCode,
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
