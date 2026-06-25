package verifiedpermissions

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// idpSchema is a minimal Cedar schema with a single Issuer namespace.
const idpSchema = `{"Issuer":{"entityTypes":{"User":{},"Program":{}},"actions":{}}}`

// handlerCase is one row of a per-action handler test table. arrange builds the
// request body (and any prerequisite state) without *testing.T; verify takes
// *testing.T for assertions.
type handlerCase struct {
	name string
	// body is the raw request body, used when arrange is nil.
	body string
	// arrange builds the request body, creating any prerequisite resources.
	arrange func(svc *Service) string
	// wantStatus is the expected HTTP status.
	wantStatus int
	// wantType is the expected error __type; empty for success responses.
	wantType string
	// verify runs additional assertions on the service state and the response.
	verify func(t *testing.T, svc *Service, w *httptest.ResponseRecorder)
}

// runHandlerCases dispatches each case against the given action and checks the
// status, error type, and any extra verification, on a fresh service per case.
func runHandlerCases(t *testing.T, action string, cases []handlerCase) {
	t.Helper()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := New(NewMemoryStorage())

			body := tc.body
			if tc.arrange != nil {
				body = tc.arrange(svc)
			}

			w := dispatch(t, svc, action, body)
			assertStatus(t, w, tc.wantStatus, tc.wantType)

			if tc.verify != nil {
				tc.verify(t, svc, w)
			}
		})
	}
}

func dispatch(t *testing.T, svc *Service, action, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("X-Amz-Target", "VerifiedPermissions."+action)

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	return w
}

// assertStatus checks the response status and, when wantType is set, the AVP
// error __type.
func assertStatus(t *testing.T, w *httptest.ResponseRecorder, wantStatus int, wantType string) {
	t.Helper()

	if w.Code != wantStatus {
		t.Fatalf("status: got %d, want %d, body=%s", w.Code, wantStatus, w.Body.String())
	}

	if wantType == "" {
		return
	}

	var resp errorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}

	if resp.Type != wantType {
		t.Errorf("__type: got %q, want %q", resp.Type, wantType)
	}
}

// jsonBody marshals v to a request body. The request structs never fail to
// marshal, so the error is ignored.
func jsonBody(v any) string {
	data, _ := json.Marshal(v)

	return string(data)
}

// Setup helpers build prerequisite state directly through storage, keeping the
// arrange callbacks free of *testing.T.

func newStore(svc *Service) string {
	return svc.storage.CreatePolicyStore("OFF", "").ID
}

func newPolicy(svc *Service, storeID, statement string) string {
	policy, _ := svc.storage.CreatePolicy(storeID, "", statement)

	return policy.ID
}

func newIdentitySource(svc *Service, storeID string) string {
	source, _ := svc.storage.CreateIdentitySource(
		storeID,
		"Issuer::User",
		"arn:aws:cognito-idp:us-east-1:000000000000:userpool/us-east-1_abc",
		[]string{"client-1"},
	)

	return source.ID
}

func putSchemaState(svc *Service, storeID string) {
	_, _ = svc.storage.PutSchema(storeID, idpSchema)
}

// newTaggedStore creates a policy store, tags it, and returns its ARN.
func newTaggedStore(svc *Service, tags map[string]string) string {
	store := svc.storage.CreatePolicyStore("OFF", "")
	svc.storage.TagResource(store.ARN, tags)

	return store.ARN
}

// isAuthBody builds an IsAuthorized request body for the IdP entities with the
// given action and permission level.
func isAuthBody(storeID, action, level string) string {
	return jsonBody(IsAuthorizedRequest{
		PolicyStoreID: storeID,
		Principal:     &EntityIdentifier{EntityType: "Issuer::User", EntityID: "user-1"},
		Action:        &ActionIdentifier{ActionType: "Issuer::Action", ActionID: action},
		Resource:      &EntityIdentifier{EntityType: "Issuer::Program", EntityID: "program-1"},
		Context: &ContextDefinition{ContextMap: map[string]AttributeValue{
			"permission_level": {String: ptr(level)},
		}},
	})
}

