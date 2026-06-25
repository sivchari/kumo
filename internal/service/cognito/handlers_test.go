package cognito

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Shared fixtures for the AdminSetUserPassword tests.
const (
	asupPoolName = "set-pwd-pool"
	asupUsername = "admin@example.com"
	asupOldPass  = "TempPass123!"
	asupNewPass  = "NewPerm456!"
	asupRegion   = "us-east-1"
)

// newSetPasswordFixture builds a service with a pool and a freshly-created user
// (FORCE_CHANGE_PASSWORD), seeded white-box through the storage layer. It
// returns the service, the store, and the created pool id.
func newSetPasswordFixture(t *testing.T) (*Service, *MemoryStorage, string) {
	t.Helper()

	store := NewMemoryStorage()
	svc := New(store)

	pool, err := store.CreateUserPool(t.Context(), &CreateUserPoolRequest{Region: asupRegion, PoolName: asupPoolName})
	if err != nil {
		t.Fatalf("CreateUserPool: %v", err)
	}

	if _, err := store.AdminCreateUser(t.Context(), &AdminCreateUserRequest{
		UserPoolID:        pool.ID,
		Username:          asupUsername,
		TemporaryPassword: asupOldPass,
	}); err != nil {
		t.Fatalf("AdminCreateUser: %v", err)
	}

	return svc, store, pool.ID
}

// dispatchCognitoAction sends a JSON-1.1 action to the service dispatcher and
// returns the recorder, exactly as the wire would deliver it.
func dispatchCognitoAction(t *testing.T, svc *Service, action, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("X-Amz-Target", "AWSCognitoIdentityProviderService."+action)

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	return w
}

// getUserStatus reads the user back through AdminGetUser and returns its status.
func getUserStatus(t *testing.T, svc *Service, poolID, username string) string {
	t.Helper()

	gw := dispatchCognitoAction(t, svc, "AdminGetUser",
		`{"UserPoolId":"`+poolID+`","Username":"`+username+`"}`)

	var resp AdminGetUserResponse
	if err := json.Unmarshal(gw.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode AdminGetUser: %v", err)
	}

	return resp.UserStatus
}

func TestAdminSetUserPassword_PermanentTransitionsToConfirmed(t *testing.T) {
	t.Parallel()

	svc, _, poolID := newSetPasswordFixture(t)

	w := dispatchCognitoAction(t, svc, "AdminSetUserPassword",
		`{"UserPoolId":"`+poolID+`","Username":"`+asupUsername+`","Password":"`+asupNewPass+`","Permanent":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}

	if got := strings.TrimSpace(w.Body.String()); got != "{}" {
		t.Errorf("body: got %q, want %q", got, "{}")
	}

	if status := getUserStatus(t, svc, poolID, asupUsername); status != string(UserStatusConfirmed) {
		t.Errorf("UserStatus: got %q, want %q", status, UserStatusConfirmed)
	}
}

func TestAdminSetUserPassword_NonPermanentSetsResetRequired(t *testing.T) {
	t.Parallel()

	svc, _, poolID := newSetPasswordFixture(t)

	w := dispatchCognitoAction(t, svc, "AdminSetUserPassword",
		`{"UserPoolId":"`+poolID+`","Username":"`+asupUsername+`","Password":"`+asupNewPass+`","Permanent":false}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}

	if status := getUserStatus(t, svc, poolID, asupUsername); status != string(UserStatusResetRequired) {
		t.Errorf("UserStatus: got %q, want %q", status, UserStatusResetRequired)
	}
}

func TestAdminSetUserPassword_PermanentEnablesAuth(t *testing.T) {
	t.Parallel()

	svc, store, poolID := newSetPasswordFixture(t)

	client, err := store.CreateUserPoolClient(t.Context(), &CreateUserPoolClientRequest{
		UserPoolID: poolID,
		ClientName: "auth-client",
	})
	if err != nil {
		t.Fatalf("CreateUserPoolClient: %v", err)
	}

	if w := dispatchCognitoAction(t, svc, "AdminSetUserPassword",
		`{"UserPoolId":"`+poolID+`","Username":"`+asupUsername+`","Password":"`+asupNewPass+`","Permanent":true}`); w.Code != http.StatusOK {
		t.Fatalf("set password status: got %d (body=%s)", w.Code, w.Body.String())
	}

	authBody := `{"UserPoolId":"` + poolID + `","ClientId":"` + client.ClientID +
		`","AuthFlow":"ADMIN_USER_PASSWORD_AUTH","AuthParameters":{"USERNAME":"` + asupUsername +
		`","PASSWORD":"` + asupNewPass + `"}}`

	aw := dispatchCognitoAction(t, svc, "AdminInitiateAuth", authBody)
	if aw.Code != http.StatusOK {
		t.Fatalf("auth status: got %d, want 200 (body=%s)", aw.Code, aw.Body.String())
	}

	var resp AdminInitiateAuthResponse
	if err := json.Unmarshal(aw.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode AdminInitiateAuth: %v", err)
	}

	if resp.AuthenticationResult == nil {
		t.Fatal("AuthenticationResult: nil, want signed tokens")
	}

	if resp.AuthenticationResult.AccessToken == "" || resp.AuthenticationResult.IDToken == "" {
		t.Errorf("tokens: got access=%q id=%q, want both non-empty",
			resp.AuthenticationResult.AccessToken, resp.AuthenticationResult.IDToken)
	}
}

