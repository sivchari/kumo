package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdaTypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/spf13/cobra"
)

// newLambdaCmd builds the Lambda command tree. Lambda uses kumo's REST
// protocol (see internal/service/lambda), which is outside cli-gen's
// auto-discovery, so these commands are hand-written like the other
// pre-generator files (s3api.go, backup.go).
//
// create-function and update-function-configuration bypass the AWS SDK and
// send raw HTTP requests: kumo functions execute through an InvokeEndpoint
// (a kumo-only extension field the SDK's generated request marshaler has no
// knowledge of, see internal/service/lambda/types.go and
// test/integration/lambda_test.go's TestLambda_InvokeWithEndpoint), so it is
// the only way to set it from the CLI.
func newLambdaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lambda",
		Short: "Lambda commands",
	}

	cmd.AddCommand(
		newLambdaCreateFunctionCmd(),
		newLambdaGetFunctionCmd(),
		newLambdaListFunctionsCmd(),
		newLambdaDeleteFunctionCmd(),
		newLambdaInvokeCmd(),
		newLambdaUpdateFunctionCodeCmd(),
		newLambdaUpdateFunctionConfigurationCmd(),
	)

	return cmd
}

// lambdaRawRequest sends a JSON request directly to the kumo server, bypassing
// the AWS SDK, and decodes a JSON response. Used by commands that need to set
// kumo's InvokeEndpoint extension field, which the SDK's request marshaler
// does not know about.
func lambdaRawRequest(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader

	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to encode request: %w", err)
		}

		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpointURL+path, reader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// lambdaCreateFunctionParams holds the flag values for create-function.
type lambdaCreateFunctionParams struct {
	functionName, runtime, role, handler string
	codeJSON, description, packageType   string
	environmentJSON, tagsJSON            string
	invokeEndpoint                       string
	timeout, memorySize                  int32
}

// buildLambdaCreateFunctionBody assembles the CreateFunction request body.
// It is built as a map, rather than a tagged struct, because the wire format
// mirrors internal/service/lambda's PascalCase JSON API (FunctionName, Role,
// InvokeEndpoint, ...) rather than idiomatic Go JSON field naming.
func buildLambdaCreateFunctionBody(p *lambdaCreateFunctionParams) (map[string]any, error) {
	var code lambdaTypes.FunctionCode
	if p.codeJSON != "" {
		if err := json.Unmarshal([]byte(p.codeJSON), &code); err != nil {
			return nil, fmt.Errorf("invalid --code JSON: %w", err)
		}
	}

	body := map[string]any{
		"FunctionName": p.functionName,
		"Role":         p.role,
		"Code":         code,
	}

	if p.runtime != "" {
		body["Runtime"] = p.runtime
	}

	if p.handler != "" {
		body["Handler"] = p.handler
	}

	if p.description != "" {
		body["Description"] = p.description
	}

	if p.timeout > 0 {
		body["Timeout"] = p.timeout
	}

	if p.memorySize > 0 {
		body["MemorySize"] = p.memorySize
	}

	if p.packageType != "" {
		body["PackageType"] = p.packageType
	}

	if p.invokeEndpoint != "" {
		body["InvokeEndpoint"] = p.invokeEndpoint
	}

	if p.environmentJSON != "" {
		var environment lambdaTypes.Environment
		if err := json.Unmarshal([]byte(p.environmentJSON), &environment); err != nil {
			return nil, fmt.Errorf("invalid --environment JSON: %w", err)
		}

		body["Environment"] = environment
	}

	if p.tagsJSON != "" {
		var tags map[string]string
		if err := json.Unmarshal([]byte(p.tagsJSON), &tags); err != nil {
			return nil, fmt.Errorf("invalid --tags JSON: %w", err)
		}

		body["Tags"] = tags
	}

	return body, nil
}

