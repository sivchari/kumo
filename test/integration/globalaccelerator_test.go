//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/globalaccelerator"
	"github.com/aws/aws-sdk-go-v2/service/globalaccelerator/types"
	"github.com/sivchari/golden"
)

func newGlobalAcceleratorClient(t *testing.T) *globalaccelerator.Client {
	t.Helper()

	cfg, err := config.LoadDefaultConfig(t.Context(),
		config.WithRegion("us-west-2"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			"test", "test", "",
		)),
	)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	return globalaccelerator.NewFromConfig(cfg, func(o *globalaccelerator.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestGlobalAccelerator_CreateAndDescribeAccelerator(t *testing.T) {
	client := newGlobalAcceleratorClient(t)
	ctx := t.Context()

	// Create accelerator.
	createOutput, err := client.CreateAccelerator(ctx, &globalaccelerator.CreateAcceleratorInput{
		Name:             aws.String("test-accelerator"),
		IdempotencyToken: aws.String("test-token-1"),
		Enabled:          aws.Bool(true),
		IpAddressType:    types.IpAddressTypeIpv4,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("AcceleratorArn", "DnsName", "DualStackDnsName", "CreatedTime", "LastModifiedTime", "IpSets", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	acceleratorArn := *createOutput.Accelerator.AcceleratorArn

	// Describe accelerator.
	describeOutput, err := client.DescribeAccelerator(ctx, &globalaccelerator.DescribeAcceleratorInput{
		AcceleratorArn: aws.String(acceleratorArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("AcceleratorArn", "DnsName", "DualStackDnsName", "CreatedTime", "LastModifiedTime", "IpSets", "ResultMetadata")).Assert(t.Name()+"_describe", describeOutput)
}

func TestGlobalAccelerator_ListAccelerators(t *testing.T) {
	client := newGlobalAcceleratorClient(t)
	ctx := t.Context()

	// Create an accelerator first.
	createOutput, err := client.CreateAccelerator(ctx, &globalaccelerator.CreateAcceleratorInput{
		Name:             aws.String("test-list-accelerator"),
		IdempotencyToken: aws.String("test-token-2"),
	})
	if err != nil {
		t.Fatal(err)
	}

	acceleratorArn := *createOutput.Accelerator.AcceleratorArn

	// List accelerators.
	listOutput, err := client.ListAccelerators(ctx, &globalaccelerator.ListAcceleratorsInput{
		MaxResults: aws.Int32(10),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Find our accelerator.
	found := false
	for _, acc := range listOutput.Accelerators {
		if *acc.AcceleratorArn == acceleratorArn {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("created accelerator %s not found in list", acceleratorArn)
	}
}

func TestGlobalAccelerator_UpdateAccelerator(t *testing.T) {
	client := newGlobalAcceleratorClient(t)
	ctx := t.Context()

	// Create accelerator.
	createOutput, err := client.CreateAccelerator(ctx, &globalaccelerator.CreateAcceleratorInput{
		Name:             aws.String("test-update-accelerator"),
		IdempotencyToken: aws.String("test-token-3"),
		Enabled:          aws.Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}

	acceleratorArn := *createOutput.Accelerator.AcceleratorArn

	// Update accelerator.
	updateOutput, err := client.UpdateAccelerator(ctx, &globalaccelerator.UpdateAcceleratorInput{
		AcceleratorArn: aws.String(acceleratorArn),
		Name:           aws.String("updated-accelerator"),
		Enabled:        aws.Bool(false),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("AcceleratorArn", "DnsName", "DualStackDnsName", "CreatedTime", "LastModifiedTime", "IpSets", "ResultMetadata")).Assert(t.Name(), updateOutput)
}

func TestGlobalAccelerator_DeleteAccelerator(t *testing.T) {
	client := newGlobalAcceleratorClient(t)
	ctx := t.Context()

	// Create accelerator.
	createOutput, err := client.CreateAccelerator(ctx, &globalaccelerator.CreateAcceleratorInput{
		Name:             aws.String("test-delete-accelerator"),
		IdempotencyToken: aws.String("test-token-4"),
		Enabled:          aws.Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}

	acceleratorArn := *createOutput.Accelerator.AcceleratorArn

	// Must disable before deletion.
	_, err = client.UpdateAccelerator(ctx, &globalaccelerator.UpdateAcceleratorInput{
		AcceleratorArn: aws.String(acceleratorArn),
		Enabled:        aws.Bool(false),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete accelerator.
	_, err = client.DeleteAccelerator(ctx, &globalaccelerator.DeleteAcceleratorInput{
		AcceleratorArn: aws.String(acceleratorArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify deletion.
	_, err = client.DescribeAccelerator(ctx, &globalaccelerator.DescribeAcceleratorInput{
		AcceleratorArn: aws.String(acceleratorArn),
	})
	if err == nil {
		t.Fatal("expected error for deleted accelerator")
	}
}

func TestGlobalAccelerator_CreateAndDescribeListener(t *testing.T) {
	client := newGlobalAcceleratorClient(t)
	ctx := t.Context()

	// Create accelerator first.
	accOutput, err := client.CreateAccelerator(ctx, &globalaccelerator.CreateAcceleratorInput{
		Name:             aws.String("test-listener-accelerator"),
		IdempotencyToken: aws.String("test-token-5"),
	})
	if err != nil {
		t.Fatal(err)
	}

	acceleratorArn := *accOutput.Accelerator.AcceleratorArn

	// Create listener.
	listenerOutput, err := client.CreateListener(ctx, &globalaccelerator.CreateListenerInput{
		AcceleratorArn: aws.String(acceleratorArn),
		PortRanges: []types.PortRange{
			{FromPort: aws.Int32(80), ToPort: aws.Int32(80)},
			{FromPort: aws.Int32(443), ToPort: aws.Int32(443)},
		},
		Protocol:         types.ProtocolTcp,
		ClientAffinity:   types.ClientAffinityNone,
		IdempotencyToken: aws.String("listener-token-1"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ListenerArn", "ResultMetadata")).Assert(t.Name()+"_create", listenerOutput)

	listenerArn := *listenerOutput.Listener.ListenerArn

	// Describe listener.
	describeOutput, err := client.DescribeListener(ctx, &globalaccelerator.DescribeListenerInput{
		ListenerArn: aws.String(listenerArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ListenerArn", "ResultMetadata")).Assert(t.Name()+"_describe", describeOutput)
}

func TestGlobalAccelerator_ListListeners(t *testing.T) {
	client := newGlobalAcceleratorClient(t)
	ctx := t.Context()

	// Create accelerator.
	accOutput, err := client.CreateAccelerator(ctx, &globalaccelerator.CreateAcceleratorInput{
		Name:             aws.String("test-list-listeners-accelerator"),
		IdempotencyToken: aws.String("test-token-6"),
	})
	if err != nil {
		t.Fatal(err)
	}

	acceleratorArn := *accOutput.Accelerator.AcceleratorArn

	// Create listener.
	_, err = client.CreateListener(ctx, &globalaccelerator.CreateListenerInput{
		AcceleratorArn: aws.String(acceleratorArn),
		PortRanges: []types.PortRange{
			{FromPort: aws.Int32(80), ToPort: aws.Int32(80)},
		},
		Protocol:         types.ProtocolTcp,
		IdempotencyToken: aws.String("listener-token-2"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// List listeners.
	listOutput, err := client.ListListeners(ctx, &globalaccelerator.ListListenersInput{
		AcceleratorArn: aws.String(acceleratorArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ListenerArn", "ResultMetadata")).Assert(t.Name(), listOutput)
}

func TestGlobalAccelerator_CreateAndDescribeEndpointGroup(t *testing.T) {
	client := newGlobalAcceleratorClient(t)
	ctx := t.Context()

	// Create accelerator.
	accOutput, err := client.CreateAccelerator(ctx, &globalaccelerator.CreateAcceleratorInput{
		Name:             aws.String("test-endpoint-group-accelerator"),
		IdempotencyToken: aws.String("test-token-7"),
	})
	if err != nil {
		t.Fatal(err)
	}

	acceleratorArn := *accOutput.Accelerator.AcceleratorArn

	// Create listener.
	listenerOutput, err := client.CreateListener(ctx, &globalaccelerator.CreateListenerInput{
		AcceleratorArn: aws.String(acceleratorArn),
		PortRanges: []types.PortRange{
			{FromPort: aws.Int32(80), ToPort: aws.Int32(80)},
		},
		Protocol:         types.ProtocolTcp,
		IdempotencyToken: aws.String("listener-token-3"),
	})
	if err != nil {
		t.Fatal(err)
	}

	listenerArn := *listenerOutput.Listener.ListenerArn

	// Create endpoint group.
	egOutput, err := client.CreateEndpointGroup(ctx, &globalaccelerator.CreateEndpointGroupInput{
		ListenerArn:           aws.String(listenerArn),
		EndpointGroupRegion:   aws.String("us-east-1"),
		TrafficDialPercentage: aws.Float32(100),
		HealthCheckProtocol:   types.HealthCheckProtocolTcp,
		IdempotencyToken:      aws.String("endpoint-group-token-1"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("EndpointGroupArn", "ResultMetadata")).Assert(t.Name()+"_create", egOutput)

	endpointGroupArn := *egOutput.EndpointGroup.EndpointGroupArn

	// Describe endpoint group.
	describeOutput, err := client.DescribeEndpointGroup(ctx, &globalaccelerator.DescribeEndpointGroupInput{
		EndpointGroupArn: aws.String(endpointGroupArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("EndpointGroupArn", "ResultMetadata")).Assert(t.Name()+"_describe", describeOutput)
}

func TestGlobalAccelerator_ListEndpointGroups(t *testing.T) {
	client := newGlobalAcceleratorClient(t)
	ctx := t.Context()

	// Create accelerator.
	accOutput, err := client.CreateAccelerator(ctx, &globalaccelerator.CreateAcceleratorInput{
		Name:             aws.String("test-list-endpoint-groups-accelerator"),
		IdempotencyToken: aws.String("test-token-8"),
	})
	if err != nil {
		t.Fatal(err)
	}

	acceleratorArn := *accOutput.Accelerator.AcceleratorArn

	// Create listener.
	listenerOutput, err := client.CreateListener(ctx, &globalaccelerator.CreateListenerInput{
		AcceleratorArn: aws.String(acceleratorArn),
		PortRanges: []types.PortRange{
			{FromPort: aws.Int32(80), ToPort: aws.Int32(80)},
		},
		Protocol:         types.ProtocolTcp,
		IdempotencyToken: aws.String("listener-token-4"),
	})
	if err != nil {
		t.Fatal(err)
	}

	listenerArn := *listenerOutput.Listener.ListenerArn

	// Create endpoint group.
	_, err = client.CreateEndpointGroup(ctx, &globalaccelerator.CreateEndpointGroupInput{
		ListenerArn:         aws.String(listenerArn),
		EndpointGroupRegion: aws.String("us-east-1"),
		IdempotencyToken:    aws.String("endpoint-group-token-2"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// List endpoint groups.
	listOutput, err := client.ListEndpointGroups(ctx, &globalaccelerator.ListEndpointGroupsInput{
		ListenerArn: aws.String(listenerArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("EndpointGroupArn", "ResultMetadata")).Assert(t.Name(), listOutput)
}

func TestGlobalAccelerator_AcceleratorNotFound(t *testing.T) {
	client := newGlobalAcceleratorClient(t)
	ctx := t.Context()

	// Try to describe a non-existent accelerator.
	_, err := client.DescribeAccelerator(ctx, &globalaccelerator.DescribeAcceleratorInput{
		AcceleratorArn: aws.String("arn:aws:globalaccelerator::000000000000:accelerator/non-existent"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent accelerator")
	}
}
