//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
	"github.com/sivchari/golden"
)

func newServiceQuotasClient(t *testing.T) *servicequotas.Client {
	t.Helper()

	return servicequotas.NewFromConfig(awsConfig(t), func(o *servicequotas.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestServiceQuotas_ListServices(t *testing.T) {
	client := newServiceQuotasClient(t)
	ctx := t.Context()

	output, err := client.ListServices(ctx, &servicequotas.ListServicesInput{
		MaxResults: aws.Int32(10),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("NextToken", "ResultMetadata")).Assert(t.Name(), output)
}

func TestServiceQuotas_GetServiceQuota(t *testing.T) {
	client := newServiceQuotasClient(t)
	ctx := t.Context()

	// Get a known quota (EC2 Running On-Demand Standard instances)
	output, err := client.GetServiceQuota(ctx, &servicequotas.GetServiceQuotaInput{
		ServiceCode: aws.String("ec2"),
		QuotaCode:   aws.String("L-1216C47A"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), output)
}

func TestServiceQuotas_GetServiceQuota_NotFound(t *testing.T) {
	client := newServiceQuotasClient(t)
	ctx := t.Context()

	// Try to get a non-existent quota
	_, err := client.GetServiceQuota(ctx, &servicequotas.GetServiceQuotaInput{
		ServiceCode: aws.String("ec2"),
		QuotaCode:   aws.String("nonexistent-quota"),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestServiceQuotas_ListServiceQuotas(t *testing.T) {
	client := newServiceQuotasClient(t)
	ctx := t.Context()

	output, err := client.ListServiceQuotas(ctx, &servicequotas.ListServiceQuotasInput{
		ServiceCode: aws.String("ec2"),
		MaxResults:  aws.Int32(10),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("NextToken", "ResultMetadata")).Assert(t.Name(), output)
}

func TestServiceQuotas_ListServiceQuotas_ServiceNotFound(t *testing.T) {
	client := newServiceQuotasClient(t)
	ctx := t.Context()

	_, err := client.ListServiceQuotas(ctx, &servicequotas.ListServiceQuotasInput{
		ServiceCode: aws.String("nonexistent-service"),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestServiceQuotas_GetAWSDefaultServiceQuota(t *testing.T) {
	client := newServiceQuotasClient(t)
	ctx := t.Context()

	output, err := client.GetAWSDefaultServiceQuota(ctx, &servicequotas.GetAWSDefaultServiceQuotaInput{
		ServiceCode: aws.String("lambda"),
		QuotaCode:   aws.String("L-B99A9384"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), output)
}

func TestServiceQuotas_ListAWSDefaultServiceQuotas(t *testing.T) {
	client := newServiceQuotasClient(t)
	ctx := t.Context()

	output, err := client.ListAWSDefaultServiceQuotas(ctx, &servicequotas.ListAWSDefaultServiceQuotasInput{
		ServiceCode: aws.String("lambda"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("NextToken", "ResultMetadata")).Assert(t.Name(), output)
}

func TestServiceQuotas_RequestServiceQuotaIncrease(t *testing.T) {
	client := newServiceQuotasClient(t)
	ctx := t.Context()

	// Request a quota increase for an adjustable quota
	output, err := client.RequestServiceQuotaIncrease(ctx, &servicequotas.RequestServiceQuotaIncreaseInput{
		ServiceCode:  aws.String("ec2"),
		QuotaCode:    aws.String("L-1216C47A"),
		DesiredValue: aws.Float64(2000),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Id", "CaseId", "Created", "LastUpdated", "ResultMetadata")).Assert(t.Name(), output)
}

func TestServiceQuotas_RequestServiceQuotaIncrease_NonAdjustable(t *testing.T) {
	client := newServiceQuotasClient(t)
	ctx := t.Context()

	// Try to request increase for a non-adjustable quota
	_, err := client.RequestServiceQuotaIncrease(ctx, &servicequotas.RequestServiceQuotaIncreaseInput{
		ServiceCode:  aws.String("sqs"),
		QuotaCode:    aws.String("L-06F64E4A"),
		DesiredValue: aws.Float64(200000),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestServiceQuotas_GetRequestedServiceQuotaChange(t *testing.T) {
	client := newServiceQuotasClient(t)
	ctx := t.Context()

	// First, create a request
	createOutput, err := client.RequestServiceQuotaIncrease(ctx, &servicequotas.RequestServiceQuotaIncreaseInput{
		ServiceCode:  aws.String("ec2"),
		QuotaCode:    aws.String("L-0E3CBAB9"),
		DesiredValue: aws.Float64(10),
	})
	if err != nil {
		t.Fatal(err)
	}
	if createOutput.RequestedQuota == nil {
		t.Fatal("RequestedQuota is nil")
	}

	// Then get the request
	output, err := client.GetRequestedServiceQuotaChange(ctx, &servicequotas.GetRequestedServiceQuotaChangeInput{
		RequestId: createOutput.RequestedQuota.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Id", "CaseId", "Created", "LastUpdated", "ResultMetadata")).Assert(t.Name(), output)
}

func TestServiceQuotas_GetRequestedServiceQuotaChange_NotFound(t *testing.T) {
	client := newServiceQuotasClient(t)
	ctx := t.Context()

	_, err := client.GetRequestedServiceQuotaChange(ctx, &servicequotas.GetRequestedServiceQuotaChangeInput{
		RequestId: aws.String("nonexistent-request-id"),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestServiceQuotas_ListRequestedServiceQuotaChangeHistory(t *testing.T) {
	client := newServiceQuotasClient(t)
	ctx := t.Context()

	// First, create a request
	_, err := client.RequestServiceQuotaIncrease(ctx, &servicequotas.RequestServiceQuotaIncreaseInput{
		ServiceCode:  aws.String("dynamodb"),
		QuotaCode:    aws.String("L-F98FE922"),
		DesiredValue: aws.Float64(50000),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Then list the requests
	output, err := client.ListRequestedServiceQuotaChangeHistory(ctx, &servicequotas.ListRequestedServiceQuotaChangeHistoryInput{
		ServiceCode: aws.String("dynamodb"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("RequestedQuotas", "Id", "CaseId", "Created", "LastUpdated", "NextToken", "ResultMetadata")).Assert(t.Name(), output)
}

func TestServiceQuotas_ListRequestedServiceQuotaChangeHistory_ByStatus(t *testing.T) {
	client := newServiceQuotasClient(t)
	ctx := t.Context()

	// Create a request
	_, err := client.RequestServiceQuotaIncrease(ctx, &servicequotas.RequestServiceQuotaIncreaseInput{
		ServiceCode:  aws.String("s3"),
		QuotaCode:    aws.String("L-DC2B2D3D"),
		DesiredValue: aws.Float64(200),
	})
	if err != nil {
		t.Fatal(err)
	}

	// List pending requests
	output, err := client.ListRequestedServiceQuotaChangeHistory(ctx, &servicequotas.ListRequestedServiceQuotaChangeHistoryInput{
		Status: "PENDING",
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("RequestedQuotas", "Id", "CaseId", "Created", "LastUpdated", "NextToken", "ResultMetadata")).Assert(t.Name(), output)
}

func TestServiceQuotas_EndToEnd(t *testing.T) {
	client := newServiceQuotasClient(t)
	ctx := t.Context()

	// 1. List services
	servicesOutput, err := client.ListServices(ctx, &servicequotas.ListServicesInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(servicesOutput.Services) == 0 {
		t.Fatal("expected at least one service")
	}

	// 2. Get quotas for a service
	quotasOutput, err := client.ListServiceQuotas(ctx, &servicequotas.ListServiceQuotasInput{
		ServiceCode: aws.String("ec2"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(quotasOutput.Quotas) == 0 {
		t.Fatal("expected at least one quota")
	}

	// 3. Get a specific quota
	var adjustableQuotaCode *string

	for _, q := range quotasOutput.Quotas {
		if q.Adjustable {
			adjustableQuotaCode = q.QuotaCode

			break
		}
	}

	if adjustableQuotaCode == nil {
		t.Fatal("should have at least one adjustable quota")
	}

	quotaOutput, err := client.GetServiceQuota(ctx, &servicequotas.GetServiceQuotaInput{
		ServiceCode: aws.String("ec2"),
		QuotaCode:   adjustableQuotaCode,
	})
	if err != nil {
		t.Fatal(err)
	}
	if quotaOutput.Quota == nil {
		t.Fatal("Quota is nil")
	}

	// 4. Request a quota increase
	requestOutput, err := client.RequestServiceQuotaIncrease(ctx, &servicequotas.RequestServiceQuotaIncreaseInput{
		ServiceCode:  aws.String("ec2"),
		QuotaCode:    adjustableQuotaCode,
		DesiredValue: aws.Float64(*quotaOutput.Quota.Value + 100),
	})
	if err != nil {
		t.Fatal(err)
	}
	if requestOutput.RequestedQuota == nil {
		t.Fatal("RequestedQuota is nil")
	}

	// 5. Get the request
	getRequestOutput, err := client.GetRequestedServiceQuotaChange(ctx, &servicequotas.GetRequestedServiceQuotaChangeInput{
		RequestId: requestOutput.RequestedQuota.Id,
	})
	if err != nil {
		t.Fatal(err)
	}
	if getRequestOutput.RequestedQuota == nil {
		t.Fatal("RequestedQuota is nil")
	}

	// 6. List request history
	historyOutput, err := client.ListRequestedServiceQuotaChangeHistory(ctx, &servicequotas.ListRequestedServiceQuotaChangeHistoryInput{
		ServiceCode: aws.String("ec2"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Should find our request in the history
	found := false

	for _, req := range historyOutput.RequestedQuotas {
		if *req.Id == *requestOutput.RequestedQuota.Id {
			found = true

			break
		}
	}

	if !found {
		t.Error("should find the created request in history")
	}
}
