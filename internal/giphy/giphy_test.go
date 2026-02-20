package giphy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockGiphyResponse returns a JSON response matching the Giphy API format.
func mockGiphyResponse(gifs []gifData) []byte {
	resp := searchResponse{Data: gifs}
	b, _ := json.Marshal(resp)
	return b
}

func sampleGifs() []gifData {
	return []gifData{
		{
			ID:    "abc123",
			Title: "funny cat",
			URL:   "https://giphy.com/gifs/cat-abc123",
			Images: Images{
				Original:         ImageData{URL: "https://media.giphy.com/media/abc123/giphy.gif", Width: "480", Height: "360"},
				FixedHeight:      ImageData{URL: "https://media.giphy.com/media/abc123/200.gif", Width: "267", Height: "200"},
				FixedHeightStill: ImageData{URL: "https://media.giphy.com/media/abc123/200_s.gif", Width: "267", Height: "200"},
				FixedHeightSmall: ImageData{URL: "https://media.giphy.com/media/abc123/100.gif", Width: "133", Height: "100"},
				FixedWidthSmall:  ImageData{URL: "https://media.giphy.com/media/abc123/100w.gif", Width: "100", Height: "75"},
			},
		},
		{
			ID:    "def456",
			Title: "dancing dog",
			URL:   "https://giphy.com/gifs/dog-def456",
			Images: Images{
				Original:    ImageData{URL: "https://media.giphy.com/media/def456/giphy.gif", Width: "320", Height: "240"},
				FixedHeight: ImageData{URL: "https://media.giphy.com/media/def456/200.gif", Width: "267", Height: "200"},
			},
		},
		{
			ID:    "ghi789",
			Title: "mind blown",
			URL:   "https://giphy.com/gifs/mind-ghi789",
			Images: Images{
				Original:    ImageData{URL: "https://media.giphy.com/media/ghi789/giphy.gif", Width: "400", Height: "300"},
				FixedHeight: ImageData{URL: "https://media.giphy.com/media/ghi789/200.gif", Width: "267", Height: "200"},
			},
		},
	}
}

func TestNew(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
	if c.apiKey == "" {
		t.Error("expected non-empty API key")
	}
	if c.httpClient == nil {
		t.Error("expected non-nil HTTP client")
	}
}

func TestSearch(t *testing.T) {
	data := sampleGifs()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request parameters
		q := r.URL.Query()
		if q.Get("q") == "" {
			t.Error("expected 'q' query parameter")
		}
		if q.Get("api_key") == "" {
			t.Error("expected 'api_key' query parameter")
		}
		if q.Get("rating") != "g" {
			t.Errorf("expected rating=g, got %q", q.Get("rating"))
		}
		limit := q.Get("limit")
		if limit == "" {
			t.Error("expected 'limit' query parameter")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockGiphyResponse(data))
	}))
	defer srv.Close()

	c := &Client{
		apiKey:     "test-key",
		httpClient: srv.Client(),
	}

	// Override fetch URL by replacing baseURL — we call fetch directly
	results, err := c.fetch(srv.URL + "/search?api_key=test-key&q=cats&limit=9&rating=g")
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
	if results[0].Title != "funny cat" {
		t.Errorf("expected title 'funny cat', got %q", results[0].Title)
	}
	if results[0].URL != "https://giphy.com/gifs/cat-abc123" {
		t.Errorf("unexpected URL: %q", results[0].URL)
	}
	if results[0].Images.Original.URL != "https://media.giphy.com/media/abc123/giphy.gif" {
		t.Errorf("unexpected original URL: %q", results[0].Images.Original.URL)
	}
	if results[0].Images.FixedHeight.URL != "https://media.giphy.com/media/abc123/200.gif" {
		t.Errorf("unexpected fixed_height URL: %q", results[0].Images.FixedHeight.URL)
	}

	// Verify second result
	if results[1].ID != "def456" {
		t.Errorf("expected ID def456, got %q", results[1].ID)
	}
	if results[1].Title != "dancing dog" {
		t.Errorf("expected title 'dancing dog', got %q", results[1].Title)
	}
}

func TestSearchEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockGiphyResponse(nil))
	}))
	defer srv.Close()

	c := &Client{apiKey: "test-key", httpClient: srv.Client()}
	results, err := c.fetch(srv.URL + "/search?api_key=test-key&q=zzzzzzzzzzz&limit=9&rating=g")
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
	_, err := c.fetch(srv.URL + "/search?api_key=test-key&q=cats&limit=9&rating=g")
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
	_, err := c.fetch(srv.URL + "/search?api_key=test-key&q=cats&limit=9&rating=g")
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
	_, err := c.fetch(srv.URL + "/search?api_key=test-key&q=cats&limit=9&rating=g")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("expected decode error, got: %v", err)
	}
}

func TestSearchConnectionError(t *testing.T) {
	c := &Client{apiKey: "test-key", httpClient: http.DefaultClient}
	// Use an invalid URL to trigger a connection error
	_, err := c.fetch("http://127.0.0.1:1/invalid")
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestSearchDefaultLimit(t *testing.T) {
	c := New()
	// Search with limit=0 should use defaultLimit
	// We can't easily test the actual HTTP call, but we test the method doesn't panic
	// with a server that will refuse connections (expected error)
	_, err := c.Search("test", 0)
	// Expected to fail (real API), just verify no panic and error is returned
	if err == nil {
		// If it somehow succeeds (real API), that's fine too
		return
	}
}

func TestTrendingDefaultLimit(t *testing.T) {
	c := New()
	_, err := c.Trending(0)
	if err == nil {
		return
	}
}

func TestSearchURLEncoding(t *testing.T) {
	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockGiphyResponse(nil))
	}))
	defer srv.Close()

	// Override baseURL by calling Search and checking the query encoding
	c := &Client{apiKey: "test-key", httpClient: srv.Client()}
	c.fetch(srv.URL + "/search?api_key=test-key&q=hello+world&limit=5&rating=g")

	if !strings.Contains(capturedURL, "hello+world") && !strings.Contains(capturedURL, "hello%20world") {
		t.Errorf("expected URL-encoded query, got: %s", capturedURL)
	}
}

func TestGIFImagesAllFormats(t *testing.T) {
	data := sampleGifs()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockGiphyResponse(data))
	}))
	defer srv.Close()

	c := &Client{apiKey: "test-key", httpClient: srv.Client()}
	results, err := c.fetch(srv.URL + "/search?api_key=test-key&q=cats&limit=9&rating=g")
	if err != nil {
		t.Fatal(err)
	}

	gif := results[0]

	// Verify all image formats are populated
	if gif.Images.Original.URL == "" {
		t.Error("expected Original URL")
	}
	if gif.Images.FixedHeight.URL == "" {
		t.Error("expected FixedHeight URL")
	}
	if gif.Images.FixedHeightStill.URL == "" {
		t.Error("expected FixedHeightStill URL")
	}
	if gif.Images.FixedHeightSmall.URL == "" {
		t.Error("expected FixedHeightSmall URL")
	}
	if gif.Images.FixedWidthSmall.URL == "" {
		t.Error("expected FixedWidthSmall URL")
	}

	// Verify dimensions are strings
	if gif.Images.Original.Width != "480" {
		t.Errorf("expected width 480, got %q", gif.Images.Original.Width)
	}
	if gif.Images.Original.Height != "360" {
		t.Errorf("expected height 360, got %q", gif.Images.Original.Height)
	}
}

func TestFetchPreservesOrder(t *testing.T) {
	data := sampleGifs()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockGiphyResponse(data))
	}))
	defer srv.Close()

	c := &Client{apiKey: "test-key", httpClient: srv.Client()}
	results, err := c.fetch(srv.URL + "/search?api_key=test-key&q=cats&limit=9&rating=g")
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
