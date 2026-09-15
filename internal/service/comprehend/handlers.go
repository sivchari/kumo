package comprehend

import (
	"encoding/json"
	"errors"
	"net/http"
)

// DetectSentiment handles the DetectSentiment API.
func (s *Service) DetectSentiment(w http.ResponseWriter, r *http.Request) {
	var req DetectSentimentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, &Error{
			Code:    errInvalidRequest,
			Message: msgInvalidRequestBody,
		})

		return
	}

	resp, err := s.analyzer.DetectSentiment(req.Text, req.LanguageCode)
	if err != nil {
		var e *Error
		if errors.As(err, &e) {
			writeError(w, e)

			return
		}

		writeError(w, &Error{
			Code:    errInternalServer,
			Message: err.Error(),
		})

		return
	}

	writeJSON(w, resp)
}

// DetectDominantLanguage handles the DetectDominantLanguage API.
func (s *Service) DetectDominantLanguage(w http.ResponseWriter, r *http.Request) {
	var req DetectDominantLanguageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, &Error{
			Code:    errInvalidRequest,
			Message: msgInvalidRequestBody,
		})

		return
	}

	resp, err := s.analyzer.DetectDominantLanguage(req.Text)
	if err != nil {
		var e *Error
		if errors.As(err, &e) {
			writeError(w, e)

			return
		}

		writeError(w, &Error{
			Code:    errInternalServer,
			Message: err.Error(),
		})

		return
	}

	writeJSON(w, resp)
}

// DetectEntities handles the DetectEntities API.
func (s *Service) DetectEntities(w http.ResponseWriter, r *http.Request) {
	var req DetectEntitiesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, &Error{
			Code:    errInvalidRequest,
			Message: msgInvalidRequestBody,
		})

		return
	}

	resp, err := s.analyzer.DetectEntities(req.Text, req.LanguageCode)
	if err != nil {
		var e *Error
		if errors.As(err, &e) {
			writeError(w, e)

			return
		}

		writeError(w, &Error{
			Code:    errInternalServer,
			Message: err.Error(),
		})

		return
	}

	writeJSON(w, resp)
}

// DetectKeyPhrases handles the DetectKeyPhrases API.
func (s *Service) DetectKeyPhrases(w http.ResponseWriter, r *http.Request) {
	var req DetectKeyPhrasesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, &Error{
			Code:    errInvalidRequest,
			Message: msgInvalidRequestBody,
		})

		return
	}

	resp, err := s.analyzer.DetectKeyPhrases(req.Text, req.LanguageCode)
	if err != nil {
		var e *Error
		if errors.As(err, &e) {
			writeError(w, e)

			return
		}

		writeError(w, &Error{
			Code:    errInternalServer,
			Message: err.Error(),
		})

		return
	}

	writeJSON(w, resp)
}

// DetectPiiEntities handles the DetectPiiEntities API.
func (s *Service) DetectPiiEntities(w http.ResponseWriter, r *http.Request) {
	var req DetectPiiEntitiesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, &Error{
			Code:    errInvalidRequest,
			Message: msgInvalidRequestBody,
		})

		return
	}

	resp, err := s.analyzer.DetectPiiEntities(req.Text, req.LanguageCode)
	if err != nil {
		var e *Error
		if errors.As(err, &e) {
			writeError(w, e)

			return
		}

		writeError(w, &Error{
			Code:    errInternalServer,
			Message: err.Error(),
		})

		return
	}

	writeJSON(w, resp)
}

// DetectSyntax handles the DetectSyntax API.
func (s *Service) DetectSyntax(w http.ResponseWriter, r *http.Request) {
	var req DetectSyntaxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, &Error{
			Code:    errInvalidRequest,
			Message: msgInvalidRequestBody,
		})

		return
	}

	resp, err := s.analyzer.DetectSyntax(req.Text, req.LanguageCode)
	if err != nil {
		var e *Error
		if errors.As(err, &e) {
			writeError(w, e)

			return
		}

		writeError(w, &Error{
			Code:    errInternalServer,
			Message: err.Error(),
		})

		return
	}

	writeJSON(w, resp)
}

// ContainsPiiEntities handles the ContainsPiiEntities API.
func (s *Service) ContainsPiiEntities(w http.ResponseWriter, r *http.Request) {
	var req ContainsPiiEntitiesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, &Error{
			Code:    errInvalidRequest,
			Message: msgInvalidRequestBody,
		})

		return
	}

	resp, err := s.analyzer.ContainsPiiEntities(req.Text, req.LanguageCode)
	if err != nil {
		var e *Error
		if errors.As(err, &e) {
			writeError(w, e)

			return
		}

		writeError(w, &Error{
			Code:    errInternalServer,
			Message: err.Error(),
		})

		return
	}

	writeJSON(w, resp)
}

// BatchDetectSentiment handles the BatchDetectSentiment API.
func (s *Service) BatchDetectSentiment(w http.ResponseWriter, r *http.Request) {
	var req BatchDetectSentimentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, &Error{
			Code:    errInvalidRequest,
			Message: msgInvalidRequestBody,
		})

		return
	}

	results := make([]BatchDetectSentimentItemResult, 0, len(req.TextList))
	errorList := make([]BatchItemError, 0)

	for i, text := range req.TextList {
		resp, err := s.analyzer.DetectSentiment(text, req.LanguageCode)
		if err != nil {
			var e *Error
			if errors.As(err, &e) {
				errorList = append(errorList, BatchItemError{
					ErrorCode:    e.Code,
					ErrorMessage: e.Message,
					Index:        i,
				})

				continue
			}

			errorList = append(errorList, BatchItemError{
				ErrorCode:    errInternalServer,
				ErrorMessage: err.Error(),
				Index:        i,
			})

			continue
		}

		results = append(results, BatchDetectSentimentItemResult{
			Index:          i,
			Sentiment:      resp.Sentiment,
			SentimentScore: resp.SentimentScore,
		})
	}

	writeJSON(w, &BatchDetectSentimentResponse{
		ErrorList:  errorList,
		ResultList: results,
	})
}

