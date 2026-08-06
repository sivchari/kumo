package cloudcontrol

import (
	"fmt"

	"github.com/sivchari/kumo/internal/service"
)

// lookupStorage finds the registered Service named serviceName and casts
// it to a type exposing `Storage() T`, letting resource handlers share the
// same in-memory store as the underlying service without coupling init()
// ordering between packages or knowing its concrete struct.
func lookupStorage[T any](serviceName string) (T, error) {
	var zero T

	for _, svc := range service.Services() {
		if svc.Name() != serviceName {
			continue
		}

		provider, ok := svc.(interface{ Storage() T })
		if !ok {
			return zero, fmt.Errorf("%s service does not expose Storage() returning the expected type", serviceName)
		}

		return provider.Storage(), nil
	}

	return zero, fmt.Errorf("%s service is not registered", serviceName)
}
