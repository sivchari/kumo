package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2Types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/spf13/cobra"
)

func newSESv2Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sesv2",
		Short: "SES v2 commands",
	}

	cmd.AddCommand(
		newSESv2SendEmailCmd(),
		newSESv2CreateEmailIdentityCmd(),
		newSESv2GetEmailIdentityCmd(),
		newSESv2ListEmailIdentitiesCmd(),
		newSESv2CreateEmailTemplateCmd(),
		newSESv2GetEmailTemplateCmd(),
		newSESv2ListEmailTemplatesCmd(),
		newSESv2DeleteEmailTemplateCmd(),
	)

	return cmd
}

// newSESv2Client builds a SES v2 client. Kumo mounts SES v2 under the "/ses"
// path prefix, but the AWS SDK serializes requests to "/v2/...", so the base
// endpoint must include the "/ses" suffix.
func newSESv2Client(cfg *aws.Config) *sesv2.Client {
	return sesv2.NewFromConfig(*cfg, func(o *sesv2.Options) {
		o.BaseEndpoint = aws.String(endpointURL + "/ses")
	})
}

func newSESv2SendEmailCmd() *cobra.Command {
	var fromEmailAddress, destinationJSON, contentJSON string

	cmd := &cobra.Command{
		Use:   "send-email",
		Short: "Send an email",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			var destination sesv2Types.Destination
			if destinationJSON != "" {
				if err := json.Unmarshal([]byte(destinationJSON), &destination); err != nil {
					return fmt.Errorf("failed to parse destination: %w", err)
				}
			}

			var content sesv2Types.EmailContent
			if err := json.Unmarshal([]byte(contentJSON), &content); err != nil {
				return fmt.Errorf("failed to parse content: %w", err)
			}

			out, err := newSESv2Client(&cfg).SendEmail(cmd.Context(), &sesv2.SendEmailInput{
				FromEmailAddress: aws.String(fromEmailAddress),
				Destination:      &destination,
				Content:          &content,
			})
			if err != nil {
				return fmt.Errorf("send-email failed: %w", err)
			}

			return encodeJSON(out)
		},
	}

	cmd.Flags().StringVar(&fromEmailAddress, "from-email-address", "", "Sender email address")
	cmd.Flags().StringVar(&destinationJSON, "destination", "", "Destination (JSON)")
	cmd.Flags().StringVar(&contentJSON, "content", "", "Email content (JSON)")

	return cmd
}

func newSESv2CreateEmailIdentityCmd() *cobra.Command {
	var emailIdentity string

	cmd := &cobra.Command{
		Use:   "create-email-identity",
		Short: "Create an email identity",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			out, err := newSESv2Client(&cfg).CreateEmailIdentity(cmd.Context(), &sesv2.CreateEmailIdentityInput{
				EmailIdentity: aws.String(emailIdentity),
			})
			if err != nil {
				return fmt.Errorf("create-email-identity failed: %w", err)
			}

			return encodeJSON(out)
		},
	}

	cmd.Flags().StringVar(&emailIdentity, "email-identity", "", "Email address or domain to verify")

	return cmd
}

func newSESv2GetEmailIdentityCmd() *cobra.Command {
	var emailIdentity string

	cmd := &cobra.Command{
		Use:   "get-email-identity",
		Short: "Get an email identity",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			out, err := newSESv2Client(&cfg).GetEmailIdentity(cmd.Context(), &sesv2.GetEmailIdentityInput{
				EmailIdentity: aws.String(emailIdentity),
			})
			if err != nil {
				return fmt.Errorf("get-email-identity failed: %w", err)
			}

			return encodeJSON(out)
		},
	}

	cmd.Flags().StringVar(&emailIdentity, "email-identity", "", "Email address or domain")

	return cmd
}

