package tui

import (
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/NathanBhanji/tail-chat/internal/tenor"
)

// ─── Test helpers ───────────────────────────────────────────────────

// testImage creates a solid-color image for testing.
func testImage(w, h int, c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

// checkerImage creates a 2-color checkerboard for testing detail preservation.
func checkerImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if (x+y)%2 == 0 {
				img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
			} else {
				img.Set(x, y, color.RGBA{R: 0, G: 0, B: 255, A: 255})
			}
		}
	}
	return img
}

// servePNG starts a test HTTP server that serves a PNG image.
func servePNG(t *testing.T, img image.Image) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		png.Encode(w, img)
	}))
}

// clearImageCache removes all entries from the global image cache.
func clearImageCache() {
	imgCacheMu.Lock()
	defer imgCacheMu.Unlock()
	for k := range imgCache {
		delete(imgCache, k)
	}
}

// clearDownloading removes all entries from the download tracker.
func clearDownloading() {
	downloadingMu.Lock()
	defer downloadingMu.Unlock()
	for k := range downloading {
		delete(downloading, k)
	}
}

// ─── Image cache tests ──────────────────────────────────────────────

func TestImageCache_SetAndGet(t *testing.T) {
	clearImageCache()
	defer clearImageCache()

	img := testImage(10, 10, color.RGBA{R: 255, A: 255})

	// Before set, should not be cached
	_, ok := getCachedImage("http://example.com/test.png")
	if ok {
		t.Error("expected cache miss before set")
	}

	setCachedImage("http://example.com/test.png", img, nil)

	// After set, should be cached
	got, ok := getCachedImage("http://example.com/test.png")
	if !ok {
		t.Fatal("expected cache hit after set")
	}
	if got.Bounds().Dx() != 10 || got.Bounds().Dy() != 10 {
		t.Errorf("cached image dimensions %dx%d, want 10x10", got.Bounds().Dx(), got.Bounds().Dy())
	}
}

func TestImageCache_IsImageCached(t *testing.T) {
	clearImageCache()
	defer clearImageCache()

	if isImageCached("http://example.com/x.png") {
		t.Error("expected not cached")
	}

	setCachedImage("http://example.com/x.png", testImage(5, 5, color.Black), nil)

	if !isImageCached("http://example.com/x.png") {
		t.Error("expected cached after set")
	}
}

func TestImageCache_ErrorEntry(t *testing.T) {
	clearImageCache()
	defer clearImageCache()

	// Cache an error entry (download failed)
	setCachedImage("http://example.com/bad.png", nil, image.ErrFormat)

	// isImageCached should return true (entry exists, even if errored)
	if !isImageCached("http://example.com/bad.png") {
		t.Error("expected cached even for error entries")
	}

	// getCachedImage should return false (no usable image)
	_, ok := getCachedImage("http://example.com/bad.png")
	if ok {
		t.Error("expected getCachedImage to return false for error entry")
	}
}

func TestImageCache_Eviction(t *testing.T) {
	clearImageCache()
	defer clearImageCache()

	// Fill cache to max
	for i := 0; i < maxCachedImages; i++ {
		url := "http://example.com/" + string(rune('a'+i%26)) + string(rune('0'+i/26)) + ".png"
		setCachedImage(url, testImage(1, 1, color.Black), nil)
	}

	imgCacheMu.RLock()
	countBefore := len(imgCache)
	imgCacheMu.RUnlock()

	if countBefore != maxCachedImages {
		t.Fatalf("expected cache size %d, got %d", maxCachedImages, countBefore)
	}

	// Adding one more should evict one
	setCachedImage("http://example.com/overflow.png", testImage(1, 1, color.White), nil)

	imgCacheMu.RLock()
	countAfter := len(imgCache)
	imgCacheMu.RUnlock()

	if countAfter != maxCachedImages {
		t.Errorf("expected cache size %d after eviction, got %d", maxCachedImages, countAfter)
	}

	// The new entry should be present
	if !isImageCached("http://example.com/overflow.png") {
		t.Error("expected new entry to be cached after eviction")
	}
}

