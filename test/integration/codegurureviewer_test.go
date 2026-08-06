//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codegurureviewer"
	"github.com/aws/aws-sdk-go-v2/service/codegurureviewer/types"
	"github.com/sivchari/golden"
)

func newCodeGuruReviewerClient(t *testing.T) *codegurureviewer.Client {
	t.Helper()

	return codegurureviewer.NewFromConfig(awsConfig(t), func(o *codegurureviewer.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestCodeGuruReviewer_AssociateRepository(t *testing.T) {
	client := newCodeGuruReviewerClient(t)
	ctx := t.Context()

	result, err := client.AssociateRepository(ctx, &codegurureviewer.AssociateRepositoryInput{
		Repository: &types.Repository{
			CodeCommit: &types.CodeCommitRepository{
				Name: aws.String("my-repo"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("AssociationArn", "AssociationId", "CreatedTimeStamp", "LastUpdatedTimeStamp", "ResultMetadata")).Assert(t.Name(), result)
}

func TestCodeGuruReviewer_DescribeRepositoryAssociation(t *testing.T) {
	client := newCodeGuruReviewerClient(t)
	ctx := t.Context()

	assocResult, err := client.AssociateRepository(ctx, &codegurureviewer.AssociateRepositoryInput{
		Repository: &types.Repository{
			CodeCommit: &types.CodeCommitRepository{
				Name: aws.String("describe-repo"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.DescribeRepositoryAssociation(ctx, &codegurureviewer.DescribeRepositoryAssociationInput{
		AssociationArn: assocResult.RepositoryAssociation.AssociationArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("AssociationArn", "AssociationId", "CreatedTimeStamp", "LastUpdatedTimeStamp", "ResultMetadata")).Assert(t.Name(), result)
}

func TestCodeGuruReviewer_DisassociateRepository(t *testing.T) {
	client := newCodeGuruReviewerClient(t)
	ctx := t.Context()

	assocResult, err := client.AssociateRepository(ctx, &codegurureviewer.AssociateRepositoryInput{
		Repository: &types.Repository{
			CodeCommit: &types.CodeCommitRepository{
				Name: aws.String("disassociate-repo"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.DisassociateRepository(ctx, &codegurureviewer.DisassociateRepositoryInput{
		AssociationArn: assocResult.RepositoryAssociation.AssociationArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("AssociationArn", "AssociationId", "CreatedTimeStamp", "LastUpdatedTimeStamp", "ResultMetadata")).Assert(t.Name(), result)
}

func TestCodeGuruReviewer_ListRepositoryAssociations(t *testing.T) {
	client := newCodeGuruReviewerClient(t)
	ctx := t.Context()

	_, err := client.AssociateRepository(ctx, &codegurureviewer.AssociateRepositoryInput{
		Repository: &types.Repository{
			CodeCommit: &types.CodeCommitRepository{
				Name: aws.String("list-repo"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.ListRepositoryAssociations(ctx, &codegurureviewer.ListRepositoryAssociationsInput{})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("AssociationArn", "AssociationId", "LastUpdatedTimeStamp", "ResultMetadata")).Assert(t.Name(), result)
}

func TestCodeGuruReviewer_CreateCodeReview(t *testing.T) {
	client := newCodeGuruReviewerClient(t)
	ctx := t.Context()

	assocResult, err := client.AssociateRepository(ctx, &codegurureviewer.AssociateRepositoryInput{
		Repository: &types.Repository{
			CodeCommit: &types.CodeCommitRepository{
				Name: aws.String("review-repo"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.CreateCodeReview(ctx, &codegurureviewer.CreateCodeReviewInput{
		Name:                     aws.String("test-review"),
		RepositoryAssociationArn: assocResult.RepositoryAssociation.AssociationArn,
		Type: &types.CodeReviewType{
			RepositoryAnalysis: &types.RepositoryAnalysis{
				RepositoryHead: &types.RepositoryHeadSourceCodeType{
					BranchName: aws.String("main"),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("CodeReviewArn", "AssociationArn", "CreatedTimeStamp", "LastUpdatedTimeStamp", "ResultMetadata")).Assert(t.Name(), result)
}

func TestCodeGuruReviewer_DescribeCodeReview(t *testing.T) {
	client := newCodeGuruReviewerClient(t)
	ctx := t.Context()

	assocResult, err := client.AssociateRepository(ctx, &codegurureviewer.AssociateRepositoryInput{
		Repository: &types.Repository{
			CodeCommit: &types.CodeCommitRepository{
				Name: aws.String("describe-review-repo"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	createResult, err := client.CreateCodeReview(ctx, &codegurureviewer.CreateCodeReviewInput{
		Name:                     aws.String("describe-review"),
		RepositoryAssociationArn: assocResult.RepositoryAssociation.AssociationArn,
		Type: &types.CodeReviewType{
			RepositoryAnalysis: &types.RepositoryAnalysis{
				RepositoryHead: &types.RepositoryHeadSourceCodeType{
					BranchName: aws.String("main"),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.DescribeCodeReview(ctx, &codegurureviewer.DescribeCodeReviewInput{
		CodeReviewArn: createResult.CodeReview.CodeReviewArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("CodeReviewArn", "AssociationArn", "CreatedTimeStamp", "LastUpdatedTimeStamp", "ResultMetadata")).Assert(t.Name(), result)
}

func TestCodeGuruReviewer_ListCodeReviews(t *testing.T) {
	client := newCodeGuruReviewerClient(t)
	ctx := t.Context()

	assocResult, err := client.AssociateRepository(ctx, &codegurureviewer.AssociateRepositoryInput{
		Repository: &types.Repository{
			CodeCommit: &types.CodeCommitRepository{
				Name: aws.String("list-review-repo"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.CreateCodeReview(ctx, &codegurureviewer.CreateCodeReviewInput{
		Name:                     aws.String("list-review"),
		RepositoryAssociationArn: assocResult.RepositoryAssociation.AssociationArn,
		Type: &types.CodeReviewType{
			RepositoryAnalysis: &types.RepositoryAnalysis{
				RepositoryHead: &types.RepositoryHeadSourceCodeType{
					BranchName: aws.String("main"),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.ListCodeReviews(ctx, &codegurureviewer.ListCodeReviewsInput{
		Type: types.TypeRepositoryAnalysis,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("CodeReviewArn", "AssociationArn", "CreatedTimeStamp", "LastUpdatedTimeStamp", "ResultMetadata")).Assert(t.Name(), result)
}

func TestCodeGuruReviewer_ListRecommendations(t *testing.T) {
	client := newCodeGuruReviewerClient(t)
	ctx := t.Context()

	assocResult, err := client.AssociateRepository(ctx, &codegurureviewer.AssociateRepositoryInput{
		Repository: &types.Repository{
			CodeCommit: &types.CodeCommitRepository{
				Name: aws.String("recommendations-repo"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	createResult, err := client.CreateCodeReview(ctx, &codegurureviewer.CreateCodeReviewInput{
		Name:                     aws.String("recommendations-review"),
		RepositoryAssociationArn: assocResult.RepositoryAssociation.AssociationArn,
		Type: &types.CodeReviewType{
			RepositoryAnalysis: &types.RepositoryAnalysis{
				RepositoryHead: &types.RepositoryHeadSourceCodeType{
					BranchName: aws.String("main"),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.ListRecommendations(ctx, &codegurureviewer.ListRecommendationsInput{
		CodeReviewArn: createResult.CodeReview.CodeReviewArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), result)
}

func TestCodeGuruReviewer_PutRecommendationFeedback(t *testing.T) {
	client := newCodeGuruReviewerClient(t)
	ctx := t.Context()

	assocResult, err := client.AssociateRepository(ctx, &codegurureviewer.AssociateRepositoryInput{
		Repository: &types.Repository{
			CodeCommit: &types.CodeCommitRepository{
				Name: aws.String("feedback-repo"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	createResult, err := client.CreateCodeReview(ctx, &codegurureviewer.CreateCodeReviewInput{
		Name:                     aws.String("feedback-review"),
		RepositoryAssociationArn: assocResult.RepositoryAssociation.AssociationArn,
		Type: &types.CodeReviewType{
			RepositoryAnalysis: &types.RepositoryAnalysis{
				RepositoryHead: &types.RepositoryHeadSourceCodeType{
					BranchName: aws.String("main"),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.PutRecommendationFeedback(ctx, &codegurureviewer.PutRecommendationFeedbackInput{
		CodeReviewArn:    createResult.CodeReview.CodeReviewArn,
		RecommendationId: aws.String("rec-1"),
		Reactions:        []types.Reaction{types.ReactionThumbsUp},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCodeGuruReviewer_DescribeRecommendationFeedback(t *testing.T) {
	client := newCodeGuruReviewerClient(t)
	ctx := t.Context()

	assocResult, err := client.AssociateRepository(ctx, &codegurureviewer.AssociateRepositoryInput{
		Repository: &types.Repository{
			CodeCommit: &types.CodeCommitRepository{
				Name: aws.String("describe-feedback-repo"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	createResult, err := client.CreateCodeReview(ctx, &codegurureviewer.CreateCodeReviewInput{
		Name:                     aws.String("describe-feedback-review"),
		RepositoryAssociationArn: assocResult.RepositoryAssociation.AssociationArn,
		Type: &types.CodeReviewType{
			RepositoryAnalysis: &types.RepositoryAnalysis{
				RepositoryHead: &types.RepositoryHeadSourceCodeType{
					BranchName: aws.String("main"),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.PutRecommendationFeedback(ctx, &codegurureviewer.PutRecommendationFeedbackInput{
		CodeReviewArn:    createResult.CodeReview.CodeReviewArn,
		RecommendationId: aws.String("rec-2"),
		Reactions:        []types.Reaction{types.ReactionThumbsDown},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.DescribeRecommendationFeedback(ctx, &codegurureviewer.DescribeRecommendationFeedbackInput{
		CodeReviewArn:    createResult.CodeReview.CodeReviewArn,
		RecommendationId: aws.String("rec-2"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("CodeReviewArn", "CreatedTimeStamp", "LastUpdatedTimeStamp", "ResultMetadata")).Assert(t.Name(), result)
}

func TestCodeGuruReviewer_ListRecommendationFeedback(t *testing.T) {
	client := newCodeGuruReviewerClient(t)
	ctx := t.Context()

	assocResult, err := client.AssociateRepository(ctx, &codegurureviewer.AssociateRepositoryInput{
		Repository: &types.Repository{
			CodeCommit: &types.CodeCommitRepository{
				Name: aws.String("list-feedback-repo"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	createResult, err := client.CreateCodeReview(ctx, &codegurureviewer.CreateCodeReviewInput{
		Name:                     aws.String("list-feedback-review"),
		RepositoryAssociationArn: assocResult.RepositoryAssociation.AssociationArn,
		Type: &types.CodeReviewType{
			RepositoryAnalysis: &types.RepositoryAnalysis{
				RepositoryHead: &types.RepositoryHeadSourceCodeType{
					BranchName: aws.String("main"),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.PutRecommendationFeedback(ctx, &codegurureviewer.PutRecommendationFeedbackInput{
		CodeReviewArn:    createResult.CodeReview.CodeReviewArn,
		RecommendationId: aws.String("rec-3"),
		Reactions:        []types.Reaction{types.ReactionThumbsUp},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.ListRecommendationFeedback(ctx, &codegurureviewer.ListRecommendationFeedbackInput{
		CodeReviewArn: createResult.CodeReview.CodeReviewArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("CodeReviewArn", "ResultMetadata")).Assert(t.Name(), result)
}

func TestCodeGuruReviewer_AssociationNotFound(t *testing.T) {
	client := newCodeGuruReviewerClient(t)
	ctx := t.Context()

	_, err := client.DescribeRepositoryAssociation(ctx, &codegurureviewer.DescribeRepositoryAssociationInput{
		AssociationArn: aws.String("arn:aws:codeguru-reviewer:us-east-1:000000000000:association:nonexistent"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent repository association")
	}
}
