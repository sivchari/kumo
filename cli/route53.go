package cli

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	route53Types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/spf13/cobra"
)

func newRoute53Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "route53",
		Short: "Route 53 commands",
	}

	cmd.AddCommand(
		newRoute53CreateHostedZoneCmd(),
		newRoute53ListHostedZonesCmd(),
		newRoute53GetHostedZoneCmd(),
		newRoute53DeleteHostedZoneCmd(),
		newRoute53ChangeResourceRecordSetsCmd(),
		newRoute53ListResourceRecordSetsCmd(),
	)

	return cmd
}

func newRoute53Client(cfg *aws.Config) *route53.Client {
	return route53.NewFromConfig(*cfg, func(o *route53.Options) {
		o.BaseEndpoint = aws.String(endpointURL)
	})
}

func newRoute53CreateHostedZoneCmd() *cobra.Command {
	var name, callerReference, hostedZoneConfigJSON string

	cmd := &cobra.Command{
		Use:   "create-hosted-zone",
		Short: "Create a hosted zone",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			input := &route53.CreateHostedZoneInput{
				Name:            aws.String(name),
				CallerReference: aws.String(callerReference),
			}

			if hostedZoneConfigJSON != "" {
				var hzConfig route53Types.HostedZoneConfig
				if err := json.Unmarshal([]byte(hostedZoneConfigJSON), &hzConfig); err != nil {
					return fmt.Errorf("failed to parse hosted-zone-config: %w", err)
				}

				input.HostedZoneConfig = &hzConfig
			}

			out, err := newRoute53Client(&cfg).CreateHostedZone(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("create-hosted-zone failed: %w", err)
			}

			return encodeJSON(out)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Domain name")
	cmd.Flags().StringVar(&callerReference, "caller-reference", "", "Unique string that identifies the request")
	cmd.Flags().StringVar(&hostedZoneConfigJSON, "hosted-zone-config", "", "Hosted zone config (JSON)")

	return cmd
}

func newRoute53ListHostedZonesCmd() *cobra.Command {
	var marker string

	var maxItems int32

	cmd := &cobra.Command{
		Use:   "list-hosted-zones",
		Short: "List hosted zones",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			input := &route53.ListHostedZonesInput{}
			if marker != "" {
				input.Marker = aws.String(marker)
			}

			if maxItems > 0 {
				input.MaxItems = aws.Int32(maxItems)
			}

			out, err := newRoute53Client(&cfg).ListHostedZones(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("list-hosted-zones failed: %w", err)
			}

			return encodeJSON(out)
		},
	}

	cmd.Flags().StringVar(&marker, "marker", "", "Pagination marker")
	cmd.Flags().Int32Var(&maxItems, "max-items", 0, "Maximum number of hosted zones")

	return cmd
}

func newRoute53GetHostedZoneCmd() *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:   "get-hosted-zone",
		Short: "Get a hosted zone",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			out, err := newRoute53Client(&cfg).GetHostedZone(cmd.Context(), &route53.GetHostedZoneInput{
				Id: aws.String(id),
			})
			if err != nil {
				return fmt.Errorf("get-hosted-zone failed: %w", err)
			}

			return encodeJSON(out)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "Hosted zone ID")

	return cmd
}

func newRoute53DeleteHostedZoneCmd() *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:   "delete-hosted-zone",
		Short: "Delete a hosted zone",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			out, err := newRoute53Client(&cfg).DeleteHostedZone(cmd.Context(), &route53.DeleteHostedZoneInput{
				Id: aws.String(id),
			})
			if err != nil {
				return fmt.Errorf("delete-hosted-zone failed: %w", err)
			}

			return encodeJSON(out)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "Hosted zone ID")

	return cmd
}

func newRoute53ChangeResourceRecordSetsCmd() *cobra.Command {
	var hostedZoneID, changeBatchJSON string

	cmd := &cobra.Command{
		Use:   "change-resource-record-sets",
		Short: "Create, update, or delete resource record sets",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			var changeBatch route53Types.ChangeBatch
			if err := json.Unmarshal([]byte(changeBatchJSON), &changeBatch); err != nil {
				return fmt.Errorf("failed to parse change-batch: %w", err)
			}

			input := &route53.ChangeResourceRecordSetsInput{
				HostedZoneId: aws.String(hostedZoneID),
				ChangeBatch:  &changeBatch,
			}

			out, err := newRoute53Client(&cfg).ChangeResourceRecordSets(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("change-resource-record-sets failed: %w", err)
			}

			return encodeJSON(out)
		},
	}

	cmd.Flags().StringVar(&hostedZoneID, "hosted-zone-id", "", "Hosted zone ID")
	cmd.Flags().StringVar(&changeBatchJSON, "change-batch", "", "Change batch (JSON)")

	return cmd
}

func newRoute53ListResourceRecordSetsCmd() *cobra.Command {
	var hostedZoneID string

	cmd := &cobra.Command{
		Use:   "list-resource-record-sets",
		Short: "List resource record sets",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			out, err := newRoute53Client(&cfg).ListResourceRecordSets(cmd.Context(), &route53.ListResourceRecordSetsInput{
				HostedZoneId: aws.String(hostedZoneID),
			})
			if err != nil {
				return fmt.Errorf("list-resource-record-sets failed: %w", err)
			}

			return encodeJSON(out)
		},
	}

	cmd.Flags().StringVar(&hostedZoneID, "hosted-zone-id", "", "Hosted zone ID")

	return cmd
}
