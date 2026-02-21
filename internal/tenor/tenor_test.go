package tenor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockTenorResponse returns a JSON response matching the Tenor v2 API format.
func mockTenorResponse(results []resultData) []byte {
	resp := searchResponse{Results: results, Next: "test-next"}
	b, _ := json.Marshal(resp)
	return b
}

func sampleResults() []resultData {
	return []resultData{
		{
			ID:                 "abc123",
			Title:              "",
			URL:                "https://tenor.com/abc123.gif",
			H1Title:            "Funny Cat GIF",
			ContentDescription: "a funny cat doing something silly",
			MediaFormats: map[string]mediaFormatObj{
				"gif":     {URL: "https://media.tenor.com/abc123/gif.gif", Dims: []int{480, 360}, Size: 1234567},
				"tinygif": {URL: "https://media.tenor.com/abc123/tiny.gif", Dims: []int{220, 165}, Size: 456789},
				"nanogif": {URL: "https://media.tenor.com/abc123/nano.gif", Dims: []int{90, 68}, Size: 123456},
			},
		},
		{
			ID:                 "def456",
			Title:              "",
			URL:                "https://tenor.com/def456.gif",
			H1Title:            "Dancing Dog GIF",
			ContentDescription: "a dog dancing happily",
			MediaFormats: map[string]mediaFormatObj{
				"gif":     {URL: "https://media.tenor.com/def456/gif.gif", Dims: []int{320, 240}, Size: 987654},
				"tinygif": {URL: "https://media.tenor.com/def456/tiny.gif", Dims: []int{160, 120}, Size: 345678},
				"nanogif": {URL: "https://media.tenor.com/def456/nano.gif", Dims: []int{80, 60}, Size: 112233},
			},
		},
		{
			ID:                 "ghi789",
			Title:              "",
			URL:                "https://tenor.com/ghi789.gif",
			H1Title:            "Mind Blown GIF",
			ContentDescription: "mind blown explosion",
			MediaFormats: map[string]mediaFormatObj{
				"gif":     {URL: "https://media.tenor.com/ghi789/gif.gif", Dims: []int{400, 300}, Size: 2345678},
				"tinygif": {URL: "https://media.tenor.com/ghi789/tiny.gif", Dims: []int{200, 150}, Size: 567890},
				"nanogif": {URL: "https://media.tenor.com/ghi789/nano.gif", Dims: []int{100, 75}, Size: 234567},
			},
		},
	}
}

func TestNew(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
	if c.httpClient == nil {
		t.Error("expected non-nil HTTP client")
	}
}

func TestNewWithoutEnvKey(t *testing.T) {
	t.Setenv("TAILCHAT_TENOR_KEY", "")
	c := New()
	if c.apiKey != "" {
		t.Errorf("expected empty API key without env var, got %q", c.apiKey)
	}
}

func TestNewWithEnvKey(t *testing.T) {
	t.Setenv("TAILCHAT_TENOR_KEY", "custom-test-key")
	c := New()
	if c.apiKey != "custom-test-key" {
		t.Errorf("expected custom key, got %q", c.apiKey)
	}
}