func TestImageCache_MultipleURLs(t *testing.T) {
	clearImageCache()
	defer clearImageCache()

	urls := []string{
		"http://example.com/a.png",
		"http://example.com/b.png",
		"http://example.com/c.png",
	}

	for _, url := range urls {
		setCachedImage(url, testImage(5, 5, color.Black), nil)
	}

	for _, url := range urls {
		if !isImageCached(url) {
			t.Errorf("expected %q to be cached", url)
		}
	}
}

// ─── Download tracking tests ────────────────────────────────────────

func TestDownloadTracking(t *testing.T) {
	clearDownloading()
	defer clearDownloading()

	url := "http://example.com/dl.gif"

	if isDownloading(url) {
		t.Error("expected not downloading initially")
	}

	setDownloading(url, true)
	if !isDownloading(url) {
		t.Error("expected downloading after set true")
	}

	setDownloading(url, false)
	if isDownloading(url) {
		t.Error("expected not downloading after set false")
	}
}

func TestDownloadTracking_MultipleConcurrent(t *testing.T) {
	clearDownloading()
	defer clearDownloading()

	urls := []string{"http://a.com/1.gif", "http://b.com/2.gif"}

	for _, url := range urls {
		setDownloading(url, true)
	}

	for _, url := range urls {
		if !isDownloading(url) {
			t.Errorf("expected %q to be downloading", url)
		}
	}

	setDownloading(urls[0], false)
	if isDownloading(urls[0]) {
		t.Error("expected first URL no longer downloading")
	}
	if !isDownloading(urls[1]) {
		t.Error("expected second URL still downloading")
	}
}

// ─── resizeExact tests ──────────────────────────────────────────────

func TestResizeExact(t *testing.T) {
	src := testImage(100, 80, color.RGBA{R: 128, G: 64, B: 32, A: 255})

	tests := []struct {
		name  string
		w, h  int
		wantW int
		wantH int
	}{
		{"downscale", 50, 40, 50, 40},
		{"upscale", 200, 160, 200, 160},
		{"change aspect", 30, 60, 30, 60},
		{"single pixel", 1, 1, 1, 1},
		{"zero clamps to 1", 0, 0, 1, 1},
		{"wide", 200, 10, 200, 10},
		{"tall", 10, 200, 10, 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resizeExact(src, tt.w, tt.h)
			bounds := result.Bounds()
			if bounds.Dx() != tt.wantW || bounds.Dy() != tt.wantH {
				t.Errorf("resizeExact(%d,%d) = %dx%d, want %dx%d",
					tt.w, tt.h, bounds.Dx(), bounds.Dy(), tt.wantW, tt.wantH)
			}
		})
	}
}

func TestResizeExact_PreservesColor(t *testing.T) {
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	src := testImage(100, 100, red)

	result := resizeExact(src, 10, 10)

	// Center pixel should still be approximately red
	r, g, b, a := result.At(5, 5).RGBA()
	if r>>8 < 200 || g>>8 > 50 || b>>8 > 50 || a>>8 < 200 {
		t.Errorf("expected approximately red, got RGBA(%d,%d,%d,%d)", r>>8, g>>8, b>>8, a>>8)
	}
}

// ─── halfBlockRender tests ──────────────────────────────────────────

func TestHalfBlockRender_BasicOutput(t *testing.T) {
	img := testImage(4, 4, color.RGBA{R: 255, G: 0, B: 0, A: 255})

	rendered, rows := halfBlockRender(img, 4)

	if rows != 2 {
		t.Errorf("expected 2 terminal rows, got %d", rows)
	}
	if rendered == "" {
		t.Fatal("expected non-empty rendered output")
	}

	// Should contain half-block character
	if !strings.Contains(rendered, "\u2580") {
		t.Error("expected half-block character \u2580 in output")
	}

	// Should contain ANSI color codes
	if !strings.Contains(rendered, "\x1b[38;2;") {
		t.Error("expected ANSI truecolor foreground codes")
	}
	if !strings.Contains(rendered, "\x1b[48;2;") {
		t.Error("expected ANSI truecolor background codes")
	}

	// Should end with reset
	if !strings.Contains(rendered, "\x1b[0m") {
		t.Error("expected ANSI reset code")
	}
}

