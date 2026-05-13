package billing

import (
	"testing"

	"github.com/sivchari/kumo/internal/awsapi"
)

func TestMeterAggregatesUsageAndCalculatesCostFromRateCard(t *testing.T) {
	meter := NewMeter()
	meter.SetRateCard(RateCard{
		Currency: "USD",
		Rates: []Rate{
			{
				Service:   "s3",
				Dimension: "requests.put_object",
				Unit:      UnitThousandRequests,
				Price:     0.005,
			},
			{
				Service:   "s3",
				Dimension: "response.bytes",
				Unit:      UnitGB,
				Price:     0.09,
			},
		},
	})

	for range 2 {
		meter.Record(&RequestUsage{
			Info: awsapi.RequestInfo{
				Service: "s3",
				Action:  "PutObject",
			},
			RequestBytes:  100,
			ResponseBytes: 1024 * 1024 * 1024,
			Status:        200,
		})
	}

	usage := meter.Usage()
	if got := usage.Quantity("s3", "PutObject", "requests.put_object"); got != 2 {
		t.Fatalf("requests.put_object = %v, want 2", got)
	}

	if got := usage.Quantity("s3", "PutObject", "response.bytes"); got != 2*1024*1024*1024 {
		t.Fatalf("response.bytes = %v, want %v", got, 2*1024*1024*1024)
	}

	cost := meter.Cost()
	if cost.Currency != "USD" {
		t.Fatalf("currency = %q, want USD", cost.Currency)
	}

	if cost.Total != 0.18001 {
		t.Fatalf("total = %v, want 0.18001", cost.Total)
	}
}
