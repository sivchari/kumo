package lambda

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/sivchari/kumo/internal/service"
)

const (
	authTypeNone             = "NONE"
	authTypeAWSIAM           = "AWS_IAM"
	invokeModeBuffered       = "BUFFERED"
	invokeModeResponseStream = "RESPONSE_STREAM"

	functionURLTimeLayout      = "2006-01-02T15:04:05.000+0000"
	functionURLCORSMaxAgeLimit = 86400
	maxListFunctionURLConfigs  = 50

	pathSegmentURL  = "url"
	pathSegmentURLs = "urls"
)

// registerFunctionURLRoutes registers the FunctionUrlConfig operations under prefix.
func (s *Service) registerFunctionURLRoutes(r service.Router, prefix string) {
	r.Handle("POST", prefix+"/2021-10-31/functions/{functionName}/url", s.CreateFunctionURLConfig)
	r.Handle("GET", prefix+"/2021-10-31/functions/{functionName}/url", s.GetFunctionURLConfig)
	r.Handle("PUT", prefix+"/2021-10-31/functions/{functionName}/url", s.UpdateFunctionURLConfig)
	r.Handle("DELETE", prefix+"/2021-10-31/functions/{functionName}/url", s.DeleteFunctionURLConfig)
	r.Handle("GET", prefix+"/2021-10-31/functions/{functionName}/urls", s.ListFunctionURLConfigs)
}

// CreateFunctionURLConfig handles CreateFunctionUrlConfig.
func (s *Service) CreateFunctionURLConfig(w http.ResponseWriter, r *http.Request) {
	name, ok := s.functionURLTarget(w, r, pathSegmentURL)
	if !ok {
		return
	}

	var req createFunctionURLConfigRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeFunctionError(w, ErrInvalidParameterValue, "Invalid request body", http.StatusBadRequest)

		return
	}

	spec, err := req.spec()
	if err != nil {
		writeFunctionError(w, ErrInvalidParameterValue, err.Error(), http.StatusBadRequest)

		return
	}

	cfg, err := s.storage.CreateFunctionURLConfig(r.Context(), name, spec)
	if err != nil {
		writeFunctionURLError(w, err)

		return
	}

	writeJSONResponse(w, http.StatusCreated, functionURLConfigToResponse(cfg, false))
}

// GetFunctionURLConfig handles GetFunctionUrlConfig.
func (s *Service) GetFunctionURLConfig(w http.ResponseWriter, r *http.Request) {
	name, ok := s.functionURLTarget(w, r, pathSegmentURL)
	if !ok {
		return
	}

	cfg, err := s.storage.GetFunctionURLConfig(r.Context(), name)
	if err != nil {
		writeFunctionURLError(w, err)

		return
	}

	writeJSONResponse(w, http.StatusOK, functionURLConfigToResponse(cfg, true))
}

// UpdateFunctionURLConfig handles UpdateFunctionUrlConfig.
func (s *Service) UpdateFunctionURLConfig(w http.ResponseWriter, r *http.Request) {
	name, ok := s.functionURLTarget(w, r, pathSegmentURL)
	if !ok {
		return
	}

	var req updateFunctionURLConfigRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeFunctionError(w, ErrInvalidParameterValue, "Invalid request body", http.StatusBadRequest)

		return
	}

	update, err := req.update()
	if err != nil {
		writeFunctionError(w, ErrInvalidParameterValue, err.Error(), http.StatusBadRequest)

		return
	}

	cfg, err := s.storage.UpdateFunctionURLConfig(r.Context(), name, update)
	if err != nil {
		writeFunctionURLError(w, err)

		return
	}

	writeJSONResponse(w, http.StatusOK, functionURLConfigToResponse(cfg, true))
}