func TestAdminSetUserPassword_Errors(t *testing.T) {
	t.Parallel()

	svc, _, poolID := newSetPasswordFixture(t)

	cases := []struct {
		name     string
		body     string
		wantCode int
		wantType string
	}{
		{
			name:     "user pool not found",
			body:     `{"UserPoolId":"us-east-1_missing","Username":"` + asupUsername + `","Password":"x","Permanent":true}`,
			wantCode: http.StatusNotFound,
			wantType: "ResourceNotFoundException",
		},
		{
			name:     "user not found",
			body:     `{"UserPoolId":"` + poolID + `","Username":"ghost","Password":"x","Permanent":true}`,
			wantCode: http.StatusNotFound,
			wantType: "UserNotFoundException",
		},
		{
			name:     "empty password",
			body:     `{"UserPoolId":"` + poolID + `","Username":"` + asupUsername + `","Password":"","Permanent":true}`,
			wantCode: http.StatusBadRequest,
			wantType: "InvalidParameterException",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			w := dispatchCognitoAction(t, svc, "AdminSetUserPassword", tc.body)
			if w.Code != tc.wantCode {
				t.Fatalf("status: got %d, want %d (body=%s)", w.Code, tc.wantCode, w.Body.String())
			}

			var resp ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode error: %v", err)
			}

			if resp.Type != tc.wantType {
				t.Errorf("__type: got %q, want %q", resp.Type, tc.wantType)
			}
		})
	}
}

// describeUserPool reads a pool back through the DescribeUserPool dispatcher and
// returns its wire output.
func describeUserPool(t *testing.T, svc *Service, poolID string) *UserPoolOutput {
	t.Helper()

	w := dispatchCognitoAction(t, svc, "DescribeUserPool", `{"UserPoolId":"`+poolID+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("DescribeUserPool status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}

	var resp DescribeUserPoolResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode DescribeUserPool: %v", err)
	}

	return resp.UserPool
}

// TestCreateUserPool_AdminCreateUserConfigEchoed verifies the stored
// admin-create-user config is echoed by DescribeUserPool instead of the AWS
// default. Without this, terraform-provider-aws sees drift on
// allow_admin_create_user_only and calls UpdateUserPool every plan
// (docs/idp-parity 14).
func TestCreateUserPool_AdminCreateUserConfigEchoed(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store)

	pool, err := store.CreateUserPool(t.Context(), &CreateUserPoolRequest{
		Region:                "us-east-1",
		PoolName:              "echo-pool",
		AdminCreateUserConfig: &AdminCreateUserConfigInput{AllowAdminCreateUserOnly: true},
	})
	if err != nil {
		t.Fatalf("CreateUserPool: %v", err)
	}

	out := describeUserPool(t, svc, pool.ID)
	if out.AdminCreateUserConfig == nil || !out.AdminCreateUserConfig.AllowAdminCreateUserOnly {
		t.Errorf("AdminCreateUserConfig: got %+v, want AllowAdminCreateUserOnly=true", out.AdminCreateUserConfig)
	}
}

// TestUpdateUserPool_AppliesChanges verifies UpdateUserPool mutates the stored
// pool in place, returns an empty document, and that the change is visible on a
// subsequent DescribeUserPool round-trip.
func TestUpdateUserPool_AppliesChanges(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store)

	pool, err := store.CreateUserPool(t.Context(), &CreateUserPoolRequest{Region: "us-east-1", PoolName: "upd-pool"})
	if err != nil {
		t.Fatalf("CreateUserPool: %v", err)
	}

	if before := describeUserPool(t, svc, pool.ID); before.AdminCreateUserConfig.AllowAdminCreateUserOnly {
		t.Fatalf("precondition: AllowAdminCreateUserOnly already true")
	}

	w := dispatchCognitoAction(t, svc, "UpdateUserPool",
		`{"UserPoolId":"`+pool.ID+`","MfaConfiguration":"OPTIONAL","AdminCreateUserConfig":{"AllowAdminCreateUserOnly":true}}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}

	if got := strings.TrimSpace(w.Body.String()); got != "{}" {
		t.Errorf("body: got %q, want %q", got, "{}")
	}

	after := describeUserPool(t, svc, pool.ID)
	if after.AdminCreateUserConfig == nil || !after.AdminCreateUserConfig.AllowAdminCreateUserOnly {
		t.Errorf("AdminCreateUserConfig: got %+v, want AllowAdminCreateUserOnly=true", after.AdminCreateUserConfig)
	}

	if after.MfaConfiguration != "OPTIONAL" {
		t.Errorf("MfaConfiguration: got %q, want OPTIONAL", after.MfaConfiguration)
	}
}

// TestUpdateUserPool_NotFound verifies updating a missing pool returns
// ResourceNotFoundException.
func TestUpdateUserPool_NotFound(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store)

	w := dispatchCognitoAction(t, svc, "UpdateUserPool", `{"UserPoolId":"us-east-1_missing"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404 (body=%s)", w.Code, w.Body.String())
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if resp.Type != "ResourceNotFoundException" {
		t.Errorf("__type: got %q, want ResourceNotFoundException", resp.Type)
	}
}
