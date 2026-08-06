//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/mq"
	"github.com/aws/aws-sdk-go-v2/service/mq/types"
	"github.com/sivchari/golden"
)

func newMQClient(t *testing.T) *mq.Client {
	t.Helper()

	return mq.NewFromConfig(awsConfig(t), func(o *mq.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestMQ_CreateAndDeleteBroker(t *testing.T) {
	client := newMQClient(t)
	ctx := t.Context()
	brokerName := "test-broker-create-delete"

	// Create broker.
	createOutput, err := client.CreateBroker(ctx, &mq.CreateBrokerInput{
		BrokerName:       aws.String(brokerName),
		EngineType:       types.EngineTypeActivemq,
		EngineVersion:    aws.String("5.17.6"),
		HostInstanceType: aws.String("mq.t3.micro"),
		DeploymentMode:   types.DeploymentModeSingleInstance,
		Users: []types.User{
			{
				Username: aws.String("admin"),
				Password: aws.String("admin12345"),
			},
		},
		PubliclyAccessible: aws.Bool(false),
	})
	if err != nil {
		t.Fatal(err)
	}

	brokerID := createOutput.BrokerId

	t.Cleanup(func() {
		_, _ = client.DeleteBroker(context.Background(), &mq.DeleteBrokerInput{
			BrokerId: brokerID,
		})
	})

	golden.New(t, golden.WithIgnoreFields("BrokerArn", "BrokerId", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	// Delete broker.
	deleteOutput, err := client.DeleteBroker(ctx, &mq.DeleteBrokerInput{
		BrokerId: brokerID,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("BrokerId", "ResultMetadata")).Assert(t.Name()+"_delete", deleteOutput)
}

func TestMQ_DescribeBroker(t *testing.T) {
	client := newMQClient(t)
	ctx := t.Context()
	brokerName := "test-broker-describe"

	// Create broker.
	createOutput, err := client.CreateBroker(ctx, &mq.CreateBrokerInput{
		BrokerName:       aws.String(brokerName),
		EngineType:       types.EngineTypeActivemq,
		EngineVersion:    aws.String("5.17.6"),
		HostInstanceType: aws.String("mq.t3.micro"),
		DeploymentMode:   types.DeploymentModeSingleInstance,
		Users: []types.User{
			{
				Username: aws.String("admin"),
				Password: aws.String("admin12345"),
			},
		},
		PubliclyAccessible: aws.Bool(false),
	})
	if err != nil {
		t.Fatal(err)
	}

	brokerID := createOutput.BrokerId

	t.Cleanup(func() {
		_, _ = client.DeleteBroker(context.Background(), &mq.DeleteBrokerInput{
			BrokerId: brokerID,
		})
	})

	// Describe broker.
	describeOutput, err := client.DescribeBroker(ctx, &mq.DescribeBrokerInput{
		BrokerId: brokerID,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"BrokerArn", "BrokerId", "Created", "ResultMetadata",
	)).Assert(t.Name(), describeOutput)
}

func TestMQ_ListBrokers(t *testing.T) {
	client := newMQClient(t)
	ctx := t.Context()
	brokerName := "test-broker-list"

	// Create broker.
	createOutput, err := client.CreateBroker(ctx, &mq.CreateBrokerInput{
		BrokerName:       aws.String(brokerName),
		EngineType:       types.EngineTypeActivemq,
		EngineVersion:    aws.String("5.17.6"),
		HostInstanceType: aws.String("mq.t3.micro"),
		DeploymentMode:   types.DeploymentModeSingleInstance,
		Users: []types.User{
			{
				Username: aws.String("admin"),
				Password: aws.String("admin12345"),
			},
		},
		PubliclyAccessible: aws.Bool(false),
	})
	if err != nil {
		t.Fatal(err)
	}

	brokerID := createOutput.BrokerId

	t.Cleanup(func() {
		_, _ = client.DeleteBroker(context.Background(), &mq.DeleteBrokerInput{
			BrokerId: brokerID,
		})
	})

	// List brokers.
	listOutput, err := client.ListBrokers(ctx, &mq.ListBrokersInput{})
	if err != nil {
		t.Fatal(err)
	}

	if len(listOutput.BrokerSummaries) == 0 {
		t.Fatal("expected at least one broker")
	}

	// Verify our broker is in the list.
	found := false
	for _, b := range listOutput.BrokerSummaries {
		if *b.BrokerId == *brokerID {
			found = true

			break
		}
	}

	if !found {
		t.Error("expected to find the created broker in the list")
	}
}

func TestMQ_UpdateBroker(t *testing.T) {
	client := newMQClient(t)
	ctx := t.Context()
	brokerName := "test-broker-update"

	// Create broker.
	createOutput, err := client.CreateBroker(ctx, &mq.CreateBrokerInput{
		BrokerName:       aws.String(brokerName),
		EngineType:       types.EngineTypeActivemq,
		EngineVersion:    aws.String("5.17.6"),
		HostInstanceType: aws.String("mq.t3.micro"),
		DeploymentMode:   types.DeploymentModeSingleInstance,
		Users: []types.User{
			{
				Username: aws.String("admin"),
				Password: aws.String("admin12345"),
			},
		},
		PubliclyAccessible:      aws.Bool(false),
		AutoMinorVersionUpgrade: aws.Bool(false),
	})
	if err != nil {
		t.Fatal(err)
	}

	brokerID := createOutput.BrokerId

	t.Cleanup(func() {
		_, _ = client.DeleteBroker(context.Background(), &mq.DeleteBrokerInput{
			BrokerId: brokerID,
		})
	})

	// Update broker.
	updateOutput, err := client.UpdateBroker(ctx, &mq.UpdateBrokerInput{
		BrokerId:                brokerID,
		AutoMinorVersionUpgrade: aws.Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("BrokerId", "ResultMetadata")).Assert(t.Name(), updateOutput)
}

func TestMQ_CreateConfiguration(t *testing.T) {
	client := newMQClient(t)
	ctx := t.Context()
	configName := "test-config"

	// Create configuration.
	createOutput, err := client.CreateConfiguration(ctx, &mq.CreateConfigurationInput{
		Name:          aws.String(configName),
		EngineType:    types.EngineTypeActivemq,
		EngineVersion: aws.String("5.17.6"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Arn", "Id", "Created", "ResultMetadata")).Assert(t.Name(), createOutput)
}

func TestMQ_BrokerNotFound(t *testing.T) {
	client := newMQClient(t)
	ctx := t.Context()

	_, err := client.DescribeBroker(ctx, &mq.DescribeBrokerInput{
		BrokerId: aws.String("nonexistent-broker-id"),
	})
	if err == nil {
		t.Error("expected error for nonexistent broker")
	}
}
