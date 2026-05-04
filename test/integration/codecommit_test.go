//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/codecommit"
	"github.com/sivchari/golden"
)

func newCodeCommitClient(t *testing.T) *codecommit.Client {
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

	return codecommit.NewFromConfig(cfg, func(o *codecommit.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
	})
}

func TestCodeCommit_CreateRepository(t *testing.T) {
	client := newCodeCommitClient(t)
	ctx := t.Context()

	output, err := client.CreateRepository(ctx, &codecommit.CreateRepositoryInput{
		RepositoryName:        aws.String("test-repo"),
		RepositoryDescription: aws.String("A test repository"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("RepositoryId", "Arn", "CloneUrlHttp", "CloneUrlSsh", "CreationDate", "LastModifiedDate", "ResultMetadata")).Assert(t.Name(), output)

	t.Cleanup(func() {
		_, _ = client.DeleteRepository(context.Background(), &codecommit.DeleteRepositoryInput{
			RepositoryName: aws.String("test-repo"),
		})
	})
}

func TestCodeCommit_DeleteRepository(t *testing.T) {
	client := newCodeCommitClient(t)
	ctx := t.Context()

	_, err := client.CreateRepository(ctx, &codecommit.CreateRepositoryInput{
		RepositoryName: aws.String("test-delete-repo"),
	})
	if err != nil {
		t.Fatal(err)
	}

	output, err := client.DeleteRepository(ctx, &codecommit.DeleteRepositoryInput{
		RepositoryName: aws.String("test-delete-repo"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("RepositoryId", "ResultMetadata")).Assert(t.Name(), output)
}

func TestCodeCommit_GetRepository(t *testing.T) {
	client := newCodeCommitClient(t)
	ctx := t.Context()

	_, err := client.CreateRepository(ctx, &codecommit.CreateRepositoryInput{
		RepositoryName:        aws.String("test-get-repo"),
		RepositoryDescription: aws.String("A test repository for get"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteRepository(context.Background(), &codecommit.DeleteRepositoryInput{
			RepositoryName: aws.String("test-get-repo"),
		})
	})

	output, err := client.GetRepository(ctx, &codecommit.GetRepositoryInput{
		RepositoryName: aws.String("test-get-repo"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("RepositoryId", "Arn", "CloneUrlHttp", "CloneUrlSsh", "CreationDate", "LastModifiedDate", "ResultMetadata")).Assert(t.Name(), output)
}

func TestCodeCommit_ListRepositories(t *testing.T) {
	client := newCodeCommitClient(t)
	ctx := t.Context()

	var createdRepos []string

	for i := 0; i < 3; i++ {
		name := "test-list-repo-" + string(rune('a'+i))
		_, err := client.CreateRepository(ctx, &codecommit.CreateRepositoryInput{
			RepositoryName: aws.String(name),
		})
		if err != nil {
			t.Fatalf("failed to create repository %d: %v", i, err)
		}

		createdRepos = append(createdRepos, name)
	}

	t.Cleanup(func() {
		for _, name := range createdRepos {
			_, _ = client.DeleteRepository(context.Background(), &codecommit.DeleteRepositoryInput{
				RepositoryName: aws.String(name),
			})
		}
	})

	output, err := client.ListRepositories(ctx, &codecommit.ListRepositoriesInput{})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("RepositoryId", "ResultMetadata")).Assert(t.Name(), output)
}

func TestCodeCommit_RepositoryNotFound(t *testing.T) {
	client := newCodeCommitClient(t)
	ctx := t.Context()

	_, err := client.GetRepository(ctx, &codecommit.GetRepositoryInput{
		RepositoryName: aws.String("non-existent-repo"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent repository")
	}
}

func TestCodeCommit_RepositoryNameAlreadyExists(t *testing.T) {
	client := newCodeCommitClient(t)
	ctx := t.Context()

	_, err := client.CreateRepository(ctx, &codecommit.CreateRepositoryInput{
		RepositoryName: aws.String("duplicate-repo"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteRepository(context.Background(), &codecommit.DeleteRepositoryInput{
			RepositoryName: aws.String("duplicate-repo"),
		})
	})

	_, err = client.CreateRepository(ctx, &codecommit.CreateRepositoryInput{
		RepositoryName: aws.String("duplicate-repo"),
	})
	if err == nil {
		t.Fatal("expected error for duplicate repository name")
	}
}

func TestCodeCommit_CreateBranch(t *testing.T) {
	client := newCodeCommitClient(t)
	ctx := t.Context()

	_, err := client.CreateRepository(ctx, &codecommit.CreateRepositoryInput{
		RepositoryName: aws.String("test-branch-repo"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteRepository(context.Background(), &codecommit.DeleteRepositoryInput{
			RepositoryName: aws.String("test-branch-repo"),
		})
	})

	// Get the commit ID of the main branch
	mainBranch, err := client.GetBranch(ctx, &codecommit.GetBranchInput{
		RepositoryName: aws.String("test-branch-repo"),
		BranchName:     aws.String("main"),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.CreateBranch(ctx, &codecommit.CreateBranchInput{
		RepositoryName: aws.String("test-branch-repo"),
		BranchName:     aws.String("feature-branch"),
		CommitId:       mainBranch.Branch.CommitId,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCodeCommit_GetBranch(t *testing.T) {
	client := newCodeCommitClient(t)
	ctx := t.Context()

	_, err := client.CreateRepository(ctx, &codecommit.CreateRepositoryInput{
		RepositoryName: aws.String("test-get-branch-repo"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteRepository(context.Background(), &codecommit.DeleteRepositoryInput{
			RepositoryName: aws.String("test-get-branch-repo"),
		})
	})

	output, err := client.GetBranch(ctx, &codecommit.GetBranchInput{
		RepositoryName: aws.String("test-get-branch-repo"),
		BranchName:     aws.String("main"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("CommitId", "ResultMetadata")).Assert(t.Name(), output)
}

func TestCodeCommit_ListBranches(t *testing.T) {
	client := newCodeCommitClient(t)
	ctx := t.Context()

	_, err := client.CreateRepository(ctx, &codecommit.CreateRepositoryInput{
		RepositoryName: aws.String("test-list-branches-repo"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteRepository(context.Background(), &codecommit.DeleteRepositoryInput{
			RepositoryName: aws.String("test-list-branches-repo"),
		})
	})

	// Get the commit ID of the main branch
	mainBranch, err := client.GetBranch(ctx, &codecommit.GetBranchInput{
		RepositoryName: aws.String("test-list-branches-repo"),
		BranchName:     aws.String("main"),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.CreateBranch(ctx, &codecommit.CreateBranchInput{
		RepositoryName: aws.String("test-list-branches-repo"),
		BranchName:     aws.String("develop"),
		CommitId:       mainBranch.Branch.CommitId,
	})
	if err != nil {
		t.Fatal(err)
	}

	output, err := client.ListBranches(ctx, &codecommit.ListBranchesInput{
		RepositoryName: aws.String("test-list-branches-repo"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), output)
}

func TestCodeCommit_BranchNotFound(t *testing.T) {
	client := newCodeCommitClient(t)
	ctx := t.Context()

	_, err := client.CreateRepository(ctx, &codecommit.CreateRepositoryInput{
		RepositoryName: aws.String("test-branch-notfound-repo"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteRepository(context.Background(), &codecommit.DeleteRepositoryInput{
			RepositoryName: aws.String("test-branch-notfound-repo"),
		})
	})

	_, err = client.GetBranch(ctx, &codecommit.GetBranchInput{
		RepositoryName: aws.String("test-branch-notfound-repo"),
		BranchName:     aws.String("non-existent-branch"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent branch")
	}
}

func TestCodeCommit_PutAndGetFile(t *testing.T) {
	client := newCodeCommitClient(t)
	ctx := t.Context()

	_, err := client.CreateRepository(ctx, &codecommit.CreateRepositoryInput{
		RepositoryName: aws.String("test-file-repo"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteRepository(context.Background(), &codecommit.DeleteRepositoryInput{
			RepositoryName: aws.String("test-file-repo"),
		})
	})

	putOutput, err := client.PutFile(ctx, &codecommit.PutFileInput{
		RepositoryName: aws.String("test-file-repo"),
		BranchName:     aws.String("main"),
		FilePath:       aws.String("hello.txt"),
		FileContent:    []byte("Hello, World!"),
		CommitMessage:  aws.String("Add hello.txt"),
		Name:           aws.String("Test Author"),
		Email:          aws.String("test@example.com"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("CommitId", "BlobId", "TreeId", "ResultMetadata")).Assert(t.Name()+"_put", putOutput)

	getOutput, err := client.GetFile(ctx, &codecommit.GetFileInput{
		RepositoryName: aws.String("test-file-repo"),
		FilePath:       aws.String("hello.txt"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("CommitId", "BlobId", "ResultMetadata")).Assert(t.Name()+"_get", getOutput)
}

func TestCodeCommit_FileNotFound(t *testing.T) {
	client := newCodeCommitClient(t)
	ctx := t.Context()

	_, err := client.CreateRepository(ctx, &codecommit.CreateRepositoryInput{
		RepositoryName: aws.String("test-filenotfound-repo"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteRepository(context.Background(), &codecommit.DeleteRepositoryInput{
			RepositoryName: aws.String("test-filenotfound-repo"),
		})
	})

	_, err = client.GetFile(ctx, &codecommit.GetFileInput{
		RepositoryName: aws.String("test-filenotfound-repo"),
		FilePath:       aws.String("non-existent.txt"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestCodeCommit_CreateRepositoryWithTags(t *testing.T) {
	client := newCodeCommitClient(t)
	ctx := t.Context()

	output, err := client.CreateRepository(ctx, &codecommit.CreateRepositoryInput{
		RepositoryName: aws.String("test-tags-repo"),
		Tags: map[string]string{
			"Environment": "Test",
			"Project":     "awsim",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("RepositoryId", "Arn", "CloneUrlHttp", "CloneUrlSsh", "CreationDate", "LastModifiedDate", "ResultMetadata")).Assert(t.Name(), output)

	t.Cleanup(func() {
		_, _ = client.DeleteRepository(context.Background(), &codecommit.DeleteRepositoryInput{
			RepositoryName: aws.String("test-tags-repo"),
		})
	})
}
