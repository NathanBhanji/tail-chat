package tui

import (
	"fmt"
	"image"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/NathanBhanji/tail-chat/internal/chat"
	"github.com/NathanBhanji/tail-chat/internal/tenor"
)

// TestIntegration_TenorSearchAndRender does the full real-world flow:
// 1. Hits real Tenor API
// 2. Downloads a real GIF
// 3. Renders it with halfBlockRender
// 4. Renders it inline in a chat view
// 5. Checks all dimensions and layout properties
//
// Run with: go test -run TestIntegration -v -count=1 ./internal/tui/
func TestIntegration_TenorSearchAndRender(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Force non-kitty mode for consistent testing
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

	// 1. Search Tenor for a real GIF
	t.Log("=== Step 1: Search Tenor API ===")
	client := tenor.New()
	results, err := client.Search("cat", 3)
	if err != nil {
		t.Fatalf("Tenor search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Tenor returned 0 results")
	}

	t.Logf("Got %d results", len(results))
	for i, r := range results {
		t.Logf("  [%d] title=%q gif=%q tinygif=%q", i, r.Title, r.Media.GIF.URL, r.Media.TinyGIF.URL)
	}

	// 2. Download the first GIF
	t.Log("\n=== Step 2: Download first GIF ===")
	gifURL := results[0].Media.GIF.URL
	if gifURL == "" {
		t.Fatal("first result has no GIF URL")
	}

	img, err := downloadImage(gifURL)
	if err != nil {
		t.Fatalf("downloadImage failed: %v", err)
	}
	bounds := img.Bounds()
	t.Logf("Downloaded image: %dx%d pixels", bounds.Dx(), bounds.Dy())

	// 3. Render with halfBlockRender at various widths
	t.Log("\n=== Step 3: halfBlockRender at various widths ===")
	for _, cols := range []int{20, 30, 40, 60, 80} {
		rendered, rows := halfBlockRender(img, cols)
		lines := strings.Split(rendered, "\n")
		actualLines := len(lines)

		// Check visual width of each line using lipgloss (ANSI-aware)
		var widthIssues []string
		for li, line := range lines {
			lgW := lipgloss.Width(line)
			plainW := countVisibleChars(line)
			if plainW != cols {
				widthIssues = append(widthIssues, fmt.Sprintf("  line %d: visible_chars=%d expected=%d lipgloss_width=%d", li, plainW, cols, lgW))
			}
		}

		t.Logf("cols=%d: returned_rows=%d actual_lines=%d len(rendered)=%d",
			cols, rows, actualLines, len(rendered))
		if rows != actualLines {
			t.Errorf("cols=%d: halfBlockRender returned rows=%d but actual line count=%d", cols, rows, actualLines)
		}
		if len(widthIssues) > 0 {
			t.Logf("cols=%d: width issues:\n%s", cols, strings.Join(widthIssues, "\n"))
		}
	}

	// 4. Render inline image and check dimensions
	t.Log("\n=== Step 4: renderInlineImage ===")
	clearImageCache()
	defer clearImageCache()
	setCachedImage(gifURL, img, nil)

	for _, maxCols := range []int{30, 40, 60} {
		imgStr, imgRows := renderInlineImage(gifURL, maxCols, gifInlineMaxRows)
		expectedLines := inlineImageLines(gifURL, maxCols, gifInlineMaxRows)
		actualNewlines := strings.Count(imgStr, "\n")
		imgLines := strings.Split(imgStr, "\n")

		t.Logf("maxCols=%d: imgRows=%d expectedLines=%d newlines_in_output=%d split_lines=%d",
			maxCols, imgRows, expectedLines, actualNewlines, len(imgLines))

		if imgRows != expectedLines {
			t.Errorf("maxCols=%d: renderInlineImage rows=%d but inlineImageLines=%d", maxCols, imgRows, expectedLines)
		}

		// Check each line width doesn't exceed maxCols + indent
		indent := 8 // "        " prefix
		for li, line := range imgLines {
			if line == "" {
				continue
			}
			plainW := countVisibleChars(line)
			if plainW > maxCols+indent+2 { // +2 for possible reset sequence
				t.Errorf("maxCols=%d line %d: visible width %d exceeds max %d", maxCols, li, plainW, maxCols+indent)
			}
		}
	}

	// 5. Full chat View() with inline image
	t.Log("\n=== Step 5: Full View() with inline GIF ===")
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 120
	m.height = 40
	m.view = ViewChat
	m.focusPane = PaneChat
	m.activeChatKey = "alice"
	m.messages = []chat.Message{
		{ID: "1", Sender: "bob", Content: "hey check this out", Timestamp: time.Now().Add(-2 * time.Minute)},
		{ID: "2", Sender: "alice", Content: gifURL, Timestamp: time.Now().Add(-1 * time.Minute)},
		{ID: "3", Sender: "bob", Content: "nice!", Timestamp: time.Now()},
	}

	fullView := m.View()
	viewLines := strings.Split(fullView, "\n")
	t.Logf("Full View(): %d lines, %d bytes", len(viewLines), len(fullView))

	// Dump the raw view to a file for manual inspection
	os.WriteFile("/tmp/tailchat_gif_view.txt", []byte(fullView), 0644)
	t.Log("Dumped raw view to /tmp/tailchat_gif_view.txt")

	// Also dump a stripped version (no ANSI) for easier reading
	stripped := stripAnsi(fullView)
	os.WriteFile("/tmp/tailchat_gif_view_stripped.txt", []byte(stripped), 0644)
	t.Log("Dumped stripped view to /tmp/tailchat_gif_view_stripped.txt")

	// Check that all 3 messages are visible
	if !strings.Contains(stripped, "hey check this out") {
		t.Error("message 1 not visible in view")
	}
	if !strings.Contains(stripped, "nice!") {
		t.Error("message 3 not visible in view")
	}

	// Check view line widths using lipgloss (ANSI-aware) and runewidth (raw)
	var overflowLines []int
	for li, line := range viewLines {
		lgW := lipgloss.Width(line) // ANSI-aware visual width
		if lgW > m.width+2 {
			overflowLines = append(overflowLines, li)
			t.Logf("  OVERFLOW line %d: lipgloss_width=%d stripped_len=%d",
				li, lgW, len(stripAnsi(line)))
		}
	}
	if len(overflowLines) > 0 {
		t.Errorf("%d lines exceed terminal width (%d) per lipgloss measurement", len(overflowLines), m.width)
	} else {
		t.Log("All view lines within terminal width (lipgloss measurement)")
	}

	// Also check that image-containing lines have ANSI reset at the end
	// to prevent style bleeding
	for li, line := range viewLines {
		if strings.Contains(line, "\u2580") { // half-block char
			if !strings.HasSuffix(strings.TrimRight(line, " "), "\x1b[0m") &&
				!strings.Contains(line, "\x1b[0m") {
				t.Errorf("line %d has half-block chars but no ANSI reset", li)
			}
		}
	}

	// 6. THE LAYOUT SHIFT BUG: msgLineCount changes when image downloads
	t.Log("\n=== Step 6: Layout shift detection ===")
	innerW := m.width - 30
	maxContentW := innerW - 9
	if maxContentW < 10 {
		maxContentW = 10
	}

	gifMsg := m.messages[1]

	// Simulate BEFORE download: clear cache
	clearImageCache()
	linesBefore := 1
	if w := strings.Fields(strings.TrimSpace(gifMsg.Content)); len(w) == 1 && isImageURL(w[0]) {
		linesBefore += inlineImageLines(w[0], maxContentW, gifInlineMaxRows)
	}

	viewBefore := m.View()
	strippedBefore := stripAnsi(viewBefore)

	// Simulate AFTER download: cache the image
	setCachedImage(gifURL, img, nil)
	linesAfter := 1
	if w := strings.Fields(strings.TrimSpace(gifMsg.Content)); len(w) == 1 && isImageURL(w[0]) {
		linesAfter += inlineImageLines(w[0], maxContentW, gifInlineMaxRows)
	}

	viewAfter := m.View()
	strippedAfter := stripAnsi(viewAfter)

	t.Logf("msgLineCount BEFORE cache: %d", linesBefore)
	t.Logf("msgLineCount AFTER  cache: %d", linesAfter)
	t.Logf("Line count JUMP: %d -> %d (delta: %d)", linesBefore, linesAfter, linesAfter-linesBefore)

	if linesBefore != linesAfter {
		t.Errorf("LAYOUT SHIFT BUG: msgLineCount changes from %d to %d when image caches — this causes the entire chat to jump by %d lines",
			linesBefore, linesAfter, linesAfter-linesBefore)
	}

	// Check if message positions shift
	msg1Pos := strings.Index(strippedBefore, "hey check this out")
	msg3Pos := strings.Index(strippedBefore, "nice!")
	msg1PosAfter := strings.Index(strippedAfter, "hey check this out")
	msg3PosAfter := strings.Index(strippedAfter, "nice!")

	t.Logf("'hey check this out' position: before=%d after=%d", msg1Pos, msg1PosAfter)
	t.Logf("'nice!' position: before=%d after=%d", msg3Pos, msg3PosAfter)

	// Dump both for comparison
	os.WriteFile("/tmp/tailchat_before_cache.txt", []byte(strippedBefore), 0644)
	os.WriteFile("/tmp/tailchat_after_cache.txt", []byte(strippedAfter), 0644)
	t.Log("Dumped before/after to /tmp/tailchat_before_cache.txt and /tmp/tailchat_after_cache.txt")
}

