package billing

import (
	"sort"
	"strings"
	"sync"

	"github.com/sivchari/kumo/internal/servicecatalog"
)

const defaultCurrency = "USD"

// Meter stores usage events and calculates cost from a caller-provided rate card.
type Meter struct {
	mu       sync.Mutex
	catalog  *servicecatalog.Catalog
	events   []UsageEvent
	rateCard RateCard
}

// NewMeter creates an empty meter.
func NewMeter(catalogs ...*servicecatalog.Catalog) *Meter {
	catalog := servicecatalog.NewDefault()
	if len(catalogs) > 0 && catalogs[0] != nil {
		catalog = catalogs[0]
	}

	return &Meter{
		catalog:  catalog,
		rateCard: RateCard{Currency: defaultCurrency},
	}
}

// Record adds one request usage event.
func (m *Meter) Record(usage *RequestUsage) {
	if usage == nil {
		return
	}

	if usage.Info.Service == "" {
		return
	}

	serviceName := m.catalog.MustNormalize(usage.Info.Service)
	event := UsageEvent{
		Service:  serviceName,
		Action:   usage.Info.Action,
		Resource: usage.Info.Resource,
		Status:   usage.Status,
		Dimensions: map[string]float64{
			"requests.total": 1,
			"request.bytes":  float64(maxInt64(usage.RequestBytes, 0)),
			"response.bytes": float64(maxInt64(usage.ResponseBytes, 0)),
		},
	}

	if usage.Info.Action != "" {
		event.Dimensions["requests."+snakeAction(usage.Info.Action)] = 1
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.events = append(m.events, event)
}

// Reset removes all usage events.
func (m *Meter) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.events = nil
}

// Usage returns aggregated usage.
func (m *Meter) Usage() UsageSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	return usageSnapshot(m.events)
}

// SetRateCard replaces the current rate card.
func (m *Meter) SetRateCard(card RateCard) {
	if card.Currency == "" {
		card.Currency = defaultCurrency
	}

	for i := range card.Rates {
		card.Rates[i].Service = m.catalog.MustNormalize(card.Rates[i].Service)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.rateCard = card
}

// RateCard returns the current rate card.
func (m *Meter) RateCard() RateCard {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.rateCard
}

// Cost calculates current usage cost from the current rate card.
func (m *Meter) Cost() CostResponse {
	m.mu.Lock()
	defer m.mu.Unlock()

	usage := usageSnapshot(m.events)
	rates := make(map[string]Rate, len(m.rateCard.Rates))

	for _, rate := range m.rateCard.Rates {
		rates[rateKey(rate.Service, rate.Dimension)] = rate
	}

	lineItems := make([]CostLineItem, 0)
	totalCost := 0.0

	for _, item := range usage.Usage {
		rate, ok := rates[rateKey(item.Service, item.Dimension)]
		if !ok {
			continue
		}

		unitCount := normalizeQuantity(item.Quantity, rate.Unit)
		cost := unitCount * rate.Price
		totalCost += cost

		lineItems = append(lineItems, CostLineItem{
			Service:   item.Service,
			Action:    item.Action,
			Resource:  item.Resource,
			Dimension: item.Dimension,
			Quantity:  item.Quantity,
			Unit:      rate.Unit,
			UnitCount: unitCount,
			Price:     rate.Price,
			Cost:      cost,
		})
	}

	sort.Slice(lineItems, func(i, j int) bool {
		if lineItems[i].Service != lineItems[j].Service {
			return lineItems[i].Service < lineItems[j].Service
		}

		if lineItems[i].Dimension != lineItems[j].Dimension {
			return lineItems[i].Dimension < lineItems[j].Dimension
		}

		return lineItems[i].Action < lineItems[j].Action
	})

	currency := m.rateCard.Currency
	if currency == "" {
		currency = defaultCurrency
	}

	return CostResponse{
		Currency:  currency,
		Total:     totalCost,
		LineItems: lineItems,
	}
}

func usageSnapshot(events []UsageEvent) UsageSnapshot {
	totals := make(map[string]UsageTotal)

	for _, event := range events {
		for dimension, quantity := range event.Dimensions {
			key := usageKey(event.Service, event.Action, event.Resource, dimension)
			total := totals[key]
			total.Service = event.Service
			total.Action = event.Action
			total.Resource = event.Resource
			total.Dimension = dimension
			total.Quantity += quantity
			totals[key] = total
		}
	}

	out := make([]UsageTotal, 0, len(totals))
	for _, total := range totals {
		out = append(out, total)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Service != out[j].Service {
			return out[i].Service < out[j].Service
		}

		if out[i].Action != out[j].Action {
			return out[i].Action < out[j].Action
		}

		if out[i].Dimension != out[j].Dimension {
			return out[i].Dimension < out[j].Dimension
		}

		return out[i].Resource < out[j].Resource
	})

	return UsageSnapshot{Usage: out}
}

func normalizeQuantity(quantity float64, unit string) float64 {
	switch unit {
	case UnitRequest:
		return quantity
	case UnitThousandRequests:
		return quantity / 1000
	case UnitMillionRequests:
		return quantity / 1000000
	case UnitByte:
		return quantity
	case UnitKB:
		return quantity / 1024
	case UnitMB:
		return quantity / (1024 * 1024)
	case UnitGB:
		return quantity / (1024 * 1024 * 1024)
	default:
		return quantity
	}
}

func usageKey(service, action, resource, dimension string) string {
	return service + "\x00" + action + "\x00" + resource + "\x00" + dimension
}

func rateKey(service, dimension string) string {
	return strings.ToLower(service) + "\x00" + strings.ToLower(dimension)
}

func snakeAction(action string) string {
	var b strings.Builder

	for i, r := range action {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}

		b.WriteRune(r)
	}

	return strings.ToLower(b.String())
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}

	return b
}
