package apigatewayv2

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"testing"
	"time"
)

// TestMemoryStorage_CreateAuthorizer_APINotFound is kept: no golden test
// exercises CreateAuthorizer against a non-existent API ID.
func TestMemoryStorage_CreateAuthorizer_APINotFound(t *testing.T) {
	t.Parallel()

	storage := NewMemoryStorage()

	_, err := storage.CreateAuthorizer(t.Context(), "missing-api", &CreateAuthorizerRequest{
		Name:           "jwt-auth",
		AuthorizerType: authorizerTypeJWT,
	})
	if err == nil {
		t.Fatal("CreateAuthorizer() with unknown apiId: expected error, got nil")
	}
}

// ---- JWT decode/validate ----

func buildTestJWT(t *testing.T, claims map[string]any) string {
	t.Helper()

	header := map[string]string{"alg": "none", "typ": "JWT"}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}

	enc := base64.RawURLEncoding

	return enc.EncodeToString(headerJSON) + "." + enc.EncodeToString(claimsJSON) + ".signature"
}

func TestDecodeJWTPayload(t *testing.T) {
	t.Parallel()

	token := buildTestJWT(t, map[string]any{"sub": "user1", "exp": 1234567890})

	claims, err := decodeJWTPayload(token)
	if err != nil {
		t.Fatalf("decodeJWTPayload() error = %v", err)
	}

	if claims["sub"] != "user1" {
		t.Errorf("claims[sub] = %v, want %q", claims["sub"], "user1")
	}
}

func TestDecodeJWTPayload_Malformed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		token string
	}{
		{name: "too few segments", token: "onlyonepart"},
		{name: "invalid base64", token: "a.!!!not-base64!!!.c"},
		{name: "invalid json", token: "a." + base64.RawURLEncoding.EncodeToString([]byte("not json")) + ".c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := decodeJWTPayload(tt.token); err == nil {
				t.Error("decodeJWTPayload() expected error, got nil")
			}
		})
	}
}

// validateJWTClaimsTestCase is a table entry shared by the
// TestValidateJWTClaims_* tests below, split by concern (expiration, issuer,
// audience/client_id) to stay within funlen.
type validateJWTClaimsTestCase struct {
	name   string
	claims map[string]any
	cfg    *JWTConfiguration
	want   bool
}