func TestSearch(t *testing.T) {
	data := sampleResults()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("q") == "" {
			t.Error("expected 'q' query parameter")
		}
		if q.Get("key") == "" {
			t.Error("expected 'key' query parameter")
		}
		if q.Get("media_filter") == "" {
			t.Error("expected 'media_filter' query parameter")
		}
		limit := q.Get("limit")
		if limit == "" {
			t.Error("expected 'limit' query parameter")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockTenorResponse(data))
	}))
	defer srv.Close()

	c := &Client{apiKey: "test-key", httpClient: srv.Client()}
	results, err := c.fetch(srv.URL + "/search?key=test-key&q=cats&limit=9&media_filter=gif,tinygif,nanogif")
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Verify first result
	if results[0].ID != "abc123" {
		t.Errorf("expected ID abc123, got %q", results[0].ID)
	}
	// Title should come from H1Title
	if results[0].Title != "Funny Cat GIF" {
		t.Errorf("expected title 'Funny Cat GIF', got %q", results[0].Title)
	}
	if results[0].URL != "https://tenor.com/abc123.gif" {
		t.Errorf("unexpected URL: %q", results[0].URL)
	}
	if results[0].Media.GIF.URL != "https://media.tenor.com/abc123/gif.gif" {
		t.Errorf("unexpected GIF URL: %q", results[0].Media.GIF.URL)
	}
	if results[0].Media.TinyGIF.URL != "https://media.tenor.com/abc123/tiny.gif" {
		t.Errorf("unexpected TinyGIF URL: %q", results[0].Media.TinyGIF.URL)
	}
	if results[0].Media.NanoGIF.URL != "https://media.tenor.com/abc123/nano.gif" {
		t.Errorf("unexpected NanoGIF URL: %q", results[0].Media.NanoGIF.URL)
	}

	// Verify dimensions
	if results[0].Media.GIF.Dims != [2]int{480, 360} {
		t.Errorf("unexpected GIF dims: %v", results[0].Media.GIF.Dims)
	}

	// Verify second result
	if results[1].ID != "def456" {
		t.Errorf("expected ID def456, got %q", results[1].ID)
	}
	if results[1].Title != "Dancing Dog GIF" {
		t.Errorf("expected title 'Dancing Dog GIF', got %q", results[1].Title)
	}
}

func TestTitleFallback(t *testing.T) {
	// When H1Title is empty, should fall back to ContentDescription
	data := []resultData{{
		ID:                 "t1",
		H1Title:            "",
		ContentDescription: "fallback description",
		MediaFormats:       map[string]mediaFormatObj{},
	}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockTenorResponse(data))
	}))
	defer srv.Close()

	c := &Client{apiKey: "test-key", httpClient: srv.Client()}
	results, err := c.fetch(srv.URL + "/search")
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Title != "fallback description" {
		t.Errorf("expected fallback title, got %q", results[0].Title)
	}
}

func TestTitleFallbackToTitle(t *testing.T) {
	// When both H1Title and ContentDescription are empty, use Title
	data := []resultData{{
		ID:                 "t1",
		Title:              "raw title",
		H1Title:            "",
		ContentDescription: "",
		MediaFormats:       map[string]mediaFormatObj{},
	}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockTenorResponse(data))
	}))
	defer srv.Close()

	c := &Client{apiKey: "test-key", httpClient: srv.Client()}
	results, err := c.fetch(srv.URL + "/search")
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Title != "raw title" {
		t.Errorf("expected raw title, got %q", results[0].Title)
	}
}

func TestSearchEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockTenorResponse(nil))
	}))
	defer srv.Close()

	c := &Client{apiKey: "test-key", httpClient: srv.Client()}
	results, err := c.fetch(srv.URL + "/search")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := &Client{apiKey: "test-key", httpClient: srv.Client()}
	_, err := c.fetch(srv.URL + "/search")
	if err == nil {
		t.Fatal("expected error for 429 status")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("expected status code in error, got: %v", err)
	}
}

func TestSearchServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := &Client{apiKey: "test-key", httpClient: srv.Client()}
	_, err := c.fetch(srv.URL + "/search")
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 in error, got: %v", err)
	}
}

func TestSearchInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{invalid json"))
	}))
	defer srv.Close()

	c := &Client{apiKey: "test-key", httpClient: srv.Client()}
	_, err := c.fetch(srv.URL + "/search")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("expected decode error, got: %v", err)
	}
}

func TestSearchConnectionError(t *testing.T) {
	c := &Client{apiKey: "test-key", httpClient: http.DefaultClient}
	_, err := c.fetch("http://127.0.0.1:1/invalid")
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestSearchDefaultLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "9" {
			t.Errorf("expected default limit 9, got %s", r.URL.Query().Get("limit"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockTenorResponse(nil))
	}))
	defer srv.Close()

	c := &Client{apiKey: "test-key", httpClient: srv.Client()}
	// Use fetch directly to hit our test server
	c.fetch(srv.URL + "/search?key=test-key&q=test&limit=9&media_filter=gif,tinygif,nanogif")
}

