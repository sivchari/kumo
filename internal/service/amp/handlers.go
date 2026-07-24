package amp

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// CreateWorkspace handles POST /workspaces.
func (s *Service) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	var req CreateWorkspaceRequest

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeAMPError(w, "InvalidRequest", "failed to read body", http.StatusBadRequest)

		return
	}

	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			writeAMPError(w, "InvalidRequest", "malformed JSON", http.StatusBadRequest)

			return
		}
	}

	ws, err := s.storage.CreateWorkspace(r.Context(), req.Alias, req.Tags)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, http.StatusAccepted, CreateWorkspaceResponse{
		WorkspaceID: ws.WorkspaceID,
		Arn:         ws.Arn,
		Status:      ws.Status,
		Tags:        ws.Tags,
	})
}

// ListWorkspaces handles GET /workspaces.
func (s *Service) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	alias := r.URL.Query().Get("alias")

	wss, err := s.storage.ListWorkspaces(r.Context(), alias)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	summaries := make([]WorkspaceSummary, 0, len(wss))
	for i := range wss {
		summaries = append(summaries, summary(&wss[i]))
	}

	writeJSON(w, http.StatusOK, ListWorkspacesResponse{Workspaces: summaries})
}

// DescribeWorkspace handles GET /workspaces/{workspaceId}.
func (s *Service) DescribeWorkspace(w http.ResponseWriter, r *http.Request) {
	ws, err := s.storage.DescribeWorkspace(r.Context(), r.PathValue("workspaceId"))
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, http.StatusOK, DescribeWorkspaceResponse{Workspace: description(ws)})
}

// DeleteWorkspace handles DELETE /workspaces/{workspaceId}.
func (s *Service) DeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	if err := s.storage.DeleteWorkspace(r.Context(), r.PathValue("workspaceId")); err != nil {
		handleStorageError(w, err)

		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// RemoteWrite proxies POST /workspaces/{id}/api/v1/remote_write to a
// real local Prometheus configured via KUMO_AMP_BACKEND. The body is
// forwarded verbatim — Prometheus's native remote_write receiver does
// the real work.
func (s *Service) RemoteWrite(w http.ResponseWriter, r *http.Request) {
	s.proxy(w, r, "/api/v1/write")
}

// QueryInstant proxies GET /workspaces/{id}/api/v1/query.
func (s *Service) QueryInstant(w http.ResponseWriter, r *http.Request) {
	s.proxy(w, r, "/api/v1/query")
}

// QueryRange proxies GET /workspaces/{id}/api/v1/query_range.
func (s *Service) QueryRange(w http.ResponseWriter, r *http.Request) {
	s.proxy(w, r, "/api/v1/query_range")
}

// proxy forwards the request to the configured Prometheus backend.
// The workspace is validated first so callers see a NotFound (matching
// AWS) when the URL path's workspace doesn't exist.
func (s *Service) proxy(w http.ResponseWriter, r *http.Request, backendPath string) {
	id := r.PathValue("workspaceId")
	if _, err := s.storage.DescribeWorkspace(r.Context(), id); err != nil {
		handleStorageError(w, err)

		return
	}

	if s.backend == "" {
		writeAMPError(w, "BackendUnavailable",
			"KUMO_AMP_BACKEND is not set; configure a real Prometheus host:port to enable data-plane proxy",
			http.StatusBadGateway)

		return
	}

	if s.backendErr != nil {
		writeAMPError(w, "BackendInvalid", s.backendErr.Error(), http.StatusInternalServerError)

		return
	}

	s.dataPlaneProxy.ServeHTTP(w, withBackendPath(r, backendPath))
}

// summary returns the smaller workspace summary shape AWS returns for
// ListWorkspaces (without status / endpoint).
func summary(ws *Workspace) WorkspaceSummary {
	return WorkspaceSummary{
		WorkspaceID: ws.WorkspaceID,
		Alias:       ws.Alias,
		Arn:         ws.Arn,
		Status:      ws.Status,
		CreatedAt:   AWSTimestamp{Time: ws.CreatedAt},
		Tags:        ws.Tags,
	}
}

func description(ws *Workspace) WorkspaceDescription {
	return WorkspaceDescription{
		WorkspaceID:        ws.WorkspaceID,
		Alias:              ws.Alias,
		Arn:                ws.Arn,
		Status:             ws.Status,
		PrometheusEndpoint: ws.PrometheusEndpoint,
		CreatedAt:          AWSTimestamp{Time: ws.CreatedAt},
		Tags:               ws.Tags,
	}
}

// handleStorageError maps the typed AMP error to an HTTP response.
func handleStorageError(w http.ResponseWriter, err error) {
	var ampErr *Error
	if errors.As(err, &ampErr) {
		status := http.StatusBadRequest
		if ampErr.Code == "ResourceNotFoundException" {
			status = http.StatusNotFound
		}

		writeAMPError(w, ampErr.Code, ampErr.Message, status)

		return
	}

	writeAMPError(w, "InternalError", err.Error(), http.StatusInternalServerError)
}

// writeAMPError writes the AWS-shape error body.
func writeAMPError(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("x-amzn-ErrorType", code)
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(ErrorResponse{Message: message})
}

// writeJSON encodes v as JSON with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(v)
}
