package downloader

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPGetUsesSourcePageAsReferer(t *testing.T) {
	const sourcePage = "https://missav.ai/en/fc2-ppv-4973768"
	var referer, origin string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		referer = r.Header.Get("Referer")
		origin = r.Header.Get("Origin")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	response, err := httpGet(WithReferer(context.Background(), sourcePage), server.URL+"/playlist.m3u8")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if referer != sourcePage {
		t.Fatalf("Referer = %q, want %q", referer, sourcePage)
	}
	if origin != "https://missav.ai" {
		t.Fatalf("Origin = %q, want %q", origin, "https://missav.ai")
	}
}
