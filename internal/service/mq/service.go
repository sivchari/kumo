package mq

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	service.Register(New(NewMemoryStorage(opts...)))
}

// Service implements the Amazon MQ service.
type Service struct {
	storage Storage
}

// New creates a new MQ service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "mq"
}

// RegisterRoutes registers the MQ routes.
func (s *Service) RegisterRoutes(r service.Router) {
	// Broker operations
	r.Handle("POST", "/v1/brokers", s.handleCreateBroker)
	r.Handle("GET", "/v1/brokers", s.handleListBrokers)
	r.Handle("GET", "/v1/brokers/{brokerId}", s.handleDescribeBroker)
	r.Handle("DELETE", "/v1/brokers/{brokerId}", s.handleDeleteBroker)
	r.Handle("PUT", "/v1/brokers/{brokerId}", s.handleUpdateBroker)

	// Configuration operations
	r.Handle("POST", "/v1/configurations", s.handleCreateConfiguration)
}

// handleCreateBroker handles POST /v1/brokers.
func (s *Service) handleCreateBroker(w http.ResponseWriter, r *http.Request) {
	s.CreateBroker(w, r)
}

// handleListBrokers handles GET /v1/brokers.
func (s *Service) handleListBrokers(w http.ResponseWriter, r *http.Request) {
	s.ListBrokers(w, r)
}

// handleDescribeBroker handles GET /v1/brokers/{brokerId}.
func (s *Service) handleDescribeBroker(w http.ResponseWriter, r *http.Request) {
	brokerID := r.PathValue("brokerId")
	s.DescribeBroker(w, r, brokerID)
}

// handleDeleteBroker handles DELETE /v1/brokers/{brokerId}.
func (s *Service) handleDeleteBroker(w http.ResponseWriter, r *http.Request) {
	brokerID := r.PathValue("brokerId")
	s.DeleteBroker(w, r, brokerID)
}

// handleUpdateBroker handles PUT /v1/brokers/{brokerId}.
func (s *Service) handleUpdateBroker(w http.ResponseWriter, r *http.Request) {
	brokerID := r.PathValue("brokerId")
	s.UpdateBroker(w, r, brokerID)
}

// handleCreateConfiguration handles POST /v1/configurations.
func (s *Service) handleCreateConfiguration(w http.ResponseWriter, r *http.Request) {
	s.CreateConfiguration(w, r)
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
		Display:     "MQ",
		Category:    "Messaging & Integration",
		Description: "Message broker (ActiveMQ/RabbitMQ)",
	}
}
