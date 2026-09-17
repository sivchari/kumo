package elbv2

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDescribeCapacityReservationReturnsEmptyResult(t *testing.T) {
	t.Parallel()

	service := New(NewMemoryStorage())
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"LoadBalancerArn":"arn:aws:elasticloadbalancing:us-east-1:000000000000:loadbalancer/app/kumo-local/123"}`))
	rec := httptest.NewRecorder()

	service.DescribeCapacityReservation(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	for _, want := range []string{
		"<DescribeCapacityReservationResponse",
		"<DescribeCapacityReservationResult>",
		"<ResponseMetadata>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response to contain %q, got %s", want, body)
		}
	}
}
