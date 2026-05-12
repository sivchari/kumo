//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/sivchari/golden"
)

func newSSMClient(t *testing.T) *ssm.Client {
	t.Helper()

	cfg, err := config.LoadDefaultConfig(t.Context(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			"test", "test", "",
		)),
	)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	return ssm.NewFromConfig(cfg, func(o *ssm.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
	})
}

func TestSSM_PutAndGetParameter(t *testing.T) {
	client := newSSMClient(t)
	ctx := t.Context()
	paramName := "/test/param1"
	paramValue := "test-value"

	// Put parameter.
	putOutput, err := client.PutParameter(ctx, &ssm.PutParameterInput{
		Name:  aws.String(paramName),
		Value: aws.String(paramValue),
		Type:  types.ParameterTypeString,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_put", putOutput)

	t.Cleanup(func() {
		_, _ = client.DeleteParameter(context.Background(), &ssm.DeleteParameterInput{
			Name: aws.String(paramName),
		})
	})

	// Get parameter.
	getOutput, err := client.GetParameter(ctx, &ssm.GetParameterInput{
		Name: aws.String(paramName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ARN", "LastModifiedDate", "ResultMetadata")).Assert(t.Name()+"_get", getOutput)
}

func TestSSM_PutParameter_Update(t *testing.T) {
	client := newSSMClient(t)
	ctx := t.Context()
	paramName := "/test/param-update"

	// Put initial parameter.
	_, err := client.PutParameter(ctx, &ssm.PutParameterInput{
		Name:  aws.String(paramName),
		Value: aws.String("initial-value"),
		Type:  types.ParameterTypeString,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteParameter(context.Background(), &ssm.DeleteParameterInput{
			Name: aws.String(paramName),
		})
	})

	// Update parameter without Overwrite - should fail.
	_, err = client.PutParameter(ctx, &ssm.PutParameterInput{
		Name:  aws.String(paramName),
		Value: aws.String("new-value"),
		Type:  types.ParameterTypeString,
	})
	if err == nil {
		t.Fatal("expected error when updating without Overwrite")
	}

	// Update parameter with Overwrite - should succeed.
	putOutput, err := client.PutParameter(ctx, &ssm.PutParameterInput{
		Name:      aws.String(paramName),
		Value:     aws.String("updated-value"),
		Type:      types.ParameterTypeString,
		Overwrite: aws.Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), putOutput)
}

func TestSSM_GetParameters(t *testing.T) {
	client := newSSMClient(t)
	ctx := t.Context()

	params := []struct {
		name  string
		value string
	}{
		{"/test/multi/param1", "value1"},
		{"/test/multi/param2", "value2"},
		{"/test/multi/param3", "value3"},
	}

	// Put parameters.
	for _, p := range params {
		_, err := client.PutParameter(ctx, &ssm.PutParameterInput{
			Name:  aws.String(p.name),
			Value: aws.String(p.value),
			Type:  types.ParameterTypeString,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Cleanup(func() {
		names := make([]string, len(params))
		for i, p := range params {
			names[i] = p.name
		}

		_, _ = client.DeleteParameters(context.Background(), &ssm.DeleteParametersInput{
			Names: names,
		})
	})

	// Get parameters including one invalid.
	names := []string{"/test/multi/param1", "/test/multi/param2", "/test/multi/nonexistent"}
	getOutput, err := client.GetParameters(ctx, &ssm.GetParametersInput{
		Names: names,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ARN", "LastModifiedDate", "ResultMetadata")).Assert(t.Name(), getOutput)
}

func TestSSM_GetParametersByPath(t *testing.T) {
	client := newSSMClient(t)
	ctx := t.Context()

	params := []struct {
		name  string
		value string
	}{
		{"/myapp/config/param1", "value1"},
		{"/myapp/config/param2", "value2"},
		{"/myapp/config/nested/param3", "value3"},
		{"no_slash_param", "value4"},
	}

	// Put parameters.
	for _, p := range params {
		_, err := client.PutParameter(ctx, &ssm.PutParameterInput{
			Name:  aws.String(p.name),
			Value: aws.String(p.value),
			Type:  types.ParameterTypeString,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Cleanup(func() {
		names := make([]string, len(params))
		for i, p := range params {
			names[i] = p.name
		}

		_, _ = client.DeleteParameters(context.Background(), &ssm.DeleteParametersInput{
			Names: names,
		})
	})

	// Get by path non-recursive.
	getOutput, err := client.GetParametersByPath(ctx, &ssm.GetParametersByPathInput{
		Path: aws.String("/myapp/config"),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ARN", "LastModifiedDate", "ResultMetadata")).Assert(t.Name()+"_nonrecursive", getOutput)

	// Get by path recursive.
	getOutput, err = client.GetParametersByPath(ctx, &ssm.GetParametersByPathInput{
		Path:      aws.String("/myapp/config"),
		Recursive: aws.Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ARN", "LastModifiedDate", "ResultMetadata")).Assert(t.Name()+"_recursive", getOutput)

	// Get by root path "/" should include parameter without leading slash.
	getOutput, err = client.GetParametersByPath(ctx, &ssm.GetParametersByPathInput{
		Path:      aws.String("/"),
		Recursive: aws.Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ARN", "LastModifiedDate", "ResultMetadata")).Assert(t.Name()+"_root_recursive", getOutput)
}

func TestSSM_DeleteParameter(t *testing.T) {
	client := newSSMClient(t)
	ctx := t.Context()
	paramName := "/test/param-delete"

	// Put parameter.
	_, err := client.PutParameter(ctx, &ssm.PutParameterInput{
		Name:  aws.String(paramName),
		Value: aws.String("test-value"),
		Type:  types.ParameterTypeString,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete parameter.
	_, err = client.DeleteParameter(ctx, &ssm.DeleteParameterInput{
		Name: aws.String(paramName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get parameter - should fail.
	_, err = client.GetParameter(ctx, &ssm.GetParameterInput{
		Name: aws.String(paramName),
	})
	if err == nil {
		t.Fatal("expected error when getting deleted parameter")
	}
}

func TestSSM_DeleteParameters(t *testing.T) {
	client := newSSMClient(t)
	ctx := t.Context()

	params := []string{"/test/delete/param1", "/test/delete/param2"}

	// Put parameters.
	for _, name := range params {
		_, err := client.PutParameter(ctx, &ssm.PutParameterInput{
			Name:  aws.String(name),
			Value: aws.String("test-value"),
			Type:  types.ParameterTypeString,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Delete parameters including one that doesn't exist.
	names := append(params, "/test/delete/nonexistent")
	deleteOutput, err := client.DeleteParameters(ctx, &ssm.DeleteParametersInput{
		Names: names,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), deleteOutput)
}

func TestSSM_DescribeParameters(t *testing.T) {
	client := newSSMClient(t)
	ctx := t.Context()

	params := []struct {
		name        string
		value       string
		description string
	}{
		{"/test/describe/param1", "value1", "Test parameter 1"},
		{"/test/describe/param2", "value2", "Test parameter 2"},
	}

	// Put parameters.
	for _, p := range params {
		_, err := client.PutParameter(ctx, &ssm.PutParameterInput{
			Name:        aws.String(p.name),
			Value:       aws.String(p.value),
			Type:        types.ParameterTypeString,
			Description: aws.String(p.description),
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Cleanup(func() {
		names := make([]string, len(params))
		for i, p := range params {
			names[i] = p.name
		}

		_, _ = client.DeleteParameters(context.Background(), &ssm.DeleteParametersInput{
			Names: names,
		})
	})

	// Describe parameters.
	descOutput, err := client.DescribeParameters(ctx, &ssm.DescribeParametersInput{})
	if err != nil {
		t.Fatal(err)
	}

	if len(descOutput.Parameters) < 2 {
		t.Errorf("expected at least 2 parameters, got %d", len(descOutput.Parameters))
	}

	// Check if our parameters are in the list.
	found := 0

	for _, p := range descOutput.Parameters {
		for _, expected := range params {
			if *p.Name == expected.name {
				found++

				if p.Description != nil && *p.Description != expected.description {
					t.Errorf("expected description %s, got %s", expected.description, *p.Description)
				}

				break
			}
		}
	}

	if found != 2 {
		t.Errorf("expected to find 2 parameters, found %d", found)
	}
}

func TestSSM_GetParameter_NotFound(t *testing.T) {
	client := newSSMClient(t)
	ctx := t.Context()

	_, err := client.GetParameter(ctx, &ssm.GetParameterInput{
		Name: aws.String("/nonexistent/param"),
	})
	if err == nil {
		t.Fatal("expected error when getting nonexistent parameter")
	}
}

func TestSSM_SecureString_WithDecryptionFalse(t *testing.T) {
	client := newSSMClient(t)
	ctx := t.Context()

	paramName := "/test/secure-param"
	paramValue := "SECURE"

	_, err := client.PutParameter(ctx, &ssm.PutParameterInput{
		Name:  aws.String(paramName),
		Value: aws.String(paramValue),
		Type:  types.ParameterTypeSecureString,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteParameter(context.Background(), &ssm.DeleteParameterInput{
			Name: aws.String(paramName),
		})
	})

	// WithDecryption=false should NOT return the actual value.
	getOutput, err := client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(paramName),
		WithDecryption: aws.Bool(false),
	})
	if err != nil {
		t.Fatal(err)
	}

	if aws.ToString(getOutput.Parameter.Value) == paramValue {
		t.Errorf("expected masked value when WithDecryption=false, got actual value %q", paramValue)
	}

	// WithDecryption=true should return the actual value.
	getOutputDecrypted, err := client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(paramName),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}

	if aws.ToString(getOutputDecrypted.Parameter.Value) != paramValue {
		t.Errorf("expected actual value %q when WithDecryption=true, got %q",
			paramValue, aws.ToString(getOutputDecrypted.Parameter.Value))
	}
}

func TestSSM_ListTagsForResource(t *testing.T) {
	client := newSSMClient(t)
	ctx := t.Context()
	paramName := "/test/tag-param"

	// Put a parameter to tag.
	_, err := client.PutParameter(ctx, &ssm.PutParameterInput{
		Name:  aws.String(paramName),
		Value: aws.String("tag-test-value"),
		Type:  types.ParameterTypeString,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteParameter(context.Background(), &ssm.DeleteParameterInput{
			Name: aws.String(paramName),
		})
	})

	// ListTagsForResource should succeed with empty tag list.
	listOutput, err := client.ListTagsForResource(ctx, &ssm.ListTagsForResourceInput{
		ResourceType: types.ResourceTypeForTaggingParameter,
		ResourceId:   aws.String(paramName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), listOutput)

	// AddTagsToResource should succeed (no-op stub).
	addOutput, err := client.AddTagsToResource(ctx, &ssm.AddTagsToResourceInput{
		ResourceType: types.ResourceTypeForTaggingParameter,
		ResourceId:   aws.String(paramName),
		Tags: []types.Tag{
			{Key: aws.String("env"), Value: aws.String("test")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_add", addOutput)

	// RemoveTagsFromResource should succeed (no-op stub).
	removeOutput, err := client.RemoveTagsFromResource(ctx, &ssm.RemoveTagsFromResourceInput{
		ResourceType: types.ResourceTypeForTaggingParameter,
		ResourceId:   aws.String(paramName),
		TagKeys:      []string{"env"},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_remove", removeOutput)
}
