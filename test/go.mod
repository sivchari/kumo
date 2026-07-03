module github.com/sivchari/kumo/test

go 1.25.0

require (
	github.com/aws/aws-lambda-go v1.54.0
	github.com/aws/aws-sdk-go v1.55.8
	github.com/aws/aws-sdk-go-v2 v1.41.9
	github.com/aws/aws-sdk-go-v2/config v1.32.7
	github.com/aws/aws-sdk-go-v2/credentials v1.19.7
	github.com/aws/aws-sdk-go-v2/service/acm v1.37.19
	github.com/aws/aws-sdk-go-v2/service/amplify v1.38.12
	github.com/aws/aws-sdk-go-v2/service/apigateway v1.38.4
	github.com/aws/aws-sdk-go-v2/service/apigatewayv2 v1.35.2
	github.com/aws/aws-sdk-go-v2/service/appmesh v1.35.9
	github.com/aws/aws-sdk-go-v2/service/appsync v1.53.1
	github.com/aws/aws-sdk-go-v2/service/athena v1.57.0
	github.com/aws/aws-sdk-go-v2/service/backup v1.54.8
	github.com/aws/aws-sdk-go-v2/service/batch v1.60.0
	github.com/aws/aws-sdk-go-v2/service/cloudcontrol v1.29.14
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.71.6
	github.com/aws/aws-sdk-go-v2/service/cloudfront v1.60.0
	github.com/aws/aws-sdk-go-v2/service/cloudtrail v1.55.6
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.54.0
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.63.1
	github.com/aws/aws-sdk-go-v2/service/codeconnections v1.10.16
	github.com/aws/aws-sdk-go-v2/service/codeguruprofiler v1.29.19
	github.com/aws/aws-sdk-go-v2/service/codegurureviewer v1.34.18
	github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider v1.58.0
	github.com/aws/aws-sdk-go-v2/service/comprehend v1.40.18
	github.com/aws/aws-sdk-go-v2/service/configservice v1.61.1
	github.com/aws/aws-sdk-go-v2/service/costexplorer v1.63.3
	github.com/aws/aws-sdk-go-v2/service/dataexchange v1.40.12
	github.com/aws/aws-sdk-go-v2/service/directoryservice v1.37.0
	github.com/aws/aws-sdk-go-v2/service/dlm v1.35.13
	github.com/aws/aws-sdk-go-v2/service/docdb v1.48.11
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.55.0
	github.com/aws/aws-sdk-go-v2/service/dynamodbstreams v1.32.16
	github.com/aws/aws-sdk-go-v2/service/ebs v1.33.12
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.285.0
	github.com/aws/aws-sdk-go-v2/service/ecr v1.55.1
	github.com/aws/aws-sdk-go-v2/service/ecs v1.71.0
	github.com/aws/aws-sdk-go-v2/service/eks v1.80.0
	github.com/aws/aws-sdk-go-v2/service/elasticache v1.51.9
	github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk v1.34.0
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2 v1.54.7
	github.com/aws/aws-sdk-go-v2/service/emrserverless v1.39.3
	github.com/aws/aws-sdk-go-v2/service/entityresolution v1.26.3
	github.com/aws/aws-sdk-go-v2/service/eventbridge v1.45.18
	github.com/aws/aws-sdk-go-v2/service/finspace v1.33.18
	github.com/aws/aws-sdk-go-v2/service/firehose v1.42.9
	github.com/aws/aws-sdk-go-v2/service/forecast v1.41.18
	github.com/aws/aws-sdk-go-v2/service/gamelift v1.50.1
	github.com/aws/aws-sdk-go-v2/service/glacier v1.32.4
	github.com/aws/aws-sdk-go-v2/service/globalaccelerator v1.35.11
	github.com/aws/aws-sdk-go-v2/service/glue v1.137.0
	github.com/aws/aws-sdk-go-v2/service/iam v1.53.2
	github.com/aws/aws-sdk-go-v2/service/kafka v1.49.0
	github.com/aws/aws-sdk-go-v2/service/kinesis v1.43.0
	github.com/aws/aws-sdk-go-v2/service/kms v1.49.5
	github.com/aws/aws-sdk-go-v2/service/lambda v1.88.0
	github.com/aws/aws-sdk-go-v2/service/location v1.50.13
	github.com/aws/aws-sdk-go-v2/service/macie2 v1.50.13
	github.com/aws/aws-sdk-go-v2/service/memorydb v1.33.12
	github.com/aws/aws-sdk-go-v2/service/mq v1.33.1
	github.com/aws/aws-sdk-go-v2/service/neptune v1.44.3
	github.com/aws/aws-sdk-go-v2/service/organizations v1.50.3
	github.com/aws/aws-sdk-go-v2/service/pinpointsmsvoicev2 v1.28.1
	github.com/aws/aws-sdk-go-v2/service/pipes v1.23.17
	github.com/aws/aws-sdk-go-v2/service/rds v1.115.0
	github.com/aws/aws-sdk-go-v2/service/redshift v1.62.5
	github.com/aws/aws-sdk-go-v2/service/rekognition v1.51.17
	github.com/aws/aws-sdk-go-v2/service/resiliencehub v1.35.10
	github.com/aws/aws-sdk-go-v2/service/route53 v1.62.2
	github.com/aws/aws-sdk-go-v2/service/route53resolver v1.42.2
	github.com/aws/aws-sdk-go-v2/service/s3 v1.96.0
	github.com/aws/aws-sdk-go-v2/service/s3control v1.68.1
	github.com/aws/aws-sdk-go-v2/service/s3tables v1.14.0
	github.com/aws/aws-sdk-go-v2/service/sagemaker v1.233.1
	github.com/aws/aws-sdk-go-v2/service/scheduler v1.17.19
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.41.1
	github.com/aws/aws-sdk-go-v2/service/securitylake v1.25.10
	github.com/aws/aws-sdk-go-v2/service/servicequotas v1.34.2
	github.com/aws/aws-sdk-go-v2/service/ses v1.34.24
	github.com/aws/aws-sdk-go-v2/service/sesv2 v1.59.1
	github.com/aws/aws-sdk-go-v2/service/sfn v1.40.6
	github.com/aws/aws-sdk-go-v2/service/sns v1.39.11
	github.com/aws/aws-sdk-go-v2/service/sqs v1.42.21
	github.com/aws/aws-sdk-go-v2/service/ssm v1.67.8
	github.com/aws/aws-sdk-go-v2/service/sts v1.41.6
	github.com/aws/aws-sdk-go-v2/service/xray v1.36.17
	github.com/aws/smithy-go v1.26.0
	github.com/sivchari/golden v0.3.0
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.4 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.25 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.25 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.11.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.21 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.18 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.0.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.13 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
)

replace github.com/sivchari/kumo => ../
