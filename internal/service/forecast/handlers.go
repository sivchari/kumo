package forecast

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// handlerFunc is a type alias for handler functions.
type handlerFunc func(http.ResponseWriter, *http.Request)

// getActionHandlers returns a map of action names to handler functions.
func (s *Service) getActionHandlers() map[string]handlerFunc {
	return map[string]handlerFunc{
		// Dataset operations
		"CreateDataset":   s.CreateDataset,
		"DescribeDataset": s.DescribeDataset,
		"ListDatasets":    s.ListDatasets,
		"DeleteDataset":   s.DeleteDataset,
		// DatasetGroup operations
		"CreateDatasetGroup":   s.CreateDatasetGroup,
		"DescribeDatasetGroup": s.DescribeDatasetGroup,
		"ListDatasetGroups":    s.ListDatasetGroups,
		"DeleteDatasetGroup":   s.DeleteDatasetGroup,
		"UpdateDatasetGroup":   s.UpdateDatasetGroup,
		// Predictor operations
		"CreatePredictor":   s.CreatePredictor,
		"DescribePredictor": s.DescribePredictor,
		"ListPredictors":    s.ListPredictors,
		"DeletePredictor":   s.DeletePredictor,
		// Forecast operations
		"CreateForecast":   s.CreateForecast,
		"DescribeForecast": s.DescribeForecast,
		"ListForecasts":    s.ListForecasts,
		"DeleteForecast":   s.DeleteForecast,
	}
}

// DispatchAction dispatches the request to the appropriate handler.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "AmazonForecast.")

	handlers := s.getActionHandlers()
	if handler, ok := handlers[action]; ok {
		handler(w, r)

		return
	}

	writeError(w, "UnknownOperationException", "The operation "+action+" is not valid.", http.StatusBadRequest)
}

// Dataset operations.

// CreateDataset handles the CreateDataset action.
func (s *Service) CreateDataset(w http.ResponseWriter, r *http.Request) {
	var input CreateDatasetInput
	if !decodeForecastRequest(w, r, &input) {
		return
	}

	datasetArn, err := s.storage.CreateDataset(r.Context(), &input)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, CreateDatasetOutput{DatasetArn: datasetArn})
}

// DescribeDataset handles the DescribeDataset action.
func (s *Service) DescribeDataset(w http.ResponseWriter, r *http.Request) {
	var input DescribeDatasetInput
	if !decodeForecastRequest(w, r, &input) {
		return
	}

	dataset, err := s.storage.DescribeDataset(r.Context(), input.DatasetArn)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, DescribeDatasetOutput{
		DatasetArn:           dataset.DatasetArn,
		DatasetName:          dataset.DatasetName,
		DatasetType:          dataset.DatasetType,
		Domain:               dataset.Domain,
		DataFrequency:        dataset.DataFrequency,
		Schema:               dataset.Schema,
		Status:               dataset.Status,
		CreationTime:         dataset.CreationTime,
		LastModificationTime: dataset.LastModificationTime,
		EncryptionConfig:     dataset.EncryptionConfig,
	})
}

// ListDatasets handles the ListDatasets action.
func (s *Service) ListDatasets(w http.ResponseWriter, r *http.Request) {
	var input ListDatasetsInput
	if !decodeForecastRequest(w, r, &input) {
		return
	}

	datasets, nextToken, err := s.storage.ListDatasets(r.Context(), input.MaxResults, input.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, ListDatasetsOutput{Datasets: datasets, NextToken: nextToken})
}

// DeleteDataset handles the DeleteDataset action.
func (s *Service) DeleteDataset(w http.ResponseWriter, r *http.Request) {
	var input DeleteDatasetInput
	if !decodeForecastRequest(w, r, &input) {
		return
	}

	if err := s.storage.DeleteDataset(r.Context(), input.DatasetArn); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// DatasetGroup operations.

// CreateDatasetGroup handles the CreateDatasetGroup action.
func (s *Service) CreateDatasetGroup(w http.ResponseWriter, r *http.Request) {
	var input CreateDatasetGroupInput
	if !decodeForecastRequest(w, r, &input) {
		return
	}

	datasetGroupArn, err := s.storage.CreateDatasetGroup(r.Context(), &input)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, CreateDatasetGroupOutput{DatasetGroupArn: datasetGroupArn})
}

