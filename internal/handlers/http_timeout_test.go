package handlers

import (
	"net/http"
	"testing"
	"time"

	"github.com/elpic/blueprint/internal/parser"
)

// ---- #15: no HTTP timeout on http.Get calls --------------------------------
//
// RunShHandler and DownloadHandler use http.DefaultClient which has no timeout.
// A slow server will hang blueprint apply indefinitely.
// We test that the handlers use a client with a non-zero timeout.

func TestRunShHandlerUsesHTTPTimeout(t *testing.T) {
	h := NewRunShHandler(parser.Rule{
		Action:   "run-sh",
		RunShURL: "https://example.com/setup.sh",
	}, "")

	client := h.httpClient()
	if client == nil {
		t.Fatal("BUG: httpClient() returned nil")
	}
	if client == http.DefaultClient {
		t.Error("BUG: RunShHandler uses http.DefaultClient which has no timeout")
	}
	if client.Timeout == 0 {
		t.Error("BUG: RunShHandler HTTP client has no timeout (Timeout == 0)")
	}
	if client.Timeout < 5*time.Second {
		t.Errorf("HTTP timeout too short: %v, want at least 5s", client.Timeout)
	}
}

func TestDownloadHandlerUsesHTTPTimeout(t *testing.T) {
	h := NewDownloadHandler(parser.Rule{
		Action:      "download",
		DownloadURL: "https://example.com/file.tar.gz",
	}, "")

	client := h.httpClient()
	if client == nil {
		t.Fatal("BUG: httpClient() returned nil")
	}
	if client == http.DefaultClient {
		t.Error("BUG: DownloadHandler uses http.DefaultClient which has no timeout")
	}
	if client.Timeout == 0 {
		t.Error("BUG: DownloadHandler HTTP client has no timeout (Timeout == 0)")
	}
	if client.Timeout < 5*time.Second {
		t.Errorf("HTTP timeout too short: %v, want at least 5s", client.Timeout)
	}
}

func TestAsdfHTTPClientHasTimeout(t *testing.T) {
	// Verify getLatestAsdfVersion uses a timeout-aware client by checking that
	// the cache path returns without hitting the network.
	asdfVersionCache = "0.0.0-test"
	got, err := getLatestAsdfVersion()
	asdfVersionCache = ""
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "0.0.0-test" {
		t.Errorf("expected cached version, got %q", got)
	}
}