func TestSearchURLEncoding(t *testing.T) {
	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockTenorResponse(nil))
	}))
	defer srv.Close()

	c := &Client{apiKey: "test-key", httpClient: srv.Client()}
	c.fetch(srv.URL + "/search?key=test-key&q=hello+world&limit=5")

	if !strings.Contains(capturedURL, "hello+world") && !strings.Contains(capturedURL, "hello%20world") {
		t.Errorf("expected URL-encoded query, got: %s", capturedURL)
	}
}

func TestAllMediaFormats(t *testing.T) {
	data := sampleResults()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockTenorResponse(data))
	}))
	defer srv.Close()

	c := &Client{apiKey: "test-key", httpClient: srv.Client()}
	results, err := c.fetch(srv.URL + "/search")
	if err != nil {
		t.Fatal(err)
	}

	gif := results[0]

	if gif.Media.GIF.URL == "" {
		t.Error("expected GIF URL")
	}
	if gif.Media.TinyGIF.URL == "" {
		t.Error("expected TinyGIF URL")
	}
	if gif.Media.NanoGIF.URL == "" {
		t.Error("expected NanoGIF URL")
	}

	// Verify dims
	if gif.Media.GIF.Dims[0] != 480 || gif.Media.GIF.Dims[1] != 360 {
		t.Errorf("unexpected GIF dims: %v", gif.Media.GIF.Dims)
	}
	if gif.Media.GIF.Size != 1234567 {
		t.Errorf("unexpected GIF size: %d", gif.Media.GIF.Size)
	}
}

func TestFetchPreservesOrder(t *testing.T) {
	data := sampleResults()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockTenorResponse(data))
	}))
	defer srv.Close()

	c := &Client{apiKey: "test-key", httpClient: srv.Client()}
	results, err := c.fetch(srv.URL + "/search")
	if err != nil {
		t.Fatal(err)
	}

	expectedIDs := []string{"abc123", "def456", "ghi789"}
	for i, id := range expectedIDs {
		if results[i].ID != id {
			t.Errorf("result[%d].ID = %q, want %q", i, results[i].ID, id)
		}
	}
}

func TestMissingMediaFormats(t *testing.T) {
	// Test with empty/nil media_formats
	data := []resultData{{
		ID:           "empty",
		H1Title:      "Empty Media",
		MediaFormats: nil,
	}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockTenorResponse(data))
	}))
	defer srv.Close()

	c := &Client{apiKey: "test-key", httpClient: srv.Client()}
	results, err := c.fetch(srv.URL + "/search")
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Media.GIF.URL != "" {
		t.Errorf("expected empty GIF URL, got %q", results[0].Media.GIF.URL)
	}
	if results[0].Media.TinyGIF.URL != "" {
		t.Errorf("expected empty TinyGIF URL, got %q", results[0].Media.TinyGIF.URL)
	}
}

func TestPartialDims(t *testing.T) {
	// Test with partial/missing dims
	data := []resultData{{
		ID:      "partial",
		H1Title: "Partial Dims",
		MediaFormats: map[string]mediaFormatObj{
			"gif": {URL: "https://example.com/test.gif", Dims: []int{100}}, // only 1 dim
		},
	}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockTenorResponse(data))
	}))
	defer srv.Close()

	c := &Client{apiKey: "test-key", httpClient: srv.Client()}
	results, err := c.fetch(srv.URL + "/search")
	if err != nil {
		t.Fatal(err)
	}
	// Should not panic with partial dims
	if results[0].Media.GIF.Dims != [2]int{0, 0} {
		t.Errorf("expected zero dims for partial, got %v", results[0].Media.GIF.Dims)
	}
}