// BatchDetectDominantLanguage handles the BatchDetectDominantLanguage API.
func (s *Service) BatchDetectDominantLanguage(w http.ResponseWriter, r *http.Request) {
	var req BatchDetectDominantLanguageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, &Error{
			Code:    errInvalidRequest,
			Message: msgInvalidRequestBody,
		})

		return
	}

	results := make([]BatchDetectDominantLanguageItemResult, 0, len(req.TextList))
	errorList := make([]BatchItemError, 0)

	for i, text := range req.TextList {
		resp, err := s.analyzer.DetectDominantLanguage(text)
		if err != nil {
			var e *Error
			if errors.As(err, &e) {
				errorList = append(errorList, BatchItemError{
					ErrorCode:    e.Code,
					ErrorMessage: e.Message,
					Index:        i,
				})

				continue
			}

			errorList = append(errorList, BatchItemError{
				ErrorCode:    errInternalServer,
				ErrorMessage: err.Error(),
				Index:        i,
			})

			continue
		}

		results = append(results, BatchDetectDominantLanguageItemResult{
			Index:     i,
			Languages: resp.Languages,
		})
	}

	writeJSON(w, &BatchDetectDominantLanguageResponse{
		ErrorList:  errorList,
		ResultList: results,
	})
}

// BatchDetectEntities handles the BatchDetectEntities API.
func (s *Service) BatchDetectEntities(w http.ResponseWriter, r *http.Request) {
	var req BatchDetectEntitiesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, &Error{
			Code:    errInvalidRequest,
			Message: msgInvalidRequestBody,
		})

		return
	}

	results := make([]BatchDetectEntitiesItemResult, 0, len(req.TextList))
	errorList := make([]BatchItemError, 0)

	for i, text := range req.TextList {
		resp, err := s.analyzer.DetectEntities(text, req.LanguageCode)
		if err != nil {
			var e *Error
			if errors.As(err, &e) {
				errorList = append(errorList, BatchItemError{
					ErrorCode:    e.Code,
					ErrorMessage: e.Message,
					Index:        i,
				})

				continue
			}

			errorList = append(errorList, BatchItemError{
				ErrorCode:    errInternalServer,
				ErrorMessage: err.Error(),
				Index:        i,
			})

			continue
		}

		results = append(results, BatchDetectEntitiesItemResult{
			Entities: resp.Entities,
			Index:    i,
		})
	}

	writeJSON(w, &BatchDetectEntitiesResponse{
		ErrorList:  errorList,
		ResultList: results,
	})
}

// BatchDetectKeyPhrases handles the BatchDetectKeyPhrases API.
func (s *Service) BatchDetectKeyPhrases(w http.ResponseWriter, r *http.Request) {
	var req BatchDetectKeyPhrasesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, &Error{
			Code:    errInvalidRequest,
			Message: msgInvalidRequestBody,
		})

		return
	}

	results := make([]BatchDetectKeyPhrasesItemResult, 0, len(req.TextList))
	errorList := make([]BatchItemError, 0)

	for i, text := range req.TextList {
		resp, err := s.analyzer.DetectKeyPhrases(text, req.LanguageCode)
		if err != nil {
			var e *Error
			if errors.As(err, &e) {
				errorList = append(errorList, BatchItemError{
					ErrorCode:    e.Code,
					ErrorMessage: e.Message,
					Index:        i,
				})

				continue
			}

			errorList = append(errorList, BatchItemError{
				ErrorCode:    errInternalServer,
				ErrorMessage: err.Error(),
				Index:        i,
			})

			continue
		}

		results = append(results, BatchDetectKeyPhrasesItemResult{
			Index:      i,
			KeyPhrases: resp.KeyPhrases,
		})
	}

	writeJSON(w, &BatchDetectKeyPhrasesResponse{
		ErrorList:  errorList,
		ResultList: results,
	})
}

// BatchDetectSyntax handles the BatchDetectSyntax API.
func (s *Service) BatchDetectSyntax(w http.ResponseWriter, r *http.Request) {
	var req BatchDetectSyntaxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, &Error{
			Code:    errInvalidRequest,
			Message: msgInvalidRequestBody,
		})

		return
	}

	results := make([]BatchDetectSyntaxItemResult, 0, len(req.TextList))
	errorList := make([]BatchItemError, 0)

	for i, text := range req.TextList {
		resp, err := s.analyzer.DetectSyntax(text, req.LanguageCode)
		if err != nil {
			var e *Error
			if errors.As(err, &e) {
				errorList = append(errorList, BatchItemError{
					ErrorCode:    e.Code,
					ErrorMessage: e.Message,
					Index:        i,
				})

				continue
			}

			errorList = append(errorList, BatchItemError{
				ErrorCode:    errInternalServer,
				ErrorMessage: err.Error(),
				Index:        i,
			})

			continue
		}

		results = append(results, BatchDetectSyntaxItemResult{
			Index:        i,
			SyntaxTokens: resp.SyntaxTokens,
		})
	}

	writeJSON(w, &BatchDetectSyntaxResponse{
		ErrorList:  errorList,
		ResultList: results,
	})
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, e *Error) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.WriteHeader(http.StatusBadRequest)

	_ = json.NewEncoder(w).Encode(e)
}
