package source

import (
	"context"
	"github.com/r14r/update-cli/lib/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchURLRejectsOversize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="demo-v1.0.0.zip"`)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(strings.Repeat("x", 1024)))
	}))
	defer srv.Close()
	_, err := Fetch(context.Background(), Options{ProjectName: "demo", Source: config.SourceConfig{Type: "url", URL: srv.URL, Version: "1.0.0"}, WorkDir: t.TempDir(), AllowHTTP: true, MaxArchiveBytes: 100})
	if err == nil || !(strings.Contains(err.Error(), "maximale Downloadgröße") || strings.Contains(err.Error(), "zu groß")) {
		t.Fatalf("expected size error, got %v", err)
	}
}

func TestDiscoverURLFallsBackFromHeadToRangeGet(t *testing.T) {
	methods := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename="demo-v1.2.3.zip"`)
		w.Header().Set("Content-Range", "bytes 0-0/12345")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("x"))
	}))
	defer srv.Close()
	m, err := Discover(context.Background(), Options{ProjectName: "demo", Source: config.SourceConfig{Type: "url", URL: srv.URL}, AllowHTTP: true, MaxArchiveBytes: 20000})
	if err != nil {
		t.Fatal(err)
	}
	if m.VersionText != "1.2.3" || m.Size != 12345 {
		t.Fatalf("unexpected metadata: %#v", m)
	}
	if len(methods) != 2 || methods[0] != "HEAD" || methods[1] != "GET" {
		t.Fatalf("unexpected methods: %#v", methods)
	}
}
