//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/scheduler"
	"github.com/aws/aws-sdk-go-v2/service/scheduler/types"
	"github.com/sivchari/golden"
)

func newSchedulerClient(t *testing.T) *scheduler.Client {
	t.Helper()

	return scheduler.NewFromConfig(awsConfig(t), func(o *scheduler.Options) {
		o.BaseEndpoint = aws.String(testEndpoint() + "/scheduler")
	})
}

func TestScheduler_CreateAndDeleteSchedule(t *testing.T) {
	client := newSchedulerClient(t)
	ctx := t.Context()

	scheduleName := "test-schedule"

	// Create schedule.
	createOutput, err := client.CreateSchedule(ctx, &scheduler.CreateScheduleInput{
		Name:               aws.String(scheduleName),
		ScheduleExpression: aws.String("rate(1 hour)"),
		FlexibleTimeWindow: &types.FlexibleTimeWindow{
			Mode: types.FlexibleTimeWindowModeOff,
		},
		Target: &types.Target{
			Arn:     aws.String("arn:aws:sqs:us-east-1:123456789012:my-queue"),
			RoleArn: aws.String("arn:aws:iam::123456789012:role/scheduler-role"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ScheduleArn", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	t.Cleanup(func() {
		_, _ = client.DeleteSchedule(context.Background(), &scheduler.DeleteScheduleInput{
			Name: aws.String(scheduleName),
		})
	})

	// Get schedule.
	getOutput, err := client.GetSchedule(ctx, &scheduler.GetScheduleInput{
		Name: aws.String(scheduleName),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Arn", "CreationDate", "LastModificationDate", "ResultMetadata")).Assert(t.Name()+"_get", getOutput)

	// Delete schedule.
	_, err = client.DeleteSchedule(context.Background(), &scheduler.DeleteScheduleInput{
		Name: aws.String(scheduleName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify schedule is deleted.
	_, err = client.GetSchedule(ctx, &scheduler.GetScheduleInput{
		Name: aws.String(scheduleName),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestScheduler_UpdateSchedule(t *testing.T) {
	client := newSchedulerClient(t)
	ctx := t.Context()

	scheduleName := "test-schedule-update"

	// Create schedule.
	_, err := client.CreateSchedule(ctx, &scheduler.CreateScheduleInput{
		Name:               aws.String(scheduleName),
		ScheduleExpression: aws.String("rate(1 hour)"),
		FlexibleTimeWindow: &types.FlexibleTimeWindow{
			Mode: types.FlexibleTimeWindowModeOff,
		},
		Target: &types.Target{
			Arn:     aws.String("arn:aws:sqs:us-east-1:123456789012:my-queue"),
			RoleArn: aws.String("arn:aws:iam::123456789012:role/scheduler-role"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteSchedule(context.Background(), &scheduler.DeleteScheduleInput{
			Name: aws.String(scheduleName),
		})
	})

	// Update schedule.
	_, err = client.UpdateSchedule(ctx, &scheduler.UpdateScheduleInput{
		Name:               aws.String(scheduleName),
		ScheduleExpression: aws.String("rate(2 hours)"),
		FlexibleTimeWindow: &types.FlexibleTimeWindow{
			Mode: types.FlexibleTimeWindowModeOff,
		},
		Target: &types.Target{
			Arn:     aws.String("arn:aws:sqs:us-east-1:123456789012:my-queue-updated"),
			RoleArn: aws.String("arn:aws:iam::123456789012:role/scheduler-role"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify update.
	getOutput, err := client.GetSchedule(ctx, &scheduler.GetScheduleInput{
		Name: aws.String(scheduleName),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Arn", "CreationDate", "LastModificationDate", "ResultMetadata")).Assert(t.Name(), getOutput)
}

func TestScheduler_ListSchedules(t *testing.T) {
	client := newSchedulerClient(t)
	ctx := t.Context()

	// Create multiple schedules.
	scheduleNames := []string{"test-list-schedule-1", "test-list-schedule-2"}

	for _, name := range scheduleNames {
		_, err := client.CreateSchedule(ctx, &scheduler.CreateScheduleInput{
			Name:               aws.String(name),
			ScheduleExpression: aws.String("rate(1 hour)"),
			FlexibleTimeWindow: &types.FlexibleTimeWindow{
				Mode: types.FlexibleTimeWindowModeOff,
			},
			Target: &types.Target{
				Arn:     aws.String("arn:aws:sqs:us-east-1:123456789012:my-queue"),
				RoleArn: aws.String("arn:aws:iam::123456789012:role/scheduler-role"),
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Cleanup(func() {
		for _, name := range scheduleNames {
			_, _ = client.DeleteSchedule(context.Background(), &scheduler.DeleteScheduleInput{
				Name: aws.String(name),
			})
		}
	})

	// List schedules.
	listOutput, err := client.ListSchedules(ctx, &scheduler.ListSchedulesInput{})
	if err != nil {
		t.Fatal(err)
	}

	if len(listOutput.Schedules) < 2 {
		t.Errorf("expected at least 2 schedules, got %d", len(listOutput.Schedules))
	}
}

func TestScheduler_CreateAndDeleteScheduleGroup(t *testing.T) {
	client := newSchedulerClient(t)
	ctx := t.Context()

	groupName := "test-schedule-group"

	// Create schedule group.
	createOutput, err := client.CreateScheduleGroup(ctx, &scheduler.CreateScheduleGroupInput{
		Name: aws.String(groupName),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ScheduleGroupArn", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	t.Cleanup(func() {
		_, _ = client.DeleteScheduleGroup(context.Background(), &scheduler.DeleteScheduleGroupInput{
			Name: aws.String(groupName),
		})
	})

	// Get schedule group.
	getOutput, err := client.GetScheduleGroup(ctx, &scheduler.GetScheduleGroupInput{
		Name: aws.String(groupName),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Arn", "CreationDate", "LastModificationDate", "ResultMetadata")).Assert(t.Name()+"_get", getOutput)

	// Delete schedule group.
	_, err = client.DeleteScheduleGroup(context.Background(), &scheduler.DeleteScheduleGroupInput{
		Name: aws.String(groupName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify schedule group is deleted.
	_, err = client.GetScheduleGroup(ctx, &scheduler.GetScheduleGroupInput{
		Name: aws.String(groupName),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestScheduler_ListScheduleGroups(t *testing.T) {
	client := newSchedulerClient(t)
	ctx := t.Context()

	// List schedule groups (should include default group).
	listOutput, err := client.ListScheduleGroups(ctx, &scheduler.ListScheduleGroupsInput{})
	if err != nil {
		t.Fatal(err)
	}

	if len(listOutput.ScheduleGroups) < 1 {
		t.Errorf("expected at least 1 schedule group, got %d", len(listOutput.ScheduleGroups))
	}

	// Check that default group exists.
	found := false

	for _, group := range listOutput.ScheduleGroups {
		if *group.Name == "default" {
			found = true

			break
		}
	}

	if !found {
		t.Error("default schedule group should exist")
	}
}

func TestScheduler_ScheduleWithGroup(t *testing.T) {
	client := newSchedulerClient(t)
	ctx := t.Context()

	groupName := "test-schedule-group-with-schedule"
	scheduleName := "test-schedule-in-group"

	// Create schedule group.
	_, err := client.CreateScheduleGroup(ctx, &scheduler.CreateScheduleGroupInput{
		Name: aws.String(groupName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteSchedule(context.Background(), &scheduler.DeleteScheduleInput{
			Name:      aws.String(scheduleName),
			GroupName: aws.String(groupName),
		})
		_, _ = client.DeleteScheduleGroup(context.Background(), &scheduler.DeleteScheduleGroupInput{
			Name: aws.String(groupName),
		})
	})

	// Create schedule in the group.
	_, err = client.CreateSchedule(ctx, &scheduler.CreateScheduleInput{
		Name:               aws.String(scheduleName),
		GroupName:          aws.String(groupName),
		ScheduleExpression: aws.String("rate(1 hour)"),
		FlexibleTimeWindow: &types.FlexibleTimeWindow{
			Mode: types.FlexibleTimeWindowModeOff,
		},
		Target: &types.Target{
			Arn:     aws.String("arn:aws:sqs:us-east-1:123456789012:my-queue"),
			RoleArn: aws.String("arn:aws:iam::123456789012:role/scheduler-role"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get schedule with group name.
	getOutput, err := client.GetSchedule(ctx, &scheduler.GetScheduleInput{
		Name:      aws.String(scheduleName),
		GroupName: aws.String(groupName),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Arn", "CreationDate", "LastModificationDate", "ResultMetadata")).Assert(t.Name(), getOutput)
}

func TestScheduler_ScheduleNotFound(t *testing.T) {
	client := newSchedulerClient(t)
	ctx := t.Context()

	// Get non-existent schedule.
	_, err := client.GetSchedule(ctx, &scheduler.GetScheduleInput{
		Name: aws.String("non-existent-schedule"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Delete non-existent schedule.
	_, err = client.DeleteSchedule(context.Background(), &scheduler.DeleteScheduleInput{
		Name: aws.String("non-existent-schedule"),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestScheduler_ScheduleGroupNotFound(t *testing.T) {
	client := newSchedulerClient(t)
	ctx := t.Context()

	// Get non-existent schedule group.
	_, err := client.GetScheduleGroup(ctx, &scheduler.GetScheduleGroupInput{
		Name: aws.String("non-existent-group"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Delete non-existent schedule group.
	_, err = client.DeleteScheduleGroup(context.Background(), &scheduler.DeleteScheduleGroupInput{
		Name: aws.String("non-existent-group"),
	})
	if err == nil {
		t.Error("expected error")
	}
}
