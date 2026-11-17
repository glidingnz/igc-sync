package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient_SendsUserAgent(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
	}))
	defer srv.Close()

	resp, err := NewClient("v1.2.3").Get(srv.URL)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	resp.Body.Close()

	want := "igc-sync/v1.2.3 (+https://github.com/glidingnz/igc-sync)"
	if got != want {
		t.Errorf("User-Agent = %q, want %q", got, want)
	}
}
