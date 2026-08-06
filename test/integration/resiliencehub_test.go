//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/resiliencehub"
	"github.com/aws/aws-sdk-go-v2/service/resiliencehub/types"
	"github.com/sivchari/golden"
)

func newResilienceHubClient(t *testing.T) *resiliencehub.Client {
	t.Helper()

	return resiliencehub.NewFromConfig(awsConfig(t), func(o *resiliencehub.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestResilienceHub_CreateAndDeleteApp(t *testing.T) {
	client := newResilienceHubClient(t)
	ctx := t.Context()

	// Create app
	createOutput, err := client.CreateApp(ctx, &resiliencehub.CreateAppInput{
		Name:        aws.String("test-app"),
		Description: aws.String("Test application for Resilience Hub"),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "AppArn", "CreationTime"),
	)
	g.Assert("create", createOutput)

	// Describe app
	describeOutput, err := client.DescribeApp(ctx, &resiliencehub.DescribeAppInput{
		AppArn: createOutput.App.AppArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	g2 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "AppArn", "CreationTime"),
	)
	g2.Assert("describe", describeOutput)

	// Delete app
	deleteOutput, err := client.DeleteApp(ctx, &resiliencehub.DeleteAppInput{
		AppArn: createOutput.App.AppArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	g3 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "AppArn"),
	)
	g3.Assert("delete", deleteOutput)
}

func TestResilienceHub_ListApps(t *testing.T) {
	client := newResilienceHubClient(t)
	ctx := t.Context()

	// Create an app first
	createOutput, err := client.CreateApp(ctx, &resiliencehub.CreateAppInput{
		Name:        aws.String("test-list-app"),
		Description: aws.String("Test list application"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteApp(ctx, &resiliencehub.DeleteAppInput{
			AppArn: createOutput.App.AppArn,
		})
	})

	// List apps
	listOutput, err := client.ListApps(ctx, &resiliencehub.ListAppsInput{})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "AppSummaries"),
	)
	g.Assert("list", listOutput)
}

func TestResilienceHub_CreateAndDeleteResiliencyPolicy(t *testing.T) {
	client := newResilienceHubClient(t)
	ctx := t.Context()

	// Create policy
	createOutput, err := client.CreateResiliencyPolicy(ctx, &resiliencehub.CreateResiliencyPolicyInput{
		PolicyName: aws.String("test-policy"),
		Tier:       types.ResiliencyPolicyTierCoreServices,
		Policy: map[string]types.FailurePolicy{
			"Software": {
				RpoInSecs: 3600,
				RtoInSecs: 3600,
			},
			"Hardware": {
				RpoInSecs: 86400,
				RtoInSecs: 86400,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "PolicyArn", "CreationTime"),
	)
	g.Assert("create", createOutput)

	// Describe policy
	describeOutput, err := client.DescribeResiliencyPolicy(ctx, &resiliencehub.DescribeResiliencyPolicyInput{
		PolicyArn: createOutput.Policy.PolicyArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	g2 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "PolicyArn", "CreationTime"),
	)
	g2.Assert("describe", describeOutput)

	// Delete policy
	deleteOutput, err := client.DeleteResiliencyPolicy(ctx, &resiliencehub.DeleteResiliencyPolicyInput{
		PolicyArn: createOutput.Policy.PolicyArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	g3 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "PolicyArn"),
	)
	g3.Assert("delete", deleteOutput)
}

func TestResilienceHub_ListResiliencyPolicies(t *testing.T) {
	client := newResilienceHubClient(t)
	ctx := t.Context()

	// Create a policy first
	createOutput, err := client.CreateResiliencyPolicy(ctx, &resiliencehub.CreateResiliencyPolicyInput{
		PolicyName: aws.String("test-list-policy"),
		Tier:       types.ResiliencyPolicyTierNonCritical,
		Policy: map[string]types.FailurePolicy{
			"Software": {
				RpoInSecs: 86400,
				RtoInSecs: 86400,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteResiliencyPolicy(ctx, &resiliencehub.DeleteResiliencyPolicyInput{
			PolicyArn: createOutput.Policy.PolicyArn,
		})
	})

	// List policies
	listOutput, err := client.ListResiliencyPolicies(ctx, &resiliencehub.ListResiliencyPoliciesInput{})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "ResiliencyPolicies"),
	)
	g.Assert("list", listOutput)
}

func TestResilienceHub_AppAssessment(t *testing.T) {
	client := newResilienceHubClient(t)
	ctx := t.Context()

	// Create an app first
	appOutput, err := client.CreateApp(ctx, &resiliencehub.CreateAppInput{
		Name:        aws.String("test-assessment-app"),
		Description: aws.String("Test app for assessment"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteApp(ctx, &resiliencehub.DeleteAppInput{
			AppArn: appOutput.App.AppArn,
		})
	})

	// Start assessment
	assessmentOutput, err := client.StartAppAssessment(ctx, &resiliencehub.StartAppAssessmentInput{
		AppArn:         appOutput.App.AppArn,
		AppVersion:     aws.String("release"),
		AssessmentName: aws.String("test-assessment"),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "AssessmentArn", "AppArn", "StartTime", "EndTime"),
	)
	g.Assert("start", assessmentOutput)

	// Describe assessment
	describeOutput, err := client.DescribeAppAssessment(ctx, &resiliencehub.DescribeAppAssessmentInput{
		AssessmentArn: assessmentOutput.Assessment.AssessmentArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	g2 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "AssessmentArn", "AppArn", "StartTime", "EndTime"),
	)
	g2.Assert("describe", describeOutput)

	// Delete assessment
	deleteOutput, err := client.DeleteAppAssessment(ctx, &resiliencehub.DeleteAppAssessmentInput{
		AssessmentArn: assessmentOutput.Assessment.AssessmentArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	g3 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "AssessmentArn"),
	)
	g3.Assert("delete", deleteOutput)
}