func runValidateJWTClaimsCases(t *testing.T, tests []validateJWTClaimsTestCase) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := validateJWTClaims(tt.claims, tt.cfg); got != tt.want {
				t.Errorf("validateJWTClaims() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateJWTClaims_Expiration(t *testing.T) {
	t.Parallel()

	future := time.Now().Add(time.Hour).Unix()
	past := time.Now().Add(-time.Hour).Unix()

	runValidateJWTClaimsCases(t, []validateJWTClaimsTestCase{
		{
			name:   "valid: no config beyond exp",
			claims: map[string]any{"exp": json.Number(intToStr(future))},
			want:   true,
		},
		{
			name:   "expired token rejected",
			claims: map[string]any{"exp": json.Number(intToStr(past))},
			want:   false,
		},
		{
			name:   "missing exp rejected",
			claims: map[string]any{},
			want:   false,
		},
	})
}

func TestValidateJWTClaims_Issuer(t *testing.T) {
	t.Parallel()

	future := time.Now().Add(time.Hour).Unix()

	runValidateJWTClaimsCases(t, []validateJWTClaimsTestCase{
		{
			name: "issuer mismatch rejected",
			claims: map[string]any{
				"exp": json.Number(intToStr(future)),
				"iss": "https://wrong.example.com",
			},
			cfg:  &JWTConfiguration{Issuer: "https://issuer.example.com"},
			want: false,
		},
		{
			name: "issuer match accepted",
			claims: map[string]any{
				"exp": json.Number(intToStr(future)),
				"iss": "https://issuer.example.com",
			},
			cfg:  &JWTConfiguration{Issuer: "https://issuer.example.com"},
			want: true,
		},
	})
}

func TestValidateJWTClaims_Audience(t *testing.T) {
	t.Parallel()

	future := time.Now().Add(time.Hour).Unix()

	runValidateJWTClaimsCases(t, []validateJWTClaimsTestCase{
		{
			name: "audience string match accepted",
			claims: map[string]any{
				"exp": json.Number(intToStr(future)),
				"aud": "aud1",
			},
			cfg:  &JWTConfiguration{Audience: []string{"aud1", "aud2"}},
			want: true,
		},
		{
			name: "audience array match accepted",
			claims: map[string]any{
				"exp": json.Number(intToStr(future)),
				"aud": []any{"other", "aud2"},
			},
			cfg:  &JWTConfiguration{Audience: []string{"aud1", "aud2"}},
			want: true,
		},
		{
			name: "audience mismatch rejected",
			claims: map[string]any{
				"exp": json.Number(intToStr(future)),
				"aud": "unknown",
			},
			cfg:  &JWTConfiguration{Audience: []string{"aud1"}},
			want: false,
		},
		{
			name: "client_id used when aud absent (Cognito access tokens)",
			claims: map[string]any{
				"exp":       json.Number(intToStr(future)),
				"client_id": "aud1",
			},
			cfg:  &JWTConfiguration{Audience: []string{"aud1"}},
			want: true,
		},
		{
			name: "aud takes precedence over client_id when both present",
			claims: map[string]any{
				"exp":       json.Number(intToStr(future)),
				"aud":       "unknown",
				"client_id": "aud1",
			},
			cfg:  &JWTConfiguration{Audience: []string{"aud1"}},
			want: false,
		},
	})
}

func TestStringifyClaimValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   any
		want string
	}{
		{name: "string", in: "hello", want: "hello"},
		{name: "number", in: json.Number("1700000000"), want: "1700000000"},
		{name: "bool true", in: true, want: "true"},
		{name: "bool false", in: false, want: "false"},
		{name: "nil", in: nil, want: ""},
		{name: "array", in: []any{"a", "b"}, want: `["a","b"]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := stringifyClaimValue(tt.in); got != tt.want {
				t.Errorf("stringifyClaimValue(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestScopesFromClaims(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		claims map[string]any
		want   []string
	}{
		{name: "absent scope", claims: map[string]any{}, want: nil},
		{name: "single scope", claims: map[string]any{"scope": "read"}, want: []string{"read"}},
		{
			name:   "space-delimited scopes",
			claims: map[string]any{"scope": "read write admin"},
			want:   []string{"read", "write", "admin"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := scopesFromClaims(tt.claims); !slices.Equal(got, tt.want) {
				t.Errorf("scopesFromClaims() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBearerToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		header         string
		identitySource []string
		wantToken      string
		wantOK         bool
	}{
		{
			name:      "missing header",
			header:    "",
			wantToken: "",
			wantOK:    false,
		},
		{
			name:      "malformed (no Bearer prefix)",
			header:    "sometoken",
			wantToken: "",
			wantOK:    false,
		},
		{
			name:      "well-formed default header",
			header:    "Bearer abc.def.ghi",
			wantToken: "abc.def.ghi",
			wantOK:    true,
		},
		{
			name:           "explicit default identity source",
			header:         "Bearer abc.def.ghi",
			identitySource: []string{"$request.header.Authorization"},
			wantToken:      "abc.def.ghi",
			wantOK:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items", nil)
			if tt.header != "" {
				r.Header.Set("Authorization", tt.header)
			}

			token, ok := bearerToken(r, tt.identitySource)
			if ok != tt.wantOK || token != tt.wantToken {
				t.Errorf("bearerToken() = (%q, %v), want (%q, %v)", token, ok, tt.wantToken, tt.wantOK)
			}
		})
	}
}

func intToStr(v int64) string {
	return strconv.FormatInt(v, 10)
}