func newSESv2ListEmailIdentitiesCmd() *cobra.Command {
	var nextToken string

	var pageSize int32

	cmd := &cobra.Command{
		Use:   "list-email-identities",
		Short: "List email identities",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			input := &sesv2.ListEmailIdentitiesInput{}
			if nextToken != "" {
				input.NextToken = aws.String(nextToken)
			}

			if pageSize > 0 {
				input.PageSize = aws.Int32(pageSize)
			}

			out, err := newSESv2Client(&cfg).ListEmailIdentities(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("list-email-identities failed: %w", err)
			}

			return encodeJSON(out)
		},
	}

	cmd.Flags().StringVar(&nextToken, "next-token", "", "Pagination token")
	cmd.Flags().Int32Var(&pageSize, "page-size", 0, "Maximum number of results")

	return cmd
}

func newSESv2CreateEmailTemplateCmd() *cobra.Command {
	var templateName, templateContentJSON string

	cmd := &cobra.Command{
		Use:   "create-email-template",
		Short: "Create an email template",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			var content sesv2Types.EmailTemplateContent
			if err := json.Unmarshal([]byte(templateContentJSON), &content); err != nil {
				return fmt.Errorf("failed to parse template-content: %w", err)
			}

			out, err := newSESv2Client(&cfg).CreateEmailTemplate(cmd.Context(), &sesv2.CreateEmailTemplateInput{
				TemplateName:    aws.String(templateName),
				TemplateContent: &content,
			})
			if err != nil {
				return fmt.Errorf("create-email-template failed: %w", err)
			}

			return encodeJSON(out)
		},
	}

	cmd.Flags().StringVar(&templateName, "template-name", "", "Template name")
	cmd.Flags().StringVar(&templateContentJSON, "template-content", "", "Template content (JSON)")

	return cmd
}

func newSESv2GetEmailTemplateCmd() *cobra.Command {
	var templateName string

	cmd := &cobra.Command{
		Use:   "get-email-template",
		Short: "Get an email template",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			out, err := newSESv2Client(&cfg).GetEmailTemplate(cmd.Context(), &sesv2.GetEmailTemplateInput{
				TemplateName: aws.String(templateName),
			})
			if err != nil {
				return fmt.Errorf("get-email-template failed: %w", err)
			}

			return encodeJSON(out)
		},
	}

	cmd.Flags().StringVar(&templateName, "template-name", "", "Template name")

	return cmd
}

func newSESv2ListEmailTemplatesCmd() *cobra.Command {
	var nextToken string

	var pageSize int32

	cmd := &cobra.Command{
		Use:   "list-email-templates",
		Short: "List email templates",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			input := &sesv2.ListEmailTemplatesInput{}
			if nextToken != "" {
				input.NextToken = aws.String(nextToken)
			}

			if pageSize > 0 {
				input.PageSize = aws.Int32(pageSize)
			}

			out, err := newSESv2Client(&cfg).ListEmailTemplates(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("list-email-templates failed: %w", err)
			}

			return encodeJSON(out)
		},
	}

	cmd.Flags().StringVar(&nextToken, "next-token", "", "Pagination token")
	cmd.Flags().Int32Var(&pageSize, "page-size", 0, "Maximum number of results")

	return cmd
}

func newSESv2DeleteEmailTemplateCmd() *cobra.Command {
	var templateName string

	cmd := &cobra.Command{
		Use:   "delete-email-template",
		Short: "Delete an email template",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			_, err = newSESv2Client(&cfg).DeleteEmailTemplate(cmd.Context(), &sesv2.DeleteEmailTemplateInput{
				TemplateName: aws.String(templateName),
			})
			if err != nil {
				return fmt.Errorf("delete-email-template failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&templateName, "template-name", "", "Template name")

	return cmd
}

// encodeJSON writes v to stdout as JSON, matching the AWS CLI's default
// output format.
func encodeJSON(v any) error {
	if err := json.NewEncoder(os.Stdout).Encode(v); err != nil {
		return fmt.Errorf("failed to encode output: %w", err)
	}

	return nil
}