// verify helpers below match handlerCase.verify so they can be referenced
// directly. They use white-box access to the store, fine for an in-package test.

func verifyStoreDeleted(t *testing.T, svc *Service, _ *httptest.ResponseRecorder) {
	t.Helper()

	if len(svc.storage.Stores) != 0 {
		t.Errorf("stores remain after delete: %d", len(svc.storage.Stores))
	}
}

func verifyPolicyDeleted(t *testing.T, svc *Service, _ *httptest.ResponseRecorder) {
	t.Helper()

	for _, policies := range svc.storage.Policies {
		if len(policies) != 0 {
			t.Errorf("policies remain after delete: %d", len(policies))
		}
	}
}

func verifyIdentitySourceDeleted(t *testing.T, svc *Service, _ *httptest.ResponseRecorder) {
	t.Helper()

	for _, sources := range svc.storage.IdentitySources {
		if len(sources) != 0 {
			t.Errorf("identity sources remain after delete: %d", len(sources))
		}
	}
}

func verifyPolicyUpdated(t *testing.T, svc *Service, _ *httptest.ResponseRecorder) {
	t.Helper()

	for _, policies := range svc.storage.Policies {
		for _, p := range policies {
			if p.Statement != writePolicyStatement {
				t.Errorf("statement not updated: %q", p.Statement)
			}
		}
	}
}

func TestCreatePolicyStore(t *testing.T) {
	t.Parallel()

	verifyIDs := func(t *testing.T, _ *Service, w *httptest.ResponseRecorder) {
		t.Helper()

		var resp CreatePolicyStoreResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if resp.PolicyStoreID == "" || resp.ARN == "" {
			t.Errorf("missing ids: %+v", resp)
		}
	}

	verifyTagged := func(t *testing.T, svc *Service, w *httptest.ResponseRecorder) {
		t.Helper()
		verifyIDs(t, svc, w)

		if len(svc.storage.Tags) != 1 {
			t.Fatalf("tagged resources: got %d, want 1", len(svc.storage.Tags))
		}
	}

	runHandlerCases(t, "CreatePolicyStore", []handlerCase{
		{name: "with validation settings", body: `{"validationSettings":{"mode":"OFF"}}`, wantStatus: http.StatusOK, verify: verifyIDs},
		{name: "empty body defaults", body: "", wantStatus: http.StatusOK, verify: verifyIDs},
		{name: "with tags", body: `{"tags":{"env":"local"}}`, wantStatus: http.StatusOK, verify: verifyTagged},
	})
}

func TestGetPolicyStore(t *testing.T) {
	t.Parallel()

	runHandlerCases(t, "GetPolicyStore", []handlerCase{
		{
			name:       "existing store",
			arrange:    func(svc *Service) string { return jsonBody(GetPolicyStoreRequest{PolicyStoreID: newStore(svc)}) },
			wantStatus: http.StatusOK,
			verify: func(t *testing.T, _ *Service, w *httptest.ResponseRecorder) {
				t.Helper()

				var resp GetPolicyStoreResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("decode: %v", err)
				}

				if resp.ARN == "" {
					t.Error("arn: empty")
				}

				if resp.ValidationSettings == nil || resp.ValidationSettings.Mode != "OFF" {
					t.Errorf("validation mode: got %+v, want OFF", resp.ValidationSettings)
				}
			},
		},
		{name: "missing store", body: `{"policyStoreId":"PS-missing"}`, wantStatus: http.StatusNotFound, wantType: errResourceNotFound},
	})
}

func TestDeletePolicyStore(t *testing.T) {
	t.Parallel()

	runHandlerCases(t, "DeletePolicyStore", []handlerCase{
		{
			name:       "existing store",
			arrange:    func(svc *Service) string { return jsonBody(DeletePolicyStoreRequest{PolicyStoreID: newStore(svc)}) },
			wantStatus: http.StatusOK,
			verify:     verifyStoreDeleted,
		},
		{name: "missing store", body: `{"policyStoreId":"PS-missing"}`, wantStatus: http.StatusNotFound, wantType: errResourceNotFound},
	})
}

