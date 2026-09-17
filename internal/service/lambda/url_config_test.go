package lambda

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
)

const (
	testURLFunction = "url-fn"
	authTypeNoneT   = "NONE"
	authTypeIAMT    = "AWS_IAM"
	invokeBuffered  = "BUFFERED"
)

var urlIDPattern = regexp.MustCompile(`^[0-9a-z]{32}$`)

// newStorageWithFunction returns a storage holding the test function.
func newStorageWithFunction(t *testing.T) *MemoryStorage {
	t.Helper()

	storage := NewMemoryStorage("http://localhost:4566")

	if _, err := storage.CreateFunction(context.Background(), &CreateFunctionRequest{
		FunctionName: testURLFunction,
		Runtime:      "provided.al2",
		Role:         "arn:aws:iam::000000000000:role/test-role",
		Handler:      "index.handler",
		Code:         FunctionCode{ZipFile: []byte("zip")},
	}); err != nil {
		t.Fatalf("CreateFunction() error = %v", err)
	}

	return storage
}

func testCORS() *FunctionURLCORS {
	return &FunctionURLCORS{
		AllowCredentials: true,
		AllowOrigins:     []string{"https://example.com"},
		AllowMethods:     []string{"GET", "POST"},
		AllowHeaders:     []string{"content-type"},
		ExposeHeaders:    []string{"x-request-id"},
		MaxAge:           3600,
	}
}

func createTestURL(t *testing.T, storage *MemoryStorage) *FunctionURLConfig {
	t.Helper()

	cfg, err := storage.CreateFunctionURLConfig(context.Background(), testURLFunction, FunctionURLConfigSpec{
		AuthType:   authTypeNoneT,
		InvokeMode: invokeBuffered,
		Cors:       testCORS(),
	})
	if err != nil {
		t.Fatalf("CreateFunctionURLConfig() error = %v", err)
	}

	return cfg
}

// functionErrorOf asserts err is a *FunctionError of wantType and returns it.
func functionErrorOf(t *testing.T, err error, wantType string) *FunctionError {
	t.Helper()

	var fnErr *FunctionError
	if !errors.As(err, &fnErr) {
		t.Fatalf("expected *FunctionError, got %T: %v", err, err)
	}

	if fnErr.Type != wantType {
		t.Fatalf("error type = %s (%s), want %s", fnErr.Type, fnErr.Message, wantType)
	}

	return fnErr
}

func assertNotFound(t *testing.T, err error) {
	t.Helper()

	_ = functionErrorOf(t, err, ErrResourceNotFound)
}

func TestFunctionURLConfig_CreateGetDelete(t *testing.T) {
	t.Parallel()

	storage := newStorageWithFunction(t)
	ctx := context.Background()

	created := createTestURL(t, storage)

	if !urlIDPattern.MatchString(created.URLID) {
		t.Errorf("URLID %q must be 32 lowercase alphanumerics", created.URLID)
	}

	if want := "https://" + created.URLID + ".lambda-url.us-east-1.on.aws/"; created.FunctionURL != want {
		t.Errorf("FunctionURL = %q, want %q", created.FunctionURL, want)
	}

	if created.FunctionArn != "arn:aws:lambda:us-east-1:000000000000:function:"+testURLFunction {
		t.Errorf("FunctionArn = %q", created.FunctionArn)
	}

	if created.AuthType != authTypeNoneT || created.InvokeMode != invokeBuffered || created.Cors == nil {
		t.Errorf("unexpected config %+v", created)
	}

	if created.CreationTime.IsZero() || !created.LastModifiedTime.Equal(created.CreationTime) {
		t.Errorf("times: creation=%v modified=%v", created.CreationTime, created.LastModifiedTime)
	}

	got, err := storage.GetFunctionURLConfig(ctx, testURLFunction)
	if err != nil || got.URLID != created.URLID {
		t.Fatalf("GetFunctionURLConfig() = %+v, %v", got, err)
	}

	if err := storage.DeleteFunctionURLConfig(ctx, testURLFunction); err != nil {
		t.Fatalf("DeleteFunctionURLConfig() error = %v", err)
	}

	_, err = storage.GetFunctionURLConfig(ctx, testURLFunction)
	fnErr := functionErrorOf(t, err, ErrResourceNotFound)

	if fnErr.Message != "The resource you requested does not exist." {
		t.Errorf("message = %q", fnErr.Message)
	}
}

