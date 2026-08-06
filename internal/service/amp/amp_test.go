package amp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRemoteWrite_NoBackend confirms the data-plane path returns 502
// + a clear hint when KUMO_AMP_BACKEND isn't configured.
func TestRemoteWrite_NoBackend(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ws, _ := store.CreateWorkspace(t.Context(), "test", nil)
	svc := New(store, "")

	req := httptest.NewRequest(http.MethodPost,
		"/workspaces/"+ws.WorkspaceID+"/api/v1/remote_write", strings.NewReader("ignored"))
	req.SetPathValue("workspaceId", ws.WorkspaceID)

	w := httptest.NewRecorder()
	svc.RemoteWrite(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 without backend, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "KUMO_AMP_BACKEND") {
		t.Fatalf("error message should mention env var: %s", w.Body.String())
	}
}

func TestNew_PrebuildsDataPlaneProxy(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage(), "127.0.0.1:9090")

	if svc.dataPlaneProxy == nil {
		t.Fatalf("expected data-plane proxy to be initialized once")
	}
}

func TestRemoteWrite_RejectsBackendWithScheme(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ws, _ := store.CreateWorkspace(t.Context(), "proxy", nil)
	svc := New(store, "http://127.0.0.1:9090")

	req := httptest.NewRequest(http.MethodPost,
		"/workspaces/"+ws.WorkspaceID+"/api/v1/remote_write", strings.NewReader("ignored"))
	req.SetPathValue("workspaceId", ws.WorkspaceID)

	w := httptest.NewRecorder()
	svc.RemoteWrite(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for invalid backend, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "without scheme") {
		t.Fatalf("error message should explain host:port backend: %s", w.Body.String())
	}
}

// TestRemoteWrite_ProxiesToBackend stands up a tiny upstream HTTP
// server playing the role of a real Prometheus, and verifies the
// remote_write body lands there byte-for-byte.
func TestRemoteWrite_ProxiesToBackend(t *testing.T) {
	t.Parallel()

	const probeBody = "snappy-compressed-protobuf-goes-here"

	var (
		gotPath string
		gotBody string
	)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	store := NewMemoryStorage()
	ws, _ := store.CreateWorkspace(t.Context(), "proxy", nil)
	svc := New(store, strings.TrimPrefix(upstream.URL, "http://"))

	req := httptest.NewRequest(http.MethodPost,
		"/workspaces/"+ws.WorkspaceID+"/api/v1/remote_write", strings.NewReader(probeBody))
	req.SetPathValue("workspaceId", ws.WorkspaceID)
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "snappy")

	w := httptest.NewRecorder()
	svc.RemoteWrite(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("proxy passed-through status mismatch: got %d, want 204", w.Code)
	}

	if gotPath != "/api/v1/write" {
		t.Fatalf("upstream path: got %q, want /api/v1/write", gotPath)
	}

	if gotBody != probeBody {
		t.Fatalf("upstream body mismatch: got %q, want %q", gotBody, probeBody)
	}
}

// TestRemoteWrite_UnknownWorkspace returns NotFound (matching AWS).
func TestRemoteWrite_UnknownWorkspace(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage(), "http://anywhere")

	req := httptest.NewRequest(http.MethodPost,
		"/workspaces/ws-bogus/api/v1/remote_write", strings.NewReader(""))
	req.SetPathValue("workspaceId", "ws-bogus")

	w := httptest.NewRecorder()
	svc.RemoteWrite(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown workspace, got %d", w.Code)
	}
}