func TestListPolicyStores(t *testing.T) {
	t.Parallel()

	wantCount := func(want int) func(t *testing.T, _ *Service, w *httptest.ResponseRecorder) {
		return func(t *testing.T, _ *Service, w *httptest.ResponseRecorder) {
			t.Helper()

			var resp ListPolicyStoresResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}

			if len(resp.PolicyStores) != want {
				t.Errorf("policyStores: got %d, want %d", len(resp.PolicyStores), want)
			}
		}
	}

	runHandlerCases(t, "ListPolicyStores", []handlerCase{
		{name: "no stores", body: `{}`, wantStatus: http.StatusOK, verify: wantCount(0)},
		{
			name: "two stores",
			arrange: func(svc *Service) string {
				newStore(svc)
				newStore(svc)

				return `{}`
			},
			wantStatus: http.StatusOK,
			verify:     wantCount(2),
		},
	})
}

func TestPutSchema(t *testing.T) {
	t.Parallel()

	runHandlerCases(t, "PutSchema", []handlerCase{
		{
			name: "valid schema",
			arrange: func(svc *Service) string {
				return jsonBody(PutSchemaRequest{PolicyStoreID: newStore(svc), Definition: &SchemaDefinition{CedarJSON: idpSchema}})
			},
			wantStatus: http.StatusOK,
			verify: func(t *testing.T, _ *Service, w *httptest.ResponseRecorder) {
				t.Helper()

				var resp PutSchemaResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("decode: %v", err)
				}

				if len(resp.Namespaces) != 1 || resp.Namespaces[0] != "Issuer" {
					t.Errorf("namespaces: got %v, want [Issuer]", resp.Namespaces)
				}
			},
		},
		{
			name: "missing store",
			arrange: func(_ *Service) string {
				return jsonBody(PutSchemaRequest{PolicyStoreID: "PS-missing", Definition: &SchemaDefinition{CedarJSON: idpSchema}})
			},
			wantStatus: http.StatusNotFound,
			wantType:   errResourceNotFound,
		},
	})
}

func TestGetSchema(t *testing.T) {
	t.Parallel()

	runHandlerCases(t, "GetSchema", []handlerCase{
		{
			name: "after put",
			arrange: func(svc *Service) string {
				storeID := newStore(svc)
				putSchemaState(svc, storeID)

				return jsonBody(GetSchemaRequest{PolicyStoreID: storeID})
			},
			wantStatus: http.StatusOK,
			verify: func(t *testing.T, _ *Service, w *httptest.ResponseRecorder) {
				t.Helper()

				var resp GetSchemaResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("decode: %v", err)
				}

				if resp.Schema != idpSchema {
					t.Errorf("schema: got %q, want %q", resp.Schema, idpSchema)
				}

				if len(resp.Namespaces) != 1 || resp.Namespaces[0] != "Issuer" {
					t.Errorf("namespaces: got %v, want [Issuer]", resp.Namespaces)
				}
			},
		},
		{
			name:       "no schema yet",
			arrange:    func(svc *Service) string { return jsonBody(GetSchemaRequest{PolicyStoreID: newStore(svc)}) },
			wantStatus: http.StatusNotFound,
			wantType:   errResourceNotFound,
		},
		{name: "missing store", body: `{"policyStoreId":"PS-missing"}`, wantStatus: http.StatusNotFound, wantType: errResourceNotFound},
	})
}

