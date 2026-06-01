// Package cli provides the kumo CLI for managing AWS resources on a kumo server.
package cli

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/spf13/cobra"
)

var (
	endpointURL string
	region      string
)

// NewRootCmd creates the root kumo CLI command.
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kumo",
		Short: "Lightweight AWS CLI for kumo",
	}

	cmd.PersistentFlags().StringVar(&endpointURL, "endpoint-url", "http://localhost:4566", "kumo endpoint URL")
	cmd.PersistentFlags().StringVar(&region, "region", "ap-northeast-1", "AWS region")

	cmd.AddCommand(
		newACMCmd(),
		newAmplifyCmd(),
		newAPIGatewayCmd(),
		newAPIGatewayV2Cmd(),
		newAppMeshCmd(),
		newAppSyncCmd(),
		newAthenaCmd(),
		newBackupCmd(),
		newS3Cmd(),
		newS3APICmd(),
		newDynamoDBCmd(),
		newSQSCmd(),
		newKMSCmd(),
		newKinesisCmd(),
		newEventsCmd(),
	)

	return cmd
}

func newAWSConfig(ctx context.Context) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load config: %w", err)
	}

	return cfg, nil
}
