// Package backup provides an AWS Backup service emulator.
package backup

import (
	"encoding/json"
	"net/http"
	"strings"
)

// CreateBackupVault handles PUT /backup-vaults/{backupVaultName}.
func (s *Service) CreateBackupVault(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("backupVaultName")
	if name == "" {
		writeError(w, "InvalidParameterValueException", "backup vault name is required", http.StatusBadRequest)

		return
	}

	var input CreateBackupVaultInput
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, "InvalidRequestException", "invalid request body", http.StatusBadRequest)

			return
		}
	}

	vault, err := s.storage.CreateVault(name, &input)
	if err != nil {
		if strings.Contains(err.Error(), "AlreadyExistsException") {
			writeError(w, "AlreadyExistsException", err.Error(), http.StatusConflict)

			return
		}

		writeError(w, "ServiceUnavailableException", err.Error(), http.StatusInternalServerError)

		return
	}

	writeJSON(w, &CreateBackupVaultResponse{
		BackupVaultArn:  vault.BackupVaultArn,
		BackupVaultName: vault.BackupVaultName,
		CreationDate:    vault.CreationDate,
	})
}

// DescribeBackupVault handles GET /backup-vaults/{backupVaultName}.
func (s *Service) DescribeBackupVault(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("backupVaultName")
	if name == "" {
		writeError(w, "InvalidParameterValueException", "backup vault name is required", http.StatusBadRequest)

		return
	}

	vault, err := s.storage.DescribeVault(name)
	if err != nil {
		writeError(w, "ResourceNotFoundException", err.Error(), http.StatusNotFound)

		return
	}

	writeJSON(w, vault)
}

// ListBackupVaults handles GET /backup-vaults.
func (s *Service) ListBackupVaults(w http.ResponseWriter, _ *http.Request) {
	vaults := s.storage.ListVaults()
	writeJSON(w, &ListBackupVaultsResponse{
		BackupVaultList: vaults,
	})
}

