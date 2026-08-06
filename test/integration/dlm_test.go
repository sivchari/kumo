//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dlm"
	"github.com/aws/aws-sdk-go-v2/service/dlm/types"
	"github.com/sivchari/golden"
)

func newDLMClient(t *testing.T) *dlm.Client {
	t.Helper()

	return dlm.NewFromConfig(awsConfig(t), func(o *dlm.Options) {
		o.BaseEndpoint = aws.String(testEndpoint() + "/dlm")
	})
}

func TestDLM_CreateAndDeleteLifecyclePolicy(t *testing.T) {
	client := newDLMClient(t)
	ctx := t.Context()

	// Create lifecycle policy.
	createOutput, err := client.CreateLifecyclePolicy(ctx, &dlm.CreateLifecyclePolicyInput{
		Description:      aws.String("Test policy for EBS snapshots"),
		ExecutionRoleArn: aws.String("arn:aws:iam::123456789012:role/dlm-role"),
		State:            types.SettablePolicyStateValuesEnabled,
		PolicyDetails: &types.PolicyDetails{
			ResourceTypes: []types.ResourceTypeValues{types.ResourceTypeValuesVolume},
			TargetTags: []types.Tag{
				{
					Key:   aws.String("Backup"),
					Value: aws.String("true"),
				},
			},
			Schedules: []types.Schedule{
				{
					Name: aws.String("Daily snapshots"),
					CreateRule: &types.CreateRule{
						Interval:     aws.Int32(24),
						IntervalUnit: types.IntervalUnitValuesHours,
						Times:        []string{"03:00"},
					},
					RetainRule: &types.RetainRule{
						Count: aws.Int32(7),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	policyID := *createOutput.PolicyId

	t.Cleanup(func() {
		_, _ = client.DeleteLifecyclePolicy(context.Background(), &dlm.DeleteLifecyclePolicyInput{
			PolicyId: aws.String(policyID),
		})
	})

	// Get lifecycle policy.
	getOutput, err := client.GetLifecyclePolicy(ctx, &dlm.GetLifecyclePolicyInput{
		PolicyId: aws.String(policyID),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("PolicyArn", "PolicyId", "DateCreated", "DateModified")).Assert(t.Name()+"_get", getOutput)

	// Delete lifecycle policy.
	_, err = client.DeleteLifecyclePolicy(context.Background(), &dlm.DeleteLifecyclePolicyInput{
		PolicyId: aws.String(policyID),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify policy is deleted.
	_, err = client.GetLifecyclePolicy(ctx, &dlm.GetLifecyclePolicyInput{
		PolicyId: aws.String(policyID),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestDLM_UpdateLifecyclePolicy(t *testing.T) {
	client := newDLMClient(t)
	ctx := t.Context()

	// Create lifecycle policy.
	createOutput, err := client.CreateLifecyclePolicy(ctx, &dlm.CreateLifecyclePolicyInput{
		Description:      aws.String("Test policy"),
		ExecutionRoleArn: aws.String("arn:aws:iam::123456789012:role/dlm-role"),
		State:            types.SettablePolicyStateValuesEnabled,
		PolicyDetails: &types.PolicyDetails{
			ResourceTypes: []types.ResourceTypeValues{types.ResourceTypeValuesVolume},
			TargetTags: []types.Tag{
				{
					Key:   aws.String("Backup"),
					Value: aws.String("true"),
				},
			},
			Schedules: []types.Schedule{
				{
					Name: aws.String("Daily snapshots"),
					CreateRule: &types.CreateRule{
						Interval:     aws.Int32(24),
						IntervalUnit: types.IntervalUnitValuesHours,
					},
					RetainRule: &types.RetainRule{
						Count: aws.Int32(7),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	policyID := *createOutput.PolicyId

	t.Cleanup(func() {
		_, _ = client.DeleteLifecyclePolicy(context.Background(), &dlm.DeleteLifecyclePolicyInput{
			PolicyId: aws.String(policyID),
		})
	})

	// Update lifecycle policy.
	_, err = client.UpdateLifecyclePolicy(ctx, &dlm.UpdateLifecyclePolicyInput{
		PolicyId:    aws.String(policyID),
		Description: aws.String("Updated test policy"),
		State:       types.SettablePolicyStateValuesDisabled,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify update.
	getOutput, err := client.GetLifecyclePolicy(ctx, &dlm.GetLifecyclePolicyInput{
		PolicyId: aws.String(policyID),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("PolicyArn", "PolicyId", "DateCreated", "DateModified")).Assert(t.Name(), getOutput)
}

func TestDLM_GetLifecyclePolicies(t *testing.T) {
	client := newDLMClient(t)
	ctx := t.Context()

	// Create multiple lifecycle policies.
	var policyIDs []string

	for i := 0; i < 2; i++ {
		createOutput, err := client.CreateLifecyclePolicy(ctx, &dlm.CreateLifecyclePolicyInput{
			Description:      aws.String("Test policy for listing"),
			ExecutionRoleArn: aws.String("arn:aws:iam::123456789012:role/dlm-role"),
			State:            types.SettablePolicyStateValuesEnabled,
			PolicyDetails: &types.PolicyDetails{
				ResourceTypes: []types.ResourceTypeValues{types.ResourceTypeValuesVolume},
				TargetTags: []types.Tag{
					{
						Key:   aws.String("Backup"),
						Value: aws.String("true"),
					},
				},
				Schedules: []types.Schedule{
					{
						Name: aws.String("Daily snapshots"),
						CreateRule: &types.CreateRule{
							Interval:     aws.Int32(24),
							IntervalUnit: types.IntervalUnitValuesHours,
						},
						RetainRule: &types.RetainRule{
							Count: aws.Int32(7),
						},
					},
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		policyIDs = append(policyIDs, *createOutput.PolicyId)
	}

	t.Cleanup(func() {
		for _, id := range policyIDs {
			_, _ = client.DeleteLifecyclePolicy(context.Background(), &dlm.DeleteLifecyclePolicyInput{
				PolicyId: aws.String(id),
			})
		}
	})

	// Get lifecycle policies.
	listOutput, err := client.GetLifecyclePolicies(ctx, &dlm.GetLifecyclePoliciesInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listOutput.Policies) < 2 {
		t.Errorf("expected at least 2 policies, got %d", len(listOutput.Policies))
	}
}

func TestDLM_PolicyNotFound(t *testing.T) {
	client := newDLMClient(t)
	ctx := t.Context()

	// Get non-existent policy.
	_, err := client.GetLifecyclePolicy(ctx, &dlm.GetLifecyclePolicyInput{
		PolicyId: aws.String("policy-non-existent"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Delete non-existent policy.
	_, err = client.DeleteLifecyclePolicy(context.Background(), &dlm.DeleteLifecyclePolicyInput{
		PolicyId: aws.String("policy-non-existent"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Update non-existent policy.
	_, err = client.UpdateLifecyclePolicy(ctx, &dlm.UpdateLifecyclePolicyInput{
		PolicyId:    aws.String("policy-non-existent"),
		Description: aws.String("Updated description"),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestDLM_FilterPoliciesByState(t *testing.T) {
	client := newDLMClient(t)
	ctx := t.Context()

	// Create an enabled policy.
	createOutput, err := client.CreateLifecyclePolicy(ctx, &dlm.CreateLifecyclePolicyInput{
		Description:      aws.String("Enabled policy"),
		ExecutionRoleArn: aws.String("arn:aws:iam::123456789012:role/dlm-role"),
		State:            types.SettablePolicyStateValuesEnabled,
		PolicyDetails: &types.PolicyDetails{
			ResourceTypes: []types.ResourceTypeValues{types.ResourceTypeValuesVolume},
			TargetTags: []types.Tag{
				{
					Key:   aws.String("Backup"),
					Value: aws.String("true"),
				},
			},
			Schedules: []types.Schedule{
				{
					Name: aws.String("Daily"),
					CreateRule: &types.CreateRule{
						Interval:     aws.Int32(24),
						IntervalUnit: types.IntervalUnitValuesHours,
					},
					RetainRule: &types.RetainRule{
						Count: aws.Int32(7),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	policyID := *createOutput.PolicyId

	t.Cleanup(func() {
		_, _ = client.DeleteLifecyclePolicy(context.Background(), &dlm.DeleteLifecyclePolicyInput{
			PolicyId: aws.String(policyID),
		})
	})

	// Filter by state ENABLED.
	listOutput, err := client.GetLifecyclePolicies(ctx, &dlm.GetLifecyclePoliciesInput{
		State: types.GettablePolicyStateValuesEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(listOutput.Policies) < 1 {
		t.Errorf("expected at least 1 policy, got %d", len(listOutput.Policies))
	}

	// Verify all returned policies are enabled.
	for _, policy := range listOutput.Policies {
		if policy.State != types.GettablePolicyStateValuesEnabled {
			t.Errorf("expected state ENABLED, got %v", policy.State)
		}
	}
}