func TestCreatePolicy(t *testing.T) {
	t.Parallel()

	policyBody := func(storeID, statement string) string {
		return jsonBody(CreatePolicyRequest{
			PolicyStoreID: storeID,
			Definition:    &PolicyDefinition{Static: &StaticPolicyDefinition{Statement: statement}},
		})
	}

	runHandlerCases(t, "CreatePolicy", []handlerCase{
		{
			name:       "valid static policy",
			arrange:    func(svc *Service) string { return policyBody(newStore(svc), readPolicyStatement) },
			wantStatus: http.StatusOK,
			verify: func(t *testing.T, _ *Service, w *httptest.ResponseRecorder) {
				t.Helper()

				var resp PolicyMutationResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("decode: %v", err)
				}

				if resp.PolicyID == "" || resp.PolicyType != "STATIC" {
					t.Errorf("unexpected response: %+v", resp)
				}
			},
		},
		{
			name:       "invalid statement",
			arrange:    func(svc *Service) string { return policyBody(newStore(svc), "not a cedar policy") },
			wantStatus: http.StatusBadRequest,
			wantType:   errValidation,
		},
		{
			name:       "missing definition",
			arrange:    func(svc *Service) string { return jsonBody(CreatePolicyRequest{PolicyStoreID: newStore(svc)}) },
			wantStatus: http.StatusBadRequest,
			wantType:   errValidation,
		},
		{
			name:       "missing store",
			arrange:    func(_ *Service) string { return policyBody("PS-missing", readPolicyStatement) },
			wantStatus: http.StatusNotFound,
			wantType:   errResourceNotFound,
		},
	})
}

func TestGetPolicy(t *testing.T) {
	t.Parallel()

	runHandlerCases(t, "GetPolicy", []handlerCase{
		{
			name: "existing policy",
			arrange: func(svc *Service) string {
				storeID := newStore(svc)

				return jsonBody(GetPolicyRequest{PolicyStoreID: storeID, PolicyID: newPolicy(svc, storeID, readPolicyStatement)})
			},
			wantStatus: http.StatusOK,
			verify: func(t *testing.T, _ *Service, w *httptest.ResponseRecorder) {
				t.Helper()

				var resp GetPolicyResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("decode: %v", err)
				}

				if resp.Definition == nil || resp.Definition.Static == nil || resp.Definition.Static.Statement != readPolicyStatement {
					t.Fatalf("definition: %+v", resp.Definition)
				}
			},
		},
		{
			name: "missing policy",
			arrange: func(svc *Service) string {
				return jsonBody(GetPolicyRequest{PolicyStoreID: newStore(svc), PolicyID: "PB-missing"})
			},
			wantStatus: http.StatusNotFound,
			wantType:   errResourceNotFound,
		},
		{name: "missing store", body: `{"policyStoreId":"PS-missing","policyId":"PB-x"}`, wantStatus: http.StatusNotFound, wantType: errResourceNotFound},
	})
}

func TestListPolicies(t *testing.T) {
	t.Parallel()

	runHandlerCases(t, "ListPolicies", []handlerCase{
		{
			name: "with one policy",
			arrange: func(svc *Service) string {
				storeID := newStore(svc)
				newPolicy(svc, storeID, readPolicyStatement)

				return jsonBody(ListPoliciesRequest{PolicyStoreID: storeID})
			},
			wantStatus: http.StatusOK,
			verify: func(t *testing.T, _ *Service, w *httptest.ResponseRecorder) {
				t.Helper()

				var resp ListPoliciesResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("decode: %v", err)
				}

				if len(resp.Policies) != 1 {
					t.Fatalf("policies: got %d, want 1", len(resp.Policies))
				}
			},
		},
		{name: "missing store", body: `{"policyStoreId":"PS-missing"}`, wantStatus: http.StatusNotFound, wantType: errResourceNotFound},
	})
}

