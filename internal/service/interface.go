// Package service provides interfaces and utilities for AWS service implementations.
package service

import (
	"net/http"
)

// Service is the common interface for all AWS service implementations.
type Service interface {
	// Name returns the service name (e.g., "s3", "sqs", "dynamodb").
	Name() string

	// RegisterRoutes registers the service's routes with the router.
	RegisterRoutes(r Router)
}

// Router is the interface for registering HTTP routes.
type Router interface {
	// Handle registers a handler for the given method and pattern.
	Handle(method, pattern string, handler http.HandlerFunc)

	// HandleFunc is an alias for Handle for compatibility.
	HandleFunc(method, pattern string, handler http.HandlerFunc)
}

// Handler is the interface for operation handlers.
type Handler interface {
	// ServeHTTP handles the HTTP request.
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

// ExecuteAPIHandler is an optional interface for services that expose a
// deployed API invoke surface (execute-api). The router dispatches
// virtual-hosted execute-api requests ({apiId}.execute-api.<host>) to it.
type ExecuteAPIHandler interface {
	// HandleExecuteAPI handles an invocation for apiID. invokePath is the
	// request path after the host, e.g. "/dev/items" (including the stage
	// segment). It returns false when apiID is not managed by this service,
	// so the router can try another handler.
	HandleExecuteAPI(w http.ResponseWriter, r *http.Request, apiID, invokePath string) bool
}

// JSONProtocolService is an optional interface for services using AWS JSON 1.0 protocol.
// Services implementing this interface will have their handlers dispatched via
// a unified POST / endpoint based on the X-Amz-Target header.
type JSONProtocolService interface {
	// TargetPrefix returns the X-Amz-Target prefix for this service.
	// e.g., "AmazonSQS" for SQS, "DynamoDB_20120810" for DynamoDB
	TargetPrefix() string

	// DispatchAction handles the JSON protocol request after routing.
	DispatchAction(w http.ResponseWriter, r *http.Request)

	// JSONProtocol is a marker method to distinguish from QueryProtocolService.
	JSONProtocol()
}

// QueryProtocolService is an optional interface for services using AWS Query protocol.
// Services implementing this interface will have their handlers dispatched via
// a unified POST / endpoint, with form data converted to JSON before dispatch.
type QueryProtocolService interface {
	// TargetPrefix returns the target prefix for this service.
	// This is used to set the X-Amz-Target header after converting
	// the Query request to JSON format.
	// e.g., "AmazonSimpleNotificationService" for SNS
	TargetPrefix() string

	// DispatchAction handles the request after Query-to-JSON conversion.
	DispatchAction(w http.ResponseWriter, r *http.Request)

	// Actions returns the list of action names this service handles.
	// This is used by the dispatcher to route requests to the correct service.
	Actions() []string

	// ServiceIdentifier returns the SDK service identifier sent in the User-Agent header.
	// e.g., "rds", "neptune", "ec2"
	// This is used to disambiguate actions shared by multiple Query protocol services.
	ServiceIdentifier() string

	// QueryProtocol is a marker method to distinguish from JSONProtocolService.
	QueryProtocol()
}

// CBORProtocolService is an optional interface for services using Smithy RPC v2 CBOR protocol.
// Services implementing this interface will have their handlers dispatched via
// URL-based routing: /service/{serviceName}/operation/{operationName}.
type CBORProtocolService interface {
	// ServiceName returns the Smithy service name used in the URL path.
	// e.g., "GraniteServiceVersion20100801" for CloudWatch
	ServiceName() string

	// DispatchCBORAction handles the RPC v2 CBOR protocol request.
	// The operation name is extracted from the URL path.
	DispatchCBORAction(w http.ResponseWriter, r *http.Request, operation string)

	// CBORProtocol is a marker method for CBOR protocol services.
	CBORProtocol()
}
