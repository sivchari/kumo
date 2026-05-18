//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/sivchari/golden"
)

func newGlueClient(t *testing.T) *glue.Client {
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

	return glue.NewFromConfig(cfg, func(o *glue.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
	})
}

func TestGlue_CreateAndGetDatabase(t *testing.T) {
	client := newGlueClient(t)
	ctx := t.Context()

	dbName := "test_database"

	// Create database.
	_, err := client.CreateDatabase(ctx, &glue.CreateDatabaseInput{
		DatabaseInput: &types.DatabaseInput{
			Name:        aws.String(dbName),
			Description: aws.String("Test database"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get database.
	getOutput, err := client.GetDatabase(ctx, &glue.GetDatabaseInput{
		Name: aws.String(dbName),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("CreateTime", "ResultMetadata")).Assert(t.Name(), getOutput)
}

func TestGlue_GetDatabases(t *testing.T) {
	client := newGlueClient(t)
	ctx := t.Context()

	// Create a database.
	dbName := "list_test_database"
	_, err := client.CreateDatabase(ctx, &glue.CreateDatabaseInput{
		DatabaseInput: &types.DatabaseInput{
			Name: aws.String(dbName),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get databases.
	listOutput, err := client.GetDatabases(ctx, &glue.GetDatabasesInput{
		MaxResults: aws.Int32(10),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Find our database.
	found := false

	for _, db := range listOutput.DatabaseList {
		if db.Name != nil && *db.Name == dbName {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("database %s not found in list", dbName)
	}
}

func TestGlue_DeleteDatabase(t *testing.T) {
	client := newGlueClient(t)
	ctx := t.Context()

	dbName := "delete_test_database"

	// Create a database.
	_, err := client.CreateDatabase(ctx, &glue.CreateDatabaseInput{
		DatabaseInput: &types.DatabaseInput{
			Name: aws.String(dbName),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete the database.
	_, err = client.DeleteDatabase(ctx, &glue.DeleteDatabaseInput{
		Name: aws.String(dbName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify it's deleted.
	_, err = client.GetDatabase(ctx, &glue.GetDatabaseInput{
		Name: aws.String(dbName),
	})
	if err == nil {
		t.Fatal("expected error when getting deleted database")
	}
}

func TestGlue_CreateAndGetTable(t *testing.T) {
	client := newGlueClient(t)
	ctx := t.Context()

	dbName := "table_test_database"
	tableName := "test_table"

	// Create database first.
	_, err := client.CreateDatabase(ctx, &glue.CreateDatabaseInput{
		DatabaseInput: &types.DatabaseInput{
			Name: aws.String(dbName),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create table.
	_, err = client.CreateTable(ctx, &glue.CreateTableInput{
		DatabaseName: aws.String(dbName),
		TableInput: &types.TableInput{
			Name:        aws.String(tableName),
			Description: aws.String("Test table"),
			TableType:   aws.String("EXTERNAL_TABLE"),
			StorageDescriptor: &types.StorageDescriptor{
				Columns: []types.Column{
					{
						Name: aws.String("id"),
						Type: aws.String("int"),
					},
					{
						Name: aws.String("name"),
						Type: aws.String("string"),
					},
				},
				Location:     aws.String("s3://test-bucket/data/"),
				InputFormat:  aws.String("org.apache.hadoop.mapred.TextInputFormat"),
				OutputFormat: aws.String("org.apache.hadoop.hive.ql.io.HiveIgnoreKeyTextOutputFormat"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get table.
	getOutput, err := client.GetTable(ctx, &glue.GetTableInput{
		DatabaseName: aws.String(dbName),
		Name:         aws.String(tableName),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("CreateTime", "UpdateTime", "IsRegisteredWithLakeFormation", "ResultMetadata")).Assert(t.Name(), getOutput)
}

func TestGlue_GetTables(t *testing.T) {
	client := newGlueClient(t)
	ctx := t.Context()

	dbName := "get_tables_database"
	tableName := "list_test_table"

	// Create database.
	_, err := client.CreateDatabase(ctx, &glue.CreateDatabaseInput{
		DatabaseInput: &types.DatabaseInput{
			Name: aws.String(dbName),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create table.
	_, err = client.CreateTable(ctx, &glue.CreateTableInput{
		DatabaseName: aws.String(dbName),
		TableInput: &types.TableInput{
			Name: aws.String(tableName),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get tables.
	listOutput, err := client.GetTables(ctx, &glue.GetTablesInput{
		DatabaseName: aws.String(dbName),
		MaxResults:   aws.Int32(10),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Find our table.
	found := false

	for _, table := range listOutput.TableList {
		if table.Name != nil && *table.Name == tableName {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("table %s not found in list", tableName)
	}
}

func TestGlue_DeleteTable(t *testing.T) {
	client := newGlueClient(t)
	ctx := t.Context()

	dbName := "delete_table_database"
	tableName := "delete_test_table"

	// Create database.
	_, err := client.CreateDatabase(ctx, &glue.CreateDatabaseInput{
		DatabaseInput: &types.DatabaseInput{
			Name: aws.String(dbName),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create table.
	_, err = client.CreateTable(ctx, &glue.CreateTableInput{
		DatabaseName: aws.String(dbName),
		TableInput: &types.TableInput{
			Name: aws.String(tableName),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete table.
	_, err = client.DeleteTable(ctx, &glue.DeleteTableInput{
		DatabaseName: aws.String(dbName),
		Name:         aws.String(tableName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify it's deleted.
	_, err = client.GetTable(ctx, &glue.GetTableInput{
		DatabaseName: aws.String(dbName),
		Name:         aws.String(tableName),
	})
	if err == nil {
		t.Fatal("expected error when getting deleted table")
	}
}

func TestGlue_CreateAndDeleteJob(t *testing.T) {
	client := newGlueClient(t)
	ctx := t.Context()

	jobName := "test_job"

	// Create job.
	createOutput, err := client.CreateJob(ctx, &glue.CreateJobInput{
		Name:        aws.String(jobName),
		Description: aws.String("Test ETL job"),
		Role:        aws.String("arn:aws:iam::000000000000:role/GlueRole"),
		Command: &types.JobCommand{
			Name:           aws.String("glueetl"),
			ScriptLocation: aws.String("s3://test-bucket/scripts/etl.py"),
			PythonVersion:  aws.String("3"),
		},
		GlueVersion:     aws.String("3.0"),
		NumberOfWorkers: aws.Int32(10),
		WorkerType:      types.WorkerTypeG1x,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	// Delete job.
	deleteOutput, err := client.DeleteJob(ctx, &glue.DeleteJobInput{
		JobName: aws.String(jobName),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_delete", deleteOutput)
}

func TestGlue_StartJobRun(t *testing.T) {
	client := newGlueClient(t)
	ctx := t.Context()

	jobName := "run_test_job"

	// Create job first.
	_, err := client.CreateJob(ctx, &glue.CreateJobInput{
		Name: aws.String(jobName),
		Role: aws.String("arn:aws:iam::000000000000:role/GlueRole"),
		Command: &types.JobCommand{
			Name:           aws.String("glueetl"),
			ScriptLocation: aws.String("s3://test-bucket/scripts/etl.py"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Start job run.
	runOutput, err := client.StartJobRun(ctx, &glue.StartJobRunInput{
		JobName: aws.String(jobName),
		Arguments: map[string]string{
			"--input":  "s3://test-bucket/input/",
			"--output": "s3://test-bucket/output/",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("JobRunId", "ResultMetadata")).Assert(t.Name(), runOutput)
}

func TestGlue_TagOperations(t *testing.T) {
	client := newGlueClient(t)
	ctx := t.Context()

	dbName := "tag_test_database"

	// Create a database to tag.
	_, err := client.CreateDatabase(ctx, &glue.CreateDatabaseInput{
		DatabaseInput: &types.DatabaseInput{
			Name: aws.String(dbName),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDatabase(ctx, &glue.DeleteDatabaseInput{
			Name: aws.String(dbName),
		})
	})

	resourceArn := "arn:aws:glue:us-east-1:000000000000:database/" + dbName

	// TagResource — add tags.
	_, err = client.TagResource(ctx, &glue.TagResourceInput{
		ResourceArn: aws.String(resourceArn),
		TagsToAdd: map[string]string{
			"env":     "test",
			"project": "kumo",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// GetTags — verify tags were added.
	getOutput, err := client.GetTags(ctx, &glue.GetTagsInput{
		ResourceArn: aws.String(resourceArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_after_tag", getOutput)

	// UntagResource — remove one tag.
	_, err = client.UntagResource(ctx, &glue.UntagResourceInput{
		ResourceArn:  aws.String(resourceArn),
		TagsToRemove: []string{"project"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// GetTags — verify tag was removed.
	getOutput2, err := client.GetTags(ctx, &glue.GetTagsInput{
		ResourceArn: aws.String(resourceArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_after_untag", getOutput2)

	// GetTags — verify empty resource returns empty tags.
	getOutput3, err := client.GetTags(ctx, &glue.GetTagsInput{
		ResourceArn: aws.String("arn:aws:glue:us-east-1:000000000000:database/nonexistent"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_empty", getOutput3)
}

func TestGlue_GetNonExistentDatabase(t *testing.T) {
	client := newGlueClient(t)
	ctx := t.Context()

	// Try to get a non-existent database.
	_, err := client.GetDatabase(ctx, &glue.GetDatabaseInput{
		Name: aws.String("non_existent_database"),
	})
	if err == nil {
		t.Fatal("expected error when getting non-existent database")
	}
}

func TestGlue_CreateDuplicateDatabase(t *testing.T) {
	client := newGlueClient(t)
	ctx := t.Context()

	dbName := "duplicate_test_database"

	// Create database.
	_, err := client.CreateDatabase(ctx, &glue.CreateDatabaseInput{
		DatabaseInput: &types.DatabaseInput{
			Name: aws.String(dbName),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Try to create the same database again.
	_, err = client.CreateDatabase(ctx, &glue.CreateDatabaseInput{
		DatabaseInput: &types.DatabaseInput{
			Name: aws.String(dbName),
		},
	})
	if err == nil {
		t.Fatal("expected error when creating duplicate database")
	}
}
