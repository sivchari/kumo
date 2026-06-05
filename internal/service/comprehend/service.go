package comprehend

import (
	"net/http"
	"strings"

	"github.com/sivchari/kumo/internal/service"
)

// Compile-time checks to ensure Service implements required interfaces.
var (
	_ service.Service             = (*Service)(nil)
	_ service.JSONProtocolService = (*Service)(nil)
)

func init() {
	service.Register(New())
}

// Service implements the AWS Comprehend service.
type Service struct {
	analyzer *Analyzer
	handlers map[string]http.HandlerFunc
}

// New creates a new Comprehend service.
func New() *Service {
	s := &Service{
		analyzer: NewAnalyzer(),
	}
	s.initHandlers()

	return s
}

// initHandlers initializes the action handlers map.
func (s *Service) initHandlers() {
	s.handlers = map[string]http.HandlerFunc{
		"DetectSentiment":             s.DetectSentiment,
		"DetectDominantLanguage":      s.DetectDominantLanguage,
		"DetectEntities":              s.DetectEntities,
		"DetectKeyPhrases":            s.DetectKeyPhrases,
		"DetectPiiEntities":           s.DetectPiiEntities,
		"DetectSyntax":                s.DetectSyntax,
		"ContainsPiiEntities":         s.ContainsPiiEntities,
		"BatchDetectSentiment":        s.BatchDetectSentiment,
		"BatchDetectDominantLanguage": s.BatchDetectDominantLanguage,
		"BatchDetectEntities":         s.BatchDetectEntities,
		"BatchDetectKeyPhrases":       s.BatchDetectKeyPhrases,
		"BatchDetectSyntax":           s.BatchDetectSyntax,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "comprehend"
}

// RegisterRoutes registers the service routes.
// Note: Comprehend uses AWS JSON 1.1 protocol via the JSONProtocolService interface,
// so no direct routes are registered here.
func (s *Service) RegisterRoutes(_ service.Router) {
	// No routes to register - Comprehend uses JSON protocol dispatcher
}

// TargetPrefix returns the X-Amz-Target header prefix for Comprehend.
func (s *Service) TargetPrefix() string {
	return "Comprehend_20171127"
}

// JSONProtocol is a marker method that indicates Comprehend uses AWS JSON 1.1 protocol.
func (s *Service) JSONProtocol() {}

// DispatchAction handles the JSON protocol request after routing.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	if target == "" {
		writeError(w, &Error{
			Code:    "MissingAction",
			Message: "X-Amz-Target header is required",
		})

		return
	}

	// Extract the action from the target (e.g., "Comprehend_20171127.DetectSentiment" -> "DetectSentiment").
	action := target
	if idx := strings.LastIndex(target, "."); idx != -1 {
		action = target[idx+1:]
	}

	handler, ok := s.handlers[action]
	if !ok {
		writeError(w, &Error{
			Code:    "UnknownOperationException",
			Message: "Unknown operation: " + action,
		})

		return
	}

	handler(w, r)
}

// Meta returns the service's documentation metadata.
func (s *Service) Meta() service.Meta {
	return service.Meta{
		Display:     "Comprehend",
		Category:    "Analytics & ML",
		Description: "NLP service",
	}
}
