package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseServiceFromUserAgent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		userAgent string
		want      string
	}{
		{
			name:      "standard AWS SDK Go v2 with rds",
			userAgent: "aws-sdk-go-v2/1.36.3 ua/2.1 api/rds#1.5.0 os/linux lang/go#1.25.0",
			want:      "rds",
		},
		{
			name:      "neptune",
			userAgent: "aws-sdk-go-v2/1.36.3 ua/2.1 api/neptune#1.48.11 os/linux lang/go#1.25.0",
			want:      "neptune",
		},
		{
			name:      "docdb",
			userAgent: "aws-sdk-go-v2/1.36.3 api/docdb#1.48.11",
			want:      "docdb",
		},
		{
			name:      "no api token",
			userAgent: "aws-sdk-go-v2/1.36.3 ua/2.1",
			want:      "",
		},
		{
			name:      "empty string",
			userAgent: "",
			want:      "",
		},
		{
			name:      "Java sdk sns upper case",
			userAgent: "aws-sdk-java/2.44.12 md/io#sync md/http#Apache ua/2.1 api/SNS#2.44.x os/Linux#6.1 lang/java#17 md/OpenJDK_64-Bit_Server_VM#17 md/vendor#OpenJDK md/en_US",
			want:      "sns",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := parseServiceFromUserAgent(tt.userAgent)
			if got != tt.want {
				t.Errorf("parseServiceFromUserAgent(%q) = %q, want %q", tt.userAgent, got, tt.want)
			}
		})
	}
}

func TestQueryDispatcher_RoutesViaUserAgent(t *testing.T) {
	t.Parallel()

	d := NewQueryProtocolDispatcher()

	called := false

	d.RegisterAction("DescribeInstances", "EC2", "ec2", func(w http.ResponseWriter, _ *http.Request) {
		called = true

		w.WriteHeader(http.StatusOK)
	})

	body := strings.NewReader("Action=DescribeInstances&Version=2016-11-15")
	req := httptest.NewRequest(http.MethodPost, "/", body)

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "aws-sdk-go-v2/1.36.3 api/ec2#1.0.0 os/linux")

	rec := httptest.NewRecorder()

	d.ServeHTTP(rec, req)

	if !called {
		t.Error("expected handler to be called")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestQueryDispatcher_DisambiguatesOverlappingActions(t *testing.T) {
	t.Parallel()

	d := NewQueryProtocolDispatcher()

	rdsHandler := false
	neptuneHandler := false

	d.RegisterAction("CreateDBCluster", "AmazonRDSv19", "rds", func(w http.ResponseWriter, _ *http.Request) {
		rdsHandler = true

		w.WriteHeader(http.StatusOK)
	})
	d.RegisterAction("CreateDBCluster", "AmazonNeptuneDataService", "neptune", func(w http.ResponseWriter, _ *http.Request) {
		neptuneHandler = true

		w.WriteHeader(http.StatusOK)
	})

	body := strings.NewReader("Action=CreateDBCluster&Version=2014-10-31")
	req := httptest.NewRequest(http.MethodPost, "/", body)

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "aws-sdk-go-v2/1.36.3 api/neptune#1.48.11 os/linux")

	rec := httptest.NewRecorder()

	d.ServeHTTP(rec, req)

	if rdsHandler {
		t.Error("RDS handler should not have been called")
	}

	if !neptuneHandler {
		t.Error("Neptune handler should have been called")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestQueryDispatcher_MissingUserAgent_UniqueAction(t *testing.T) {
	t.Parallel()

	d := NewQueryProtocolDispatcher()

	called := false

	d.RegisterAction("CreateDBCluster", "AmazonRDSv19", "rds", func(w http.ResponseWriter, _ *http.Request) {
		called = true

		w.WriteHeader(http.StatusOK)
	})

	body := strings.NewReader("Action=CreateDBCluster&Version=2014-10-31")
	req := httptest.NewRequest(http.MethodPost, "/", body)

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()

	d.ServeHTTP(rec, req)

	// When the action is unique across all services, the dispatcher should
	// fall back to action-based lookup even without a User-Agent header.
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	if !called {
		t.Error("expected handler to be called via fallback")
	}
}

func TestQueryDispatcher_MissingUserAgent_AmbiguousAction(t *testing.T) {
	t.Parallel()

	d := NewQueryProtocolDispatcher()

	d.RegisterAction("CreateDBCluster", "AmazonRDSv19", "rds", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	d.RegisterAction("CreateDBCluster", "AmazonNeptuneDataService", "neptune", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	body := strings.NewReader("Action=CreateDBCluster&Version=2014-10-31")
	req := httptest.NewRequest(http.MethodPost, "/", body)

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()

	d.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "AmbiguousAction") {
		t.Errorf("expected AmbiguousAction error, got %s", rec.Body.String())
	}
}

func TestQueryDispatcher_UnknownService(t *testing.T) {
	t.Parallel()

	d := NewQueryProtocolDispatcher()

	d.RegisterAction("CreateDBCluster", "AmazonRDSv19", "rds", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	body := strings.NewReader("Action=CreateDBCluster&Version=2014-10-31")
	req := httptest.NewRequest(http.MethodPost, "/", body)

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "aws-sdk-go-v2/1.36.3 api/unknown#1.0.0 os/linux")

	rec := httptest.NewRecorder()

	d.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "UnknownService") {
		t.Errorf("expected UnknownService error, got %s", rec.Body.String())
	}
}

func TestQueryDispatcher_UnknownAction(t *testing.T) {
	t.Parallel()

	d := NewQueryProtocolDispatcher()

	d.RegisterAction("DescribeInstances", "EC2", "ec2", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	body := strings.NewReader("Action=NonExistentAction&Version=2016-11-15")
	req := httptest.NewRequest(http.MethodPost, "/", body)

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "aws-sdk-go-v2/1.36.3 api/ec2#1.0.0 os/linux")

	rec := httptest.NewRecorder()

	d.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "UnknownAction") {
		t.Errorf("expected UnknownAction error, got %s", rec.Body.String())
	}
}