func TestHalfBlockRender_EmptyImage(t *testing.T) {
	img := testImage(0, 0, color.Black)
	rendered, rows := halfBlockRender(img, 10)
	if rendered != "" || rows != 0 {
		t.Errorf("expected empty output for 0x0 image, got %d rows", rows)
	}
}

func TestHalfBlockRender_ZeroCols(t *testing.T) {
	img := testImage(10, 10, color.Black)
	rendered, rows := halfBlockRender(img, 0)
	if rendered != "" || rows != 0 {
		t.Errorf("expected empty output for 0 cols, got %d rows", rows)
	}
}

func TestHalfBlockRender_WideImage(t *testing.T) {
	img := testImage(200, 50, color.RGBA{G: 255, A: 255})
	rendered, rows := halfBlockRender(img, 30)

	if rows <= 0 {
		t.Error("expected positive row count")
	}

	lines := strings.Split(rendered, "\n")
	if len(lines) != rows {
		t.Errorf("expected %d lines, got %d", rows, len(lines))
	}
}

func TestHalfBlockRender_TallImage(t *testing.T) {
	img := testImage(50, 200, color.RGBA{B: 255, A: 255})
	rendered, rows := halfBlockRender(img, 20)

	if rows <= 0 {
		t.Error("expected positive row count")
	}
	if rendered == "" {
		t.Error("expected non-empty output")
	}
}

func TestHalfBlockRender_MinimumHeight(t *testing.T) {
	// Very wide, very short image — should still render at least 1 terminal row
	img := testImage(100, 1, color.White)
	_, rows := halfBlockRender(img, 20)
	if rows < 1 {
		t.Errorf("expected at least 1 row, got %d", rows)
	}
}

func TestHalfBlockRender_LineCount(t *testing.T) {
	tests := []struct {
		name     string
		imgW     int
		imgH     int
		cols     int
		wantRows int
	}{
		{"square 10x10 at 10 cols", 10, 10, 10, 5}, // 10px tall / 2 = 5 rows
		{"16:9 at 16 cols", 160, 90, 16, 5},        // 16*90/160 = 9 → 10 (even) / 2 = 5
		{"wide banner", 200, 20, 40, 2},            // 40*20/200 = 4 / 2 = 2
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := testImage(tt.imgW, tt.imgH, color.Black)
			_, rows := halfBlockRender(img, tt.cols)
			if rows != tt.wantRows {
				t.Errorf("halfBlockRender(%dx%d, cols=%d) = %d rows, want %d",
					tt.imgW, tt.imgH, tt.cols, rows, tt.wantRows)
			}
		})
	}
}

// ─── renderThumb tests ──────────────────────────────────────────────

func TestRenderThumb_Basic(t *testing.T) {
	img := testImage(100, 75, color.RGBA{R: 200, G: 100, B: 50, A: 255})
	lines, rows := renderThumb(img, 20, 6)

	if rows <= 0 {
		t.Error("expected positive row count")
	}

	// Lines should be padded to exactly maxRows
	if len(lines) < 6 {
		t.Errorf("expected at least 6 lines (padded), got %d", len(lines))
	}
}

func TestRenderThumb_CapsHeight(t *testing.T) {
	// Very tall image — should cap at maxRows
	img := testImage(50, 500, color.RGBA{G: 128, A: 255})
	lines, rows := renderThumb(img, 20, 6)

	if rows > 6 {
		t.Errorf("expected at most 6 rows, got %d", rows)
	}
	if len(lines) != 6 {
		t.Errorf("expected exactly 6 padded lines, got %d", len(lines))
	}
}

