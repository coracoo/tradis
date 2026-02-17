package system

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeAndResolveNavigationIcon_Remote404Fallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<!doctype html><html><head><link rel="icon" href="/real.ico"></head><body>ok</body></html>`))
		case "/favicon.ico":
			w.Header().Set("Content-Type", "image/x-icon")
			w.WriteHeader(http.StatusNotFound)
		case "/real.ico":
			w.Header().Set("Content-Type", "image/x-icon")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte{0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x10, 0x10})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	got := normalizeAndResolveNavigationIcon(srv.URL+"/favicon.ico", srv.URL, "")
	if got != srv.URL+"/real.ico" {
		t.Fatalf("unexpected icon: %s", got)
	}
}

