package server

import (
	"github.com/sivchari/kumo/internal/service"
	"github.com/sivchari/kumo/internal/service/apigateway"
)

// wireAPIGatewayToLambda injects the Lambda service into the API Gateway
// service so that REQUEST-type Lambda authorizers can be invoked in-process
// during execute-api dispatch.
//
// Without this wiring, API Gateway has no way to call the authorizer Lambda and
// every CUSTOM-authorized method is denied (apigateway.Service.invoker is nil).
// The pattern mirrors wireSNStoSQS: it runs once after all services are
// registered. The Lambda service is matched structurally against
// apigateway.LambdaInvoker, so this file does not import the lambda package and
// no import cycle is introduced.
func wireAPIGatewayToLambda(registry *service.Registry) {
	apigwSvc, ok := registry.Get("apigateway")
	if !ok {
		return
	}

	lambdaSvc, ok := registry.Get("lambda")
	if !ok {
		return
	}

	apigwTyped, ok := apigwSvc.(*apigateway.Service)
	if !ok {
		return
	}

	invoker, ok := lambdaSvc.(apigateway.LambdaInvoker)
	if !ok {
		return
	}

	apigwTyped.SetLambdaInvoker(invoker)
}
