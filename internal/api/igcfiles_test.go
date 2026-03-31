package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchIgcFiles_SinglePage(t *testing.T) {
	body := `{
		"success": true,
		"data": [
			{"id": 1, "filename": "2026-04-01_ZKJ_001.igc", "flight_date": "2026-04-01",
			 "rego": "ZKJ", "flight_number": 1, "size_bytes": 12345,
			 "file_hash": "abc123def456abc123def456abc123def456abc123def456abc123def456abcd",
			 "url": "https://storage.example.com/igc/2026-04-01_ZKJ_001.igc"},
			{"id": 2, "filename": "2026-04-01_ZKL_001.igc", "flight_date": "2026-04-01",
			 "rego": "ZKL", "flight_number": 1, "size_bytes": 9876,
			 "file_hash": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
			 "url": "https://storage.example.com/igc/2026-04-01_ZKL_001.igc"}
		],
		"next_page_url": null,
		"current_page": 1,
		"last_page": 1,
		"total": 2
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/events/42/igc-files" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Write([]byte(body))
	}))
	defer srv.Close()

	files, err := FetchIgcFiles(srv.URL, 42, srv.Client())
	if err != nil {
		t.Fatalf("FetchIgcFiles error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0].Filename != "2026-04-01_ZKJ_001.igc" {
		t.Errorf("unexpected filename: %s", files[0].Filename)
	}
	if files[0].FileHash != "abc123def456abc123def456abc123def456abc123def456abc123def456abcd" {
		t.Errorf("unexpected file_hash: %s", files[0].FileHash)
	}
}

func TestFetchIgcFiles_Pagination(t *testing.T) {
	page1 := `{
		"success": true,
		"data": [{"id": 1, "filename": "file1.igc", "flight_date": "2026-04-01",
		           "rego": "ZKA", "flight_number": 1, "size_bytes": 100,
		           "file_hash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		           "url": "https://example.com/file1.igc"}],
		"next_page_url": "%s/api/v1/events/5/igc-files?page=2",
		"current_page": 1, "last_page": 2
	}`

	page2 := `{
		"success": true,
		"data": [{"id": 2, "filename": "file2.igc", "flight_date": "2026-04-02",
		           "rego": "ZKB", "flight_number": 1, "size_bytes": 200,
		           "file_hash": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		           "url": "https://example.com/file2.igc"}],
		"next_page_url": null,
		"current_page": 2, "last_page": 2
	}`

	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			w.Write([]byte(page2))
		} else {
			w.Write([]byte(fmt.Sprintf(page1, srvURL)))
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

	files, err := FetchIgcFiles(srv.URL, 5, srv.Client())
	if err != nil {
		t.Fatalf("FetchIgcFiles pagination error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files across pages, got %d", len(files))
	}
	if files[0].Filename != "file1.igc" || files[1].Filename != "file2.igc" {
		t.Errorf("unexpected files: %v", files)
	}
}

func TestFetchIgcFiles_Empty(t *testing.T) {
	body := `{"success": true, "data": [], "next_page_url": null, "total": 0}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()

	files, err := FetchIgcFiles(srv.URL, 334, srv.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestFetchIgcFiles_ResolvesRelativeURL(t *testing.T) {
	body := `{
		"success": true,
		"data": [{
			"id": 1, "filename": "test.igc", "flight_date": "2026-04-01",
			"rego": "ZKJ", "flight_number": 1, "size_bytes": 100,
			"file_hash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"url": "/storage/igcfiles/331/test.igc"
		}],
		"next_page_url": null, "total": 1
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()

	files, err := FetchIgcFiles(srv.URL, 1, srv.Client())
	if err != nil {
		t.Fatalf("FetchIgcFiles error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	want := srv.URL + "/storage/igcfiles/331/test.igc"
	if files[0].URL != want {
		t.Errorf("URL not resolved: got %q, want %q", files[0].URL, want)
	}
}

func TestFetchIgcFiles_PreservesAbsoluteURL(t *testing.T) {
	absURL := "https://cdn.example.com/igc/test.igc"
	body := `{
		"success": true,
		"data": [{
			"id": 1, "filename": "test.igc", "flight_date": "2026-04-01",
			"rego": "ZKJ", "flight_number": 1, "size_bytes": 100,
			"file_hash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"url": "` + absURL + `"
		}],
		"next_page_url": null, "total": 1
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()

	files, err := FetchIgcFiles(srv.URL, 1, srv.Client())
	if err != nil {
		t.Fatalf("FetchIgcFiles error: %v", err)
	}
	if files[0].URL != absURL {
		t.Errorf("absolute URL should be unchanged: got %q, want %q", files[0].URL, absURL)
	}
}

func TestFetchIgcFiles_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer srv.Close()

	_, err := FetchIgcFiles(srv.URL, 1, srv.Client())
	if err == nil {
		t.Fatal("expected error for 429 response")
	}
}

func TestIgcFile_JSONFields(t *testing.T) {
	sample := `{
		"id": 10,
		"filename": "2026-04-01_ZKJ_001.igc",
		"flight_date": "2026-04-01",
		"rego": "ZKJ",
		"flight_number": 1,
		"size_bytes": 54321,
		"file_hash": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"url": "https://cdn.gliding.net.nz/igc/file.igc"
	}`
	var f IgcFile
	if err := json.Unmarshal([]byte(sample), &f); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if f.Rego != "ZKJ" {
		t.Errorf("expected rego ZKJ, got %s", f.Rego)
	}
	if f.SizeBytes != 54321 {
		t.Errorf("expected size_bytes 54321, got %d", f.SizeBytes)
	}
}
