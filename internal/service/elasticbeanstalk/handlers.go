// Package elasticbeanstalk provides AWS Elastic Beanstalk service emulation.
package elasticbeanstalk

import (
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service"
)

// DispatchAction routes the request to the appropriate handler based on Action parameter.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	action := extractAction(r)

	switch action {
	case "CreateApplication":
		s.CreateApplication(w, r)
	case "DescribeApplications":
		s.DescribeApplications(w, r)
	case "UpdateApplication":
		s.UpdateApplication(w, r)
	case "DeleteApplication":
		s.DeleteApplication(w, r)
	case "CreateEnvironment":
		s.CreateEnvironment(w, r)
	case "DescribeEnvironments":
		s.DescribeEnvironments(w, r)
	case "TerminateEnvironment":
		s.TerminateEnvironment(w, r)
	default:
		writeError(w, "InvalidAction", fmt.Sprintf("The action '%s' is not valid", action), http.StatusBadRequest)
	}
}

// CreateApplication handles the CreateApplication action.
func (s *Service) CreateApplication(w http.ResponseWriter, r *http.Request) {
	var req CreateApplicationInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errAppNotFound, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ApplicationName == "" {
		writeError(w, errAppNotFound, "ApplicationName is required", http.StatusBadRequest)

		return
	}

	app, err := s.storage.CreateApplication(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLCreateApplicationResponse{
		Xmlns:  ebXMLNS,
		Result: XMLCreateApplicationResult{Application: toXMLApp(app)},
		Meta:   XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DescribeApplications handles the DescribeApplications action.
func (s *Service) DescribeApplications(w http.ResponseWriter, r *http.Request) {
	var req DescribeApplicationsInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errAppNotFound, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	apps, err := s.storage.DescribeApplications(r.Context(), req.ApplicationNames)
	if err != nil {
		handleError(w, err)

		return
	}

	xmlApps := make([]XMLApplicationDescription, 0, len(apps))
	for _, app := range apps {
		xmlApps = append(xmlApps, toXMLApp(&app))
	}

	writeXMLResponse(w, XMLDescribeApplicationsResponse{
		Xmlns:  ebXMLNS,
		Result: XMLDescribeApplicationsResult{Applications: xmlApps},
		Meta:   XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// UpdateApplication handles the UpdateApplication action.
func (s *Service) UpdateApplication(w http.ResponseWriter, r *http.Request) {
	var req UpdateApplicationInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errAppNotFound, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ApplicationName == "" {
		writeError(w, errAppNotFound, "ApplicationName is required", http.StatusBadRequest)

		return
	}

	app, err := s.storage.UpdateApplication(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLUpdateApplicationResponse{
		Xmlns:  ebXMLNS,
		Result: XMLUpdateApplicationResult{Application: toXMLApp(app)},
		Meta:   XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DeleteApplication handles the DeleteApplication action.
func (s *Service) DeleteApplication(w http.ResponseWriter, r *http.Request) {
	var req DeleteApplicationInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errAppNotFound, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ApplicationName == "" {
		writeError(w, errAppNotFound, "ApplicationName is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteApplication(r.Context(), req.ApplicationName); err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLDeleteApplicationResponse{
		Xmlns: ebXMLNS,
		Meta:  XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// CreateEnvironment handles the CreateEnvironment action.
func (s *Service) CreateEnvironment(w http.ResponseWriter, r *http.Request) {
	var req CreateEnvironmentInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errEnvNotFound, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ApplicationName == "" {
		writeError(w, errEnvNotFound, "ApplicationName is required", http.StatusBadRequest)

		return
	}

	if req.EnvironmentName == "" {
		writeError(w, errEnvNotFound, "EnvironmentName is required", http.StatusBadRequest)

		return
	}

	env, err := s.storage.CreateEnvironment(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLCreateEnvironmentResponse{
		Xmlns:  ebXMLNS,
		Result: XMLCreateEnvironmentResult{XMLEnvironmentDescription: toXMLEnv(env)},
		Meta:   XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DescribeEnvironments handles the DescribeEnvironments action.
func (s *Service) DescribeEnvironments(w http.ResponseWriter, r *http.Request) {
	var req DescribeEnvironmentsInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errEnvNotFound, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	envs, err := s.storage.DescribeEnvironments(r.Context(), req.ApplicationName, req.EnvironmentNames)
	if err != nil {
		handleError(w, err)

		return
	}

	xmlEnvs := make([]XMLEnvironmentDescription, 0, len(envs))

	for i := range envs {
		xmlEnvs = append(xmlEnvs, toXMLEnv(&envs[i]))
	}

	writeXMLResponse(w, XMLDescribeEnvironmentsResponse{
		Xmlns:  ebXMLNS,
		Result: XMLDescribeEnvironmentsResult{Environments: xmlEnvs},
		Meta:   XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// TerminateEnvironment handles the TerminateEnvironment action.
func (s *Service) TerminateEnvironment(w http.ResponseWriter, r *http.Request) {
	var req TerminateEnvironmentInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errEnvNotFound, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	envName := req.EnvironmentName
	if envName == "" {
		writeError(w, errEnvNotFound, "EnvironmentName is required", http.StatusBadRequest)

		return
	}

	env, err := s.storage.TerminateEnvironment(r.Context(), envName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeXMLResponse(w, XMLTerminateEnvironmentResponse{
		Xmlns:  ebXMLNS,
		Result: XMLTerminateEnvironmentResult{XMLEnvironmentDescription: toXMLEnv(env)},
		Meta:   XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// Helper functions.

func toXMLApp(app *ApplicationDescription) XMLApplicationDescription {
	return XMLApplicationDescription{
		ApplicationName: app.ApplicationName,
		Description:     app.Description,
		DateCreated:     app.DateCreated,
		DateUpdated:     app.DateUpdated,
		ApplicationArn:  app.ApplicationArn,
	}
}

func toXMLEnv(env *EnvironmentDescription) XMLEnvironmentDescription {
	return XMLEnvironmentDescription{
		ApplicationName:   env.ApplicationName,
		EnvironmentID:     env.EnvironmentID,
		EnvironmentName:   env.EnvironmentName,
		Description:       env.Description,
		SolutionStackName: env.SolutionStackName,
		Status:            env.Status,
		Health:            env.Health,
		DateCreated:       env.DateCreated,
		DateUpdated:       env.DateUpdated,
		EnvironmentArn:    env.EnvironmentArn,
	}
}

func extractAction(r *http.Request) string {
	target := r.Header.Get("X-Amz-Target")
	if target != "" {
		if idx := strings.LastIndex(target, "."); idx >= 0 {
			return target[idx+1:]
		}
	}

	return r.URL.Query().Get("Action")
}

func writeXMLResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code, message string, status int) {
	requestID := uuid.New().String()

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("x-amzn-RequestId", requestID)
	w.WriteHeader(status)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(XMLErrorResponse{
		Error: XMLError{
			Type:    "Sender",
			Code:    code,
			Message: message,
		},
		RequestID: requestID,
	})
}

func handleError(w http.ResponseWriter, err error) {
	var svcErr *ServiceError
	if errors.As(err, &svcErr) {
		writeError(w, svcErr.Code, svcErr.Message, http.StatusBadRequest)

		return
	}

	writeError(w, "InternalFailure", err.Error(), http.StatusInternalServerError)
}