func newLambdaCreateFunctionCmd() *cobra.Command {
	var p lambdaCreateFunctionParams

	cmd := &cobra.Command{
		Use:   "create-function",
		Short: "Create a Lambda function",
		RunE: func(cmd *cobra.Command, _ []string) error {
			body, err := buildLambdaCreateFunctionBody(&p)
			if err != nil {
				return err
			}

			var out map[string]any
			if err := lambdaRawRequest(cmd.Context(), http.MethodPost, "/2015-03-31/functions", body, &out); err != nil {
				return fmt.Errorf("create-function failed: %w", err)
			}

			if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
				return fmt.Errorf("failed to encode output: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&p.functionName, "function-name", "", "Function name")
	cmd.Flags().StringVar(&p.runtime, "runtime", "", "Runtime identifier (e.g. python3.12, nodejs20.x)")
	cmd.Flags().StringVar(&p.role, "role", "", "Execution role ARN")
	cmd.Flags().StringVar(&p.handler, "handler", "", "Function entry point")
	cmd.Flags().StringVar(&p.codeJSON, "code", "", "Function code (JSON, e.g. {\"ZipFile\":\"<base64>\"})")
	cmd.Flags().StringVar(&p.description, "description", "", "Function description")
	cmd.Flags().Int32Var(&p.timeout, "timeout", 0, "Function timeout in seconds")
	cmd.Flags().Int32Var(&p.memorySize, "memory-size", 0, "Function memory size in MB")
	cmd.Flags().StringVar(&p.packageType, "package-type", "", "Package type (Zip or Image)")
	cmd.Flags().StringVar(&p.environmentJSON, "environment", "", "Environment variables (JSON, e.g. {\"Variables\":{\"KEY\":\"VALUE\"}})")
	cmd.Flags().StringVar(&p.tagsJSON, "tags", "", "Tags (JSON, e.g. {\"KEY\":\"VALUE\"})")
	cmd.Flags().StringVar(&p.invokeEndpoint, "invoke-endpoint", "", "kumo extension: HTTP endpoint kumo proxies invocations to")

	return cmd
}

func newLambdaGetFunctionCmd() *cobra.Command {
	var functionName string

	cmd := &cobra.Command{
		Use:   "get-function",
		Short: "Get a Lambda function",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			client := lambda.NewFromConfig(cfg, func(o *lambda.Options) {
				o.BaseEndpoint = aws.String(endpointURL)
			})

			out, err := client.GetFunction(cmd.Context(), &lambda.GetFunctionInput{
				FunctionName: aws.String(functionName),
			})
			if err != nil {
				return fmt.Errorf("get-function failed: %w", err)
			}

			if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
				return fmt.Errorf("failed to encode output: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&functionName, "function-name", "", "Function name")

	return cmd
}

func newLambdaListFunctionsCmd() *cobra.Command {
	var marker string

	var maxItems int32

	cmd := &cobra.Command{
		Use:   "list-functions",
		Short: "List Lambda functions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			client := lambda.NewFromConfig(cfg, func(o *lambda.Options) {
				o.BaseEndpoint = aws.String(endpointURL)
			})

			input := &lambda.ListFunctionsInput{}
			if marker != "" {
				input.Marker = aws.String(marker)
			}

			if maxItems > 0 {
				input.MaxItems = aws.Int32(maxItems)
			}

			out, err := client.ListFunctions(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("list-functions failed: %w", err)
			}

			if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
				return fmt.Errorf("failed to encode output: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&marker, "marker", "", "Pagination marker")
	cmd.Flags().Int32Var(&maxItems, "max-items", 0, "Maximum number of functions to return")

	return cmd
}

func newLambdaDeleteFunctionCmd() *cobra.Command {
	var functionName string

	cmd := &cobra.Command{
		Use:   "delete-function",
		Short: "Delete a Lambda function",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			client := lambda.NewFromConfig(cfg, func(o *lambda.Options) {
				o.BaseEndpoint = aws.String(endpointURL)
			})

			_, err = client.DeleteFunction(cmd.Context(), &lambda.DeleteFunctionInput{
				FunctionName: aws.String(functionName),
			})
			if err != nil {
				return fmt.Errorf("delete-function failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&functionName, "function-name", "", "Function name")

	return cmd
}

// lambdaInvokeParams holds the flag values for invoke.
type lambdaInvokeParams struct {
	functionName, payload                  string
	invocationType, logType, clientContext string
	qualifier                              string
}

// buildLambdaInvokeInput translates flag values into an InvokeInput.
func buildLambdaInvokeInput(p *lambdaInvokeParams) *lambda.InvokeInput {
	input := &lambda.InvokeInput{
		FunctionName: aws.String(p.functionName),
		Payload:      []byte(p.payload),
	}

	if p.invocationType != "" {
		input.InvocationType = lambdaTypes.InvocationType(p.invocationType)
	}

	if p.logType != "" {
		input.LogType = lambdaTypes.LogType(p.logType)
	}

	if p.clientContext != "" {
		input.ClientContext = aws.String(p.clientContext)
	}

	if p.qualifier != "" {
		input.Qualifier = aws.String(p.qualifier)
	}

	return input
}

// writeLambdaInvokePayload writes the invocation payload to outfile, matching
// `aws lambda invoke`'s positional outfile argument. As a kumo-only
// convenience for the local-dev loop, "-" writes to stdout instead of a file
// (the real AWS CLI has no such shorthand).
func writeLambdaInvokePayload(outfile string, payload []byte) error {
	if outfile == "-" {
		if _, err := os.Stdout.Write(payload); err != nil {
			return fmt.Errorf("failed to write payload to stdout: %w", err)
		}

		return nil
	}

	if err := os.WriteFile(outfile, payload, 0o600); err != nil {
		return fmt.Errorf("failed to write payload to %s: %w", outfile, err)
	}

	return nil
}

// lambdaInvokeMetadata builds the invocation metadata `aws lambda invoke`
// prints to stdout (everything but the payload, which goes to outfile).
func lambdaInvokeMetadata(out *lambda.InvokeOutput) map[string]any {
	metadata := map[string]any{"StatusCode": out.StatusCode}
	if out.FunctionError != nil {
		metadata["FunctionError"] = *out.FunctionError
	}

	if out.LogResult != nil {
		metadata["LogResult"] = *out.LogResult
	}

	if out.ExecutedVersion != nil {
		metadata["ExecutedVersion"] = *out.ExecutedVersion
	}

	return metadata
}

func newLambdaInvokeCmd() *cobra.Command {
	var p lambdaInvokeParams

	cmd := &cobra.Command{
		Use:   "invoke <outfile>",
		Short: "Invoke a Lambda function",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			client := lambda.NewFromConfig(cfg, func(o *lambda.Options) {
				o.BaseEndpoint = aws.String(endpointURL)
			})

			out, err := client.Invoke(cmd.Context(), buildLambdaInvokeInput(&p))
			if err != nil {
				return fmt.Errorf("invoke failed: %w", err)
			}

			if err := writeLambdaInvokePayload(args[0], out.Payload); err != nil {
				return err
			}

			if err := json.NewEncoder(os.Stdout).Encode(lambdaInvokeMetadata(out)); err != nil {
				return fmt.Errorf("failed to encode output: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&p.functionName, "function-name", "", "Function name")
	cmd.Flags().StringVar(&p.payload, "payload", "", "JSON payload to send to the function")
	cmd.Flags().StringVar(&p.invocationType, "invocation-type", "", "Invocation type (Event, RequestResponse, or DryRun)")
	cmd.Flags().StringVar(&p.logType, "log-type", "", "Set to Tail to include the execution log in the response")
	cmd.Flags().StringVar(&p.clientContext, "client-context", "", "Base64-encoded client context data")
	cmd.Flags().StringVar(&p.qualifier, "qualifier", "", "Version or alias to invoke")

	return cmd
}

func newLambdaUpdateFunctionCodeCmd() *cobra.Command {
	var functionName, s3Bucket, s3Key, s3ObjectVersion, imageURI string

	var publish bool

	cmd := &cobra.Command{
		Use:   "update-function-code",
		Short: "Update a Lambda function's code",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			client := lambda.NewFromConfig(cfg, func(o *lambda.Options) {
				o.BaseEndpoint = aws.String(endpointURL)
			})

			input := &lambda.UpdateFunctionCodeInput{
				FunctionName: aws.String(functionName),
				Publish:      publish,
			}

			if s3Bucket != "" {
				input.S3Bucket = aws.String(s3Bucket)
			}

			if s3Key != "" {
				input.S3Key = aws.String(s3Key)
			}

			if s3ObjectVersion != "" {
				input.S3ObjectVersion = aws.String(s3ObjectVersion)
			}

			if imageURI != "" {
				input.ImageUri = aws.String(imageURI)
			}

			out, err := client.UpdateFunctionCode(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("update-function-code failed: %w", err)
			}

			if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
				return fmt.Errorf("failed to encode output: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&functionName, "function-name", "", "Function name")
	cmd.Flags().StringVar(&s3Bucket, "s3-bucket", "", "S3 bucket containing the deployment package")
	cmd.Flags().StringVar(&s3Key, "s3-key", "", "S3 key of the deployment package")
	cmd.Flags().StringVar(&s3ObjectVersion, "s3-object-version", "", "S3 object version of the deployment package")
	cmd.Flags().StringVar(&imageURI, "image-uri", "", "Container image URI")
	cmd.Flags().BoolVar(&publish, "publish", false, "Publish a new version after updating the code")

	return cmd
}

// lambdaUpdateFunctionConfigurationParams holds the flag values for
// update-function-configuration.
type lambdaUpdateFunctionConfigurationParams struct {
	functionName, role, handler, description string
	runtime, environmentJSON, invokeEndpoint string
	timeout, memorySize                      int32
}

// buildLambdaUpdateFunctionConfigurationBody assembles the
// UpdateFunctionConfiguration request body. Built as a map for the same
// reason as buildLambdaCreateFunctionBody: the wire format mirrors
// internal/service/lambda's PascalCase JSON API.
func buildLambdaUpdateFunctionConfigurationBody(p *lambdaUpdateFunctionConfigurationParams) (map[string]any, error) {
	body := map[string]any{}

	if p.description != "" {
		body["Description"] = p.description
	}

	if p.handler != "" {
		body["Handler"] = p.handler
	}

	if p.memorySize > 0 {
		body["MemorySize"] = p.memorySize
	}

	if p.role != "" {
		body["Role"] = p.role
	}

	if p.runtime != "" {
		body["Runtime"] = p.runtime
	}

	if p.timeout > 0 {
		body["Timeout"] = p.timeout
	}

	if p.invokeEndpoint != "" {
		body["InvokeEndpoint"] = p.invokeEndpoint
	}

	if p.environmentJSON != "" {
		var environment lambdaTypes.Environment
		if err := json.Unmarshal([]byte(p.environmentJSON), &environment); err != nil {
			return nil, fmt.Errorf("invalid --environment JSON: %w", err)
		}

		body["Environment"] = environment
	}

	return body, nil
}

func newLambdaUpdateFunctionConfigurationCmd() *cobra.Command {
	var p lambdaUpdateFunctionConfigurationParams

	cmd := &cobra.Command{
		Use:   "update-function-configuration",
		Short: "Update a Lambda function's configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			body, err := buildLambdaUpdateFunctionConfigurationBody(&p)
			if err != nil {
				return err
			}

			path := "/2015-03-31/functions/" + p.functionName + "/configuration"

			var out map[string]any
			if err := lambdaRawRequest(cmd.Context(), http.MethodPut, path, body, &out); err != nil {
				return fmt.Errorf("update-function-configuration failed: %w", err)
			}

			if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
				return fmt.Errorf("failed to encode output: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&p.functionName, "function-name", "", "Function name")
	cmd.Flags().StringVar(&p.role, "role", "", "Execution role ARN")
	cmd.Flags().StringVar(&p.handler, "handler", "", "Function entry point")
	cmd.Flags().StringVar(&p.description, "description", "", "Function description")
	cmd.Flags().StringVar(&p.runtime, "runtime", "", "Runtime identifier (e.g. python3.12, nodejs20.x)")
	cmd.Flags().Int32Var(&p.timeout, "timeout", 0, "Function timeout in seconds")
	cmd.Flags().Int32Var(&p.memorySize, "memory-size", 0, "Function memory size in MB")
	cmd.Flags().StringVar(&p.environmentJSON, "environment", "", "Environment variables (JSON, e.g. {\"Variables\":{\"KEY\":\"VALUE\"}})")
	cmd.Flags().StringVar(&p.invokeEndpoint, "invoke-endpoint", "", "kumo extension: HTTP endpoint kumo proxies invocations to")

	return cmd
}