// DeleteFunctionURLConfig handles DeleteFunctionUrlConfig.
func (s *Service) DeleteFunctionURLConfig(w http.ResponseWriter, r *http.Request) {
	name, ok := s.functionURLTarget(w, r, pathSegmentURL)
	if !ok {
		return
	}

	if err := s.storage.DeleteFunctionURLConfig(r.Context(), name); err != nil {
		writeFunctionURLError(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListFunctionURLConfigs handles ListFunctionUrlConfigs. A function has at
// most one URL, so Marker is accepted but never needed.
func (s *Service) ListFunctionURLConfigs(w http.ResponseWriter, r *http.Request) {
	name, ok := s.functionURLTarget(w, r, pathSegmentURLs)
	if !ok {
		return
	}

	if err := validateMaxItems(r.URL.Query().Get("MaxItems")); err != nil {
		writeFunctionError(w, ErrInvalidParameterValue, err.Error(), http.StatusBadRequest)

		return
	}

	configs, err := s.storage.ListFunctionURLConfigs(r.Context(), name)
	if err != nil {
		writeFunctionURLError(w, err)

		return
	}

	resp := listFunctionURLConfigsResponse{FunctionURLConfigs: make([]functionURLConfigResponse, 0, len(configs))}
	for _, cfg := range configs {
		resp.FunctionURLConfigs = append(resp.FunctionURLConfigs, functionURLConfigToResponse(cfg, true))
	}

	writeJSONResponse(w, http.StatusOK, resp)
}

// functionURLTarget resolves the function a FunctionUrlConfig request addresses
// (name, partial ARN or ARN). kumo has no aliases, so a qualified target from
// the identifier or the Qualifier parameter is reported as not found. It writes
// the error response and returns false when the request cannot be served.
func (s *Service) functionURLTarget(w http.ResponseWriter, r *http.Request, sub string) (string, bool) {
	identifier := extractFunctionNameFromSubresource(r.URL.Path, sub)
	if identifier == "" {
		writeFunctionError(w, ErrInvalidParameterValue, "FunctionName is required", http.StatusBadRequest)

		return "", false
	}

	name, qualifier := resolveFunctionName(identifier)
	if q := r.URL.Query().Get("Qualifier"); q != "" {
		qualifier = q
	}

	if qualifier == "" {
		return name, true
	}

	target := name + ":" + qualifier
	if fn, err := s.storage.GetFunction(r.Context(), name); err == nil {
		target = fn.FunctionArn + ":" + qualifier
	}

	writeFunctionError(w, ErrResourceNotFound, "Function not found: "+target, http.StatusNotFound)

	return "", false
}

// resolveFunctionName splits a function identifier (name, name:qualifier,
// partial ARN or full ARN) into the bare name and the qualifier, if any.
func resolveFunctionName(identifier string) (string, string) {
	rest := identifier
	if i := strings.LastIndex(identifier, ":function:"); i >= 0 {
		rest = identifier[i+len(":function:"):]
	}

	name, qualifier, _ := strings.Cut(rest, ":")

	return name, qualifier
}

// writeFunctionURLError maps a storage error to the Lambda error response.
func writeFunctionURLError(w http.ResponseWriter, err error) {
	var fnErr *FunctionError
	if !errors.As(err, &fnErr) {
		writeFunctionError(w, ErrServiceException, "Internal server error", http.StatusInternalServerError)

		return
	}

	status := http.StatusBadRequest

	switch fnErr.Type {
	case ErrResourceNotFound:
		status = http.StatusNotFound
	case ErrResourceConflict:
		status = http.StatusConflict
	}

	writeFunctionError(w, fnErr.Type, fnErr.Message, status)
}

func (req *createFunctionURLConfigRequest) spec() (FunctionURLConfigSpec, error) {
	if req.AuthType == "" {
		return FunctionURLConfigSpec{}, errors.New("AuthType is required")
	}

	if err := validateAuthType("AuthType", req.AuthType); err != nil {
		return FunctionURLConfigSpec{}, err
	}

	invokeMode := req.InvokeMode
	if invokeMode == "" {
		invokeMode = invokeModeBuffered
	}

	if err := validateInvokeMode(invokeMode); err != nil {
		return FunctionURLConfigSpec{}, err
	}

	if err := validateCORS(req.Cors); err != nil {
		return FunctionURLConfigSpec{}, err
	}

	cors := req.Cors
	if cors.isEmpty() {
		cors = nil
	}

	return FunctionURLConfigSpec{AuthType: req.AuthType, InvokeMode: invokeMode, Cors: cors}, nil
}

func (req *updateFunctionURLConfigRequest) update() (FunctionURLConfigUpdate, error) {
	if req.AuthType != nil {
		if err := validateAuthType("AuthType", *req.AuthType); err != nil {
			return FunctionURLConfigUpdate{}, err
		}
	}

	if req.InvokeMode != nil {
		if err := validateInvokeMode(*req.InvokeMode); err != nil {
			return FunctionURLConfigUpdate{}, err
		}
	}

	if err := validateCORS(req.Cors); err != nil {
		return FunctionURLConfigUpdate{}, err
	}

	update := FunctionURLConfigUpdate{AuthType: req.AuthType, InvokeMode: req.InvokeMode, Cors: req.Cors}

	// An empty Cors object removes the CORS configuration (this is what
	// terraform sends when the cors block is dropped).
	if req.Cors != nil && req.Cors.isEmpty() {
		update.Cors = nil
		update.ClearCors = true
	}

	return update, nil
}

// validateAuthType checks a function URL auth type value; field names the
// request field in the error message.
func validateAuthType(field, authType string) error {
	if authType != authTypeNone && authType != authTypeAWSIAM {
		return fmt.Errorf("%s must be one of %s, %s", field, authTypeAWSIAM, authTypeNone)
	}

	return nil
}

func validateInvokeMode(invokeMode string) error {
	if invokeMode != invokeModeBuffered && invokeMode != invokeModeResponseStream {
		return fmt.Errorf("InvokeMode must be one of %s, %s", invokeModeBuffered, invokeModeResponseStream)
	}

	return nil
}

func validateCORS(cors *FunctionURLCORS) error {
	if cors != nil && (cors.MaxAge < 0 || cors.MaxAge > functionURLCORSMaxAgeLimit) {
		return fmt.Errorf("Cors.MaxAge must be between 0 and %d", functionURLCORSMaxAgeLimit)
	}

	return nil
}

// validateMaxItems checks the optional MaxItems query parameter (1-50).
func validateMaxItems(raw string) error {
	if raw == "" {
		return nil
	}

	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > maxListFunctionURLConfigs {
		return fmt.Errorf("MaxItems must be between 1 and %d", maxListFunctionURLConfigs)
	}

	return nil
}

// functionURLConfigToResponse renders a config; withModified adds
// LastModifiedTime, which AWS omits from the create response.
func functionURLConfigToResponse(cfg *FunctionURLConfig, withModified bool) functionURLConfigResponse {
	resp := functionURLConfigResponse{
		AuthType:     cfg.AuthType,
		Cors:         cfg.Cors,
		CreationTime: cfg.CreationTime.Format(functionURLTimeLayout),
		FunctionArn:  cfg.FunctionArn,
		FunctionURL:  cfg.FunctionURL,
		InvokeMode:   cfg.InvokeMode,
	}

	if withModified {
		resp.LastModifiedTime = cfg.LastModifiedTime.Format(functionURLTimeLayout)
	}

	return resp
}