// DescribeDatasetGroup handles the DescribeDatasetGroup action.
func (s *Service) DescribeDatasetGroup(w http.ResponseWriter, r *http.Request) {
	var input DescribeDatasetGroupInput
	if !decodeForecastRequest(w, r, &input) {
		return
	}

	dg, err := s.storage.DescribeDatasetGroup(r.Context(), input.DatasetGroupArn)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, DescribeDatasetGroupOutput{
		DatasetGroupArn:      dg.DatasetGroupArn,
		DatasetGroupName:     dg.DatasetGroupName,
		Domain:               dg.Domain,
		DatasetArns:          dg.DatasetArns,
		Status:               dg.Status,
		CreationTime:         dg.CreationTime,
		LastModificationTime: dg.LastModificationTime,
	})
}

// ListDatasetGroups handles the ListDatasetGroups action.
func (s *Service) ListDatasetGroups(w http.ResponseWriter, r *http.Request) {
	var input ListDatasetGroupsInput
	if !decodeForecastRequest(w, r, &input) {
		return
	}

	groups, nextToken, err := s.storage.ListDatasetGroups(r.Context(), input.MaxResults, input.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, ListDatasetGroupsOutput{DatasetGroups: groups, NextToken: nextToken})
}

// DeleteDatasetGroup handles the DeleteDatasetGroup action.
func (s *Service) DeleteDatasetGroup(w http.ResponseWriter, r *http.Request) {
	var input DeleteDatasetGroupInput
	if !decodeForecastRequest(w, r, &input) {
		return
	}

	if err := s.storage.DeleteDatasetGroup(r.Context(), input.DatasetGroupArn); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// UpdateDatasetGroup handles the UpdateDatasetGroup action.
func (s *Service) UpdateDatasetGroup(w http.ResponseWriter, r *http.Request) {
	var input UpdateDatasetGroupInput
	if !decodeForecastRequest(w, r, &input) {
		return
	}

	if err := s.storage.UpdateDatasetGroup(r.Context(), input.DatasetGroupArn, input.DatasetArns); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// Predictor operations.

// CreatePredictor handles the CreatePredictor action.
func (s *Service) CreatePredictor(w http.ResponseWriter, r *http.Request) {
	var input CreatePredictorInput
	if !decodeForecastRequest(w, r, &input) {
		return
	}

	predictorArn, err := s.storage.CreatePredictor(r.Context(), &input)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, CreatePredictorOutput{PredictorArn: predictorArn})
}

// DescribePredictor handles the DescribePredictor action.
func (s *Service) DescribePredictor(w http.ResponseWriter, r *http.Request) {
	var input DescribePredictorInput
	if !decodeForecastRequest(w, r, &input) {
		return
	}

	p, err := s.storage.DescribePredictor(r.Context(), input.PredictorArn)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, DescribePredictorOutput{
		PredictorArn:         p.PredictorArn,
		PredictorName:        p.PredictorName,
		AlgorithmArn:         p.AlgorithmArn,
		ForecastHorizon:      p.ForecastHorizon,
		ForecastTypes:        p.ForecastTypes,
		InputDataConfig:      p.InputDataConfig,
		FeaturizationConfig:  p.FeaturizationConfig,
		Status:               p.Status,
		CreationTime:         p.CreationTime,
		LastModificationTime: p.LastModificationTime,
		Message:              p.Message,
	})
}

