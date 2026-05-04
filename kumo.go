// Package kumo provides a public API for running an in-process AWS service emulator.
//
// Usage:
//
//	srv := kumo.NewServer()
//	defer srv.Close()
//
//	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
//	    o.BaseEndpoint = aws.String(srv.URL)
//	})
package kumo

import (
	"net/http/httptest"

	"github.com/sivchari/kumo/internal/server"
	_ "github.com/sivchari/kumo/internal/service/acm"
	_ "github.com/sivchari/kumo/internal/service/amplify"
	_ "github.com/sivchari/kumo/internal/service/apigateway"
	_ "github.com/sivchari/kumo/internal/service/appmesh"
	_ "github.com/sivchari/kumo/internal/service/appsync"
	_ "github.com/sivchari/kumo/internal/service/athena"
	_ "github.com/sivchari/kumo/internal/service/backup"
	_ "github.com/sivchari/kumo/internal/service/batch"
	_ "github.com/sivchari/kumo/internal/service/ce"
	_ "github.com/sivchari/kumo/internal/service/cloudformation"
	_ "github.com/sivchari/kumo/internal/service/cloudfront"
	_ "github.com/sivchari/kumo/internal/service/cloudtrail"
	_ "github.com/sivchari/kumo/internal/service/cloudwatch"
	_ "github.com/sivchari/kumo/internal/service/cloudwatchlogs"
	_ "github.com/sivchari/kumo/internal/service/codecommit"
	_ "github.com/sivchari/kumo/internal/service/codeconnections"
	_ "github.com/sivchari/kumo/internal/service/codeguruprofiler"
	_ "github.com/sivchari/kumo/internal/service/codegurureviewer"
	_ "github.com/sivchari/kumo/internal/service/cognito"
	_ "github.com/sivchari/kumo/internal/service/comprehend"
	_ "github.com/sivchari/kumo/internal/service/configservice"
	_ "github.com/sivchari/kumo/internal/service/dataexchange"
	_ "github.com/sivchari/kumo/internal/service/dlm"
	_ "github.com/sivchari/kumo/internal/service/documentdb"
	_ "github.com/sivchari/kumo/internal/service/ds"
	_ "github.com/sivchari/kumo/internal/service/dynamodb"
	_ "github.com/sivchari/kumo/internal/service/ebs"
	_ "github.com/sivchari/kumo/internal/service/ec2"
	_ "github.com/sivchari/kumo/internal/service/ecr"
	_ "github.com/sivchari/kumo/internal/service/ecs"
	_ "github.com/sivchari/kumo/internal/service/eks"
	_ "github.com/sivchari/kumo/internal/service/elasticache"
	_ "github.com/sivchari/kumo/internal/service/elasticbeanstalk"
	_ "github.com/sivchari/kumo/internal/service/elbv2"
	_ "github.com/sivchari/kumo/internal/service/emrserverless"
	_ "github.com/sivchari/kumo/internal/service/entityresolution"
	_ "github.com/sivchari/kumo/internal/service/eventbridge"
	_ "github.com/sivchari/kumo/internal/service/finspace"
	_ "github.com/sivchari/kumo/internal/service/firehose"
	_ "github.com/sivchari/kumo/internal/service/forecast"
	_ "github.com/sivchari/kumo/internal/service/gamelift"
	_ "github.com/sivchari/kumo/internal/service/glacier"
	_ "github.com/sivchari/kumo/internal/service/globalaccelerator"
	_ "github.com/sivchari/kumo/internal/service/glue"
	_ "github.com/sivchari/kumo/internal/service/iam"
	_ "github.com/sivchari/kumo/internal/service/kafka"
	_ "github.com/sivchari/kumo/internal/service/kinesis"
	_ "github.com/sivchari/kumo/internal/service/kms"
	_ "github.com/sivchari/kumo/internal/service/lambda"
	_ "github.com/sivchari/kumo/internal/service/location"
	_ "github.com/sivchari/kumo/internal/service/macie2"
	_ "github.com/sivchari/kumo/internal/service/memorydb"
	_ "github.com/sivchari/kumo/internal/service/mq"
	_ "github.com/sivchari/kumo/internal/service/neptune"
	_ "github.com/sivchari/kumo/internal/service/organizations"
	_ "github.com/sivchari/kumo/internal/service/pipes"
	_ "github.com/sivchari/kumo/internal/service/rds"
	_ "github.com/sivchari/kumo/internal/service/redshift"
	_ "github.com/sivchari/kumo/internal/service/rekognition"
	_ "github.com/sivchari/kumo/internal/service/resiliencehub"
	_ "github.com/sivchari/kumo/internal/service/route53"
	_ "github.com/sivchari/kumo/internal/service/route53resolver"
	_ "github.com/sivchari/kumo/internal/service/s3"
	_ "github.com/sivchari/kumo/internal/service/s3control"
	_ "github.com/sivchari/kumo/internal/service/s3tables"
	_ "github.com/sivchari/kumo/internal/service/sagemaker"
	_ "github.com/sivchari/kumo/internal/service/scheduler"
	_ "github.com/sivchari/kumo/internal/service/secretsmanager"
	_ "github.com/sivchari/kumo/internal/service/securitylake"
	_ "github.com/sivchari/kumo/internal/service/servicequotas"
	_ "github.com/sivchari/kumo/internal/service/sesv2"
	_ "github.com/sivchari/kumo/internal/service/sfn"
	_ "github.com/sivchari/kumo/internal/service/sns"
	_ "github.com/sivchari/kumo/internal/service/sqs"
	_ "github.com/sivchari/kumo/internal/service/ssm"
	_ "github.com/sivchari/kumo/internal/service/sts"
	_ "github.com/sivchari/kumo/internal/service/xray"
)

// Server is an in-process AWS service emulator.
// It wraps httptest.Server to provide a familiar API for Go testing.
type Server struct {
	// URL is the base URL of the server in the form "http://host:port".
	URL string

	httpServer *httptest.Server
}

// NewServer creates and starts a new in-process AWS emulator server.
// The server listens on a random available port on localhost.
// Use srv.URL as the BaseEndpoint for AWS SDK clients.
func NewServer() *Server {
	cfg := server.DefaultConfig()
	cfg.LogLevel = 100 // Suppress all logs in test mode.
	internalSrv := server.New(cfg)

	ts := httptest.NewServer(internalSrv.Handler())

	return &Server{
		URL:        ts.URL,
		httpServer: ts,
	}
}

// Close shuts down the server.
func (s *Server) Close() {
	if s.httpServer != nil {
		s.httpServer.Close()
	}
}
