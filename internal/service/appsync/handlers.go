package appsync

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/sivchari/kumo/internal/service"
)

// CreateGraphqlAPI handles the CreateGraphqlAPI operation.
func (s *Service) CreateGraphqlAPI(w http.ResponseWriter, r *http.Request) {
	var req CreateGraphqlAPIInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeError(w, errInvalidRequest, "Name is required", http.StatusBadRequest)

		return
	}

	if req.AuthenticationType == "" {
		writeError(w, errInvalidRequest, "AuthenticationType is required", http.StatusBadRequest)

		return
	}

	api, err := s.storage.CreateGraphqlAPI(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, CreateGraphqlAPIOutput{
		GraphqlAPI: api,
	})
}

// DeleteGraphqlAPI handles the DeleteGraphqlAPI operation.
func (s *Service) DeleteGraphqlAPI(w http.ResponseWriter, r *http.Request) {
	apiID := extractPathParam(r, "apiId")
	if apiID == "" {
		writeError(w, errInvalidRequest, "apiId is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteGraphqlAPI(r.Context(), apiID); err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// GetGraphqlAPI handles the GetGraphqlAPI operation.
func (s *Service) GetGraphqlAPI(w http.ResponseWriter, r *http.Request) {
	apiID := extractPathParam(r, "apiId")
	if apiID == "" {
		writeError(w, errInvalidRequest, "apiId is required", http.StatusBadRequest)

		return
	}

	api, err := s.storage.GetGraphqlAPI(r.Context(), apiID)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, GetGraphqlAPIOutput{
		GraphqlAPI: api,
	})
}

// ListGraphqlAPIs handles the ListGraphqlAPIs operation.
func (s *Service) ListGraphqlAPIs(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	var maxResults int32

	if maxResultsStr := query.Get("maxResults"); maxResultsStr != "" {
		val, err := strconv.ParseInt(maxResultsStr, 10, 32)
		if err != nil {
			writeError(w, errInvalidRequest, "Invalid maxResults parameter", http.StatusBadRequest)

			return
		}

		maxResults = int32(val)
	}

	input := &ListGraphqlAPIsInput{
		NextToken:  query.Get("nextToken"),
		MaxResults: maxResults,
		APIType:    query.Get("apiType"),
		Owner:      query.Get("owner"),
	}

	apis, nextToken, err := s.storage.ListGraphqlAPIs(r.Context(), input)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, ListGraphqlAPIsOutput{
		GraphqlAPIs: apis,
		NextToken:   nextToken,
	})
}

// CreateDataSource handles the CreateDataSource operation.
func (s *Service) CreateDataSource(w http.ResponseWriter, r *http.Request) {
	apiID := extractPathParam(r, "apiId")
	if apiID == "" {
		writeError(w, errInvalidRequest, "apiId is required", http.StatusBadRequest)

		return
	}

	var req CreateDataSourceInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	req.APIID = apiID

	if req.Name == "" {
		writeError(w, errInvalidRequest, "Name is required", http.StatusBadRequest)

		return
	}

	if req.Type == "" {
		writeError(w, errInvalidRequest, "Type is required", http.StatusBadRequest)

		return
	}

	dataSource, err := s.storage.CreateDataSource(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, CreateDataSourceOutput{
		DataSource: dataSource,
	})
}

// CreateResolver handles the CreateResolver operation.
func (s *Service) CreateResolver(w http.ResponseWriter, r *http.Request) {
	apiID := extractPathParam(r, "apiId")
	if apiID == "" {
		writeError(w, errInvalidRequest, "apiId is required", http.StatusBadRequest)

		return
	}

	typeName := extractPathParam(r, "typeName")
	if typeName == "" {
		writeError(w, errInvalidRequest, "typeName is required", http.StatusBadRequest)

		return
	}

	var req CreateResolverInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	req.APIID = apiID
	req.TypeName = typeName

	if req.FieldName == "" {
		writeError(w, errInvalidRequest, "FieldName is required", http.StatusBadRequest)

		return
	}

	resolver, err := s.storage.CreateResolver(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, CreateResolverOutput{
		Resolver: resolver,
	})
}

// StartSchemaCreation handles the StartSchemaCreation operation.
func (s *Service) StartSchemaCreation(w http.ResponseWriter, r *http.Request) {
	apiID := extractPathParam(r, "apiId")
	if apiID == "" {
		writeError(w, errInvalidRequest, "apiId is required", http.StatusBadRequest)

		return
	}

	var req StartSchemaCreationInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if len(req.Definition) == 0 {
		writeError(w, errInvalidRequest, "Definition is required", http.StatusBadRequest)

		return
	}

	status, err := s.storage.StartSchemaCreation(r.Context(), apiID, req.Definition)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, StartSchemaCreationOutput{
		Status: status.Status,
	})
}

// Helper functions.

// extractPathParam extracts a path parameter from the request URL.
func extractPathParam(r *http.Request, param string) string {
	// The server router should set path parameters.
	// For now, we'll parse them from the URL path.
	// Expected paths:
	// - /apis/{apiId}
	// - /apis/{apiId}/datasources
	// - /apis/{apiId}/types/{typeName}/resolvers
	// - /apis/{apiId}/schemacreation
	pathValue := r.PathValue(param)
	if pathValue != "" {
		return pathValue
	}

	return ""
}

// writeJSONResponse writes a JSON response with HTTP 200 OK.
func writeJSONResponse(w http.ResponseWriter, v any) {
	service.WriteJSONResponse(w, service.ContentTypeJSON, v)
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, code, message string, status int) {
	service.WriteJSONError(w, service.ContentTypeJSON, code, message, status)
}

// handleStorageError handles storage errors and writes appropriate response.
func handleStorageError(w http.ResponseWriter, err error) {
	var appsyncErr *Error
	if errors.As(err, &appsyncErr) {
		status := http.StatusBadRequest

		switch appsyncErr.Code {
		case errNotFound:
			status = http.StatusNotFound
		case errConflict:
			status = http.StatusConflict
		}

		writeError(w, appsyncErr.Code, appsyncErr.Message, status)

		return
	}

	writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)
}
