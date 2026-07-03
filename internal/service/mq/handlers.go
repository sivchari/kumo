package mq

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service"
)

// CreateBroker handles the CreateBroker API.
func (s *Service) CreateBroker(w http.ResponseWriter, r *http.Request) {
	var req CreateBrokerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, ErrBadRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.BrokerName == "" {
		writeError(w, ErrBadRequest, "brokerName is required", http.StatusBadRequest)

		return
	}

	if req.EngineType == "" {
		writeError(w, ErrBadRequest, "engineType is required", http.StatusBadRequest)

		return
	}

	if req.EngineVersion == "" {
		writeError(w, ErrBadRequest, "engineVersion is required", http.StatusBadRequest)

		return
	}

	if req.HostInstanceType == "" {
		writeError(w, ErrBadRequest, "hostInstanceType is required", http.StatusBadRequest)

		return
	}

	if req.DeploymentMode == "" {
		req.DeploymentMode = DeploymentModeSingleInstance
	}

	broker, err := s.storage.CreateBroker(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &CreateBrokerResponse{
		BrokerArn: broker.BrokerArn,
		BrokerID:  broker.BrokerID,
	}

	writeJSONResponse(w, resp)
}

// DeleteBroker handles the DeleteBroker API.
func (s *Service) DeleteBroker(w http.ResponseWriter, r *http.Request, brokerID string) {
	if brokerID == "" {
		writeError(w, ErrBadRequest, "brokerId is required", http.StatusBadRequest)

		return
	}

	err := s.storage.DeleteBroker(r.Context(), brokerID)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &DeleteBrokerResponse{
		BrokerID: brokerID,
	}

	writeJSONResponse(w, resp)
}

// DescribeBroker handles the DescribeBroker API.
func (s *Service) DescribeBroker(w http.ResponseWriter, r *http.Request, brokerID string) {
	if brokerID == "" {
		writeError(w, ErrBadRequest, "brokerId is required", http.StatusBadRequest)

		return
	}

	broker, err := s.storage.DescribeBroker(r.Context(), brokerID)
	if err != nil {
		handleError(w, err)

		return
	}

	users := make([]*UserSummary, len(broker.Users))
	for i, u := range broker.Users {
		users[i] = &UserSummary{
			Username: u.Username,
		}
	}

	resp := &DescribeBrokerResponse{
		BrokerArn:            broker.BrokerArn,
		BrokerID:             broker.BrokerID,
		BrokerName:           broker.BrokerName,
		BrokerState:          broker.BrokerState,
		Created:              broker.Created.Format("2006-01-02T15:04:05.000Z"),
		DeploymentMode:       broker.DeploymentMode,
		EngineType:           broker.EngineType,
		EngineVersion:        broker.EngineVersion,
		HostInstanceType:     broker.HostInstanceType,
		AutoMinorVersionUpgr: broker.AutoMinorVersionUpgr,
		PubliclyAccessible:   broker.PubliclyAccessible,
		Users:                users,
		Tags:                 broker.Tags,
		BrokerInstances: []*BrokerInstance{
			{
				ConsoleURL: "https://localhost:8162",
				Endpoints:  []string{"ssl://localhost:61617"},
			},
		},
	}

	if broker.Configuration != nil {
		resp.Configurations = &ConfigurationsResponse{
			Current: &ConfigurationIDResponse{
				ID:       broker.Configuration.ID,
				Revision: broker.Configuration.Revision,
			},
		}
	}

	writeJSONResponse(w, resp)
}

// ListBrokers handles the ListBrokers API.
func (s *Service) ListBrokers(w http.ResponseWriter, r *http.Request) {
	maxResults := 100
	nextToken := r.URL.Query().Get("nextToken")

	brokers, newNextToken, err := s.storage.ListBrokers(r.Context(), maxResults, nextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	summaries := make([]*BrokerSummary, len(brokers))
	for i, b := range brokers {
		summaries[i] = &BrokerSummary{
			BrokerArn:        b.BrokerArn,
			BrokerID:         b.BrokerID,
			BrokerName:       b.BrokerName,
			BrokerState:      b.BrokerState,
			Created:          b.Created.Format("2006-01-02T15:04:05.000Z"),
			DeploymentMode:   b.DeploymentMode,
			EngineType:       b.EngineType,
			HostInstanceType: b.HostInstanceType,
		}
	}

	resp := &ListBrokersResponse{
		BrokerSummaries: summaries,
		NextToken:       newNextToken,
	}

	writeJSONResponse(w, resp)
}

// UpdateBroker handles the UpdateBroker API.
func (s *Service) UpdateBroker(w http.ResponseWriter, r *http.Request, brokerID string) {
	if brokerID == "" {
		writeError(w, ErrBadRequest, "brokerId is required", http.StatusBadRequest)

		return
	}

	var req UpdateBrokerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, ErrBadRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	req.BrokerID = brokerID

	broker, err := s.storage.UpdateBroker(r.Context(), brokerID, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &UpdateBrokerResponse{
		BrokerID:             broker.BrokerID,
		EngineVersion:        broker.EngineVersion,
		HostInstanceType:     broker.HostInstanceType,
		AutoMinorVersionUpgr: broker.AutoMinorVersionUpgr,
	}

	if broker.Configuration != nil {
		resp.Configuration = &ConfigurationIDResponse{
			ID:       broker.Configuration.ID,
			Revision: broker.Configuration.Revision,
		}
	}

	writeJSONResponse(w, resp)
}

// CreateConfiguration handles the CreateConfiguration API.
func (s *Service) CreateConfiguration(w http.ResponseWriter, r *http.Request) {
	var req CreateConfigurationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, ErrBadRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeError(w, ErrBadRequest, "name is required", http.StatusBadRequest)

		return
	}

	if req.EngineType == "" {
		writeError(w, ErrBadRequest, "engineType is required", http.StatusBadRequest)

		return
	}

	if req.EngineVersion == "" {
		writeError(w, ErrBadRequest, "engineVersion is required", http.StatusBadRequest)

		return
	}

	config, err := s.storage.CreateConfiguration(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &CreateConfigurationResponse{
		Arn:     config.Arn,
		Created: config.Created.Format("2006-01-02T15:04:05.000Z"),
		ID:      config.ID,
		Name:    config.Name,
		LatestRevision: &ConfigurationRevisionResp{
			Created:     config.LatestRevision.Created.Format("2006-01-02T15:04:05.000Z"),
			Description: config.LatestRevision.Description,
			Revision:    config.LatestRevision.Revision,
		},
	}

	writeJSONResponse(w, resp)
}

// handleError handles Error and writes appropriate response.
func handleError(w http.ResponseWriter, err error) {
	var mqErr *Error
	if errors.As(err, &mqErr) {
		status := http.StatusBadRequest

		switch mqErr.Type {
		case ErrNotFound:
			status = http.StatusNotFound
		case ErrConflict:
			status = http.StatusConflict
		case ErrForbidden:
			status = http.StatusForbidden
		case ErrInternalServer:
			status = http.StatusInternalServerError
		}

		writeError(w, mqErr.Type, mqErr.Message, status)

		return
	}

	writeError(w, ErrInternalServer, "Internal server error", http.StatusInternalServerError)
}

// writeJSONResponse writes a JSON response with status 200 OK.
func writeJSONResponse(w http.ResponseWriter, v any) {
	service.WriteJSONResponse(w, service.ContentTypeJSON, v)
}

// writeError writes an MQ error response.
func writeError(w http.ResponseWriter, errType, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Amzn-Requestid", uuid.New().String())
	w.Header().Set("X-Amzn-Errortype", errType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(&Error{
		Type:    errType,
		Message: message,
	})
}
