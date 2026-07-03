package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigatewayv2Types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/spf13/cobra"
)

func newAPIGatewayV2Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apigatewayv2",
		Short: "API Gateway v2 (HTTP API) commands",
	}

	cmd.AddCommand(
		newAPIGatewayV2CreateAPICmd(),
		newAPIGatewayV2GetAPIsCmd(),
		newAPIGatewayV2GetAPICmd(),
		newAPIGatewayV2DeleteAPICmd(),
		newAPIGatewayV2CreateRouteCmd(),
		newAPIGatewayV2GetRoutesCmd(),
		newAPIGatewayV2DeleteRouteCmd(),
		newAPIGatewayV2CreateIntegrationCmd(),
		newAPIGatewayV2GetIntegrationsCmd(),
		newAPIGatewayV2DeleteIntegrationCmd(),
		newAPIGatewayV2CreateStageCmd(),
		newAPIGatewayV2GetStagesCmd(),
		newAPIGatewayV2DeleteStageCmd(),
		newAPIGatewayV2CreateDeploymentCmd(),
		newAPIGatewayV2GetDeploymentsCmd(),
	)

	return cmd
}

func apigatewayV2Client(cmd *cobra.Command) (*apigatewayv2.Client, error) {
	cfg, err := newAWSConfig(cmd.Context())
	if err != nil {
		return nil, err
	}

	return apigatewayv2.NewFromConfig(cfg, func(o *apigatewayv2.Options) {
		o.BaseEndpoint = aws.String(endpointURL)
	}), nil
}

func encodeAPIGatewayV2Output(out any) error {
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		return fmt.Errorf("failed to encode output: %w", err)
	}

	return nil
}

func newAPIGatewayV2CreateAPICmd() *cobra.Command {
	var name, protocolType, description string

	cmd := &cobra.Command{
		Use:   "create-api",
		Short: "Create an API",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := apigatewayV2Client(cmd)
			if err != nil {
				return err
			}

			input := &apigatewayv2.CreateApiInput{
				Name:         aws.String(name),
				ProtocolType: apigatewayv2Types.ProtocolType(protocolType),
			}

			if description != "" {
				input.Description = aws.String(description)
			}

			out, err := client.CreateApi(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("create-api failed: %w", err)
			}

			return encodeAPIGatewayV2Output(out)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "API name")
	cmd.Flags().StringVar(&protocolType, "protocol-type", "HTTP", "Protocol type (HTTP or WEBSOCKET)")
	cmd.Flags().StringVar(&description, "description", "", "API description")

	return cmd
}

func newAPIGatewayV2GetAPIsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get-apis",
		Short: "List APIs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := apigatewayV2Client(cmd)
			if err != nil {
				return err
			}

			out, err := client.GetApis(cmd.Context(), &apigatewayv2.GetApisInput{})
			if err != nil {
				return fmt.Errorf("get-apis failed: %w", err)
			}

			return encodeAPIGatewayV2Output(out)
		},
	}
}

func newAPIGatewayV2GetAPICmd() *cobra.Command {
	var apiID string

	cmd := &cobra.Command{
		Use:   "get-api",
		Short: "Get an API",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := apigatewayV2Client(cmd)
			if err != nil {
				return err
			}

			out, err := client.GetApi(cmd.Context(), &apigatewayv2.GetApiInput{
				ApiId: aws.String(apiID),
			})
			if err != nil {
				return fmt.Errorf("get-api failed: %w", err)
			}

			return encodeAPIGatewayV2Output(out)
		},
	}

	cmd.Flags().StringVar(&apiID, "api-id", "", "API ID")

	return cmd
}