// ListPredictors handles the ListPredictors action.
func (s *Service) ListPredictors(w http.ResponseWriter, r *http.Request) {
	var input ListPredictorsInput
	if !decodeForecastRequest(w, r, &input) {
		return
	}

	predictors, nextToken, err := s.storage.ListPredictors(r.Context(), input.MaxResults, input.NextToken, input.Filters)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, ListPredictorsOutput{Predictors: predictors, NextToken: nextToken})
}

// DeletePredictor handles the DeletePredictor action.
func (s *Service) DeletePredictor(w http.ResponseWriter, r *http.Request) {
	var input DeletePredictorInput
	if !decodeForecastRequest(w, r, &input) {
		return
	}

	if err := s.storage.DeletePredictor(r.Context(), input.PredictorArn); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// Forecast operations.

// CreateForecast handles the CreateForecast action.
func (s *Service) CreateForecast(w http.ResponseWriter, r *http.Request) {
	var input CreateForecastInput
	if !decodeForecastRequest(w, r, &input) {
		return
	}

	forecastArn, err := s.storage.CreateForecast(r.Context(), &input)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, CreateForecastOutput{ForecastArn: forecastArn})
}

// DescribeForecast handles the DescribeForecast action.
func (s *Service) DescribeForecast(w http.ResponseWriter, r *http.Request) {
	var input DescribeForecastInput
	if !decodeForecastRequest(w, r, &input) {
		return
	}

	f, err := s.storage.DescribeForecast(r.Context(), input.ForecastArn)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, DescribeForecastOutput{
		ForecastArn:                     f.ForecastArn,
		ForecastName:                    f.ForecastName,
		PredictorArn:                    f.PredictorArn,
		DatasetGroupArn:                 f.DatasetGroupArn,
		ForecastTypes:                   f.ForecastTypes,
		Status:                          f.Status,
		CreationTime:                    f.CreationTime,
		LastModificationTime:            f.LastModificationTime,
		EstimatedTimeRemainingInMinutes: f.EstimatedTimeRemainingInMinutes,
		Message:                         f.Message,
	})
}

// ListForecasts handles the ListForecasts action.
func (s *Service) ListForecasts(w http.ResponseWriter, r *http.Request) {
	var input ListForecastsInput
	if !decodeForecastRequest(w, r, &input) {
		return
	}

	forecasts, nextToken, err := s.storage.ListForecasts(r.Context(), input.MaxResults, input.NextToken, input.Filters)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, ListForecastsOutput{Forecasts: forecasts, NextToken: nextToken})
}

// DeleteForecast handles the DeleteForecast action.
func (s *Service) DeleteForecast(w http.ResponseWriter, r *http.Request) {
	var input DeleteForecastInput
	if !decodeForecastRequest(w, r, &input) {
		return
	}

	if err := s.storage.DeleteForecast(r.Context(), input.ForecastArn); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// Helper functions.

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, code, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.WriteHeader(statusCode)

	errResp := &Error{
		Code:    code,
		Message: message,
	}

	if err := json.NewEncoder(w).Encode(errResp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// decodeForecastRequest decodes the JSON request body into input, writing
// the standard Forecast decode-failure error and returning false on failure.
func decodeForecastRequest(w http.ResponseWriter, r *http.Request, input any) bool {
	if err := json.NewDecoder(r.Body).Decode(input); err != nil {
		writeError(w, errInvalidInputException, "Invalid request body", http.StatusBadRequest)

		return false
	}

	return true
}

func handleError(w http.ResponseWriter, err error) {
	var e *Error
	if errors.As(err, &e) {
		statusCode := http.StatusBadRequest

		switch e.Code {
		case errResourceNotFoundException:
			statusCode = http.StatusNotFound
		case errResourceAlreadyExistsException:
			statusCode = http.StatusConflict
		case errResourceInUseException:
			statusCode = http.StatusConflict
		}

		writeError(w, e.Code, e.Message, statusCode)

		return
	}

	writeError(w, "InternalServiceException", err.Error(), http.StatusInternalServerError)
}