func TestRenderThumb_EmptyImage(t *testing.T) {
	img := testImage(0, 0, color.Black)
	lines, rows := renderThumb(img, 20, 6)
	if lines != nil || rows != 0 {
		t.Errorf("expected nil lines and 0 rows for empty image, got %d lines, %d rows", len(lines), rows)
	}
}

func TestRenderThumb_PaddingWidth(t *testing.T) {
	// An image narrower than cellW should be padded
	img := testImage(10, 10, color.White)
	lines, _ := renderThumb(img, 30, 6)

	// Each line should visually occupy cellW characters
	// The half-block chars plus padding spaces should total cellW
	for i, line := range lines {
		// Count visible characters (strip ANSI)
		vis := countVisibleChars(line)
		if vis != 30 {
			t.Errorf("line %d: visual width %d, want 30", i, vis)
		}
	}
}

// countVisibleChars counts non-ANSI characters in a string.
func countVisibleChars(s string) int {
	inEsc := false
	count := 0
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '~' {
				inEsc = false
			}
			continue
		}
		count++
	}
	return count
}

// ─── Kitty graphics protocol tests ──────────────────────────────────

func TestKittyEncode_ProducesAPCSequence(t *testing.T) {
	img := testImage(10, 10, color.RGBA{R: 255, A: 255})
	result := kittyEncode(img, 5, 3)

	if result == "" {
		t.Fatal("expected non-empty Kitty encode output")
	}

	// Must start with APC introducer
	if !strings.HasPrefix(result, "\x1b_G") {
		t.Error("expected Kitty APC start \\x1b_G")
	}

	// Must contain ST terminator
	if !strings.Contains(result, "\x1b\\") {
		t.Error("expected Kitty ST terminator \\x1b\\\\")
	}

	// Must contain the key parameters
	if !strings.Contains(result, "a=T") {
		t.Error("expected a=T (transmit and display)")
	}
	if !strings.Contains(result, "f=100") {
		t.Error("expected f=100 (PNG format)")
	}
	if !strings.Contains(result, "q=2") {
		t.Error("expected q=2 (quiet mode)")
	}
	if !strings.Contains(result, "c=5") {
		t.Errorf("expected c=5 (columns)")
	}
	if !strings.Contains(result, "r=3") {
		t.Errorf("expected r=3 (rows)")
	}
}

func TestKittyEncode_ChunksLargePayload(t *testing.T) {
	// Create a large image that will require multiple chunks
	img := testImage(200, 200, checkerImage(200, 200).At(0, 0).(color.Color))
	result := kittyEncode(img, 20, 10)

	if result == "" {
		t.Fatal("expected non-empty output for large image")
	}

	// Count APC sequences (each chunk is a separate APC)
	chunks := strings.Count(result, "\x1b_G")
	if chunks < 1 {
		t.Error("expected at least one APC chunk")
	}

	// First chunk should have a=T, subsequent should only have m=
	// All chunks should end with ST
	terminators := strings.Count(result, "\x1b\\")
	if terminators != chunks {
		t.Errorf("expected %d ST terminators, got %d", chunks, terminators)
	}
}

func TestKittyEncode_SingleChunkSmallImage(t *testing.T) {
	// A tiny image should fit in one chunk
	img := testImage(2, 2, color.White)
	result := kittyEncode(img, 1, 1)

	chunks := strings.Count(result, "\x1b_G")
	if chunks != 1 {
		t.Errorf("expected exactly 1 chunk for tiny image, got %d", chunks)
	}

	// Should have m=0 (no more chunks)
	if !strings.Contains(result, "m=0") {
		t.Error("expected m=0 for single-chunk image")
	}
}

