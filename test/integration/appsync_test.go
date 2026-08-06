//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/appsync"
	"github.com/aws/aws-sdk-go-v2/service/appsync/types"
	"github.com/sivchari/golden"
)

func TestAppSync_CreateGraphqlApi(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createAppSyncClient(t)

	// Create a GraphQL API.
	result, err := client.CreateGraphqlApi(ctx, &appsync.CreateGraphqlApiInput{
		Name:               aws.String("test-api"),
		AuthenticationType: types.AuthenticationTypeApiKey,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, err := client.DeleteGraphqlApi(context.Background(), &appsync.DeleteGraphqlApiInput{
			ApiId: result.GraphqlApi.ApiId,
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	golden.New(t, golden.WithIgnoreFields("ApiId", "Arn", "Uris")).Assert(t.Name(), result)
}

func TestAppSync_GetGraphqlApi(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createAppSyncClient(t)

	// Create a GraphQL API first.
	createResult, err := client.CreateGraphqlApi(ctx, &appsync.CreateGraphqlApiInput{
		Name:               aws.String("get-test-api"),
		AuthenticationType: types.AuthenticationTypeAwsIam,
	})
	if err != nil {
		t.Fatal(err)
	}

	apiID := createResult.GraphqlApi.ApiId

	t.Cleanup(func() {
		_, err := client.DeleteGraphqlApi(context.Background(), &appsync.DeleteGraphqlApiInput{
			ApiId: apiID,
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	// Get the API.
	getResult, err := client.GetGraphqlApi(ctx, &appsync.GetGraphqlApiInput{
		ApiId: apiID,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ApiId", "Arn", "Uris")).Assert(t.Name(), getResult)
}

func TestAppSync_ListGraphqlApis(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createAppSyncClient(t)

	// Create some APIs.
	var apiIDs []*string

	for i := 0; i < 3; i++ {
		result, err := client.CreateGraphqlApi(ctx, &appsync.CreateGraphqlApiInput{
			Name:               aws.String("list-test-api"),
			AuthenticationType: types.AuthenticationTypeApiKey,
		})
		if err != nil {
			t.Fatal(err)
		}
		apiIDs = append(apiIDs, result.GraphqlApi.ApiId)
	}

	t.Cleanup(func() {
		for _, apiID := range apiIDs {
			_, err := client.DeleteGraphqlApi(context.Background(), &appsync.DeleteGraphqlApiInput{
				ApiId: apiID,
			})
			if err != nil {
				t.Fatal(err)
			}
		}
	})

	// List APIs.
	listResult, err := client.ListGraphqlApis(ctx, &appsync.ListGraphqlApisInput{})
	if err != nil {
		t.Fatal(err)
	}

	if len(listResult.GraphqlApis) < 3 {
		t.Errorf("expected at least 3 APIs, got %d", len(listResult.GraphqlApis))
	}
}

func TestAppSync_ListGraphqlApis_Pagination(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createAppSyncClient(t)

	// Create some APIs for pagination testing.
	var apiIDs []*string

	for i := 0; i < 3; i++ {
		result, err := client.CreateGraphqlApi(ctx, &appsync.CreateGraphqlApiInput{
			Name:               aws.String("pagination-test-api"),
			AuthenticationType: types.AuthenticationTypeApiKey,
		})
		if err != nil {
			t.Fatal(err)
		}
		apiIDs = append(apiIDs, result.GraphqlApi.ApiId)
	}

	t.Cleanup(func() {
		// Clean up APIs created by this test.
		for _, apiID := range apiIDs {
			_, err := client.DeleteGraphqlApi(context.Background(), &appsync.DeleteGraphqlApiInput{
				ApiId: apiID,
			})
			if err != nil {
				t.Fatal(err)
			}
		}
	})

	// Verify pagination works by iterating through all pages.
	var nextToken *string

	pageCount := 0
	totalResults := 0

	for {
		listResult, err := client.ListGraphqlApis(ctx, &appsync.ListGraphqlApisInput{
			MaxResults: 2,
			NextToken:  nextToken,
		})
		if err != nil {
			t.Fatal(err)
		}
		if listResult == nil {
			t.Fatal("expected listResult to be non-nil")
		}

		// Each page should have at most maxResults items.
		if len(listResult.GraphqlApis) > 2 {
			t.Errorf("expected at most 2 items per page, got %d", len(listResult.GraphqlApis))
		}

		totalResults += len(listResult.GraphqlApis)
		pageCount++

		if listResult.NextToken == nil {
			break
		}

		nextToken = listResult.NextToken

		// Safety limit to prevent infinite loop.
		if pageCount > 100 {
			t.Fatal("Too many pages, possible infinite loop")
		}
	}

	// Verify we got at least the APIs we created.
	if totalResults < 3 {
		t.Errorf("expected at least 3 APIs, got %d", totalResults)
	}

	// Verify pagination was actually used (we should have multiple pages if there are 3+ APIs).
	if totalResults >= 3 && pageCount < 2 {
		t.Errorf("with 3+ APIs and maxResults=2, expected at least 2 pages, got %d", pageCount)
	}
}

func TestAppSync_DeleteGraphqlApi(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createAppSyncClient(t)

	// Create an API.
	createResult, err := client.CreateGraphqlApi(ctx, &appsync.CreateGraphqlApiInput{
		Name:               aws.String("delete-test-api"),
		AuthenticationType: types.AuthenticationTypeApiKey,
	})
	if err != nil {
		t.Fatal(err)
	}

	apiID := createResult.GraphqlApi.ApiId

	// Delete the API.
	_, err = client.DeleteGraphqlApi(ctx, &appsync.DeleteGraphqlApiInput{
		ApiId: apiID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Try to get the deleted API - should fail.
	_, err = client.GetGraphqlApi(ctx, &appsync.GetGraphqlApiInput{
		ApiId: apiID,
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestAppSync_CreateDataSource(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createAppSyncClient(t)

	// Create an API first.
	apiResult, err := client.CreateGraphqlApi(ctx, &appsync.CreateGraphqlApiInput{
		Name:               aws.String("datasource-test-api"),
		AuthenticationType: types.AuthenticationTypeApiKey,
	})
	if err != nil {
		t.Fatal(err)
	}

	apiID := apiResult.GraphqlApi.ApiId

	t.Cleanup(func() {
		_, err := client.DeleteGraphqlApi(context.Background(), &appsync.DeleteGraphqlApiInput{
			ApiId: apiID,
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	// Create a data source.
	dsResult, err := client.CreateDataSource(ctx, &appsync.CreateDataSourceInput{
		ApiId:       apiID,
		Name:        aws.String("test-datasource"),
		Type:        types.DataSourceTypeNone,
		Description: aws.String("Test data source"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("DataSourceArn")).Assert(t.Name(), dsResult)
}

func TestAppSync_CreateResolver(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createAppSyncClient(t)

	// Create an API first.
	apiResult, err := client.CreateGraphqlApi(ctx, &appsync.CreateGraphqlApiInput{
		Name:               aws.String("resolver-test-api"),
		AuthenticationType: types.AuthenticationTypeApiKey,
	})
	if err != nil {
		t.Fatal(err)
	}

	apiID := apiResult.GraphqlApi.ApiId

	t.Cleanup(func() {
		_, err := client.DeleteGraphqlApi(context.Background(), &appsync.DeleteGraphqlApiInput{
			ApiId: apiID,
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	// Create a data source first.
	_, err = client.CreateDataSource(ctx, &appsync.CreateDataSourceInput{
		ApiId: apiID,
		Name:  aws.String("resolver-datasource"),
		Type:  types.DataSourceTypeNone,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create a resolver.
	resolverResult, err := client.CreateResolver(ctx, &appsync.CreateResolverInput{
		ApiId:          apiID,
		TypeName:       aws.String("Query"),
		FieldName:      aws.String("getItem"),
		DataSourceName: aws.String("resolver-datasource"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResolverArn")).Assert(t.Name(), resolverResult)
}

func TestAppSync_StartSchemaCreation(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createAppSyncClient(t)

	// Create an API first.
	apiResult, err := client.CreateGraphqlApi(ctx, &appsync.CreateGraphqlApiInput{
		Name:               aws.String("schema-test-api"),
		AuthenticationType: types.AuthenticationTypeApiKey,
	})
	if err != nil {
		t.Fatal(err)
	}

	apiID := apiResult.GraphqlApi.ApiId

	t.Cleanup(func() {
		_, err := client.DeleteGraphqlApi(context.Background(), &appsync.DeleteGraphqlApiInput{
			ApiId: apiID,
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	// Start schema creation.
	schema := []byte(`
		type Query {
			getItem(id: ID!): Item
		}
		type Item {
			id: ID!
			name: String
		}
	`)

	schemaResult, err := client.StartSchemaCreation(ctx, &appsync.StartSchemaCreationInput{
		ApiId:      apiID,
		Definition: schema,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), schemaResult)
}

func createAppSyncClient(t *testing.T) *appsync.Client {
	t.Helper()

	return appsync.NewFromConfig(awsConfig(t), func(o *appsync.Options) {
		o.BaseEndpoint = aws.String(testEndpoint() + "/appsync")
	})
}
