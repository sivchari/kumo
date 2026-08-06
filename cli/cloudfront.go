package cli

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cloudfrontTypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/spf13/cobra"
)

func newCloudFrontCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cloudfront",
		Short: "CloudFront commands",
	}

	cmd.AddCommand(
		newCloudFrontCreateDistributionCmd(),
		newCloudFrontGetDistributionCmd(),
		newCloudFrontListDistributionsCmd(),
		newCloudFrontDeleteDistributionCmd(),
		newCloudFrontCreateInvalidationCmd(),
	)

	return cmd
}

func newCloudFrontClient(cfg *aws.Config) *cloudfront.Client {
	return cloudfront.NewFromConfig(*cfg, func(o *cloudfront.Options) {
		o.BaseEndpoint = aws.String(endpointURL)
	})
}

func newCloudFrontCreateDistributionCmd() *cobra.Command {
	var distributionConfigJSON string

	cmd := &cobra.Command{
		Use:   "create-distribution",
		Short: "Create a distribution",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			var distConfig cloudfrontTypes.DistributionConfig
			if err := json.Unmarshal([]byte(distributionConfigJSON), &distConfig); err != nil {
				return fmt.Errorf("failed to parse distribution-config: %w", err)
			}

			out, err := newCloudFrontClient(&cfg).CreateDistribution(cmd.Context(), &cloudfront.CreateDistributionInput{
				DistributionConfig: &distConfig,
			})
			if err != nil {
				return fmt.Errorf("create-distribution failed: %w", err)
			}

			return encodeJSON(out)
		},
	}

	cmd.Flags().StringVar(&distributionConfigJSON, "distribution-config", "", "Distribution config (JSON)")

	return cmd
}

func newCloudFrontGetDistributionCmd() *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:   "get-distribution",
		Short: "Get a distribution",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			out, err := newCloudFrontClient(&cfg).GetDistribution(cmd.Context(), &cloudfront.GetDistributionInput{
				Id: aws.String(id),
			})
			if err != nil {
				return fmt.Errorf("get-distribution failed: %w", err)
			}

			return encodeJSON(out)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "Distribution ID")

	return cmd
}

func newCloudFrontListDistributionsCmd() *cobra.Command {
	var marker string

	var maxItems int32

	cmd := &cobra.Command{
		Use:   "list-distributions",
		Short: "List distributions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			input := &cloudfront.ListDistributionsInput{}
			if marker != "" {
				input.Marker = aws.String(marker)
			}

			if maxItems > 0 {
				input.MaxItems = aws.Int32(maxItems)
			}

			out, err := newCloudFrontClient(&cfg).ListDistributions(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("list-distributions failed: %w", err)
			}

			return encodeJSON(out)
		},
	}

	cmd.Flags().StringVar(&marker, "marker", "", "Pagination marker")
	cmd.Flags().Int32Var(&maxItems, "max-items", 0, "Maximum number of distributions")

	return cmd
}

func newCloudFrontDeleteDistributionCmd() *cobra.Command {
	var id, ifMatch string

	cmd := &cobra.Command{
		Use:   "delete-distribution",
		Short: "Delete a distribution",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			_, err = newCloudFrontClient(&cfg).DeleteDistribution(cmd.Context(), &cloudfront.DeleteDistributionInput{
				Id:      aws.String(id),
				IfMatch: aws.String(ifMatch),
			})
			if err != nil {
				return fmt.Errorf("delete-distribution failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "Distribution ID")
	cmd.Flags().StringVar(&ifMatch, "if-match", "", "ETag of the distribution to delete")

	return cmd
}

func newCloudFrontCreateInvalidationCmd() *cobra.Command {
	var distributionID, invalidationBatchJSON string

	cmd := &cobra.Command{
		Use:   "create-invalidation",
		Short: "Create an invalidation",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			var batch cloudfrontTypes.InvalidationBatch
			if err := json.Unmarshal([]byte(invalidationBatchJSON), &batch); err != nil {
				return fmt.Errorf("failed to parse invalidation-batch: %w", err)
			}

			input := &cloudfront.CreateInvalidationInput{
				DistributionId:    aws.String(distributionID),
				InvalidationBatch: &batch,
			}

			out, err := newCloudFrontClient(&cfg).CreateInvalidation(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("create-invalidation failed: %w", err)
			}

			return encodeJSON(out)
		},
	}

	cmd.Flags().StringVar(&distributionID, "distribution-id", "", "Distribution ID")
	cmd.Flags().StringVar(&invalidationBatchJSON, "invalidation-batch", "", "Invalidation batch (JSON)")

	return cmd
}