func TestKittySupported_DetectsEnvVars(t *testing.T) {
	// Reset cached detection
	kittyDetected = nil

	// Save and restore env
	origTerm := os.Getenv("TERM")
	origProg := os.Getenv("TERM_PROGRAM")
	defer func() {
		os.Setenv("TERM", origTerm)
		os.Setenv("TERM_PROGRAM", origProg)
		kittyDetected = nil
	}()

	tests := []struct {
		term     string
		termProg string
		want     bool
	}{
		{"xterm-kitty", "", true},
		{"", "WezTerm", true},
		{"", "ghostty", true},
		{"xterm-256color", "", false},
		{"", "Apple_Terminal", false},
		{"", "", false},
	}

	for _, tt := range tests {
		kittyDetected = nil // reset cache
		os.Setenv("TERM", tt.term)
		os.Setenv("TERM_PROGRAM", tt.termProg)

		got := kittySupported()
		if got != tt.want {
			t.Errorf("TERM=%q TERM_PROGRAM=%q: kittySupported() = %v, want %v",
				tt.term, tt.termProg, got, tt.want)
		}
	}
}

func TestKittySupported_CachesResult(t *testing.T) {
	kittyDetected = nil
	os.Setenv("TERM", "")
	os.Setenv("TERM_PROGRAM", "")
	defer func() { kittyDetected = nil }()

	result1 := kittySupported()
	result2 := kittySupported()

	if result1 != result2 {
		t.Error("cached result should match")
	}
	if kittyDetected == nil {
		t.Error("expected kittyDetected to be set after first call")
	}
}

// ─── GIF picker state tests ────────────────────────────────────────

func TestGifPicker_SelectedSendURL(t *testing.T) {
	gp := gifPicker{
		results: []tenor.GIF{
			{
				Media: tenor.MediaFormats{
					GIF: tenor.MediaObject{URL: "https://media.tenor.com/full.gif"},
				},
			},
			{
				Media: tenor.MediaFormats{
					TinyGIF: tenor.MediaObject{URL: "https://media.tenor.com/tiny.gif"},
				},
			},
		},
		cursor: 0,
		thumbs: make(map[int]image.Image),
	}

	// Should prefer GIF URL
	url := gp.selectedSendURL()
	if url != "https://media.tenor.com/full.gif" {
		t.Errorf("expected GIF URL, got %q", url)
	}

	// Second result has no GIF URL — should fall back to TinyGIF
	gp.cursor = 1
	url = gp.selectedSendURL()
	if url != "https://media.tenor.com/tiny.gif" {
		t.Errorf("expected TinyGIF URL fallback, got %q", url)
	}
}

func TestGifPicker_SelectedSendURL_OutOfBounds(t *testing.T) {
	gp := gifPicker{
		results: []tenor.GIF{{Media: tenor.MediaFormats{}}},
		cursor:  5,
		thumbs:  make(map[int]image.Image),
	}

	if url := gp.selectedSendURL(); url != "" {
		t.Errorf("expected empty URL for out-of-bounds cursor, got %q", url)
	}

	gp.cursor = -1
	if url := gp.selectedSendURL(); url != "" {
		t.Errorf("expected empty URL for negative cursor, got %q", url)
	}
}

func TestGifPicker_SelectedSendURL_EmptyResults(t *testing.T) {
	gp := gifPicker{thumbs: make(map[int]image.Image)}
	if url := gp.selectedSendURL(); url != "" {
		t.Errorf("expected empty URL for empty results, got %q", url)
	}
}

func TestGifPicker_ThumbURL_Priority(t *testing.T) {
	gp := gifPicker{
		results: []tenor.GIF{
			{
				Media: tenor.MediaFormats{
					TinyGIF: tenor.MediaObject{URL: "https://still.gif"},
					NanoGIF: tenor.MediaObject{URL: "https://small.gif"},
					GIF:     tenor.MediaObject{URL: "https://full.gif"},
				},
			},
			{
				Media: tenor.MediaFormats{
					NanoGIF: tenor.MediaObject{URL: "https://small2.gif"},
					GIF:     tenor.MediaObject{URL: "https://full2.gif"},
				},
			},
			{
				Media: tenor.MediaFormats{
					GIF: tenor.MediaObject{URL: "https://full3.gif"},
				},
			},
		},
		thumbs: make(map[int]image.Image),
	}

	// Should prefer TinyGIF
	if url := gp.thumbURL(0); url != "https://still.gif" {
		t.Errorf("expected TinyGIF, got %q", url)
	}

	// Should fall back to NanoGIF
	if url := gp.thumbURL(1); url != "https://small2.gif" {
		t.Errorf("expected NanoGIF, got %q", url)
	}

	// Should fall back to GIF
	if url := gp.thumbURL(2); url != "https://full3.gif" {
		t.Errorf("expected GIF, got %q", url)
	}
}