// DeleteBackupVault handles DELETE /backup-vaults/{backupVaultName}.
func (s *Service) DeleteBackupVault(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("backupVaultName")
	if name == "" {
		writeError(w, "InvalidParameterValueException", "backup vault name is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteVault(name); err != nil {
		writeError(w, "ResourceNotFoundException", err.Error(), http.StatusNotFound)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// CreateBackupPlan handles PUT /backup/plans.
func (s *Service) CreateBackupPlan(w http.ResponseWriter, r *http.Request) {
	var input CreateBackupPlanInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, "InvalidRequestException", "invalid request body", http.StatusBadRequest)

		return
	}

	if input.BackupPlan == nil || input.BackupPlan.BackupPlanName == "" {
		writeError(w, "InvalidParameterValueException", "backup plan name is required", http.StatusBadRequest)

		return
	}

	plan, err := s.storage.CreatePlan(&input)
	if err != nil {
		writeError(w, "ServiceUnavailableException", err.Error(), http.StatusInternalServerError)

		return
	}

	writeJSON(w, &CreateBackupPlanResponse{
		BackupPlanArn: plan.BackupPlanArn,
		BackupPlanID:  plan.BackupPlanID,
		CreationDate:  plan.CreationDate,
		VersionID:     plan.VersionID,
	})
}

// GetBackupPlan handles GET /backup/plans/{backupPlanId}.
func (s *Service) GetBackupPlan(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("backupPlanId")
	if planID == "" {
		writeError(w, "InvalidParameterValueException", "backup plan ID is required", http.StatusBadRequest)

		return
	}

	plan, err := s.storage.GetPlan(planID)
	if err != nil {
		writeError(w, "ResourceNotFoundException", err.Error(), http.StatusNotFound)

		return
	}

	writeJSON(w, &GetBackupPlanResponse{
		BackupPlan:    plan.BackupPlan,
		BackupPlanArn: plan.BackupPlanArn,
		BackupPlanID:  plan.BackupPlanID,
		CreationDate:  plan.CreationDate,
		VersionID:     plan.VersionID,
	})
}

// ListBackupPlans handles GET /backup/plans.
func (s *Service) ListBackupPlans(w http.ResponseWriter, _ *http.Request) {
	plans := s.storage.ListPlans()
	writeJSON(w, &ListBackupPlansResponse{
		BackupPlansList: plans,
	})
}

// DeleteBackupPlan handles DELETE /backup/plans/{backupPlanId}.
func (s *Service) DeleteBackupPlan(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("backupPlanId")
	if planID == "" {
		writeError(w, "InvalidParameterValueException", "backup plan ID is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeletePlan(planID); err != nil {
		writeError(w, "ResourceNotFoundException", err.Error(), http.StatusNotFound)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// CreateBackupSelection handles PUT /backup/plans/{backupPlanId}/selections.
func (s *Service) CreateBackupSelection(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("backupPlanId")
	if planID == "" {
		writeError(w, "InvalidParameterValueException", "backup plan ID is required", http.StatusBadRequest)

		return
	}

	var input CreateBackupSelectionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, "InvalidRequestException", "invalid request body", http.StatusBadRequest)

		return
	}

	if input.BackupSelection == nil || input.BackupSelection.SelectionName == "" {
		writeError(w, "InvalidParameterValueException", "selection name is required", http.StatusBadRequest)

		return
	}

	selection, err := s.storage.CreateSelection(planID, &input)
	if err != nil {
		if strings.Contains(err.Error(), "ResourceNotFoundException") {
			writeError(w, "ResourceNotFoundException", err.Error(), http.StatusNotFound)

			return
		}

		writeError(w, "ServiceUnavailableException", err.Error(), http.StatusInternalServerError)

		return
	}

	writeJSON(w, &CreateBackupSelectionResponse{
		BackupPlanID: selection.BackupPlanID,
		CreationDate: selection.CreationDate,
		SelectionID:  selection.SelectionID,
	})
}

// GetBackupSelection handles GET /backup/plans/{backupPlanId}/selections/{selectionId}.
func (s *Service) GetBackupSelection(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("backupPlanId")
	selectionID := r.PathValue("selectionId")

	if planID == "" || selectionID == "" {
		writeError(w, "InvalidParameterValueException", "plan ID and selection ID are required", http.StatusBadRequest)

		return
	}

	selection, err := s.storage.GetSelection(planID, selectionID)
	if err != nil {
		writeError(w, "ResourceNotFoundException", err.Error(), http.StatusNotFound)

		return
	}

	writeJSON(w, &GetBackupSelectionResponse{
		BackupPlanID:    selection.BackupPlanID,
		BackupSelection: selection.BackupSelection,
		CreationDate:    selection.CreationDate,
		SelectionID:     selection.SelectionID,
	})
}

// ListBackupSelections handles GET /backup/plans/{backupPlanId}/selections.
func (s *Service) ListBackupSelections(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("backupPlanId")
	if planID == "" {
		writeError(w, "InvalidParameterValueException", "backup plan ID is required", http.StatusBadRequest)

		return
	}

	selections := s.storage.ListSelections(planID)
	writeJSON(w, &ListBackupSelectionsResponse{
		BackupSelectionsList: selections,
	})
}

// DeleteBackupSelection handles DELETE /backup/plans/{backupPlanId}/selections/{selectionId}.
func (s *Service) DeleteBackupSelection(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("backupPlanId")
	selectionID := r.PathValue("selectionId")

	if planID == "" || selectionID == "" {
		writeError(w, "InvalidParameterValueException", "plan ID and selection ID are required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteSelection(planID, selectionID); err != nil {
		writeError(w, "ResourceNotFoundException", err.Error(), http.StatusNotFound)

		return
	}

	w.WriteHeader(http.StatusOK)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("x-amzn-ErrorType", code)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(&ErrorResponse{
		Code:    code,
		Message: message,
	})
}