func TestUpdatePolicy(t *testing.T) {
	t.Parallel()

	updateBody := func(storeID, policyID, statement string) string {
		return jsonBody(UpdatePolicyRequest{
			PolicyStoreID: storeID,
			PolicyID:      policyID,
			Definition:    &PolicyDefinition{Static: &StaticPolicyDefinition{Statement: statement}},
		})
	}

	runHandlerCases(t, "UpdatePolicy", []handlerCase{
		{
			name: "valid update",
			arrange: func(svc *Service) string {
				storeID := newStore(svc)

				return updateBody(storeID, newPolicy(svc, storeID, readPolicyStatement), writePolicyStatement)
			},
			wantStatus: http.StatusOK,
			verify:     verifyPolicyUpdated,
		},
		{
			name: "missing definition",
			arrange: func(svc *Service) string {
				return jsonBody(UpdatePolicyRequest{PolicyStoreID: newStore(svc), PolicyID: "PB-x"})
			},
			wantStatus: http.StatusBadRequest,
			wantType:   errValidation,
		},
		{
			name: "invalid statement",
			arrange: func(svc *Service) string {
				storeID := newStore(svc)

				return updateBody(storeID, newPolicy(svc, storeID, readPolicyStatement), "broken")
			},
			wantStatus: http.StatusBadRequest,
			wantType:   errValidation,
		},
		{
			name:       "missing store and policy",
			arrange:    func(_ *Service) string { return updateBody("PS-missing", "PB-x", readPolicyStatement) },
			wantStatus: http.StatusNotFound,
			wantType:   errResourceNotFound,
		},
	})
}

func TestDeletePolicy(t *testing.T) {
	t.Parallel()

	runHandlerCases(t, "DeletePolicy", []handlerCase{
		{
			name: "existing policy",
			arrange: func(svc *Service) string {
				storeID := newStore(svc)

				return jsonBody(DeletePolicyRequest{PolicyStoreID: storeID, PolicyID: newPolicy(svc, storeID, readPolicyStatement)})
			},
			wantStatus: http.StatusOK,
			verify:     verifyPolicyDeleted,
		},
		{
			name: "missing policy",
			arrange: func(svc *Service) string {
				return jsonBody(DeletePolicyRequest{PolicyStoreID: newStore(svc), PolicyID: "PB-missing"})
			},
			wantStatus: http.StatusNotFound,
			wantType:   errResourceNotFound,
		},
	})
}

func TestCreateIdentitySource(t *testing.T) {
	t.Parallel()

	withConfig := func(storeID string) string {
		return jsonBody(CreateIdentitySourceRequest{
			PolicyStoreID:       storeID,
			PrincipalEntityType: "Issuer::User",
			Configuration: &IdentitySourceConfiguration{
				CognitoUserPoolConfiguration: &CognitoUserPoolConfiguration{
					UserPoolARN: "arn:aws:cognito-idp:us-east-1:000000000000:userpool/us-east-1_abc",
					ClientIDs:   []string{"client-1"},
				},
			},
		})
	}

	runHandlerCases(t, "CreateIdentitySource", []handlerCase{
		{
			name:       "with cognito configuration",
			arrange:    func(svc *Service) string { return withConfig(newStore(svc)) },
			wantStatus: http.StatusOK,
			verify: func(t *testing.T, _ *Service, w *httptest.ResponseRecorder) {
				t.Helper()

				var resp CreateIdentitySourceResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("decode: %v", err)
				}

				if resp.IdentitySourceID == "" {
					t.Error("identitySourceId: empty")
				}
			},
		},
		{
			name: "without configuration",
			arrange: func(svc *Service) string {
				return jsonBody(CreateIdentitySourceRequest{PolicyStoreID: newStore(svc), PrincipalEntityType: "Issuer::User"})
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing store",
			arrange:    func(_ *Service) string { return withConfig("PS-missing") },
			wantStatus: http.StatusNotFound,
			wantType:   errResourceNotFound,
		},
	})
}