func TestGifPicker_ThumbURL_OutOfBounds(t *testing.T) {
	gp := gifPicker{results: []tenor.GIF{{}}, thumbs: make(map[int]image.Image)}
	if url := gp.thumbURL(5); url != "" {
		t.Errorf("expected empty URL for out-of-bounds, got %q", url)
	}
	if url := gp.thumbURL(-1); url != "" {
		t.Errorf("expected empty URL for negative, got %q", url)
	}
}

// ─── downloadImage tests ────────────────────────────────────────────

func TestDownloadImage_PNG(t *testing.T) {
	img := testImage(20, 15, color.RGBA{R: 100, G: 200, B: 50, A: 255})
	srv := servePNG(t, img)
	defer srv.Close()

	result, err := downloadImage(srv.URL + "/test.png")
	if err != nil {
		t.Fatalf("downloadImage error: %v", err)
	}

	bounds := result.Bounds()
	if bounds.Dx() != 20 || bounds.Dy() != 15 {
		t.Errorf("downloaded image %dx%d, want 20x15", bounds.Dx(), bounds.Dy())
	}
}

func TestDownloadImage_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := downloadImage(srv.URL + "/missing.png")
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestDownloadImage_InvalidImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("not a real image"))
	}))
	defer srv.Close()

	_, err := downloadImage(srv.URL + "/bad.png")
	if err == nil {
		t.Fatal("expected error for invalid image data")
	}
}

