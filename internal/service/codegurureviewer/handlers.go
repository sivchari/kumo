// Package codegurureviewer implements the AWS CodeGuru Reviewer service.
package codegurureviewer

import (
	"encoding/json"
	"net/http"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, errType, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("x-amzn-ErrorType", errType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(&ErrorResponse{
		Message: message,
		Type:    errType,
	})
}

// AssociateRepository handles POST /associations.
func (s *Service) AssociateRepository(w http.ResponseWriter, r *http.Request) {
	var input AssociateRepositoryInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, "ValidationException", "invalid request body", http.StatusBadRequest)

		return
	}

	assoc := s.storage.AssociateRepository(&input)

	writeJSON(w, &AssociateRepositoryResponse{
		RepositoryAssociation: assoc,
		Tags:                  assoc.Tags,
	})
}

// DescribeRepositoryAssociation handles GET /associations/{AssociationArn}.
func (s *Service) DescribeRepositoryAssociation(w http.ResponseWriter, r *http.Request) {
	arn := r.PathValue("AssociationArn")

	assoc, err := s.storage.DescribeRepositoryAssociation(arn)
	if err != nil {
		writeError(w, "NotFoundException", err.Error(), http.StatusNotFound)

		return
	}

	writeJSON(w, &AssociateRepositoryResponse{
		RepositoryAssociation: assoc,
		Tags:                  assoc.Tags,
	})
}

// DisassociateRepository handles DELETE /associations/{AssociationArn}.
func (s *Service) DisassociateRepository(w http.ResponseWriter, r *http.Request) {
	arn := r.PathValue("AssociationArn")

	assoc, err := s.storage.DisassociateRepository(arn)
	if err != nil {
		writeError(w, "NotFoundException", err.Error(), http.StatusNotFound)

		return
	}

	writeJSON(w, &AssociateRepositoryResponse{
		RepositoryAssociation: assoc,
		Tags:                  assoc.Tags,
	})
}

// ListRepositoryAssociations handles GET /associations.
func (s *Service) ListRepositoryAssociations(w http.ResponseWriter, _ *http.Request) {
	associations := s.storage.ListRepositoryAssociations()

	writeJSON(w, &ListRepositoryAssociationsResponse{
		RepositoryAssociationSummaries: associations,
	})
}

// CreateCodeReview handles POST /codereviews.
func (s *Service) CreateCodeReview(w http.ResponseWriter, r *http.Request) {
	var input CreateCodeReviewInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, "ValidationException", "invalid request body", http.StatusBadRequest)

		return
	}

	review, err := s.storage.CreateCodeReview(&input)
	if err != nil {
		writeError(w, "NotFoundException", err.Error(), http.StatusNotFound)

		return
	}

	writeJSON(w, &CodeReviewResponse{
		CodeReview: review,
	})
}

// DescribeCodeReview handles GET /codereviews/{CodeReviewArn}.
func (s *Service) DescribeCodeReview(w http.ResponseWriter, r *http.Request) {
	arn := r.PathValue("CodeReviewArn")

	review, err := s.storage.DescribeCodeReview(arn)
	if err != nil {
		writeError(w, "ResourceNotFoundException", err.Error(), http.StatusNotFound)

		return
	}

	writeJSON(w, &CodeReviewResponse{
		CodeReview: review,
	})
}

// ListCodeReviews handles GET /codereviews.
func (s *Service) ListCodeReviews(w http.ResponseWriter, _ *http.Request) {
	reviews := s.storage.ListCodeReviews()

	writeJSON(w, &ListCodeReviewsResponse{
		CodeReviewSummaries: reviews,
	})
}

// ListRecommendations handles GET /codereviews/{CodeReviewArn}/Recommendations.
func (s *Service) ListRecommendations(w http.ResponseWriter, r *http.Request) {
	arn := r.PathValue("CodeReviewArn")
	recommendations := s.storage.ListRecommendations(arn)

	writeJSON(w, &ListRecommendationsResponse{
		RecommendationSummaries: recommendations,
	})
}

// PutRecommendationFeedback handles PUT /feedback.
func (s *Service) PutRecommendationFeedback(w http.ResponseWriter, r *http.Request) {
	var input PutRecommendationFeedbackInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, "ValidationException", "invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.PutRecommendationFeedback(&input); err != nil {
		writeError(w, "ResourceNotFoundException", err.Error(), http.StatusNotFound)

		return
	}

	writeJSON(w, struct{}{})
}

// DescribeRecommendationFeedback handles GET /feedback/{CodeReviewArn}.
func (s *Service) DescribeRecommendationFeedback(w http.ResponseWriter, r *http.Request) {
	arn := r.PathValue("CodeReviewArn")
	recommendationID := r.URL.Query().Get("RecommendationId")

	fb, err := s.storage.DescribeRecommendationFeedback(arn, recommendationID)
	if err != nil {
		writeError(w, "ResourceNotFoundException", err.Error(), http.StatusNotFound)

		return
	}

	writeJSON(w, &RecommendationFeedbackResponse{
		RecommendationFeedback: fb,
	})
}

// ListRecommendationFeedback handles GET /feedback/{CodeReviewArn}/RecommendationFeedback.
func (s *Service) ListRecommendationFeedback(w http.ResponseWriter, r *http.Request) {
	arn := r.PathValue("CodeReviewArn")
	feedbackList := s.storage.ListRecommendationFeedback(arn)

	writeJSON(w, &ListRecommendationFeedbackResponse{
		RecommendationFeedbackSummaries: feedbackList,
	})
}
