package cognito

import "net/http"

// GetUserPoolMfaConfig reports MFA as OFF for any user pool.
//
// Required for terraform compatibility — terraform-provider-aws calls
// GetUserPoolMfaConfig on every refresh of aws_cognito_user_pool;
// without it, `tofu apply` errors with InvalidAction immediately after
// the refresh completes the (now-fixed) resourceUserPoolRead.
//
// MFA configuration is not modeled in storage yet. Same shape as the
// other refresh stubs in kumo — wire-level no-op with the door open
// for real persistence later.
func (s *Service) GetUserPoolMfaConfig(w http.ResponseWriter, _ *http.Request) {
	writeResponse(w, getUserPoolMFAConfigResponse{
		MfaConfiguration: "OFF",
	})
}

// SetUserPoolMfaConfig accepts and discards an MFA configuration update.
func (s *Service) SetUserPoolMfaConfig(w http.ResponseWriter, _ *http.Request) {
	writeResponse(w, getUserPoolMFAConfigResponse{
		MfaConfiguration: "OFF",
	})
}