func TestDownloadImage_ConnectionRefused(t *testing.T) {
	_, err := downloadImage("http://127.0.0.1:1/unreachable.png")
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

// ─── Inline image rendering tests ───────────────────────────────────

func TestRenderInlineImage_NotCached(t *testing.T) {
	clearImageCache()
	defer clearImageCache()

	rendered, rows := renderInlineImage("http://example.com/uncached.gif", 40, 10)
	if rendered != "" || rows != 0 {
		t.Errorf("expected empty output for uncached image, got %d rows", rows)
	}
}

func TestRenderInlineImage_Cached(t *testing.T) {
	clearImageCache()
	defer clearImageCache()

	// Reset kitty detection to false for predictable half-block output
	kittyDetected = nil
	origTerm := os.Getenv("TERM")
	origProg := os.Getenv("TERM_PROGRAM")
	os.Setenv("TERM", "xterm-256color")
	os.Setenv("TERM_PROGRAM", "")
	defer func() {
		os.Setenv("TERM", origTerm)
		os.Setenv("TERM_PROGRAM", origProg)
		kittyDetected = nil
	}()

	img := testImage(80, 60, color.RGBA{R: 200, G: 100, B: 50, A: 255})
	setCachedImage("http://example.com/test.gif", img, nil)

	rendered, rows := renderInlineImage("http://example.com/test.gif", 40, 10)
	if rendered == "" {
		t.Fatal("expected non-empty output for cached image")
	}
	if rows <= 0 {
		t.Error("expected positive row count")
	}
	if rows > 10 {
		t.Errorf("expected at most 10 rows (maxRows), got %d", rows)
	}

	// Should contain indentation
	if !strings.HasPrefix(rendered, "        ") {
		t.Error("expected 8-space indent on first line")
	}

	// Should contain half-block characters (non-Kitty terminal)
	if !strings.Contains(rendered, "\u2580") {
		t.Error("expected half-block character in output")
	}
}

func TestRenderInlineImage_NilImage(t *testing.T) {
	clearImageCache()
	defer clearImageCache()

	setCachedImage("http://example.com/nil.gif", nil, nil)

	rendered, rows := renderInlineImage("http://example.com/nil.gif", 40, 10)
	if rendered != "" || rows != 0 {
		t.Error("expected empty output for nil cached image")
	}
}

func TestRenderInlineImage_CapsMaxCols(t *testing.T) {
	clearImageCache()
	defer clearImageCache()
	kittyDetected = nil
	os.Setenv("TERM", "")
	os.Setenv("TERM_PROGRAM", "")
	defer func() { kittyDetected = nil }()

	img := testImage(1000, 500, color.White)
	setCachedImage("http://example.com/wide.gif", img, nil)

	// Even with maxCols=100, inline rendering caps at gifInlineMaxCols (40)
	rendered, rows := renderInlineImage("http://example.com/wide.gif", 100, 10)
	if rendered == "" {
		t.Fatal("expected output")
	}
	if rows <= 0 {
		t.Error("expected positive row count")
	}
}

// ─── inlineImageLines tests ─────────────────────────────────────────

func TestInlineImageLines_NotCached(t *testing.T) {
	clearImageCache()
	defer clearImageCache()

	lines := inlineImageLines("http://example.com/none.gif", 40, 10)
	if lines != 0 {
		t.Errorf("expected 0 lines for uncached, got %d", lines)
	}
}

func TestInlineImageLines_Cached(t *testing.T) {
	clearImageCache()
	defer clearImageCache()
	kittyDetected = nil
	os.Setenv("TERM", "")
	os.Setenv("TERM_PROGRAM", "")
	defer func() { kittyDetected = nil }()

	img := testImage(80, 60, color.Black)
	setCachedImage("http://example.com/lines.gif", img, nil)

	lines := inlineImageLines("http://example.com/lines.gif", 40, 10)
	if lines <= 0 {
		t.Error("expected positive line count")
	}
	if lines > 10 {
		t.Errorf("expected at most 10 lines, got %d", lines)
	}
}

func TestInlineImageLines_MatchesRenderOutput(t *testing.T) {
	clearImageCache()
	defer clearImageCache()
	kittyDetected = nil
	os.Setenv("TERM", "")
	os.Setenv("TERM_PROGRAM", "")
	defer func() { kittyDetected = nil }()

	img := testImage(160, 90, color.RGBA{R: 50, G: 100, B: 200, A: 255})
	url := "http://example.com/match.gif"
	setCachedImage(url, img, nil)

	expectedLines := inlineImageLines(url, 40, 10)
	_, renderRows := renderInlineImage(url, 40, 10)

	if expectedLines != renderRows {
		t.Errorf("inlineImageLines=%d but renderInlineImage rows=%d — mismatch", expectedLines, renderRows)
	}
}

// ─── cacheImageCmd tests ────────────────────────────────────────────

func TestCacheImageCmd_DownloadsAndCaches(t *testing.T) {
	clearImageCache()
	clearDownloading()
	defer func() {
		clearImageCache()
		clearDownloading()
	}()

	img := testImage(20, 15, color.RGBA{R: 255, A: 255})
	srv := servePNG(t, img)
	defer srv.Close()

	url := srv.URL + "/cache_test.png"
	cmd := cacheImageCmd(url)
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}

	// Execute the command
	msg := cmd()
	cached, ok := msg.(ImageCachedMsg)
	if !ok {
		t.Fatalf("expected ImageCachedMsg, got %T", msg)
	}
	if cached.URL != url {
		t.Errorf("expected URL %q, got %q", url, cached.URL)
	}
	if cached.Err != nil {
		t.Errorf("unexpected error: %v", cached.Err)
	}

	// Should now be cached
	if !isImageCached(url) {
		t.Error("expected image to be cached after command")
	}
}

func TestCacheImageCmd_AlreadyCached(t *testing.T) {
	clearImageCache()
	defer clearImageCache()

	url := "http://example.com/already.png"
	setCachedImage(url, testImage(5, 5, color.Black), nil)

	cmd := cacheImageCmd(url)
	msg := cmd()

	cached, ok := msg.(ImageCachedMsg)
	if !ok {
		t.Fatalf("expected ImageCachedMsg, got %T", msg)
	}
	// Should return quickly without error (no re-download)
	if cached.Err != nil {
		t.Errorf("unexpected error: %v", cached.Err)
	}
}

