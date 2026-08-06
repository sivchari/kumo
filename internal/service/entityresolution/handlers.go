package entityresolution

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Schema mapping handlers.

// CreateSchemaMapping handles POST /schemas.
func (s *Service) CreateSchemaMapping(w http.ResponseWriter, r *http.Request) {
	var req CreateSchemaMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.SchemaName == "" {
		writeError(w, errValidation, "schemaName is required", http.StatusBadRequest)

		return
	}

	if len(req.MappedInputFields) == 0 {
		writeError(w, errValidation, "mappedInputFields is required", http.StatusBadRequest)

		return
	}

	schema, err := s.storage.CreateSchemaMapping(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, schema)
}

// GetSchemaMapping handles GET /schemas/{schemaName}.
func (s *Service) GetSchemaMapping(w http.ResponseWriter, r *http.Request) {
	schemaName := r.PathValue("schemaName")
	if schemaName == "" {
		writeError(w, errValidation, "schemaName is required", http.StatusBadRequest)

		return
	}

	schema, err := s.storage.GetSchemaMapping(r.Context(), schemaName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, schema)
}

// DeleteSchemaMapping handles DELETE /schemas/{schemaName}.
func (s *Service) DeleteSchemaMapping(w http.ResponseWriter, r *http.Request) {
	schemaName := r.PathValue("schemaName")
	if schemaName == "" {
		writeError(w, errValidation, "schemaName is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteSchemaMapping(r.Context(), schemaName); err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, map[string]string{"message": "Schema mapping deleted"})
}

// ListSchemaMappings handles GET /schemas.
func (s *Service) ListSchemaMappings(w http.ResponseWriter, r *http.Request) {
	summaries, err := s.storage.ListSchemaMappings(r.Context())
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &ListSchemaMappingsResponse{
		SchemaList: summaries,
	})
}

// Matching workflow handlers.

// CreateMatchingWorkflow handles POST /matchingworkflows.
func (s *Service) CreateMatchingWorkflow(w http.ResponseWriter, r *http.Request) {
	var req CreateMatchingWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.WorkflowName == "" {
		writeError(w, errValidation, "workflowName is required", http.StatusBadRequest)

		return
	}

	workflow, err := s.storage.CreateMatchingWorkflow(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, workflow)
}

// GetMatchingWorkflow handles GET /matchingworkflows/{workflowName}.
func (s *Service) GetMatchingWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowName := r.PathValue("workflowName")
	if workflowName == "" {
		writeError(w, errValidation, "workflowName is required", http.StatusBadRequest)

		return
	}

	workflow, err := s.storage.GetMatchingWorkflow(r.Context(), workflowName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, workflow)
}

// DeleteMatchingWorkflow handles DELETE /matchingworkflows/{workflowName}.
func (s *Service) DeleteMatchingWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowName := r.PathValue("workflowName")
	if workflowName == "" {
		writeError(w, errValidation, "workflowName is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteMatchingWorkflow(r.Context(), workflowName); err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, map[string]string{"message": "Matching workflow deleted"})
}

// ListMatchingWorkflows handles GET /matchingworkflows.
func (s *Service) ListMatchingWorkflows(w http.ResponseWriter, r *http.Request) {
	summaries, err := s.storage.ListMatchingWorkflows(r.Context())
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &ListMatchingWorkflowsResponse{
		WorkflowSummaries: summaries,
	})
}

// ID mapping workflow handlers.

// CreateIDMappingWorkflow handles POST /idmappingworkflows.
func (s *Service) CreateIDMappingWorkflow(w http.ResponseWriter, r *http.Request) {
	var req CreateIDMappingWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.WorkflowName == "" {
		writeError(w, errValidation, "workflowName is required", http.StatusBadRequest)

		return
	}

	workflow, err := s.storage.CreateIDMappingWorkflow(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, workflow)
}

// GetIDMappingWorkflow handles GET /idmappingworkflows/{workflowName}.
func (s *Service) GetIDMappingWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowName := r.PathValue("workflowName")
	if workflowName == "" {
		writeError(w, errValidation, "workflowName is required", http.StatusBadRequest)

		return
	}

	workflow, err := s.storage.GetIDMappingWorkflow(r.Context(), workflowName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, workflow)
}

// DeleteIDMappingWorkflow handles DELETE /idmappingworkflows/{workflowName}.
func (s *Service) DeleteIDMappingWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowName := r.PathValue("workflowName")
	if workflowName == "" {
		writeError(w, errValidation, "workflowName is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteIDMappingWorkflow(r.Context(), workflowName); err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, map[string]string{"message": "ID mapping workflow deleted"})
}

// ListIDMappingWorkflows handles GET /idmappingworkflows.
func (s *Service) ListIDMappingWorkflows(w http.ResponseWriter, r *http.Request) {
	summaries, err := s.storage.ListIDMappingWorkflows(r.Context())
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &ListIDMappingWorkflowsResponse{
		WorkflowSummaries: summaries,
	})
}

// Provider service handlers.

// ListProviderServices handles GET /providerservices.
func (s *Service) ListProviderServices(w http.ResponseWriter, r *http.Request) {
	services, err := s.storage.ListProviderServices(r.Context())
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &ListProviderServicesResponse{
		ProviderServiceSummaries: services,
	})
}

// Helper functions.

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(map[string]string{
		"code":    code,
		"message": message,
	})
}

func handleError(w http.ResponseWriter, err error) {
	var erErr *Error
	if errors.As(err, &erErr) {
		status := http.StatusBadRequest

		switch erErr.Code {
		case errNotFound:
			status = http.StatusNotFound
		case errConflict:
			status = http.StatusConflict
		}

		writeError(w, erErr.Code, erErr.Message, status)

		return
	}

	writeError(w, errInternalError, "Internal server error", http.StatusInternalServerError)
}