func TestGetIdentitySource(t *testing.T) {
	t.Parallel()

	runHandlerCases(t, "GetIdentitySource", []handlerCase{
		{
			name: "existing source",
			arrange: func(svc *Service) string {
				storeID := newStore(svc)

				return jsonBody(GetIdentitySourceRequest{PolicyStoreID: storeID, IdentitySourceID: newIdentitySource(svc, storeID)})
			},
			wantStatus: http.StatusOK,
			verify: func(t *testing.T, _ *Service, w *httptest.ResponseRecorder) {
				t.Helper()

				var resp GetIdentitySourceResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("decode: %v", err)
				}

				if resp.IdentitySourceID == "" {
					t.Error("identitySourceId: empty")
				}

				if resp.PrincipalEntityType != "Issuer::User" {
					t.Errorf("principalEntityType: got %q, want Issuer::User", resp.PrincipalEntityType)
				}

				if resp.Configuration == nil || resp.Configuration.CognitoUserPoolConfiguration == nil {
					t.Fatal("configuration.cognitoUserPoolConfiguration: missing")
				}

				cfg := resp.Configuration.CognitoUserPoolConfiguration
				if cfg.UserPoolARN != "arn:aws:cognito-idp:us-east-1:000000000000:userpool/us-east-1_abc" {
					t.Errorf("userPoolArn: got %q", cfg.UserPoolARN)
				}

				if len(cfg.ClientIDs) != 1 || cfg.ClientIDs[0] != "client-1" {
					t.Errorf("clientIds: got %v, want [client-1]", cfg.ClientIDs)
				}
			},
		},
		{
			name: "missing source",
			arrange: func(svc *Service) string {
				return jsonBody(GetIdentitySourceRequest{PolicyStoreID: newStore(svc), IdentitySourceID: "IS-missing"})
			},
			wantStatus: http.StatusNotFound,
			wantType:   errResourceNotFound,
		},
	})
}

func TestListIdentitySources(t *testing.T) {
	t.Parallel()

	runHandlerCases(t, "ListIdentitySources", []handlerCase{
		{
			name: "with one source",
			arrange: func(svc *Service) string {
				storeID := newStore(svc)
				newIdentitySource(svc, storeID)

				return jsonBody(ListIdentitySourcesRequest{PolicyStoreID: storeID})
			},
			wantStatus: http.StatusOK,
			verify: func(t *testing.T, _ *Service, w *httptest.ResponseRecorder) {
				t.Helper()

				var resp ListIdentitySourcesResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("decode: %v", err)
				}

				if len(resp.IdentitySources) != 1 {
					t.Fatalf("identitySources: got %d, want 1", len(resp.IdentitySources))
				}
			},
		},
		{name: "missing store", body: `{"policyStoreId":"PS-missing"}`, wantStatus: http.StatusNotFound, wantType: errResourceNotFound},
	})
}

func TestDeleteIdentitySource(t *testing.T) {
	t.Parallel()

	runHandlerCases(t, "DeleteIdentitySource", []handlerCase{
		{
			name: "existing source",
			arrange: func(svc *Service) string {
				storeID := newStore(svc)

				return jsonBody(DeleteIdentitySourceRequest{PolicyStoreID: storeID, IdentitySourceID: newIdentitySource(svc, storeID)})
			},
			wantStatus: http.StatusOK,
			verify:     verifyIdentitySourceDeleted,
		},
		{
			name: "missing source",
			arrange: func(svc *Service) string {
				return jsonBody(DeleteIdentitySourceRequest{PolicyStoreID: newStore(svc), IdentitySourceID: "IS-missing"})
			},
			wantStatus: http.StatusNotFound,
			wantType:   errResourceNotFound,
		},
	})
}

func TestListTagsForResource(t *testing.T) {
	t.Parallel()

	wantTags := func(want map[string]string) func(t *testing.T, _ *Service, w *httptest.ResponseRecorder) {
		return func(t *testing.T, _ *Service, w *httptest.ResponseRecorder) {
			t.Helper()

			var resp ListTagsForResourceResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}

			if len(resp.Tags) != len(want) {
				t.Fatalf("tags: got %v, want %v", resp.Tags, want)
			}

			for k, v := range want {
				if resp.Tags[k] != v {
					t.Errorf("tag %q: got %q, want %q", k, resp.Tags[k], v)
				}
			}
		}
	}

	runHandlerCases(t, "ListTagsForResource", []handlerCase{
		{
			name: "tagged store",
			arrange: func(svc *Service) string {
				return jsonBody(ListTagsForResourceRequest{ResourceARN: newTaggedStore(svc, map[string]string{"env": "local"})})
			},
			wantStatus: http.StatusOK,
			verify:     wantTags(map[string]string{"env": "local"}),
		},
		{
			name:       "untagged arn returns empty",
			body:       `{"resourceArn":"arn:aws:verifiedpermissions::000000000000:policy-store/PS-none"}`,
			wantStatus: http.StatusOK,
			verify:     wantTags(map[string]string{}),
		},
		{name: "missing resourceArn", body: `{}`, wantStatus: http.StatusBadRequest, wantType: errValidation},
	})
}