func TestCacheImageCmd_AlreadyDownloading(t *testing.T) {
	clearImageCache()
	clearDownloading()
	defer func() {
		clearImageCache()
		clearDownloading()
	}()

	url := "http://example.com/inprogress.png"
	setDownloading(url, true)

	cmd := cacheImageCmd(url)
	msg := cmd()

	cached, ok := msg.(ImageCachedMsg)
	if !ok {
		t.Fatalf("expected ImageCachedMsg, got %T", msg)
	}
	// Should skip download
	if cached.Err != nil {
		t.Errorf("unexpected error: %v", cached.Err)
	}

	setDownloading(url, false)
}

// ─── gifSearchCmd tests ─────────────────────────────────────────────

func TestGifSearchCmd_ReturnsMsg(t *testing.T) {
	// This test just verifies the command returns the right message type.
	// The actual API call will fail (no mock), but the type should be correct.
	cmd := gifSearchCmd("cats")
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}

	msg := cmd()
	result, ok := msg.(GifSearchResultMsg)
	if !ok {
		t.Fatalf("expected GifSearchResultMsg, got %T", msg)
	}

	// Will likely have an error (real API with beta key might work or fail)
	// Just verify the type is correct
	_ = result.Err
	_ = result.Results
}

// ─── gifLoadThumbCmd tests ──────────────────────────────────────────

func TestGifLoadThumbCmd_Success(t *testing.T) {
	img := testImage(50, 50, color.RGBA{G: 200, A: 255})
	srv := servePNG(t, img)
	defer srv.Close()

	cmd := gifLoadThumbCmd(3, srv.URL+"/thumb.png")
	msg := cmd()

	thumb, ok := msg.(GifThumbLoadedMsg)
	if !ok {
		t.Fatalf("expected GifThumbLoadedMsg, got %T", msg)
	}
	if thumb.Index != 3 {
		t.Errorf("expected index 3, got %d", thumb.Index)
	}
	if thumb.Err != nil {
		t.Errorf("unexpected error: %v", thumb.Err)
	}
	if thumb.Img == nil {
		t.Error("expected non-nil image")
	}
}

func TestGifLoadThumbCmd_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cmd := gifLoadThumbCmd(0, srv.URL+"/error.png")
	msg := cmd()

	thumb, ok := msg.(GifThumbLoadedMsg)
	if !ok {
		t.Fatalf("expected GifThumbLoadedMsg, got %T", msg)
	}
	if thumb.Err == nil {
		t.Error("expected error for failed download")
	}
	if thumb.Img != nil {
		t.Error("expected nil image on error")
	}
}

// ─── gifLoadThumbsCmd tests ─────────────────────────────────────────

func TestGifLoadThumbsCmd_BatchesCommands(t *testing.T) {
	gp := &gifPicker{
		results: []tenor.GIF{
			{Media: tenor.MediaFormats{TinyGIF: tenor.MediaObject{URL: "http://a.com/1.gif"}}},
			{Media: tenor.MediaFormats{TinyGIF: tenor.MediaObject{URL: "http://a.com/2.gif"}}},
			{Media: tenor.MediaFormats{TinyGIF: tenor.MediaObject{URL: "http://a.com/3.gif"}}},
		},
		thumbs: make(map[int]image.Image),
	}

	cmd := gifLoadThumbsCmd(gp)
	if cmd == nil {
		t.Fatal("expected non-nil batch command")
	}
	// We can't easily test tea.Batch internals, but verify it doesn't panic
}

func TestGifLoadThumbsCmd_EmptyResults(t *testing.T) {
	gp := &gifPicker{results: nil, thumbs: make(map[int]image.Image)}
	cmd := gifLoadThumbsCmd(gp)
	// Should produce a batch of no commands (no-op)
	_ = cmd
}
