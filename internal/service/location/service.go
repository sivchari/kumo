// Package location provides a mock implementation of Amazon Location Service.
package location

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

// Compile-time check to ensure Service implements service.Service.
var _ service.Service = (*Service)(nil)

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	service.Register(New(NewMemoryStorage(opts...)))
}

// Service implements the Amazon Location service.
type Service struct {
	storage Storage
}

// New creates a new Location service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "location"
}

// RegisterRoutes registers the Location Service routes.
// Location Service uses REST API with various HTTP methods.
func (s *Service) RegisterRoutes(r service.Router) {
	// Map operations
	r.Handle("POST", "/maps/v0/maps", s.CreateMap)
	r.Handle("GET", "/maps/v0/maps/{MapName}", s.DescribeMap)
	r.Handle("PATCH", "/maps/v0/maps/{MapName}", s.UpdateMap)
	r.Handle("DELETE", "/maps/v0/maps/{MapName}", s.DeleteMap)
	r.Handle("POST", "/maps/v0/list-maps", s.ListMaps)

	// Place index operations
	r.Handle("POST", "/places/v0/indexes", s.CreatePlaceIndex)
	r.Handle("GET", "/places/v0/indexes/{IndexName}", s.DescribePlaceIndex)
	r.Handle("PATCH", "/places/v0/indexes/{IndexName}", s.UpdatePlaceIndex)
	r.Handle("DELETE", "/places/v0/indexes/{IndexName}", s.DeletePlaceIndex)
	r.Handle("POST", "/places/v0/list-indexes", s.ListPlaceIndexes)

	// Route calculator operations
	r.Handle("POST", "/routes/v0/calculators", s.CreateRouteCalculator)
	r.Handle("GET", "/routes/v0/calculators/{CalculatorName}", s.DescribeRouteCalculator)
	r.Handle("PATCH", "/routes/v0/calculators/{CalculatorName}", s.UpdateRouteCalculator)
	r.Handle("DELETE", "/routes/v0/calculators/{CalculatorName}", s.DeleteRouteCalculator)
	r.Handle("POST", "/routes/v0/list-calculators", s.ListRouteCalculators)

	// Geofence collection operations
	r.Handle("POST", "/geofencing/v0/collections", s.CreateGeofenceCollection)
	r.Handle("GET", "/geofencing/v0/collections/{CollectionName}", s.DescribeGeofenceCollection)
	r.Handle("PATCH", "/geofencing/v0/collections/{CollectionName}", s.UpdateGeofenceCollection)
	r.Handle("DELETE", "/geofencing/v0/collections/{CollectionName}", s.DeleteGeofenceCollection)
	r.Handle("POST", "/geofencing/v0/list-collections", s.ListGeofenceCollections)

	// Tracker operations
	r.Handle("POST", "/tracking/v0/trackers", s.CreateTracker)
	r.Handle("GET", "/tracking/v0/trackers/{TrackerName}", s.DescribeTracker)
	r.Handle("PATCH", "/tracking/v0/trackers/{TrackerName}", s.UpdateTracker)
	r.Handle("DELETE", "/tracking/v0/trackers/{TrackerName}", s.DeleteTracker)
	r.Handle("POST", "/tracking/v0/list-trackers", s.ListTrackers)
}

// Close saves the storage state if persistence is enabled.
func (s *Service) Close() error {
	if c, ok := s.storage.(io.Closer); ok {
		if err := c.Close(); err != nil {
			return fmt.Errorf("failed to close storage: %w", err)
		}
	}

	return nil
}

// Meta returns the service's documentation metadata.
func (s *Service) Meta() service.Meta {
	return service.Meta{
		Display:     "Location",
		Category:    "Networking & Content Delivery",
		Description: "Location-based services",
	}
}
