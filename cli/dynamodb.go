package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/spf13/cobra"
)

func newDynamoDBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dynamodb",
		Short: "DynamoDB commands",
	}

	cmd.AddCommand(
		newDynamoDBCreateTableCmd(),
		newDynamoDBUpdateTimeToLiveCmd(),
	)

	return cmd
}

//nolint:funlen // CLI flag setup requires many lines.
func newDynamoDBCreateTableCmd() *cobra.Command {
	var (
		tableName      string
		attrDefs       []string
		keySchema      []string
		billingMode    string
		provThroughput string
		gsiJSON        string
		lsiJSON        string
	)

	cmd := &cobra.Command{
		Use:   "create-table",
		Short: "Create a DynamoDB table",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			client := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
				o.BaseEndpoint = aws.String(endpointURL)
			})

			input := &dynamodb.CreateTableInput{
				TableName: aws.String(tableName),
			}

			if billingMode != "" {
				input.BillingMode = ddbTypes.BillingMode(billingMode)
			}

			input.AttributeDefinitions = parseAttributeDefinitions(attrDefs)
			input.KeySchema = parseKeySchema(keySchema)

			if provThroughput != "" {
				input.ProvisionedThroughput = parseProvisionedThroughput(provThroughput)
			}

			if gsiJSON != "" {
				gsis, err := parseGSI(gsiJSON)
				if err != nil {
					return err
				}

				input.GlobalSecondaryIndexes = gsis
			}

			if lsiJSON != "" {
				lsis, err := parseLSI(lsiJSON)
				if err != nil {
					return err
				}

				input.LocalSecondaryIndexes = lsis
			}

			out, err := client.CreateTable(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("create-table failed: %w", err)
			}

			if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
				return fmt.Errorf("failed to encode output: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&tableName, "table-name", "", "Table name")
	cmd.Flags().StringArrayVar(&attrDefs, "attribute-definitions", nil, "Attribute definitions (key=value pairs)")
	cmd.Flags().StringArrayVar(&keySchema, "key-schema", nil, "Key schema (key=value pairs)")
	cmd.Flags().StringVar(&billingMode, "billing-mode", "", "Billing mode (PROVISIONED or PAY_PER_REQUEST)")
	cmd.Flags().StringVar(&provThroughput, "provisioned-throughput", "", "Provisioned throughput (key=value)")
	cmd.Flags().StringVar(&gsiJSON, "global-secondary-indexes", "", "Global secondary indexes (JSON)")
	cmd.Flags().StringVar(&lsiJSON, "local-secondary-indexes", "", "Local secondary indexes (JSON)")
	// Unused flags that appear in init scripts.
	cmd.Flags().String("table-class", "", "Table class (ignored)")
	cmd.Flags().String("region", "", "Region override (ignored, uses global --region)")

	return cmd
}

