//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/entityresolution"
	"github.com/aws/aws-sdk-go-v2/service/entityresolution/types"
	"github.com/sivchari/golden"
)

func newEntityResolutionClient(t *testing.T) *entityresolution.Client {
	t.Helper()

	return entityresolution.NewFromConfig(awsConfig(t), func(o *entityresolution.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestEntityResolution_CreateAndDeleteSchemaMapping(t *testing.T) {
	client := newEntityResolutionClient(t)
	ctx := t.Context()
	schemaName := "test-schema"

	createResult, err := client.CreateSchemaMapping(ctx, &entityresolution.CreateSchemaMappingInput{
		SchemaName: aws.String(schemaName),
		MappedInputFields: []types.SchemaInputAttribute{
			{
				FieldName: aws.String("email"),
				Type:      types.SchemaAttributeTypeEmailAddress,
			},
			{
				FieldName: aws.String("name"),
				Type:      types.SchemaAttributeTypeName,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("SchemaArn", "ResultMetadata")).Assert(t.Name()+"_create", createResult)

	// Delete
	_, err = client.DeleteSchemaMapping(ctx, &entityresolution.DeleteSchemaMappingInput{
		SchemaName: aws.String(schemaName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestEntityResolution_GetSchemaMapping(t *testing.T) {
	client := newEntityResolutionClient(t)
	ctx := t.Context()
	schemaName := "test-get-schema"

	_, err := client.CreateSchemaMapping(ctx, &entityresolution.CreateSchemaMappingInput{
		SchemaName:  aws.String(schemaName),
		Description: aws.String("test description"),
		MappedInputFields: []types.SchemaInputAttribute{
			{
				FieldName: aws.String("phone"),
				Type:      types.SchemaAttributeTypePhoneNumber,
			},
			{
				FieldName: aws.String("address"),
				Type:      types.SchemaAttributeTypeAddress,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteSchemaMapping(context.Background(), &entityresolution.DeleteSchemaMappingInput{
			SchemaName: aws.String(schemaName),
		})
	})

	getResult, err := client.GetSchemaMapping(ctx, &entityresolution.GetSchemaMappingInput{
		SchemaName: aws.String(schemaName),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("SchemaArn", "CreatedAt", "UpdatedAt", "ResultMetadata")).Assert(t.Name()+"_get", getResult)
}

func TestEntityResolution_ListSchemaMappings(t *testing.T) {
	client := newEntityResolutionClient(t)
	ctx := t.Context()
	schemaName := "test-list-schema"

	_, err := client.CreateSchemaMapping(ctx, &entityresolution.CreateSchemaMappingInput{
		SchemaName: aws.String(schemaName),
		MappedInputFields: []types.SchemaInputAttribute{
			{
				FieldName: aws.String("id"),
				Type:      types.SchemaAttributeTypeUniqueId,
			},
			{
				FieldName: aws.String("name"),
				Type:      types.SchemaAttributeTypeName,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteSchemaMapping(context.Background(), &entityresolution.DeleteSchemaMappingInput{
			SchemaName: aws.String(schemaName),
		})
	})

	listResult, err := client.ListSchemaMappings(ctx, &entityresolution.ListSchemaMappingsInput{})
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, s := range listResult.SchemaList {
		if *s.SchemaName == schemaName {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("expected to find schema %s in list", schemaName)
	}
}

func TestEntityResolution_CreateAndDeleteMatchingWorkflow(t *testing.T) {
	client := newEntityResolutionClient(t)
	ctx := t.Context()
	workflowName := "test-matching-workflow"

	createResult, err := client.CreateMatchingWorkflow(ctx, &entityresolution.CreateMatchingWorkflowInput{
		WorkflowName: aws.String(workflowName),
		InputSourceConfig: []types.InputSource{
			{
				InputSourceARN: aws.String("arn:aws:glue:us-east-1:000000000000:table/db/table1"),
				SchemaName:     aws.String("test-schema"),
			},
		},
		OutputSourceConfig: []types.OutputSource{
			{
				OutputS3Path: aws.String("s3://bucket/output/"),
				Output: []types.OutputAttribute{
					{
						Name: aws.String("id"),
					},
				},
			},
		},
		ResolutionTechniques: &types.ResolutionTechniques{
			ResolutionType: types.ResolutionTypeRuleMatching,
		},
		RoleArn: aws.String("arn:aws:iam::000000000000:role/test-role"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("WorkflowArn", "ResultMetadata")).Assert(t.Name()+"_create", createResult)

	// Delete
	_, err = client.DeleteMatchingWorkflow(ctx, &entityresolution.DeleteMatchingWorkflowInput{
		WorkflowName: aws.String(workflowName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestEntityResolution_GetMatchingWorkflow(t *testing.T) {
	client := newEntityResolutionClient(t)
	ctx := t.Context()
	workflowName := "test-get-matching-workflow"

	_, err := client.CreateMatchingWorkflow(ctx, &entityresolution.CreateMatchingWorkflowInput{
		WorkflowName: aws.String(workflowName),
		InputSourceConfig: []types.InputSource{
			{
				InputSourceARN: aws.String("arn:aws:glue:us-east-1:000000000000:table/db/table1"),
				SchemaName:     aws.String("test-schema"),
			},
		},
		OutputSourceConfig: []types.OutputSource{
			{
				OutputS3Path: aws.String("s3://bucket/output/"),
				Output: []types.OutputAttribute{
					{Name: aws.String("id")},
				},
			},
		},
		ResolutionTechniques: &types.ResolutionTechniques{
			ResolutionType: types.ResolutionTypeRuleMatching,
		},
		RoleArn: aws.String("arn:aws:iam::000000000000:role/test-role"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteMatchingWorkflow(context.Background(), &entityresolution.DeleteMatchingWorkflowInput{
			WorkflowName: aws.String(workflowName),
		})
	})

	getResult, err := client.GetMatchingWorkflow(ctx, &entityresolution.GetMatchingWorkflowInput{
		WorkflowName: aws.String(workflowName),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("WorkflowArn", "CreatedAt", "UpdatedAt", "ResultMetadata")).Assert(t.Name()+"_get", getResult)
}

func TestEntityResolution_CreateAndDeleteIdMappingWorkflow(t *testing.T) {
	client := newEntityResolutionClient(t)
	ctx := t.Context()
	workflowName := "test-idmapping-workflow"

	createResult, err := client.CreateIdMappingWorkflow(ctx, &entityresolution.CreateIdMappingWorkflowInput{
		WorkflowName: aws.String(workflowName),
		InputSourceConfig: []types.IdMappingWorkflowInputSource{
			{
				InputSourceARN: aws.String("arn:aws:glue:us-east-1:000000000000:table/db/table1"),
			},
		},
		IdMappingTechniques: &types.IdMappingTechniques{
			IdMappingType: types.IdMappingTypeRuleBased,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("WorkflowArn", "ResultMetadata")).Assert(t.Name()+"_create", createResult)

	// Delete
	_, err = client.DeleteIdMappingWorkflow(ctx, &entityresolution.DeleteIdMappingWorkflowInput{
		WorkflowName: aws.String(workflowName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestEntityResolution_SchemaNotFound(t *testing.T) {
	client := newEntityResolutionClient(t)
	ctx := t.Context()

	_, err := client.GetSchemaMapping(ctx, &entityresolution.GetSchemaMappingInput{
		SchemaName: aws.String("non-existent-schema"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent schema")
	}
}

func TestEntityResolution_DuplicateSchema(t *testing.T) {
	client := newEntityResolutionClient(t)
	ctx := t.Context()
	schemaName := "test-duplicate-schema"

	_, err := client.CreateSchemaMapping(ctx, &entityresolution.CreateSchemaMappingInput{
		SchemaName: aws.String(schemaName),
		MappedInputFields: []types.SchemaInputAttribute{
			{
				FieldName: aws.String("id"),
				Type:      types.SchemaAttributeTypeUniqueId,
			},
			{
				FieldName: aws.String("name"),
				Type:      types.SchemaAttributeTypeName,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteSchemaMapping(context.Background(), &entityresolution.DeleteSchemaMappingInput{
			SchemaName: aws.String(schemaName),
		})
	})

	_, err = client.CreateSchemaMapping(ctx, &entityresolution.CreateSchemaMappingInput{
		SchemaName: aws.String(schemaName),
		MappedInputFields: []types.SchemaInputAttribute{
			{
				FieldName: aws.String("id"),
				Type:      types.SchemaAttributeTypeUniqueId,
			},
			{
				FieldName: aws.String("name"),
				Type:      types.SchemaAttributeTypeName,
			},
		},
	})
	if err == nil {
		t.Fatal("expected error for duplicate schema")
	}
}

func TestEntityResolution_ListProviderServices(t *testing.T) {
	client := newEntityResolutionClient(t)
	ctx := t.Context()

	listResult, err := client.ListProviderServices(ctx, &entityresolution.ListProviderServicesInput{})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), listResult)
}