func TestTagResource(t *testing.T) {
	t.Parallel()

	verifyTagged := func(t *testing.T, svc *Service, _ *httptest.ResponseRecorder) {
		t.Helper()

		if len(svc.storage.Tags) != 1 {
			t.Fatalf("tagged resources: got %d, want 1", len(svc.storage.Tags))
		}

		for _, tags := range svc.storage.Tags {
			if tags["env"] != "local" {
				t.Errorf("env tag: got %q, want local", tags["env"])
			}
		}
	}

	runHandlerCases(t, "TagResource", []handlerCase{
		{
			name: "tags a store",
			arrange: func(svc *Service) string {
				store := svc.storage.CreatePolicyStore("OFF", "")

				return jsonBody(TagResourceRequest{ResourceARN: store.ARN, Tags: map[string]string{"env": "local"}})
			},
			wantStatus: http.StatusOK,
			verify:     verifyTagged,
		},
		{name: "missing resourceArn", body: `{"tags":{"a":"b"}}`, wantStatus: http.StatusBadRequest, wantType: errValidation},
	})
}

func TestUntagResource(t *testing.T) {
	t.Parallel()

	verifyUntagged := func(t *testing.T, svc *Service, _ *httptest.ResponseRecorder) {
		t.Helper()

		for _, tags := range svc.storage.Tags {
			if _, ok := tags["env"]; ok {
				t.Error("env tag not removed")
			}

			if tags["team"] != "idp" {
				t.Errorf("team tag: got %q, want idp", tags["team"])
			}
		}
	}

	runHandlerCases(t, "UntagResource", []handlerCase{
		{
			name: "removes a tag",
			arrange: func(svc *Service) string {
				arn := newTaggedStore(svc, map[string]string{"env": "local", "team": "idp"})

				return jsonBody(UntagResourceRequest{ResourceARN: arn, TagKeys: []string{"env"}})
			},
			wantStatus: http.StatusOK,
			verify:     verifyUntagged,
		},
		{name: "missing resourceArn", body: `{"tagKeys":["x"]}`, wantStatus: http.StatusBadRequest, wantType: errValidation},
	})
}

func TestIsAuthorized(t *testing.T) {
	t.Parallel()

	// authRequest sets up a store with the read and write policies, then returns
	// an IsAuthorized body for the given action and permission level.
	authRequest := func(action, level string) func(svc *Service) string {
		return func(svc *Service) string {
			storeID := newStore(svc)
			newPolicy(svc, storeID, readPolicyStatement)
			newPolicy(svc, storeID, writePolicyStatement)

			return isAuthBody(storeID, action, level)
		}
	}

	wantDecision := func(decision string) func(t *testing.T, _ *Service, w *httptest.ResponseRecorder) {
		return func(t *testing.T, _ *Service, w *httptest.ResponseRecorder) {
			t.Helper()

			var resp IsAuthorizedResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}

			if resp.Decision != decision {
				t.Fatalf("decision: got %q, want %q", resp.Decision, decision)
			}
		}
	}

	runHandlerCases(t, "IsAuthorized", []handlerCase{
		{name: "read by read-only is allowed", arrange: authRequest("Read", "read-only"), wantStatus: http.StatusOK, verify: wantDecision("ALLOW")},
		{name: "read by read-write is allowed", arrange: authRequest("Read", "read-write"), wantStatus: http.StatusOK, verify: wantDecision("ALLOW")},
		{name: "write by read-write is allowed", arrange: authRequest("Write", "read-write"), wantStatus: http.StatusOK, verify: wantDecision("ALLOW")},
		{name: "write by read-only is denied", arrange: authRequest("Write", "read-only"), wantStatus: http.StatusOK, verify: wantDecision("DENY")},
	})
}