func newDynamoDBUpdateTimeToLiveCmd() *cobra.Command {
	var tableName, spec string

	cmd := &cobra.Command{
		Use:   "update-time-to-live",
		Short: "Update time to live settings",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := newAWSConfig(cmd.Context())
			if err != nil {
				return err
			}

			client := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
				o.BaseEndpoint = aws.String(endpointURL)
			})

			//nolint:tagliatelle // AWS CLI JSON format uses PascalCase.
			var ttl struct {
				Enabled       string `json:"Enabled"`
				AttributeName string `json:"AttributeName"`
			}

			_ = json.Unmarshal([]byte(spec), &ttl)

			_, err = client.UpdateTimeToLive(cmd.Context(), &dynamodb.UpdateTimeToLiveInput{
				TableName: aws.String(tableName),
				TimeToLiveSpecification: &ddbTypes.TimeToLiveSpecification{
					Enabled:       aws.Bool(strings.EqualFold(ttl.Enabled, "true")),
					AttributeName: aws.String(ttl.AttributeName),
				},
			})
			if err != nil {
				return fmt.Errorf("update-time-to-live failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&tableName, "table-name", "", "Table name")
	cmd.Flags().StringVar(&spec, "time-to-live-specification", "", "TTL specification (JSON)")
	cmd.Flags().String("region", "", "Region override (ignored)")

	return cmd
}

func parseAttributeDefinitions(args []string) []ddbTypes.AttributeDefinition {
	defs := make([]ddbTypes.AttributeDefinition, 0, len(args))

	for _, arg := range args {
		m := parseKV(arg)
		defs = append(defs, ddbTypes.AttributeDefinition{
			AttributeName: aws.String(m["AttributeName"]),
			AttributeType: ddbTypes.ScalarAttributeType(m["AttributeType"]),
		})
	}

	return defs
}

func parseKeySchema(args []string) []ddbTypes.KeySchemaElement {
	schema := make([]ddbTypes.KeySchemaElement, 0, len(args))

	for _, arg := range args {
		m := parseKV(arg)
		schema = append(schema, ddbTypes.KeySchemaElement{
			AttributeName: aws.String(m["AttributeName"]),
			KeyType:       ddbTypes.KeyType(m["KeyType"]),
		})
	}

	return schema
}

func parseProvisionedThroughput(s string) *ddbTypes.ProvisionedThroughput {
	m := parseKV(s)

	var rcu, wcu int64

	_, _ = fmt.Sscanf(m["ReadCapacityUnits"], "%d", &rcu)
	_, _ = fmt.Sscanf(m["WriteCapacityUnits"], "%d", &wcu)

	return &ddbTypes.ProvisionedThroughput{
		ReadCapacityUnits:  aws.Int64(rcu),
		WriteCapacityUnits: aws.Int64(wcu),
	}
}

// normalizeIndexJSON accepts either a JSON array (`[{...}]`) or a single JSON
// object (`{...}`) and returns the array form. aws CLI normalizes object input
// the same way before sending the DynamoDB request.
func normalizeIndexJSON(s string) string {
	if strings.HasPrefix(strings.TrimLeftFunc(s, unicode.IsSpace), "{") {
		return "[" + s + "]"
	}

	return s
}

func parseGSI(s string) ([]ddbTypes.GlobalSecondaryIndex, error) {
	//nolint:tagliatelle // AWS CLI JSON format uses PascalCase.
	var raw []struct {
		IndexName string `json:"IndexName"`
		KeySchema []struct {
			AttributeName string `json:"AttributeName"`
			KeyType       string `json:"KeyType"`
		} `json:"KeySchema"`
		Projection struct {
			ProjectionType string `json:"ProjectionType"`
		} `json:"Projection"`
		ProvisionedThroughput *struct {
			ReadCapacityUnits  int64 `json:"ReadCapacityUnits"`
			WriteCapacityUnits int64 `json:"WriteCapacityUnits"`
		} `json:"ProvisionedThroughput,omitempty"`
	}

	normalized := normalizeIndexJSON(s)
	if err := json.Unmarshal([]byte(normalized), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse --global-secondary-indexes: invalid json: %w received: %s", err, s)
	}

	gsis := make([]ddbTypes.GlobalSecondaryIndex, 0, len(raw))

	for _, g := range raw {
		gsi := ddbTypes.GlobalSecondaryIndex{
			IndexName:  aws.String(g.IndexName),
			Projection: &ddbTypes.Projection{ProjectionType: ddbTypes.ProjectionType(g.Projection.ProjectionType)},
		}

		for _, ks := range g.KeySchema {
			gsi.KeySchema = append(gsi.KeySchema, ddbTypes.KeySchemaElement{
				AttributeName: aws.String(ks.AttributeName),
				KeyType:       ddbTypes.KeyType(ks.KeyType),
			})
		}

		if g.ProvisionedThroughput != nil {
			gsi.ProvisionedThroughput = &ddbTypes.ProvisionedThroughput{
				ReadCapacityUnits:  aws.Int64(g.ProvisionedThroughput.ReadCapacityUnits),
				WriteCapacityUnits: aws.Int64(g.ProvisionedThroughput.WriteCapacityUnits),
			}
		}

		gsis = append(gsis, gsi)
	}

	return gsis, nil
}

func parseLSI(s string) ([]ddbTypes.LocalSecondaryIndex, error) {
	//nolint:tagliatelle // AWS CLI JSON format uses PascalCase.
	var raw []struct {
		IndexName string `json:"IndexName"`
		KeySchema []struct {
			AttributeName string `json:"AttributeName"`
			KeyType       string `json:"KeyType"`
		} `json:"KeySchema"`
		Projection struct {
			ProjectionType string `json:"ProjectionType"`
		} `json:"Projection"`
	}

	normalized := normalizeIndexJSON(s)
	if err := json.Unmarshal([]byte(normalized), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse --local-secondary-indexes: invalid json: %w received: %s", err, s)
	}

	lsis := make([]ddbTypes.LocalSecondaryIndex, 0, len(raw))

	for _, l := range raw {
		lsi := ddbTypes.LocalSecondaryIndex{
			IndexName:  aws.String(l.IndexName),
			Projection: &ddbTypes.Projection{ProjectionType: ddbTypes.ProjectionType(l.Projection.ProjectionType)},
		}

		for _, ks := range l.KeySchema {
			lsi.KeySchema = append(lsi.KeySchema, ddbTypes.KeySchemaElement{
				AttributeName: aws.String(ks.AttributeName),
				KeyType:       ddbTypes.KeyType(ks.KeyType),
			})
		}

		lsis = append(lsis, lsi)
	}

	return lsis, nil
}

func parseKV(s string) map[string]string {
	m := make(map[string]string)

	for _, pair := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(pair, "=")
		if ok {
			m[k] = v
		}
	}

	return m
}
