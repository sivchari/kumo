package cligen

import (
	"reflect"

	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/amplify"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/appmesh"
	"github.com/aws/aws-sdk-go-v2/service/appsync"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/firehose"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// sdkBinding describes how a kumo service maps onto an aws-sdk-go-v2 client:
// the concrete client type (used for reflection-based method resolution), the
// import path used by generated CLI files, and the Go identifier prefix used
// for generated command constructors (new{CLIName}Cmd), matching the existing
// hand-written cli/*.go convention (e.g. eventbridge -> "Events", to match
// cli/events.go).
type sdkBinding struct {
	ClientType reflect.Type
	ImportPath string
	CLIName    string
	// CommandUse overrides the cobra Use for the service command when the
	// AWS CLI's command name differs from Service.Name() (e.g. the Step
	// Functions service is named "states" but the AWS CLI command is
	// "stepfunctions"). Empty means use Service.Name().
	CommandUse string
}

// sdkBindings is the static "serviceName -> aws-sdk-go-v2 client" map seeding
// PR1's auto-discovery. It is intentionally small (the 13 services planned
// for the first migration batch); services with a JSON/Query protocol but no
// entry here are skipped with a diagnostic rather than generated, and adding
// support for another service is a one-line addition here.
//
// Because reflect cannot import an arbitrary package by string at runtime,
// each entry's ClientType is obtained via a real Go import of the
// corresponding aws-sdk-go-v2 service package.
var sdkBindings = map[string]sdkBinding{
	"acm": {
		ClientType: reflect.TypeOf(acm.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/acm",
		CLIName:    "ACM",
	},
	"amplify": {
		ClientType: reflect.TypeOf(amplify.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/amplify",
		CLIName:    "Amplify",
	},
	"apigateway": {
		ClientType: reflect.TypeOf(apigateway.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/apigateway",
		CLIName:    "APIGateway",
	},
	"apigatewayv2": {
		ClientType: reflect.TypeOf(apigatewayv2.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/apigatewayv2",
		CLIName:    "APIGatewayV2",
	},
	"appmesh": {
		ClientType: reflect.TypeOf(appmesh.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/appmesh",
		CLIName:    "AppMesh",
	},
	"appsync": {
		ClientType: reflect.TypeOf(appsync.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/appsync",
		CLIName:    "AppSync",
	},
	"athena": {
		ClientType: reflect.TypeOf(athena.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/athena",
		CLIName:    "Athena",
	},
	"backup": {
		ClientType: reflect.TypeOf(backup.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/backup",
		CLIName:    "Backup",
	},
	"dynamodb": {
		ClientType: reflect.TypeOf(dynamodb.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/dynamodb",
		CLIName:    "DynamoDB",
	},
	// The kumo service package is internal/service/eventbridge, but its
	// Service.Name() returns "events" (matching cli/events.go's existing
	// newEventsCmd / "events" Use string) - sdkBindings is keyed by
	// service.Service.Name(), not by package/directory name.
	"events": {
		ClientType: reflect.TypeOf(eventbridge.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/eventbridge",
		CLIName:    "Events",
	},
	"kinesis": {
		ClientType: reflect.TypeOf(kinesis.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/kinesis",
		CLIName:    "Kinesis",
	},
	"kms": {
		ClientType: reflect.TypeOf(kms.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/kms",
		CLIName:    "KMS",
	},
	"secretsmanager": {
		ClientType: reflect.TypeOf(secretsmanager.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/secretsmanager",
		CLIName:    "SecretsManager",
	},
	"sns": {
		ClientType: reflect.TypeOf(sns.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/sns",
		CLIName:    "SNS",
	},
	"sqs": {
		ClientType: reflect.TypeOf(sqs.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/sqs",
		CLIName:    "SQS",
	},
	"ssm": {
		ClientType: reflect.TypeOf(ssm.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/ssm",
		CLIName:    "SSM",
	},
	"sts": {
		ClientType: reflect.TypeOf(sts.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/sts",
		CLIName:    "STS",
	},
	// The kumo service package is internal/service/cloudwatchlogs, but its
	// Service.Name() returns "logs" (matching the AWS CLI's own `aws logs`
	// command name) - keyed by service.Service.Name(), not package name.
	"logs": {
		ClientType: reflect.TypeOf(cloudwatchlogs.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs",
		CLIName:    "Logs",
	},
	// The kumo service package is internal/service/sfn, but its
	// Service.Name() returns "states" (matching the AWS CLI's own
	// `aws stepfunctions` service identifier, which uses "states" as the
	// API/X-Amz-Target namespace) - keyed by service.Service.Name().
	"states": {
		ClientType: reflect.TypeOf(sfn.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/sfn",
		CLIName:    "SFN",
		CommandUse: "stepfunctions",
	},
	"iam": {
		ClientType: reflect.TypeOf(iam.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/iam",
		CLIName:    "IAM",
	},
	"cloudformation": {
		ClientType: reflect.TypeOf(cloudformation.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/cloudformation",
		CLIName:    "CloudFormation",
	},
	"ec2": {
		ClientType: reflect.TypeOf(ec2.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/ec2",
		CLIName:    "EC2",
	},
	"ecr": {
		ClientType: reflect.TypeOf(ecr.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/ecr",
		CLIName:    "ECR",
	},
	"ecs": {
		ClientType: reflect.TypeOf(ecs.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/ecs",
		CLIName:    "ECS",
	},
	// The kumo service package is internal/service/cloudwatch, but its
	// Service.Name() returns "monitoring" (the AWS service ID); the AWS
	// CLI's own command name is "cloudwatch" - keyed by
	// service.Service.Name(), with CommandUse overriding the cobra Use.
	"monitoring": {
		ClientType: reflect.TypeOf(cloudwatch.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/cloudwatch",
		CLIName:    "CloudWatch",
		CommandUse: "cloudwatch",
	},
	"dynamodbstreams": {
		ClientType: reflect.TypeOf(dynamodbstreams.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/dynamodbstreams",
		CLIName:    "DynamoDBStreams",
	},
	"firehose": {
		ClientType: reflect.TypeOf(firehose.Client{}),
		ImportPath: "github.com/aws/aws-sdk-go-v2/service/firehose",
		CLIName:    "Firehose",
	},
}