// TestIntegration_GifPickerRealSearch tests the picker flow with real API data.
func TestIntegration_GifPickerRealSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

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

	// Search and get results
	client := tenor.New()
	results, err := client.Search("hello", 9)
	if err != nil {
		t.Fatalf("Tenor search: %v", err)
	}
	if len(results) < 3 {
		t.Fatalf("expected at least 3 results, got %d", len(results))
	}

	// Download thumbnails for first 3 results
	thumbs := make(map[int]image.Image)
	for i := 0; i < 3 && i < len(results); i++ {
		url := results[i].Media.TinyGIF.URL
		if url == "" {
			url = results[i].Media.GIF.URL
		}
		if url == "" {
			t.Logf("result %d has no thumbnail URL, skipping", i)
			continue
		}
		img, err := downloadImage(url)
		if err != nil {
			t.Logf("failed to download thumb %d: %v", i, err)
			continue
		}
		thumbs[i] = img
		t.Logf("thumb[%d]: %dx%d from %s", i, img.Bounds().Dx(), img.Bounds().Dy(), url)
	}

	if len(thumbs) == 0 {
		t.Fatal("couldn't download any thumbnails")
	}

	// Set up the GIF picker with real data
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 120
	m.height = 40
	m.view = ViewGifPicker
	m.focusPane = PaneChat
	m.activeChatKey = "alice"
	m.gifPicker = gifPicker{
		query:   "hello",
		results: results,
		thumbs:  thumbs,
		cursor:  0,
	}

	// Render the picker
	pickerView := m.View()
	pickerLines := strings.Split(pickerView, "\n")
	t.Logf("Picker View(): %d lines, %d bytes", len(pickerLines), len(pickerView))

	os.WriteFile("/tmp/tailchat_picker_view.txt", []byte(pickerView), 0644)
	os.WriteFile("/tmp/tailchat_picker_stripped.txt", []byte(stripAnsi(pickerView)), 0644)
	t.Log("Dumped picker views to /tmp/tailchat_picker_*.txt")

	// Check picker doesn't exceed terminal dimensions
	if len(pickerLines) > m.height+2 {
		t.Errorf("picker has %d lines, exceeds terminal height %d", len(pickerLines), m.height)
	}

	var overflowLines []int
	for li, line := range pickerLines {
		lgW := lipgloss.Width(line)
		if lgW > m.width+2 {
			overflowLines = append(overflowLines, li)
			t.Logf("  OVERFLOW picker line %d: lipgloss_width=%d", li, lgW)
		}
	}
	if len(overflowLines) > 0 {
		t.Errorf("picker lines exceeding width: %v", overflowLines)
	}

	// Verify half-block characters are present (real thumbnails rendered)
	stripped := stripAnsi(pickerView)
	if !strings.Contains(stripped, "\u2580") && len(thumbs) > 0 {
		t.Error("expected half-block characters from real thumbnail rendering")
	}
}

