//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/configservice"
	"github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/sivchari/golden"
)

func newConfigServiceClient(t *testing.T) *configservice.Client {
	t.Helper()

	return configservice.NewFromConfig(awsConfig(t), func(o *configservice.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

// cleanupExistingRecorders removes any existing configuration recorders.
// AWS Config only allows one recorder per region.
func cleanupExistingRecorders(t *testing.T, client *configservice.Client) {
	t.Helper()
	ctx := t.Context()

	resp, err := client.DescribeConfigurationRecorders(ctx, &configservice.DescribeConfigurationRecordersInput{})
	if err != nil {
		return
	}

	for _, recorder := range resp.ConfigurationRecorders {
		if recorder.Name != nil {
			_, _ = client.DeleteConfigurationRecorder(context.Background(), &configservice.DeleteConfigurationRecorderInput{
				ConfigurationRecorderName: recorder.Name,
			})
		}
	}
}

func TestConfigService_PutAndDeleteConfigurationRecorder(t *testing.T) {
	client := newConfigServiceClient(t)
	ctx := t.Context()

	// Clean up any existing recorders first.
	cleanupExistingRecorders(t, client)

	recorderName := "test-recorder-create-delete"
	roleARN := "arn:aws:iam::123456789012:role/config-role"

	// Put configuration recorder.
	_, err := client.PutConfigurationRecorder(ctx, &configservice.PutConfigurationRecorderInput{
		ConfigurationRecorder: &types.ConfigurationRecorder{
			Name:    aws.String(recorderName),
			RoleARN: aws.String(roleARN),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteConfigurationRecorder(context.Background(), &configservice.DeleteConfigurationRecorderInput{
			ConfigurationRecorderName: aws.String(recorderName),
		})
	})

	// Verify recorder was created.
	descOutput, err := client.DescribeConfigurationRecorders(ctx, &configservice.DescribeConfigurationRecordersInput{})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ConfigRuleArn", "ConfigRuleId", "ResultMetadata")).Assert(t.Name()+"/created", descOutput)

	// Delete recorder.
	_, err = client.DeleteConfigurationRecorder(context.Background(), &configservice.DeleteConfigurationRecorderInput{
		ConfigurationRecorderName: aws.String(recorderName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify recorder is deleted.
	descOutput, err = client.DescribeConfigurationRecorders(ctx, &configservice.DescribeConfigurationRecordersInput{})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ConfigRuleArn", "ConfigRuleId", "ResultMetadata")).Assert(t.Name()+"/deleted", descOutput)
}

func TestConfigService_DescribeConfigurationRecorders(t *testing.T) {
	client := newConfigServiceClient(t)
	ctx := t.Context()

	// Clean up any existing recorders first.
	cleanupExistingRecorders(t, client)

	recorderName := "test-recorder-describe"
	roleARN := "arn:aws:iam::123456789012:role/config-role"

	// Put configuration recorder.
	_, err := client.PutConfigurationRecorder(ctx, &configservice.PutConfigurationRecorderInput{
		ConfigurationRecorder: &types.ConfigurationRecorder{
			Name:    aws.String(recorderName),
			RoleARN: aws.String(roleARN),
			RecordingGroup: &types.RecordingGroup{
				AllSupported: true,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteConfigurationRecorder(context.Background(), &configservice.DeleteConfigurationRecorderInput{
			ConfigurationRecorderName: aws.String(recorderName),
		})
	})

	// Describe all recorders.
	descOutput, err := client.DescribeConfigurationRecorders(ctx, &configservice.DescribeConfigurationRecordersInput{})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ConfigRuleArn", "ConfigRuleId", "ResultMetadata")).Assert(t.Name()+"/all", descOutput)

	// Describe specific recorder.
	descOutput, err = client.DescribeConfigurationRecorders(ctx, &configservice.DescribeConfigurationRecordersInput{
		ConfigurationRecorderNames: []string{recorderName},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ConfigRuleArn", "ConfigRuleId", "ResultMetadata")).Assert(t.Name()+"/specific", descOutput)
}

func TestConfigService_StartAndStopConfigurationRecorder(t *testing.T) {
	client := newConfigServiceClient(t)
	ctx := t.Context()

	// Clean up any existing recorders first.
	cleanupExistingRecorders(t, client)

	recorderName := "test-recorder-start-stop"
	roleARN := "arn:aws:iam::123456789012:role/config-role"

	// Put configuration recorder.
	_, err := client.PutConfigurationRecorder(ctx, &configservice.PutConfigurationRecorderInput{
		ConfigurationRecorder: &types.ConfigurationRecorder{
			Name:    aws.String(recorderName),
			RoleARN: aws.String(roleARN),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteConfigurationRecorder(context.Background(), &configservice.DeleteConfigurationRecorderInput{
			ConfigurationRecorderName: aws.String(recorderName),
		})
	})

	// Start recording.
	_, err = client.StartConfigurationRecorder(ctx, &configservice.StartConfigurationRecorderInput{
		ConfigurationRecorderName: aws.String(recorderName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Stop recording.
	_, err = client.StopConfigurationRecorder(ctx, &configservice.StopConfigurationRecorderInput{
		ConfigurationRecorderName: aws.String(recorderName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestConfigService_PutAndDeleteConfigRule(t *testing.T) {
	client := newConfigServiceClient(t)
	ctx := t.Context()

	ruleName := "test-rule-create-delete"

	// Put config rule.
	_, err := client.PutConfigRule(ctx, &configservice.PutConfigRuleInput{
		ConfigRule: &types.ConfigRule{
			ConfigRuleName: aws.String(ruleName),
			Source: &types.Source{
				Owner:            types.OwnerAws,
				SourceIdentifier: aws.String("S3_BUCKET_VERSIONING_ENABLED"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteConfigRule(context.Background(), &configservice.DeleteConfigRuleInput{
			ConfigRuleName: aws.String(ruleName),
		})
	})

	// Verify rule was created.
	descOutput, err := client.DescribeConfigRules(ctx, &configservice.DescribeConfigRulesInput{
		ConfigRuleNames: []string{ruleName},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ConfigRuleArn", "ConfigRuleId", "ResultMetadata")).Assert(t.Name()+"/created", descOutput)

	// Delete rule.
	_, err = client.DeleteConfigRule(ctx, &configservice.DeleteConfigRuleInput{
		ConfigRuleName: aws.String(ruleName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify rule is deleted.
	descOutput, err = client.DescribeConfigRules(ctx, &configservice.DescribeConfigRulesInput{
		ConfigRuleNames: []string{ruleName},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ConfigRuleArn", "ConfigRuleId", "ResultMetadata")).Assert(t.Name()+"/deleted", descOutput)
}

func TestConfigService_DescribeConfigRules(t *testing.T) {
	client := newConfigServiceClient(t)
	ctx := t.Context()

	ruleName := "test-rule-describe"

	// Put config rule.
	_, err := client.PutConfigRule(ctx, &configservice.PutConfigRuleInput{
		ConfigRule: &types.ConfigRule{
			ConfigRuleName: aws.String(ruleName),
			Description:    aws.String("Test rule for describe"),
			Source: &types.Source{
				Owner:            types.OwnerAws,
				SourceIdentifier: aws.String("S3_BUCKET_VERSIONING_ENABLED"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteConfigRule(context.Background(), &configservice.DeleteConfigRuleInput{
			ConfigRuleName: aws.String(ruleName),
		})
	})

	// Describe all rules.
	descOutput, err := client.DescribeConfigRules(ctx, &configservice.DescribeConfigRulesInput{})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ConfigRules", "ConfigRuleArn", "ConfigRuleId", "ResultMetadata")).Assert(t.Name()+"/all", descOutput)

	// Describe specific rule.
	descOutput, err = client.DescribeConfigRules(ctx, &configservice.DescribeConfigRulesInput{
		ConfigRuleNames: []string{ruleName},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ConfigRuleArn", "ConfigRuleId", "ResultMetadata")).Assert(t.Name()+"/specific", descOutput)
}

func TestConfigService_GetComplianceDetailsByConfigRule(t *testing.T) {
	client := newConfigServiceClient(t)
	ctx := t.Context()

	ruleName := "test-rule-compliance"

	// Put config rule.
	_, err := client.PutConfigRule(ctx, &configservice.PutConfigRuleInput{
		ConfigRule: &types.ConfigRule{
			ConfigRuleName: aws.String(ruleName),
			Source: &types.Source{
				Owner:            types.OwnerAws,
				SourceIdentifier: aws.String("S3_BUCKET_VERSIONING_ENABLED"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteConfigRule(context.Background(), &configservice.DeleteConfigRuleInput{
			ConfigRuleName: aws.String(ruleName),
		})
	})

	// GetComplianceDetailsByConfigRule returns empty list for MVP.
	output, err := client.GetComplianceDetailsByConfigRule(ctx, &configservice.GetComplianceDetailsByConfigRuleInput{
		ConfigRuleName: aws.String(ruleName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ConfigRuleArn", "ConfigRuleId", "ResultMetadata")).Assert(t.Name(), output)
}

func TestConfigService_ConfigurationRecorderNotFound(t *testing.T) {
	client := newConfigServiceClient(t)
	ctx := t.Context()

	// Delete non-existent recorder.
	_, err := client.DeleteConfigurationRecorder(context.Background(), &configservice.DeleteConfigurationRecorderInput{
		ConfigurationRecorderName: aws.String("non-existent-recorder"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Start non-existent recorder.
	_, err = client.StartConfigurationRecorder(ctx, &configservice.StartConfigurationRecorderInput{
		ConfigurationRecorderName: aws.String("non-existent-recorder"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Stop non-existent recorder.
	_, err = client.StopConfigurationRecorder(ctx, &configservice.StopConfigurationRecorderInput{
		ConfigurationRecorderName: aws.String("non-existent-recorder"),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestConfigService_ConfigRuleNotFound(t *testing.T) {
	client := newConfigServiceClient(t)
	ctx := t.Context()

	// Delete non-existent rule.
	_, err := client.DeleteConfigRule(ctx, &configservice.DeleteConfigRuleInput{
		ConfigRuleName: aws.String("non-existent-rule"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Get compliance for non-existent rule.
	_, err = client.GetComplianceDetailsByConfigRule(ctx, &configservice.GetComplianceDetailsByConfigRuleInput{
		ConfigRuleName: aws.String("non-existent-rule"),
	})
	if err == nil {
		t.Error("expected error")
	}
}