func TestIsAuthorizedErrors(t *testing.T) {
	t.Parallel()

	invalidContext := func(svc *Service) string {
		storeID := newStore(svc)
		newPolicy(svc, storeID, readPolicyStatement)

		// An empty AttributeValue (no field set) makes buildRequest fail.
		return jsonBody(IsAuthorizedRequest{
			PolicyStoreID: storeID,
			Principal:     &EntityIdentifier{EntityType: "Issuer::User", EntityID: "u1"},
			Action:        &ActionIdentifier{ActionType: "Issuer::Action", ActionID: "Read"},
			Resource:      &EntityIdentifier{EntityType: "Issuer::Program", EntityID: "p1"},
			Context:       &ContextDefinition{ContextMap: map[string]AttributeValue{"permission_level": {}}},
		})
	}

	brokenPolicy := func(svc *Service) string {
		storeID := newStore(svc)

		// Inject an unparseable policy directly; CreatePolicy would reject it.
		svc.storage.Policies[storeID] = map[string]*Policy{
			"bad": {ID: "bad", PolicyStoreID: storeID, Statement: "this is not cedar"},
		}

		return jsonBody(IsAuthorizedRequest{
			PolicyStoreID: storeID,
			Principal:     &EntityIdentifier{EntityType: "Issuer::User", EntityID: "u1"},
			Action:        &ActionIdentifier{ActionType: "Issuer::Action", ActionID: "Read"},
			Resource:      &EntityIdentifier{EntityType: "Issuer::Program", EntityID: "p1"},
		})
	}

	runHandlerCases(t, "IsAuthorized", []handlerCase{
		{name: "missing policyStoreId", body: `{}`, wantStatus: http.StatusBadRequest, wantType: errValidation},
		{name: "store not found", body: `{"policyStoreId":"PS-missing"}`, wantStatus: http.StatusNotFound, wantType: errResourceNotFound},
		{name: "invalid context value", arrange: invalidContext, wantStatus: http.StatusBadRequest, wantType: errValidation},
		{name: "broken stored policy", arrange: brokenPolicy, wantStatus: http.StatusBadRequest, wantType: errValidation},
	})
}

func TestDispatch_UnknownAction(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage())

	w := dispatch(t, svc, "Frobnicate", `{}`)
	assertStatus(t, w, http.StatusBadRequest, errInvalidAction)
}

// TestHandlers_MalformedBody asserts every action rejects an unparseable body
// with a 400, covering the request-decode error branch in each handler.
func TestHandlers_MalformedBody(t *testing.T) {
	t.Parallel()

	actions := []string{
		"CreatePolicyStore", "GetPolicyStore", "DeletePolicyStore", "ListPolicyStores",
		"PutSchema", "GetSchema", "CreatePolicy", "GetPolicy", "ListPolicies",
		"UpdatePolicy", "DeletePolicy", "CreateIdentitySource", "GetIdentitySource",
		"ListIdentitySources", "DeleteIdentitySource", "IsAuthorized",
		"ListTagsForResource", "TagResource", "UntagResource",
	}

	svc := New(NewMemoryStorage())

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			t.Parallel()

			w := dispatch(t, svc, action, `{"oops"`)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("%s: status %d, want 400, body=%s", action, w.Code, w.Body.String())
			}
		})
	}
}

// errReader is an io.Reader that always fails, to exercise body-read errors.
type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("read failure")
}

func TestReadJSONRequest_BodyReadError(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/", errReader{})

	var v map[string]any
	if err := readJSONRequest(req, &v); err == nil {
		t.Fatal("expected error from a failing body reader, got nil")
	}
}

func TestWriteServiceError_FallbackToInternal(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	writeServiceError(w, errors.New("not a service error"))

	assertStatus(t, w, http.StatusInternalServerError, errInternal)
}
