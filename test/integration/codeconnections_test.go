//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codeconnections"
	"github.com/aws/aws-sdk-go-v2/service/codeconnections/types"
	"github.com/sivchari/golden"
)

func newCodeConnectionsClient(t *testing.T) *codeconnections.Client {
	t.Helper()

	return codeconnections.NewFromConfig(awsConfig(t), func(o *codeconnections.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestCodeConnections_CreateAndDeleteConnection(t *testing.T) {
	client := newCodeConnectionsClient(t)
	ctx := t.Context()

	// Create connection.
	createOutput, err := client.CreateConnection(ctx, &codeconnections.CreateConnectionInput{
		ConnectionName: aws.String("test-connection"),
		ProviderType:   types.ProviderTypeGithub,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ConnectionArn", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	connectionArn := *createOutput.ConnectionArn

	// Delete connection.
	_, err = client.DeleteConnection(ctx, &codeconnections.DeleteConnectionInput{
		ConnectionArn: aws.String(connectionArn),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCodeConnections_GetConnection(t *testing.T) {
	client := newCodeConnectionsClient(t)
	ctx := t.Context()

	// Create connection.
	createOutput, err := client.CreateConnection(ctx, &codeconnections.CreateConnectionInput{
		ConnectionName: aws.String("test-get-connection"),
		ProviderType:   types.ProviderTypeGithub,
	})
	if err != nil {
		t.Fatal(err)
	}

	connectionArn := *createOutput.ConnectionArn

	t.Cleanup(func() {
		_, _ = client.DeleteConnection(context.Background(), &codeconnections.DeleteConnectionInput{
			ConnectionArn: aws.String(connectionArn),
		})
	})

	// Get connection.
	getOutput, err := client.GetConnection(ctx, &codeconnections.GetConnectionInput{
		ConnectionArn: aws.String(connectionArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ConnectionArn", "ResultMetadata")).Assert(t.Name()+"_get", getOutput)
}

func TestCodeConnections_ListConnections(t *testing.T) {
	client := newCodeConnectionsClient(t)
	ctx := t.Context()

	// Create a few connections.
	var createdArns []string

	for i := 0; i < 3; i++ {
		output, err := client.CreateConnection(ctx, &codeconnections.CreateConnectionInput{
			ConnectionName: aws.String("test-list-connection-" + string(rune('a'+i))),
			ProviderType:   types.ProviderTypeGithub,
		})
		if err != nil {
			t.Fatalf("failed to create connection %d: %v", i, err)
		}

		createdArns = append(createdArns, *output.ConnectionArn)
	}

	t.Cleanup(func() {
		for _, arn := range createdArns {
			_, _ = client.DeleteConnection(context.Background(), &codeconnections.DeleteConnectionInput{
				ConnectionArn: aws.String(arn),
			})
		}
	})

	// List connections.
	listOutput, err := client.ListConnections(ctx, &codeconnections.ListConnectionsInput{
		MaxResults: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ConnectionArn", "ResultMetadata")).Assert(t.Name()+"_list", listOutput)
}

func TestCodeConnections_CreateAndDeleteHost(t *testing.T) {
	client := newCodeConnectionsClient(t)
	ctx := t.Context()

	// Create host.
	createOutput, err := client.CreateHost(ctx, &codeconnections.CreateHostInput{
		Name:             aws.String("test-host"),
		ProviderType:     types.ProviderTypeGithubEnterpriseServer,
		ProviderEndpoint: aws.String("https://github.example.com"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("HostArn", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	hostArn := *createOutput.HostArn

	// Delete host.
	_, err = client.DeleteHost(ctx, &codeconnections.DeleteHostInput{
		HostArn: aws.String(hostArn),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCodeConnections_GetHost(t *testing.T) {
	client := newCodeConnectionsClient(t)
	ctx := t.Context()

	// Create host.
	createOutput, err := client.CreateHost(ctx, &codeconnections.CreateHostInput{
		Name:             aws.String("test-get-host"),
		ProviderType:     types.ProviderTypeGithubEnterpriseServer,
		ProviderEndpoint: aws.String("https://github.example.com"),
	})
	if err != nil {
		t.Fatal(err)
	}

	hostArn := *createOutput.HostArn

	t.Cleanup(func() {
		_, _ = client.DeleteHost(context.Background(), &codeconnections.DeleteHostInput{
			HostArn: aws.String(hostArn),
		})
	})

	// Get host.
	getOutput, err := client.GetHost(ctx, &codeconnections.GetHostInput{
		HostArn: aws.String(hostArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_get", getOutput)
}

func TestCodeConnections_ListHosts(t *testing.T) {
	client := newCodeConnectionsClient(t)
	ctx := t.Context()

	// Create a host.
	createOutput, err := client.CreateHost(ctx, &codeconnections.CreateHostInput{
		Name:             aws.String("test-list-host"),
		ProviderType:     types.ProviderTypeGithubEnterpriseServer,
		ProviderEndpoint: aws.String("https://github.example.com"),
	})
	if err != nil {
		t.Fatal(err)
	}

	hostArn := *createOutput.HostArn

	t.Cleanup(func() {
		_, _ = client.DeleteHost(context.Background(), &codeconnections.DeleteHostInput{
			HostArn: aws.String(hostArn),
		})
	})

	// List hosts.
	listOutput, err := client.ListHosts(ctx, &codeconnections.ListHostsInput{
		MaxResults: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("HostArn", "ResultMetadata")).Assert(t.Name()+"_list", listOutput)
}

func TestCodeConnections_UpdateHost(t *testing.T) {
	client := newCodeConnectionsClient(t)
	ctx := t.Context()

	// Create host.
	createOutput, err := client.CreateHost(ctx, &codeconnections.CreateHostInput{
		Name:             aws.String("test-update-host"),
		ProviderType:     types.ProviderTypeGithubEnterpriseServer,
		ProviderEndpoint: aws.String("https://github.example.com"),
	})
	if err != nil {
		t.Fatal(err)
	}

	hostArn := *createOutput.HostArn

	t.Cleanup(func() {
		_, _ = client.DeleteHost(context.Background(), &codeconnections.DeleteHostInput{
			HostArn: aws.String(hostArn),
		})
	})

	// Update host.
	_, err = client.UpdateHost(ctx, &codeconnections.UpdateHostInput{
		HostArn:          aws.String(hostArn),
		ProviderEndpoint: aws.String("https://github-new.example.com"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify update.
	getOutput, err := client.GetHost(ctx, &codeconnections.GetHostInput{
		HostArn: aws.String(hostArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_get", getOutput)
}

func TestCodeConnections_ConnectionNotFound(t *testing.T) {
	client := newCodeConnectionsClient(t)
	ctx := t.Context()

	// Try to get a non-existent connection.
	_, err := client.GetConnection(ctx, &codeconnections.GetConnectionInput{
		ConnectionArn: aws.String("arn:aws:codeconnections:us-east-1:000000000000:connection/non-existent"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent connection")
	}
}

func TestCodeConnections_HostNotFound(t *testing.T) {
	client := newCodeConnectionsClient(t)
	ctx := t.Context()

	// Try to get a non-existent host.
	_, err := client.GetHost(ctx, &codeconnections.GetHostInput{
		HostArn: aws.String("arn:aws:codeconnections:us-east-1:000000000000:host/non-existent"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent host")
	}
}

func TestCodeConnections_TagResource(t *testing.T) {
	client := newCodeConnectionsClient(t)
	ctx := t.Context()

	// Create connection.
	createOutput, err := client.CreateConnection(ctx, &codeconnections.CreateConnectionInput{
		ConnectionName: aws.String("test-tag-connection"),
		ProviderType:   types.ProviderTypeGithub,
	})
	if err != nil {
		t.Fatal(err)
	}

	connectionArn := *createOutput.ConnectionArn

	t.Cleanup(func() {
		_, _ = client.DeleteConnection(context.Background(), &codeconnections.DeleteConnectionInput{
			ConnectionArn: aws.String(connectionArn),
		})
	})

	// Tag resource.
	_, err = client.TagResource(ctx, &codeconnections.TagResourceInput{
		ResourceArn: aws.String(connectionArn),
		Tags: []types.Tag{
			{Key: aws.String("Environment"), Value: aws.String("Test")},
			{Key: aws.String("Project"), Value: aws.String("awsim")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// List tags.
	listTagsOutput, err := client.ListTagsForResource(ctx, &codeconnections.ListTagsForResourceInput{
		ResourceArn: aws.String(connectionArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_list_tags", listTagsOutput)
}

func TestCodeConnections_UntagResource(t *testing.T) {
	client := newCodeConnectionsClient(t)
	ctx := t.Context()

	// Create connection with tags.
	createOutput, err := client.CreateConnection(ctx, &codeconnections.CreateConnectionInput{
		ConnectionName: aws.String("test-untag-connection"),
		ProviderType:   types.ProviderTypeGithub,
		Tags: []types.Tag{
			{Key: aws.String("Environment"), Value: aws.String("Test")},
			{Key: aws.String("Project"), Value: aws.String("awsim")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	connectionArn := *createOutput.ConnectionArn

	t.Cleanup(func() {
		_, _ = client.DeleteConnection(context.Background(), &codeconnections.DeleteConnectionInput{
			ConnectionArn: aws.String(connectionArn),
		})
	})

	// Untag resource.
	_, err = client.UntagResource(ctx, &codeconnections.UntagResourceInput{
		ResourceArn: aws.String(connectionArn),
		TagKeys:     []string{"Environment"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// List tags.
	listTagsOutput, err := client.ListTagsForResource(ctx, &codeconnections.ListTagsForResourceInput{
		ResourceArn: aws.String(connectionArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_list_tags", listTagsOutput)
}
