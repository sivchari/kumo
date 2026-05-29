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

func newKMSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kms",
		Short: "KMS commands",
	}

	cmd.AddCommand(
		newKMSCreateKeyCmd(),
		newKMSCreateAliasCmd(),
		newKMSSignCmd(),
		newKMSVerifyCmd(),
		newKMSGetPublicKeyCmd(),
	)

	return cmd
}

func newKMSCreateKeyCmd() *cobra.Command {
	var description, keyUsage, keySpec string

	cmd := &cobra.Command{
		Use:   "create-key",
		Short: "Create a KMS key",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			client := kms.NewFromConfig(cfg, func(o *kms.Options) {
				o.BaseEndpoint = aws.String(endpointURL)
			})

			input := &kms.CreateKeyInput{}
			if description != "" {
				input.Description = aws.String(description)
			}

			if keyUsage != "" {
				input.KeyUsage = types.KeyUsageType(keyUsage)
			}

			if keySpec != "" {
				input.KeySpec = types.KeySpec(keySpec)
			}

			out, err := client.CreateKey(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("create-key failed: %w", err)
			}

			if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
				return fmt.Errorf("failed to encode output: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&description, "description", "", "Key description")
	cmd.Flags().StringVar(&keyUsage, "key-usage", "", "Key usage (ENCRYPT_DECRYPT or SIGN_VERIFY)")
	cmd.Flags().StringVar(&keySpec, "key-spec", "", "Key spec (e.g. SYMMETRIC_DEFAULT, RSA_2048, ECC_NIST_P256)")

	return cmd
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

func newKMSGetPublicKeyCmd() *cobra.Command {
	var keyID string

	cmd := &cobra.Command{
		Use:   "get-public-key",
		Short: "Get the public key of an asymmetric KMS key",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			client := kms.NewFromConfig(cfg, func(o *kms.Options) {
				o.BaseEndpoint = aws.String(endpointURL)
			})

			out, err := client.GetPublicKey(cmd.Context(), &kms.GetPublicKeyInput{
				KeyId: aws.String(keyID),
			})
			if err != nil {
				return fmt.Errorf("get-public-key failed: %w", err)
			}

			if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
				return fmt.Errorf("failed to encode output: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&keyID, "key-id", "", "Key ID, ARN, or alias")

	return cmd
}

func newKMSCreateAliasCmd() *cobra.Command {
	var targetKeyID, aliasName string

	cmd := &cobra.Command{
		Use:   "create-alias",
		Short: "Create a KMS alias",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			client := kms.NewFromConfig(cfg, func(o *kms.Options) {
				o.BaseEndpoint = aws.String(endpointURL)
			})

			_, err = client.CreateAlias(cmd.Context(), &kms.CreateAliasInput{
				TargetKeyId: aws.String(targetKeyID),
				AliasName:   aws.String(aliasName),
			})
			if err != nil {
				return fmt.Errorf("create-alias failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&targetKeyID, "target-key-id", "", "Target key ID")
	cmd.Flags().StringVar(&aliasName, "alias-name", "", "Alias name")

	return cmd
}
