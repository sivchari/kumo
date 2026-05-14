module github.com/sivchari/kumo

go 1.25.0

require (
	github.com/aws/aws-sdk-go-v2 v1.41.7
	github.com/aws/aws-sdk-go-v2/config v1.32.17
	github.com/aws/aws-sdk-go-v2/credentials v1.19.16
	github.com/aws/aws-sdk-go-v2/service/acm v1.38.3
	github.com/aws/aws-sdk-go-v2/service/amplify v1.38.16
	github.com/aws/aws-sdk-go-v2/service/apigateway v1.39.3
	github.com/aws/aws-sdk-go-v2/service/appmesh v1.35.14
	github.com/aws/aws-sdk-go-v2/service/appsync v1.53.7
	github.com/aws/aws-sdk-go-v2/service/athena v1.57.6
	github.com/aws/aws-sdk-go-v2/service/backup v1.55.2
	github.com/aws/aws-sdk-go-v2/service/batch v1.64.2
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.71.11
	github.com/aws/aws-sdk-go-v2/service/cloudfront v1.64.0
	github.com/aws/aws-sdk-go-v2/service/cloudtrail v1.55.11
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.57.0
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.73.0
	github.com/aws/aws-sdk-go-v2/service/codeconnections v1.10.22
	github.com/aws/aws-sdk-go-v2/service/codeguruprofiler v1.29.22
	github.com/aws/aws-sdk-go-v2/service/codegurureviewer v1.34.22
	github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider v1.60.2
	github.com/aws/aws-sdk-go-v2/service/comprehend v1.40.23
	github.com/aws/aws-sdk-go-v2/service/configservice v1.62.3
	github.com/aws/aws-sdk-go-v2/service/costexplorer v1.63.8
	github.com/aws/aws-sdk-go-v2/service/dataexchange v1.41.2
	github.com/aws/aws-sdk-go-v2/service/directoryservice v1.38.18
	github.com/aws/aws-sdk-go-v2/service/dlm v1.36.2
	github.com/aws/aws-sdk-go-v2/service/docdb v1.48.15
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.57.3
	github.com/aws/aws-sdk-go-v2/service/ebs v1.33.16
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.302.0
	github.com/aws/aws-sdk-go-v2/service/ecr v1.57.2
	github.com/aws/aws-sdk-go-v2/service/ecs v1.79.1
	github.com/aws/aws-sdk-go-v2/service/eks v1.83.0
	github.com/aws/aws-sdk-go-v2/service/elasticache v1.52.2
	github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk v1.34.4
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2 v1.54.12
	github.com/aws/aws-sdk-go-v2/service/emrserverless v1.40.2
	github.com/aws/aws-sdk-go-v2/service/entityresolution v1.27.0
	github.com/aws/aws-sdk-go-v2/service/eventbridge v1.45.25
	github.com/aws/aws-sdk-go-v2/service/finspace v1.33.23
	github.com/aws/aws-sdk-go-v2/service/firehose v1.42.16
	github.com/aws/aws-sdk-go-v2/service/forecast v1.41.23
	github.com/aws/aws-sdk-go-v2/service/gamelift v1.54.0
	github.com/aws/aws-sdk-go-v2/service/glacier v1.32.8
	github.com/aws/aws-sdk-go-v2/service/globalaccelerator v1.35.18
	github.com/aws/aws-sdk-go-v2/service/glue v1.142.0
	github.com/aws/aws-sdk-go-v2/service/iam v1.53.10
	github.com/aws/aws-sdk-go-v2/service/kafka v1.51.0
	github.com/aws/aws-sdk-go-v2/service/kinesis v1.43.7
	github.com/aws/aws-sdk-go-v2/service/kms v1.51.1
	github.com/aws/aws-sdk-go-v2/service/lambda v1.90.1
	github.com/aws/aws-sdk-go-v2/service/location v1.51.1
	github.com/aws/aws-sdk-go-v2/service/macie2 v1.51.2
	github.com/aws/aws-sdk-go-v2/service/memorydb v1.33.16
	github.com/aws/aws-sdk-go-v2/service/mq v1.34.22
	github.com/aws/aws-sdk-go-v2/service/neptune v1.44.5
	github.com/aws/aws-sdk-go-v2/service/organizations v1.51.3
	github.com/aws/aws-sdk-go-v2/service/pinpointsmsvoicev2 v1.28.1
	github.com/aws/aws-sdk-go-v2/service/pipes v1.23.22
	github.com/aws/aws-sdk-go-v2/service/rds v1.118.2
	github.com/aws/aws-sdk-go-v2/service/redshift v1.62.8
	github.com/aws/aws-sdk-go-v2/service/rekognition v1.51.24
	github.com/aws/aws-sdk-go-v2/service/resiliencehub v1.35.15
	github.com/aws/aws-sdk-go-v2/service/route53 v1.62.7
	github.com/aws/aws-sdk-go-v2/service/route53resolver v1.43.0
	github.com/aws/aws-sdk-go-v2/service/s3 v1.100.1
	github.com/aws/aws-sdk-go-v2/service/s3control v1.70.1
	github.com/aws/aws-sdk-go-v2/service/s3tables v1.15.2
	github.com/aws/aws-sdk-go-v2/service/sagemaker v1.247.0
	github.com/aws/aws-sdk-go-v2/service/scheduler v1.17.24
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.41.7
	github.com/aws/aws-sdk-go-v2/service/securitylake v1.25.15
	github.com/aws/aws-sdk-go-v2/service/servicequotas v1.34.7
	github.com/aws/aws-sdk-go-v2/service/sesv2 v1.60.4
	github.com/aws/aws-sdk-go-v2/service/sfn v1.41.0
	github.com/aws/aws-sdk-go-v2/service/sns v1.39.17
	github.com/aws/aws-sdk-go-v2/service/sqs v1.42.27
	github.com/aws/aws-sdk-go-v2/service/ssm v1.68.6
	github.com/aws/aws-sdk-go-v2/service/sts v1.42.1
	github.com/aws/aws-sdk-go-v2/service/xray v1.36.23
	github.com/aws/smithy-go v1.25.1
	github.com/fxamacker/cbor/v2 v2.9.0
	github.com/google/uuid v1.6.0
	github.com/sivchari/golden v0.3.0
	github.com/spf13/cobra v1.10.2
	golang.org/x/crypto v0.47.0
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.10 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.23 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.23 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.23 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.24 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.11.23 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.23 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.23 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.0.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.21 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	golang.org/x/sys v0.40.0 // indirect
)
