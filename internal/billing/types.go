// Package billing meters emulated AWS API usage and applies caller-provided rate cards.
package billing

import "github.com/sivchari/kumo/internal/awsapi"

const (
	// UnitRequest prices one raw request.
	UnitRequest = "request"
	// UnitThousandRequests prices one thousand requests.
	UnitThousandRequests = "1000_requests"
	// UnitMillionRequests prices one million requests.
	UnitMillionRequests = "1000000_requests"
	// UnitByte prices one raw byte.
	UnitByte = "byte"
	// UnitKB prices 1024 bytes.
	UnitKB = "KB"
	// UnitMB prices 1024^2 bytes.
	UnitMB = "MB"
	// UnitGB prices 1024^3 bytes.
	UnitGB = "GB"
)

// RequestUsage is one metered HTTP request after it passed through kumo.
type RequestUsage struct {
	Info          awsapi.RequestInfo
	RequestBytes  int64
	ResponseBytes int64
	Status        int
}

// UsageEvent is the append-only metering shape behind the aggregate snapshot.
type UsageEvent struct {
	Service    string             `json:"service"`
	Action     string             `json:"action,omitempty"`
	Resource   string             `json:"resource,omitempty"`
	Status     int                `json:"status"`
	Dimensions map[string]float64 `json:"dimensions"`
}

// UsageTotal is an aggregated usage quantity.
type UsageTotal struct {
	Service   string  `json:"service"`
	Action    string  `json:"action,omitempty"`
	Resource  string  `json:"resource,omitempty"`
	Dimension string  `json:"dimension"`
	Quantity  float64 `json:"quantity"`
}

// UsageSnapshot is returned by /kumo/billing/usage.
type UsageSnapshot struct {
	Usage []UsageTotal `json:"usage"`
}

// Quantity returns one aggregate quantity for tests and callers.
func (s UsageSnapshot) Quantity(service, action, dimension string) float64 {
	for _, total := range s.Usage {
		if total.Service == service && total.Action == action && total.Dimension == dimension {
			return total.Quantity
		}
	}

	return 0
}

// RateCard contains caller-provided prices. kumo deliberately does not bake
// changing AWS public prices into the binary.
type RateCard struct {
	Currency string `json:"currency"`
	Rates    []Rate `json:"rates"`
}

// Rate prices one usage dimension for one service.
type Rate struct {
	Service   string  `json:"service"`
	Dimension string  `json:"dimension"`
	Unit      string  `json:"unit"`
	Price     float64 `json:"price"`
}

// CostResponse is returned by /kumo/billing/cost.
type CostResponse struct {
	Currency  string         `json:"currency"`
	Total     float64        `json:"total"`
	LineItems []CostLineItem `json:"lineItems"`
}

// CostLineItem is one rate-applied usage total.
type CostLineItem struct {
	Service   string  `json:"service"`
	Action    string  `json:"action,omitempty"`
	Resource  string  `json:"resource,omitempty"`
	Dimension string  `json:"dimension"`
	Quantity  float64 `json:"quantity"`
	Unit      string  `json:"unit"`
	UnitCount float64 `json:"unitCount"`
	Price     float64 `json:"price"`
	Cost      float64 `json:"cost"`
}