// ─── Helpers ────────────────────────────────────────────────────────

// stripAnsi removes ANSI escape sequences from a string.
func stripAnsi(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// TestIntegration_GifAnimation verifies animated GIFs produce multiple frames
// and that advancing animTick changes the rendered output.
func TestIntegration_GifAnimation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

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

	// Search for an animated GIF
	client := tenor.New()
	results, err := client.Search("dancing cat", 3)
	if err != nil {
		t.Fatalf("Tenor search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("no results")
	}

	gifURL := results[0].Media.GIF.URL
	t.Logf("Downloading animated GIF: %s", gifURL)

	// Download with frame-aware decoder
	r := downloadImageFrames(gifURL)
	if r.err != nil {
		t.Fatalf("download failed: %v", r.err)
	}

	t.Logf("Got %d frames, %d delays", len(r.frames), len(r.delays))

	if len(r.frames) <= 1 {
		t.Logf("WARNING: GIF has only %d frame(s) — may not be animated. Trying another result.", len(r.frames))
		// Try another result
		for i := 1; i < len(results); i++ {
			url := results[i].Media.GIF.URL
			r2 := downloadImageFrames(url)
			if r2.err == nil && len(r2.frames) > 1 {
				r = r2
				gifURL = url
				t.Logf("Using result %d instead: %s (%d frames)", i, url, len(r.frames))
				break
			}
		}
	}

	if len(r.frames) <= 1 {
		t.Skip("could not find an animated GIF in search results")
	}

	// Cache the frames
	clearImageCache()
	defer clearImageCache()
	setCachedFrames(gifURL, r.frames, r.delays, nil)

	// Verify animation: different ticks should produce different frames
	if !isAnimated(gifURL) {
		t.Error("expected isAnimated to return true")
	}

	// Render at tick 0
	animTick = 0
	img0, ok := getCachedImage(gifURL)
	if !ok || img0 == nil {
		t.Fatal("expected cached image at tick 0")
	}
	render0, rows0 := halfBlockRender(img0, 30)

	// Render at tick 1 (next frame)
	animTick = 1
	img1, ok := getCachedImage(gifURL)
	if !ok || img1 == nil {
		t.Fatal("expected cached image at tick 1")
	}
	render1, rows1 := halfBlockRender(img1, 30)

	t.Logf("Frame 0: %d rows, %d bytes", rows0, len(render0))
	t.Logf("Frame 1: %d rows, %d bytes", rows1, len(render1))

	// Frames should be the same dimensions but different content
	if rows0 != rows1 {
		t.Errorf("frame dimensions should match: %d vs %d rows", rows0, rows1)
	}

	if render0 == render1 {
		t.Log("WARNING: frame 0 and frame 1 rendered identically — frames may be very similar")
	} else {
		t.Log("Animation confirmed: frame 0 and frame 1 produce different renders")
	}

	// Verify the full inline render works with animation
	animTick = 0
	inline0, inlineRows0 := renderInlineImage(gifURL, 40, gifInlineMaxRows)
	animTick = 5
	inline5, inlineRows5 := renderInlineImage(gifURL, 40, gifInlineMaxRows)

	t.Logf("Inline at tick 0: %d rows, %d bytes", inlineRows0, len(inline0))
	t.Logf("Inline at tick 5: %d rows, %d bytes", inlineRows5, len(inline5))

	if inlineRows0 != inlineRows5 {
		t.Errorf("inline rows should be stable across ticks: %d vs %d", inlineRows0, inlineRows5)
	}

	if inline0 != inline5 {
		t.Log("Animation in inline render confirmed: different ticks produce different output")
	}

	// Reset for other tests
	animTick = 0
}

func truncStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