func TestFunctionURLConfig_CreateTwiceConflicts(t *testing.T) {
	t.Parallel()

	storage := newStorageWithFunction(t)
	created := createTestURL(t, storage)

	_, err := storage.CreateFunctionURLConfig(context.Background(), testURLFunction, FunctionURLConfigSpec{AuthType: authTypeNoneT, InvokeMode: invokeBuffered})
	fnErr := functionErrorOf(t, err, ErrResourceConflict)

	want := "Failed to create function url config for [functionArn = " + created.FunctionArn + "]. Error message:  FunctionUrlConfig exists for this Lambda function"
	if fnErr.Message != want {
		t.Errorf("message = %q, want %q", fnErr.Message, want)
	}
}

func TestFunctionURLConfig_MissingFunctionOrConfig(t *testing.T) {
	t.Parallel()

	storage := newStorageWithFunction(t)
	ctx := context.Background()

	_, err := storage.CreateFunctionURLConfig(ctx, "missing", FunctionURLConfigSpec{AuthType: authTypeNoneT, InvokeMode: invokeBuffered})
	assertNotFound(t, err)

	_, err = storage.UpdateFunctionURLConfig(ctx, testURLFunction, FunctionURLConfigUpdate{})
	assertNotFound(t, err)

	err = storage.DeleteFunctionURLConfig(ctx, testURLFunction)
	assertNotFound(t, err)
}

func TestFunctionURLConfig_UpdateAppliesOnlyPresentFields(t *testing.T) {
	t.Parallel()

	storage := newStorageWithFunction(t)
	created := createTestURL(t, storage)

	authType := authTypeIAMT

	updated, err := storage.UpdateFunctionURLConfig(context.Background(), testURLFunction, FunctionURLConfigUpdate{AuthType: &authType})
	if err != nil {
		t.Fatalf("UpdateFunctionURLConfig() error = %v", err)
	}

	if updated.AuthType != authTypeIAMT || updated.InvokeMode != invokeBuffered || updated.Cors == nil || updated.Cors.MaxAge != 3600 {
		t.Errorf("partial update changed unrelated fields: %+v (cors %+v)", updated, updated.Cors)
	}

	if updated.URLID != created.URLID || updated.FunctionURL != created.FunctionURL || !updated.CreationTime.Equal(created.CreationTime) {
		t.Errorf("update must keep the URL identity: %+v", updated)
	}

	if updated.LastModifiedTime.Before(created.LastModifiedTime) {
		t.Errorf("LastModifiedTime went backwards")
	}

	replaced, err := storage.UpdateFunctionURLConfig(context.Background(), testURLFunction, FunctionURLConfigUpdate{Cors: &FunctionURLCORS{AllowOrigins: []string{"*"}}})
	if err != nil {
		t.Fatalf("UpdateFunctionURLConfig(cors) error = %v", err)
	}

	if replaced.Cors.MaxAge != 0 || len(replaced.Cors.AllowOrigins) != 1 || replaced.Cors.AllowOrigins[0] != "*" {
		t.Errorf("Cors must be replaced wholesale, got %+v", replaced.Cors)
	}
}

func TestFunctionURLConfig_UpdateWithEmptyCORSClearsIt(t *testing.T) {
	t.Parallel()

	storage := newStorageWithFunction(t)
	createTestURL(t, storage)

	cleared, err := storage.UpdateFunctionURLConfig(context.Background(), testURLFunction, FunctionURLConfigUpdate{ClearCors: true})
	if err != nil {
		t.Fatalf("UpdateFunctionURLConfig() error = %v", err)
	}

	if cleared.Cors != nil {
		t.Fatalf("ClearCors must remove the configuration, got %+v", cleared.Cors)
	}
}

func TestUpdateFunctionURLConfigRequest_EmptyCORSMeansClear(t *testing.T) {
	t.Parallel()

	update, err := (&updateFunctionURLConfigRequest{Cors: &FunctionURLCORS{}}).update()
	if err != nil {
		t.Fatalf("update() error = %v", err)
	}

	if !update.ClearCors || update.Cors != nil {
		t.Fatalf("an empty Cors object must clear the configuration, got %+v", update)
	}

	spec, err := (&createFunctionURLConfigRequest{AuthType: authTypeNoneT, Cors: &FunctionURLCORS{}}).spec()
	if err != nil {
		t.Fatalf("spec() error = %v", err)
	}

	if spec.Cors != nil {
		t.Fatalf("an empty Cors object on create must be stored as absent, got %+v", spec.Cors)
	}
}

func TestFunctionURLConfig_DeleteFunctionCascades(t *testing.T) {
	t.Parallel()

	storage := newStorageWithFunction(t)
	createTestURL(t, storage)

	if err := storage.DeleteFunction(context.Background(), testURLFunction); err != nil {
		t.Fatalf("DeleteFunction() error = %v", err)
	}

	if _, exists := storage.FunctionURLs[testURLFunction]; exists {
		t.Fatal("deleting the function must drop its URL config")
	}
}