func newAPIGatewayV2DeleteAPICmd() *cobra.Command {
	var apiID string

	cmd := &cobra.Command{
		Use:   "delete-api",
		Short: "Delete an API",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := apigatewayV2Client(cmd)
			if err != nil {
				return err
			}

			_, err = client.DeleteApi(cmd.Context(), &apigatewayv2.DeleteApiInput{
				ApiId: aws.String(apiID),
			})
			if err != nil {
				return fmt.Errorf("delete-api failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&apiID, "api-id", "", "API ID")

	return cmd
}

func newAPIGatewayV2CreateRouteCmd() *cobra.Command {
	var apiID, routeKey, target string

	cmd := &cobra.Command{
		Use:   "create-route",
		Short: "Create a route",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := apigatewayV2Client(cmd)
			if err != nil {
				return err
			}

			input := &apigatewayv2.CreateRouteInput{
				ApiId:    aws.String(apiID),
				RouteKey: aws.String(routeKey),
			}

			if target != "" {
				input.Target = aws.String(target)
			}

			out, err := client.CreateRoute(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("create-route failed: %w", err)
			}

			return encodeAPIGatewayV2Output(out)
		},
	}

	cmd.Flags().StringVar(&apiID, "api-id", "", "API ID")
	cmd.Flags().StringVar(&routeKey, "route-key", "", "Route key (e.g. 'GET /pets')")
	cmd.Flags().StringVar(&target, "target", "", "Route target (e.g. 'integrations/{id}')")

	return cmd
}

func newAPIGatewayV2GetRoutesCmd() *cobra.Command {
	var apiID string

	cmd := &cobra.Command{
		Use:   "get-routes",
		Short: "List routes",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := apigatewayV2Client(cmd)
			if err != nil {
				return err
			}

			out, err := client.GetRoutes(cmd.Context(), &apigatewayv2.GetRoutesInput{
				ApiId: aws.String(apiID),
			})
			if err != nil {
				return fmt.Errorf("get-routes failed: %w", err)
			}

			return encodeAPIGatewayV2Output(out)
		},
	}

	cmd.Flags().StringVar(&apiID, "api-id", "", "API ID")

	return cmd
}

func newAPIGatewayV2DeleteRouteCmd() *cobra.Command {
	var apiID, routeID string

	cmd := &cobra.Command{
		Use:   "delete-route",
		Short: "Delete a route",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := apigatewayV2Client(cmd)
			if err != nil {
				return err
			}

			_, err = client.DeleteRoute(cmd.Context(), &apigatewayv2.DeleteRouteInput{
				ApiId:   aws.String(apiID),
				RouteId: aws.String(routeID),
			})
			if err != nil {
				return fmt.Errorf("delete-route failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&apiID, "api-id", "", "API ID")
	cmd.Flags().StringVar(&routeID, "route-id", "", "Route ID")

	return cmd
}

func newAPIGatewayV2CreateIntegrationCmd() *cobra.Command {
	var apiID, integrationType, integrationURI, integrationMethod, payloadFormatVersion string

	cmd := &cobra.Command{
		Use:   "create-integration",
		Short: "Create an integration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := apigatewayV2Client(cmd)
			if err != nil {
				return err
			}

			input := &apigatewayv2.CreateIntegrationInput{
				ApiId:           aws.String(apiID),
				IntegrationType: apigatewayv2Types.IntegrationType(integrationType),
			}

			if integrationURI != "" {
				input.IntegrationUri = aws.String(integrationURI)
			}

			if integrationMethod != "" {
				input.IntegrationMethod = aws.String(integrationMethod)
			}

			if payloadFormatVersion != "" {
				input.PayloadFormatVersion = aws.String(payloadFormatVersion)
			}

			out, err := client.CreateIntegration(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("create-integration failed: %w", err)
			}

			return encodeAPIGatewayV2Output(out)
		},
	}

	cmd.Flags().StringVar(&apiID, "api-id", "", "API ID")
	cmd.Flags().StringVar(&integrationType, "integration-type", "", "Integration type (AWS_PROXY, HTTP_PROXY, MOCK, etc.)")
	cmd.Flags().StringVar(&integrationURI, "integration-uri", "", "Integration URI")
	cmd.Flags().StringVar(&integrationMethod, "integration-method", "", "Integration HTTP method")
	cmd.Flags().StringVar(&payloadFormatVersion, "payload-format-version", "", "Payload format version (1.0 or 2.0)")

	return cmd
}

func newAPIGatewayV2GetIntegrationsCmd() *cobra.Command {
	var apiID string

	cmd := &cobra.Command{
		Use:   "get-integrations",
		Short: "List integrations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := apigatewayV2Client(cmd)
			if err != nil {
				return err
			}

			out, err := client.GetIntegrations(cmd.Context(), &apigatewayv2.GetIntegrationsInput{
				ApiId: aws.String(apiID),
			})
			if err != nil {
				return fmt.Errorf("get-integrations failed: %w", err)
			}

			return encodeAPIGatewayV2Output(out)
		},
	}

	cmd.Flags().StringVar(&apiID, "api-id", "", "API ID")

	return cmd
}

func newAPIGatewayV2DeleteIntegrationCmd() *cobra.Command {
	var apiID, integrationID string

	cmd := &cobra.Command{
		Use:   "delete-integration",
		Short: "Delete an integration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := apigatewayV2Client(cmd)
			if err != nil {
				return err
			}

			_, err = client.DeleteIntegration(cmd.Context(), &apigatewayv2.DeleteIntegrationInput{
				ApiId:         aws.String(apiID),
				IntegrationId: aws.String(integrationID),
			})
			if err != nil {
				return fmt.Errorf("delete-integration failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&apiID, "api-id", "", "API ID")
	cmd.Flags().StringVar(&integrationID, "integration-id", "", "Integration ID")

	return cmd
}

func newAPIGatewayV2CreateStageCmd() *cobra.Command {
	var apiID, stageName, description string

	var autoDeploy bool

	cmd := &cobra.Command{
		Use:   "create-stage",
		Short: "Create a stage",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := apigatewayV2Client(cmd)
			if err != nil {
				return err
			}

			input := &apigatewayv2.CreateStageInput{
				ApiId:      aws.String(apiID),
				StageName:  aws.String(stageName),
				AutoDeploy: aws.Bool(autoDeploy),
			}

			if description != "" {
				input.Description = aws.String(description)
			}

			out, err := client.CreateStage(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("create-stage failed: %w", err)
			}

			return encodeAPIGatewayV2Output(out)
		},
	}

	cmd.Flags().StringVar(&apiID, "api-id", "", "API ID")
	cmd.Flags().StringVar(&stageName, "stage-name", "", "Stage name")
	cmd.Flags().StringVar(&description, "description", "", "Stage description")
	cmd.Flags().BoolVar(&autoDeploy, "auto-deploy", false, "Auto deploy changes")

	return cmd
}

func newAPIGatewayV2GetStagesCmd() *cobra.Command {
	var apiID string

	cmd := &cobra.Command{
		Use:   "get-stages",
		Short: "List stages",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := apigatewayV2Client(cmd)
			if err != nil {
				return err
			}

			out, err := client.GetStages(cmd.Context(), &apigatewayv2.GetStagesInput{
				ApiId: aws.String(apiID),
			})
			if err != nil {
				return fmt.Errorf("get-stages failed: %w", err)
			}

			return encodeAPIGatewayV2Output(out)
		},
	}

	cmd.Flags().StringVar(&apiID, "api-id", "", "API ID")

	return cmd
}

func newAPIGatewayV2DeleteStageCmd() *cobra.Command {
	var apiID, stageName string

	cmd := &cobra.Command{
		Use:   "delete-stage",
		Short: "Delete a stage",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := apigatewayV2Client(cmd)
			if err != nil {
				return err
			}

			_, err = client.DeleteStage(cmd.Context(), &apigatewayv2.DeleteStageInput{
				ApiId:     aws.String(apiID),
				StageName: aws.String(stageName),
			})
			if err != nil {
				return fmt.Errorf("delete-stage failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&apiID, "api-id", "", "API ID")
	cmd.Flags().StringVar(&stageName, "stage-name", "", "Stage name")

	return cmd
}

func newAPIGatewayV2CreateDeploymentCmd() *cobra.Command {
	var apiID, stageName, description string

	cmd := &cobra.Command{
		Use:   "create-deployment",
		Short: "Create a deployment",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := apigatewayV2Client(cmd)
			if err != nil {
				return err
			}

			input := &apigatewayv2.CreateDeploymentInput{
				ApiId: aws.String(apiID),
			}

			if stageName != "" {
				input.StageName = aws.String(stageName)
			}

			if description != "" {
				input.Description = aws.String(description)
			}

			out, err := client.CreateDeployment(cmd.Context(), input)
			if err != nil {
				return fmt.Errorf("create-deployment failed: %w", err)
			}

			return encodeAPIGatewayV2Output(out)
		},
	}

	cmd.Flags().StringVar(&apiID, "api-id", "", "API ID")
	cmd.Flags().StringVar(&stageName, "stage-name", "", "Stage name to deploy to")
	cmd.Flags().StringVar(&description, "description", "", "Deployment description")

	return cmd
}

func newAPIGatewayV2GetDeploymentsCmd() *cobra.Command {
	var apiID string

	cmd := &cobra.Command{
		Use:   "get-deployments",
		Short: "List deployments",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := apigatewayV2Client(cmd)
			if err != nil {
				return err
			}

			out, err := client.GetDeployments(cmd.Context(), &apigatewayv2.GetDeploymentsInput{
				ApiId: aws.String(apiID),
			})
			if err != nil {
				return fmt.Errorf("get-deployments failed: %w", err)
			}

			return encodeAPIGatewayV2Output(out)
		},
	}

	cmd.Flags().StringVar(&apiID, "api-id", "", "API ID")

	return cmd
}
