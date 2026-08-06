package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/spf13/cobra"
)

// KMSOverrideCommands returns the KMS commands cli-gen cannot auto-generate:
// Sign and Verify operate on raw message bytes and need bespoke
// base64/raw-bytes handling that the generic field-kind rules don't cover
// (see internal/cligen/overrides.go's skipAutoGeneration).
func KMSOverrideCommands() []*cobra.Command {
	return []*cobra.Command{
		newKMSSignCmd(),
		newKMSVerifyCmd(),
	}
}

func newKMSSignCmd() *cobra.Command {
	var keyID, message, messageType, signingAlgorithm string

	cmd := &cobra.Command{
		Use:   "sign",
		Short: "Sign a message with an asymmetric KMS key",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			client := kms.NewFromConfig(cfg, func(o *kms.Options) {
				o.BaseEndpoint = aws.String(endpointURL)
			})

			input := &kms.SignInput{
				KeyId:            aws.String(keyID),
				Message:          []byte(message),
				SigningAlgorithm: types.SigningAlgorithmSpec(signingAlgorithm),
			}
			if messageType != "" {
				input.MessageType = types.MessageType(messageType)
			}

			out, err := client.Sign(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("sign failed: %w", err)
			}

			if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
				return fmt.Errorf("failed to encode output: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&keyID, "key-id", "", "Key ID, ARN, or alias")
	cmd.Flags().StringVar(&message, "message", "", "Message to sign")
	cmd.Flags().StringVar(&messageType, "message-type", "", "Message type (RAW or DIGEST)")
	cmd.Flags().StringVar(&signingAlgorithm, "signing-algorithm", "", "Signing algorithm (e.g. RSASSA_PKCS1_V1_5_SHA_256)")

	return cmd
}

func newKMSVerifyCmd() *cobra.Command {
	var keyID, message, messageType, signingAlgorithm, signature string

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify a signature with an asymmetric KMS key",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			client := kms.NewFromConfig(cfg, func(o *kms.Options) {
				o.BaseEndpoint = aws.String(endpointURL)
			})

			sig, err := base64.StdEncoding.DecodeString(signature)
			if err != nil {
				return fmt.Errorf("decode signature: %w", err)
			}

			input := &kms.VerifyInput{
				KeyId:            aws.String(keyID),
				Message:          []byte(message),
				Signature:        sig,
				SigningAlgorithm: types.SigningAlgorithmSpec(signingAlgorithm),
			}
			if messageType != "" {
				input.MessageType = types.MessageType(messageType)
			}

			out, err := client.Verify(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("verify failed: %w", err)
			}

			if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
				return fmt.Errorf("failed to encode output: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&keyID, "key-id", "", "Key ID, ARN, or alias")
	cmd.Flags().StringVar(&message, "message", "", "Message that was signed")
	cmd.Flags().StringVar(&signature, "signature", "", "Base64-encoded signature")
	cmd.Flags().StringVar(&messageType, "message-type", "", "Message type (RAW or DIGEST)")
	cmd.Flags().StringVar(&signingAlgorithm, "signing-algorithm", "", "Signing algorithm (e.g. RSASSA_PKCS1_V1_5_SHA_256)")

	return cmd
}