func TestFunctionURLConfig_ListShape(t *testing.T) {
	t.Parallel()

	storage := newStorageWithFunction(t)
	ctx := context.Background()

	configs, err := storage.ListFunctionURLConfigs(ctx, testURLFunction)
	if err != nil || configs == nil || len(configs) != 0 {
		t.Fatalf("empty list = %v, %v; want a non-nil empty slice", configs, err)
	}

	created := createTestURL(t, storage)

	configs, err = storage.ListFunctionURLConfigs(ctx, testURLFunction)
	if err != nil || len(configs) != 1 || configs[0].URLID != created.URLID {
		t.Fatalf("list after create = %v, %v", configs, err)
	}

	_, err = storage.ListFunctionURLConfigs(ctx, "missing")
	assertNotFound(t, err)
}

func TestFunctionURLConfig_ReturnsClones(t *testing.T) {
	t.Parallel()

	storage := newStorageWithFunction(t)
	created := createTestURL(t, storage)

	created.AuthType = "tampered"
	created.Cors.AllowOrigins[0] = "tampered"

	got, err := storage.GetFunctionURLConfig(context.Background(), testURLFunction)
	if err != nil {
		t.Fatalf("GetFunctionURLConfig() error = %v", err)
	}

	if got.AuthType != authTypeNoneT || got.Cors.AllowOrigins[0] != "https://example.com" {
		t.Fatalf("storage must not share memory with callers: %+v %+v", got, got.Cors)
	}
}

func TestFunctionURLConfig_SnapshotRoundTrip(t *testing.T) {
	t.Parallel()

	storage := newStorageWithFunction(t)
	created := createTestURL(t, storage)

	snapshot, err := json.Marshal(storage)
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	if !strings.Contains(string(snapshot), `"functionUrls"`) {
		t.Fatalf("snapshot must persist function URLs: %s", snapshot)
	}

	restored := NewMemoryStorage("http://localhost:4566")
	if err := json.Unmarshal(snapshot, restored); err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}

	got, err := restored.GetFunctionURLConfig(context.Background(), testURLFunction)
	if err != nil || got.URLID != created.URLID {
		t.Fatalf("restored config = %+v, %v", got, err)
	}

	// Snapshots written before function URLs existed have no functionUrls key.
	legacy := NewMemoryStorage("http://localhost:4566")
	if err := json.Unmarshal([]byte(`{"functions":{},"eventSourceMappings":{}}`), legacy); err != nil {
		t.Fatalf("UnmarshalJSON(legacy) error = %v", err)
	}

	if legacy.FunctionURLs == nil {
		t.Fatal("FunctionURLs must be initialised when restoring an old snapshot")
	}
}

func TestNewFunctionURLID_IsUnique(t *testing.T) {
	t.Parallel()

	seen := make(map[string]bool)

	for range 100 {
		id, err := newFunctionURLID()
		if err != nil {
			t.Fatalf("newFunctionURLID() error = %v", err)
		}

		if !urlIDPattern.MatchString(id) || seen[id] {
			t.Fatalf("bad or duplicate id %q", id)
		}

		seen[id] = true
	}
}

func TestResolveFunctionName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		identifier, name, qualifier string
	}{
		{"fn", "fn", ""},
		{"fn:prod", "fn", "prod"},
		{"000000000000:function:fn", "fn", ""},
		{"arn:aws:lambda:us-east-1:000000000000:function:fn", "fn", ""},
		{"arn:aws:lambda:us-east-1:000000000000:function:fn:prod", "fn", "prod"},
	}

	for _, tc := range cases {
		name, qualifier := resolveFunctionName(tc.identifier)
		if name != tc.name || qualifier != tc.qualifier {
			t.Errorf("resolveFunctionName(%q) = %q, %q; want %q, %q", tc.identifier, name, qualifier, tc.name, tc.qualifier)
		}
	}
}

func TestBuildPermissionStatement_FunctionURLAuthTypeCondition(t *testing.T) {
	t.Parallel()

	stmt := buildPermissionStatement(&addPermissionRequest{
		StatementID:         "AllowURL",
		Action:              "lambda:InvokeFunctionUrl",
		Principal:           "*",
		SourceAccount:       "000000000000",
		FunctionURLAuthType: authTypeNoneT,
	}, "arn:aws:lambda:us-east-1:000000000000:function:fn")

	equals, ok := stmt.Condition["StringEquals"].(map[string]string)
	if !ok {
		t.Fatalf("StringEquals condition missing: %v", stmt.Condition)
	}

	if equals["lambda:FunctionUrlAuthType"] != authTypeNoneT || equals["AWS:SourceAccount"] != "000000000000" {
		t.Fatalf("StringEquals = %v, want both FunctionUrlAuthType and SourceAccount", equals)
	}
}
