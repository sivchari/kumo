//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3tables"
	"github.com/sivchari/golden"
)

func newS3TablesClient(t *testing.T) *s3tables.Client {
	t.Helper()

	return s3tables.NewFromConfig(awsConfig(t), func(o *s3tables.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestS3Tables_CreateAndDeleteTableBucket(t *testing.T) {
	client := newS3TablesClient(t)
	ctx := t.Context()
	bucketName := "test-table-bucket"

	// Create table bucket
	createResult, err := client.CreateTableBucket(ctx, &s3tables.CreateTableBucketInput{
		Name: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Arn", "ResultMetadata")).Assert(t.Name()+"_create", createResult)

	arn := *createResult.Arn

	// Delete table bucket
	_, err = client.DeleteTableBucket(context.Background(), &s3tables.DeleteTableBucketInput{
		TableBucketARN: aws.String(arn),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestS3Tables_GetTableBucket(t *testing.T) {
	client := newS3TablesClient(t)
	ctx := t.Context()
	bucketName := "test-get-table-bucket"

	// Create table bucket
	createResult, err := client.CreateTableBucket(ctx, &s3tables.CreateTableBucketInput{
		Name: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	arn := *createResult.Arn

	t.Cleanup(func() {
		_, _ = client.DeleteTableBucket(context.Background(), &s3tables.DeleteTableBucketInput{
			TableBucketARN: aws.String(arn),
		})
	})

	// Get table bucket
	getResult, err := client.GetTableBucket(ctx, &s3tables.GetTableBucketInput{
		TableBucketARN: aws.String(arn),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Arn", "CreatedAt", "ResultMetadata")).Assert(t.Name(), getResult)
}

func TestS3Tables_ListTableBuckets(t *testing.T) {
	client := newS3TablesClient(t)
	ctx := t.Context()
	bucketName := "test-list-table-bucket"

	// Create table bucket
	createResult, err := client.CreateTableBucket(ctx, &s3tables.CreateTableBucketInput{
		Name: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	arn := *createResult.Arn

	t.Cleanup(func() {
		_, _ = client.DeleteTableBucket(context.Background(), &s3tables.DeleteTableBucketInput{
			TableBucketARN: aws.String(arn),
		})
	})

	// List table buckets
	listResult, err := client.ListTableBuckets(ctx, &s3tables.ListTableBucketsInput{})
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, bucket := range listResult.TableBuckets {
		if bucket.Name != nil && *bucket.Name == bucketName {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected to find table bucket %s in list", bucketName)
	}
}

func TestS3Tables_CreateAndDeleteNamespace(t *testing.T) {
	client := newS3TablesClient(t)
	ctx := t.Context()
	bucketName := "test-namespace-bucket"
	namespaceName := "testnamespace"

	// Create table bucket
	createBucketResult, err := client.CreateTableBucket(ctx, &s3tables.CreateTableBucketInput{
		Name: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	arn := *createBucketResult.Arn

	t.Cleanup(func() {
		_, _ = client.DeleteNamespace(context.Background(), &s3tables.DeleteNamespaceInput{
			TableBucketARN: aws.String(arn),
			Namespace:      aws.String(namespaceName),
		})
		_, _ = client.DeleteTableBucket(context.Background(), &s3tables.DeleteTableBucketInput{
			TableBucketARN: aws.String(arn),
		})
	})

	// Create namespace
	createNsResult, err := client.CreateNamespace(ctx, &s3tables.CreateNamespaceInput{
		TableBucketARN: aws.String(arn),
		Namespace:      []string{namespaceName},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("TableBucketARN", "ResultMetadata")).Assert(t.Name()+"_create", createNsResult)

	// Delete namespace
	_, err = client.DeleteNamespace(context.Background(), &s3tables.DeleteNamespaceInput{
		TableBucketARN: aws.String(arn),
		Namespace:      aws.String(namespaceName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestS3Tables_GetNamespace(t *testing.T) {
	client := newS3TablesClient(t)
	ctx := t.Context()
	bucketName := "test-get-namespace-bucket"
	namespaceName := "testgetnamespace"

	// Create table bucket
	createBucketResult, err := client.CreateTableBucket(ctx, &s3tables.CreateTableBucketInput{
		Name: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	arn := *createBucketResult.Arn

	t.Cleanup(func() {
		_, _ = client.DeleteNamespace(context.Background(), &s3tables.DeleteNamespaceInput{
			TableBucketARN: aws.String(arn),
			Namespace:      aws.String(namespaceName),
		})
		_, _ = client.DeleteTableBucket(context.Background(), &s3tables.DeleteTableBucketInput{
			TableBucketARN: aws.String(arn),
		})
	})

	// Create namespace
	_, err = client.CreateNamespace(ctx, &s3tables.CreateNamespaceInput{
		TableBucketARN: aws.String(arn),
		Namespace:      []string{namespaceName},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get namespace
	getResult, err := client.GetNamespace(ctx, &s3tables.GetNamespaceInput{
		TableBucketARN: aws.String(arn),
		Namespace:      aws.String(namespaceName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("TableBucketARN", "CreatedAt", "ResultMetadata")).Assert(t.Name(), getResult)
}

func TestS3Tables_ListNamespaces(t *testing.T) {
	client := newS3TablesClient(t)
	ctx := t.Context()
	bucketName := "test-list-namespaces-bucket"
	namespaceName := "testlistnamespace"

	// Create table bucket
	createBucketResult, err := client.CreateTableBucket(ctx, &s3tables.CreateTableBucketInput{
		Name: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	arn := *createBucketResult.Arn

	t.Cleanup(func() {
		_, _ = client.DeleteNamespace(context.Background(), &s3tables.DeleteNamespaceInput{
			TableBucketARN: aws.String(arn),
			Namespace:      aws.String(namespaceName),
		})
		_, _ = client.DeleteTableBucket(context.Background(), &s3tables.DeleteTableBucketInput{
			TableBucketARN: aws.String(arn),
		})
	})

	// Create namespace
	_, err = client.CreateNamespace(ctx, &s3tables.CreateNamespaceInput{
		TableBucketARN: aws.String(arn),
		Namespace:      []string{namespaceName},
	})
	if err != nil {
		t.Fatal(err)
	}

	// List namespaces
	listResult, err := client.ListNamespaces(ctx, &s3tables.ListNamespacesInput{
		TableBucketARN: aws.String(arn),
	})
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, ns := range listResult.Namespaces {
		if len(ns.Namespace) > 0 && ns.Namespace[0] == namespaceName {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected to find namespace %s in list", namespaceName)
	}
}

func TestS3Tables_CreateAndDeleteTable(t *testing.T) {
	client := newS3TablesClient(t)
	ctx := t.Context()
	bucketName := "test-table-bucket-for-table"
	namespaceName := "testtablenamespace"
	tableName := "testtable"

	// Create table bucket
	createBucketResult, err := client.CreateTableBucket(ctx, &s3tables.CreateTableBucketInput{
		Name: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	arn := *createBucketResult.Arn

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &s3tables.DeleteTableInput{
			TableBucketARN: aws.String(arn),
			Namespace:      aws.String(namespaceName),
			Name:           aws.String(tableName),
		})
		_, _ = client.DeleteNamespace(context.Background(), &s3tables.DeleteNamespaceInput{
			TableBucketARN: aws.String(arn),
			Namespace:      aws.String(namespaceName),
		})
		_, _ = client.DeleteTableBucket(context.Background(), &s3tables.DeleteTableBucketInput{
			TableBucketARN: aws.String(arn),
		})
	})

	// Create namespace
	_, err = client.CreateNamespace(ctx, &s3tables.CreateNamespaceInput{
		TableBucketARN: aws.String(arn),
		Namespace:      []string{namespaceName},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create table
	createTableResult, err := client.CreateTable(ctx, &s3tables.CreateTableInput{
		TableBucketARN: aws.String(arn),
		Namespace:      aws.String(namespaceName),
		Name:           aws.String(tableName),
		Format:         "ICEBERG",
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("TableARN", "VersionToken", "ResultMetadata")).Assert(t.Name()+"_create", createTableResult)

	// Delete table
	_, err = client.DeleteTable(context.Background(), &s3tables.DeleteTableInput{
		TableBucketARN: aws.String(arn),
		Namespace:      aws.String(namespaceName),
		Name:           aws.String(tableName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestS3Tables_GetTable(t *testing.T) {
	client := newS3TablesClient(t)
	ctx := t.Context()
	bucketName := "test-get-table-test-bucket"
	namespaceName := "testgettablenamespace"
	tableName := "testgettable"

	// Create table bucket
	createBucketResult, err := client.CreateTableBucket(ctx, &s3tables.CreateTableBucketInput{
		Name: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	arn := *createBucketResult.Arn

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &s3tables.DeleteTableInput{
			TableBucketARN: aws.String(arn),
			Namespace:      aws.String(namespaceName),
			Name:           aws.String(tableName),
		})
		_, _ = client.DeleteNamespace(context.Background(), &s3tables.DeleteNamespaceInput{
			TableBucketARN: aws.String(arn),
			Namespace:      aws.String(namespaceName),
		})
		_, _ = client.DeleteTableBucket(context.Background(), &s3tables.DeleteTableBucketInput{
			TableBucketARN: aws.String(arn),
		})
	})

	// Create namespace
	_, err = client.CreateNamespace(ctx, &s3tables.CreateNamespaceInput{
		TableBucketARN: aws.String(arn),
		Namespace:      []string{namespaceName},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create table
	_, err = client.CreateTable(ctx, &s3tables.CreateTableInput{
		TableBucketARN: aws.String(arn),
		Namespace:      aws.String(namespaceName),
		Name:           aws.String(tableName),
		Format:         "ICEBERG",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get table
	getResult, err := client.GetTable(ctx, &s3tables.GetTableInput{
		TableBucketARN: aws.String(arn),
		Namespace:      aws.String(namespaceName),
		Name:           aws.String(tableName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("TableBucketARN", "TableARN", "VersionToken", "CreatedAt", "ModifiedAt", "ResultMetadata")).Assert(t.Name(), getResult)
}

func TestS3Tables_ListTables(t *testing.T) {
	client := newS3TablesClient(t)
	ctx := t.Context()
	bucketName := "test-list-tables-bucket"
	namespaceName := "testlisttablesnamespace"
	tableName := "testlisttable"

	// Create table bucket
	createBucketResult, err := client.CreateTableBucket(ctx, &s3tables.CreateTableBucketInput{
		Name: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	arn := *createBucketResult.Arn

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &s3tables.DeleteTableInput{
			TableBucketARN: aws.String(arn),
			Namespace:      aws.String(namespaceName),
			Name:           aws.String(tableName),
		})
		_, _ = client.DeleteNamespace(context.Background(), &s3tables.DeleteNamespaceInput{
			TableBucketARN: aws.String(arn),
			Namespace:      aws.String(namespaceName),
		})
		_, _ = client.DeleteTableBucket(context.Background(), &s3tables.DeleteTableBucketInput{
			TableBucketARN: aws.String(arn),
		})
	})

	// Create namespace
	_, err = client.CreateNamespace(ctx, &s3tables.CreateNamespaceInput{
		TableBucketARN: aws.String(arn),
		Namespace:      []string{namespaceName},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create table
	_, err = client.CreateTable(ctx, &s3tables.CreateTableInput{
		TableBucketARN: aws.String(arn),
		Namespace:      aws.String(namespaceName),
		Name:           aws.String(tableName),
		Format:         "ICEBERG",
	})
	if err != nil {
		t.Fatal(err)
	}

	// List tables
	listResult, err := client.ListTables(ctx, &s3tables.ListTablesInput{
		TableBucketARN: aws.String(arn),
		Namespace:      aws.String(namespaceName),
	})
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, table := range listResult.Tables {
		if table.Name != nil && *table.Name == tableName {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected to find table %s in list", tableName)
	}
}

func TestS3Tables_TableBucketNotFound(t *testing.T) {
	client := newS3TablesClient(t)
	ctx := t.Context()

	// Try to get non-existent table bucket
	_, err := client.GetTableBucket(ctx, &s3tables.GetTableBucketInput{
		TableBucketARN: aws.String("arn:aws:s3tables:us-east-1:000000000000:bucket/non-existent"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent table bucket")
	}
}

func TestS3Tables_NamespaceNotFound(t *testing.T) {
	client := newS3TablesClient(t)
	ctx := t.Context()
	bucketName := "test-ns-not-found-bucket"

	// Create table bucket
	createBucketResult, err := client.CreateTableBucket(ctx, &s3tables.CreateTableBucketInput{
		Name: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	arn := *createBucketResult.Arn

	t.Cleanup(func() {
		_, _ = client.DeleteTableBucket(context.Background(), &s3tables.DeleteTableBucketInput{
			TableBucketARN: aws.String(arn),
		})
	})

	// Try to get non-existent namespace
	_, err = client.GetNamespace(ctx, &s3tables.GetNamespaceInput{
		TableBucketARN: aws.String(arn),
		Namespace:      aws.String("non-existent-namespace"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent namespace")
	}
}

func TestS3Tables_TableNotFound(t *testing.T) {
	client := newS3TablesClient(t)
	ctx := t.Context()
	bucketName := "test-table-not-found-bucket"
	namespaceName := "testtablenotfoundnamespace"

	// Create table bucket
	createBucketResult, err := client.CreateTableBucket(ctx, &s3tables.CreateTableBucketInput{
		Name: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	arn := *createBucketResult.Arn

	t.Cleanup(func() {
		_, _ = client.DeleteNamespace(context.Background(), &s3tables.DeleteNamespaceInput{
			TableBucketARN: aws.String(arn),
			Namespace:      aws.String(namespaceName),
		})
		_, _ = client.DeleteTableBucket(context.Background(), &s3tables.DeleteTableBucketInput{
			TableBucketARN: aws.String(arn),
		})
	})

	// Create namespace
	_, err = client.CreateNamespace(ctx, &s3tables.CreateNamespaceInput{
		TableBucketARN: aws.String(arn),
		Namespace:      []string{namespaceName},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Try to get non-existent table
	_, err = client.GetTable(ctx, &s3tables.GetTableInput{
		TableBucketARN: aws.String(arn),
		Namespace:      aws.String(namespaceName),
		Name:           aws.String("non-existent-table"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent table")
	}
}
