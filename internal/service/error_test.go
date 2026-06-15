package service_test

import (
	"errors"
	"testing"

	"github.com/sivchari/kumo/internal/service"
)

func TestCodedError(t *testing.T) {
	t.Parallel()

	err := error(&service.CodedError{Code: "ValidationException", Message: "bad input"})

	if err.Error() != "bad input" {
		t.Errorf("Error() = %q, want %q", err.Error(), "bad input")
	}

	var ce *service.CodedError
	if !errors.As(err, &ce) {
		t.Fatal("errors.As failed to match *CodedError")
	}

	if ce.Code != "ValidationException" {
		t.Errorf("Code = %q, want ValidationException", ce.Code)
	}
}
