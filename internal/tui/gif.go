package tui

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	imagedraw "image/draw"
	"image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/image/draw"

	"github.com/NathanBhanji/tail-chat/internal/tenor"
)

// ─── Constants ──────────────────────────────────────────────────────

const (
	gifPickerGridCols = 3  // columns in picker grid
	gifThumbH         = 6  // terminal rows per thumbnail in picker
	gifInlineMaxCols  = 40 // max terminal cols for inline image in chat
	gifInlineMaxRows  = 10 // max terminal rows for inline image in chat
	kittyChunkSize    = 4096
	maxCachedImages   = 200
)

// ─── Tea message types ──────────────────────────────────────────────

// GifSearchResultMsg is sent when Tenor search results arrive.
type GifSearchResultMsg struct {
	Results []tenor.GIF
	Err     error
}

// GifThumbLoadedMsg is sent when a picker thumbnail finishes downloading.
type GifThumbLoadedMsg struct {
	Index int
	Img   image.Image
	Err   error
}

// ImageCachedMsg is sent when an inline chat image finishes downloading.
type ImageCachedMsg struct {
	URL string
	Err error
}

// ─── GIF picker state ───────────────────────────────────────────────

type gifPicker struct {
	query   string
	results []tenor.GIF
	thumbs  map[int]image.Image
	cursor  int
	loading bool
	err     string
}

// selectedSendURL returns the URL to send as a chat message for the selected GIF.
func (g *gifPicker) selectedSendURL() string {
	if g.cursor < 0 || g.cursor >= len(g.results) {
		return ""
	}
	gif := g.results[g.cursor]
	if gif.Media.GIF.URL != "" {
		return gif.Media.GIF.URL
	}
	return gif.Media.TinyGIF.URL
}

// thumbURL returns the URL for downloading a picker thumbnail.
func (g *gifPicker) thumbURL(i int) string {
	if i < 0 || i >= len(g.results) {
		return ""
	}
	gif := g.results[i]
	if gif.Media.TinyGIF.URL != "" {
		return gif.Media.TinyGIF.URL
	}
	if gif.Media.NanoGIF.URL != "" {
		return gif.Media.NanoGIF.URL
	}
	return gif.Media.GIF.URL
}

// ─── Image cache ────────────────────────────────────────────────────

type cachedImg struct {
	frames []image.Image // animated GIF frames, or single-element for static
	delays []int         // delay per frame in 10ms units (from GIF spec)
	err    error
}

var (
	imgCache   = make(map[string]*cachedImg)
	imgCacheMu sync.RWMutex
	animTick   uint64 // global animation counter, incremented by AnimTickMsg
)

// AnimTickMsg advances the global animation frame counter.
type AnimTickMsg struct{}

func animTickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return AnimTickMsg{}
	})
}

// getCachedImage returns the current animation frame for the given URL.
func getCachedImage(url string) (image.Image, bool) {
	imgCacheMu.RLock()
	defer imgCacheMu.RUnlock()
	e, ok := imgCache[url]
	if !ok || e.err != nil || len(e.frames) == 0 {
		return nil, false
	}
	if len(e.frames) == 1 {
		return e.frames[0], true
	}
	// Pick frame based on global tick — approximate animation timing
	idx := int(animTick) % len(e.frames)
	return e.frames[idx], true
}

func isImageCached(url string) bool {
	imgCacheMu.RLock()
	defer imgCacheMu.RUnlock()
	_, ok := imgCache[url]
	return ok
}

func isAnimated(url string) bool {
	imgCacheMu.RLock()
	defer imgCacheMu.RUnlock()
	if e, ok := imgCache[url]; ok {
		return len(e.frames) > 1
	}
	return false
}

func setCachedImage(url string, img image.Image, err error) {
	imgCacheMu.Lock()
	defer imgCacheMu.Unlock()
	if len(imgCache) >= maxCachedImages {
		for k := range imgCache {
			delete(imgCache, k)
			break
		}
	}
	imgCache[url] = &cachedImg{frames: []image.Image{img}, err: err}
}

func setCachedFrames(url string, frames []image.Image, delays []int, err error) {
	imgCacheMu.Lock()
	defer imgCacheMu.Unlock()
	if len(imgCache) >= maxCachedImages {
		for k := range imgCache {
			delete(imgCache, k)
			break
		}
	}
	imgCache[url] = &cachedImg{frames: frames, delays: delays, err: err}
}

// ─── Download tracking ──────────────────────────────────────────────

var (
	downloading   = make(map[string]bool)
	downloadingMu sync.Mutex
)

func isDownloading(url string) bool {
	downloadingMu.Lock()
	defer downloadingMu.Unlock()
	return downloading[url]
}

func setDownloading(url string, state bool) {
	downloadingMu.Lock()
	defer downloadingMu.Unlock()
	if state {
		downloading[url] = true
	} else {
		delete(downloading, url)
	}
}

// ─── Image download + resize ────────────────────────────────────────

// downloadResult holds the result of downloading an image or GIF.
type downloadResult struct {
	frames []image.Image
	delays []int
	err    error
}

func downloadImage(url string) (image.Image, error) {
	r := downloadImageFrames(url)
	if r.err != nil {
		return nil, r.err
	}
	if len(r.frames) == 0 {
		return nil, fmt.Errorf("no frames decoded")
	}
	return r.frames[0], nil
}

// downloadImageFrames downloads an image/GIF and returns all frames.
// For animated GIFs, returns multiple frames with delay timings.
// For static images (PNG/JPEG), returns a single frame.
func downloadImageFrames(url string) downloadResult {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return downloadResult{err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return downloadResult{err: fmt.Errorf("HTTP %d", resp.StatusCode)}
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB limit
	if err != nil {
		return downloadResult{err: fmt.Errorf("read: %w", err)}
	}

	// Try decoding as animated GIF first
	if g, err := gif.DecodeAll(bytes.NewReader(data)); err == nil && len(g.Image) > 1 {
		frames := decodeGIFFrames(g)
		return downloadResult{frames: frames, delays: g.Delay}
	}

	// Fall back to single-frame decode for static images
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return downloadResult{err: fmt.Errorf("decode: %w", err)}
	}
	return downloadResult{frames: []image.Image{img}}
}

// decodeGIFFrames composites GIF frames according to disposal methods,
// producing standalone RGBA images for each frame.
func decodeGIFFrames(g *gif.GIF) []image.Image {
	canvas := image.NewRGBA(image.Rect(0, 0, g.Config.Width, g.Config.Height))
	frames := make([]image.Image, 0, len(g.Image))

	for _, frame := range g.Image {
		// Draw frame onto canvas
		imagedraw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, imagedraw.Over)

		// Snapshot the current canvas state
		snap := image.NewRGBA(canvas.Bounds())
		copy(snap.Pix, canvas.Pix)
		frames = append(frames, snap)
	}

	// Cap frames to avoid excessive memory (keep max 60 frames)
	if len(frames) > 60 {
		step := len(frames) / 60
		sampled := make([]image.Image, 0, 60)
		for i := 0; i < len(frames); i += step {
			sampled = append(sampled, frames[i])
		}
		frames = sampled
	}

	return frames
}

// resizeExact scales an image to exactly w x h pixels.
func resizeExact(img image.Image, w, h int) *image.RGBA {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	return dst
}

// ─── Kitty graphics protocol ────────────────────────────────────────

var kittyDetected *bool

func kittySupported() bool {
	if kittyDetected != nil {
		return *kittyDetected
	}
	supported := false
	term := os.Getenv("TERM")
	termProgram := os.Getenv("TERM_PROGRAM")
	if strings.Contains(term, "kitty") {
		supported = true
	}
	switch termProgram {
	case "WezTerm", "ghostty":
		supported = true
	}
	kittyDetected = &supported
	return supported
}

// kittyEncode produces Kitty graphics APC escape sequences to transmit and
// display img spanning cols terminal columns and rows terminal rows.
func kittyEncode(img image.Image, cols, rows int) string {
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		return ""
	}
	payload := base64.StdEncoding.EncodeToString(pngBuf.Bytes())
	if len(payload) == 0 {
		return ""
	}

	var sb strings.Builder
	for i := 0; i < len(payload); i += kittyChunkSize {
		end := i + kittyChunkSize
		if end > len(payload) {
			end = len(payload)
		}
		chunk := payload[i:end]
		more := 1
		if end >= len(payload) {
			more = 0
		}
		if i == 0 {
			sb.WriteString(fmt.Sprintf("\x1b_Ga=T,f=100,q=2,c=%d,r=%d,m=%d;%s\x1b\\",
				cols, rows, more, chunk))
		} else {
			sb.WriteString(fmt.Sprintf("\x1b_Gm=%d;%s\x1b\\", more, chunk))
		}
	}
	return sb.String()
}

// ─── Half-block (▀) rendering ───────────────────────────────────────

// halfBlockRender renders img as half-block characters with truecolor ANSI,
// exactly `cols` pixels wide with proportional height (minimum 2px tall).
// Returns the rendered multi-line string and the number of terminal rows.
func halfBlockRender(img image.Image, cols int) (string, int) {
	bounds := img.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	if srcW == 0 || srcH == 0 || cols == 0 {
		return "", 0
	}

	targetW := cols
	targetH := srcH * targetW / srcW
	if targetH%2 != 0 {
		targetH++
	}
	if targetH < 2 {
		targetH = 2
	}

	resized := resizeExact(img, targetW, targetH)
	termRows := targetH / 2

	var sb strings.Builder
	for y := 0; y < targetH; y += 2 {
		for x := 0; x < targetW; x++ {
			tr, tg, tb, _ := resized.At(x, y).RGBA()
			br, bg, bb, _ := resized.At(x, y+1).RGBA()
			sb.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm\u2580",
				tr>>8, tg>>8, tb>>8, br>>8, bg>>8, bb>>8))
		}
		sb.WriteString("\x1b[0m")
		if y+2 < targetH {
			sb.WriteString("\n")
		}
	}
	return sb.String(), termRows
}

// renderThumb renders a picker thumbnail at most cellW wide and maxRows tall.
// Returns lines (split on \n) padded to cellW visual width, and actual image rows.
func renderThumb(img image.Image, cellW, maxRows int) ([]string, int) {
	bounds := img.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	if srcW == 0 || srcH == 0 {
		return nil, 0
	}

	// Proportional height within limits
	targetW := cellW
	targetH := srcH * targetW / srcW
	if targetH%2 != 0 {
		targetH++
	}
	if targetH < 2 {
		targetH = 2
	}
	if targetH > maxRows*2 {
		targetH = maxRows * 2
		targetW = srcW * targetH / srcH
		if targetW > cellW {
			targetW = cellW
		}
		if targetW < 1 {
			targetW = 1
		}
	}

	resized := resizeExact(img, targetW, targetH)
	termRows := targetH / 2

	var lines []string
	for y := 0; y < targetH; y += 2 {
		var sb strings.Builder
		for x := 0; x < targetW; x++ {
			tr, tg, tb, _ := resized.At(x, y).RGBA()
			br, bg, bb, _ := resized.At(x, y+1).RGBA()
			sb.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm\u2580",
				tr>>8, tg>>8, tb>>8, br>>8, bg>>8, bb>>8))
		}
		sb.WriteString("\x1b[0m")
		// Pad to cellW visual width
		if targetW < cellW {
			sb.WriteString(strings.Repeat(" ", cellW-targetW))
		}
		lines = append(lines, sb.String())
	}

	// Pad to maxRows
	for len(lines) < maxRows {
		lines = append(lines, strings.Repeat(" ", cellW))
	}

	return lines, termRows
}

// ─── Tea commands ───────────────────────────────────────────────────

var tenorClient = tenor.New()

func gifSearchCmd(query string) tea.Cmd {
	return func() tea.Msg {
		results, err := tenorClient.Search(query, 9)
		return GifSearchResultMsg{Results: results, Err: err}
	}
}

func gifLoadThumbCmd(idx int, url string) tea.Cmd {
	return func() tea.Msg {
		img, err := downloadImage(url)
		return GifThumbLoadedMsg{Index: idx, Img: img, Err: err}
	}
}

func gifLoadThumbsCmd(gp *gifPicker) tea.Cmd {
	var cmds []tea.Cmd
	for i := range gp.results {
		url := gp.thumbURL(i)
		if url != "" {
			idx := i
			u := url
			cmds = append(cmds, gifLoadThumbCmd(idx, u))
		}
	}
	return tea.Batch(cmds...)
}

func cacheImageCmd(url string) tea.Cmd {
	return func() tea.Msg {
		if isImageCached(url) || isDownloading(url) {
			return ImageCachedMsg{URL: url}
		}
		setDownloading(url, true)
		defer setDownloading(url, false)
		r := downloadImageFrames(url)
		if r.err != nil {
			setCachedImage(url, nil, r.err)
			return ImageCachedMsg{URL: url, Err: r.err}
		}
		if len(r.frames) > 1 {
			setCachedFrames(url, r.frames, r.delays, nil)
		} else if len(r.frames) == 1 {
			setCachedImage(url, r.frames[0], nil)
		}
		return ImageCachedMsg{URL: url}
	}
}

// ─── Inline image rendering (chat messages) ─────────────────────────

// renderInlineImage renders an image for display in the chat view.
// Shows a loading placeholder if the image isn't cached yet.
// Returns the rendered string and the number of terminal lines it occupies.
func renderInlineImage(url string, maxCols, maxRows int) (string, int) {
	img, ok := getCachedImage(url)
	if !ok || img == nil {
		// Render a placeholder to prevent layout shift
		var sb strings.Builder
		sb.WriteString("        ")
		sb.WriteString(helpStyle.Render("loading image..."))
		sb.WriteString("\n")
		for i := 1; i < gifInlineMaxRows; i++ {
			sb.WriteString("\n")
		}
		return sb.String(), gifInlineMaxRows
	}

	cols := maxCols
	if cols > gifInlineMaxCols {
		cols = gifInlineMaxCols
	}

	bounds := img.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	if srcW == 0 || srcH == 0 {
		return "", 0
	}

	if kittySupported() {
		ar := float64(srcH) / float64(srcW)
		rows := int(float64(cols) * ar * 0.5) // cells are ~2:1 aspect
		if rows < 1 {
			rows = 1
		}
		if rows > maxRows {
			rows = maxRows
		}
		esc := kittyEncode(img, cols, rows)
		if esc == "" {
			return "", 0
		}
		var sb strings.Builder
		sb.WriteString("        ") // indent to align with message text
		sb.WriteString(esc)
		sb.WriteString("\n")
		for i := 1; i < rows; i++ {
			sb.WriteString("\n")
		}
		return sb.String(), rows
	}

	// Half-block fallback
	rendered, rows := halfBlockRender(img, cols)
	if rows > maxRows {
		resized := resizeExact(img, cols, maxRows*2)
		rendered, rows = halfBlockRender(resized, cols)
	}
	if rendered == "" {
		return "", 0
	}

	lines := strings.Split(rendered, "\n")
	var sb strings.Builder
	for i, line := range lines {
		sb.WriteString("        ") // indent
		sb.WriteString(line)
		if i < len(lines)-1 {
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n")
	return sb.String(), rows
}

// inlineImageLines returns the number of terminal lines an inline image would
// consume. Returns a fixed reservation height even before the image is cached,
// to prevent layout shifts when the download completes.
func inlineImageLines(url string, maxCols, maxRows int) int {
	img, ok := getCachedImage(url)
	if !ok || img == nil {
		// Reserve space before image loads to prevent layout shift.
		// Use a compact placeholder height.
		return gifInlineMaxRows
	}

	bounds := img.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	if srcW == 0 || srcH == 0 {
		return gifInlineMaxRows
	}

	cols := maxCols
	if cols > gifInlineMaxCols {
		cols = gifInlineMaxCols
	}

	if kittySupported() {
		ar := float64(srcH) / float64(srcW)
		rows := int(float64(cols) * ar * 0.5)
		if rows < 1 {
			rows = 1
		}
		if rows > maxRows {
			rows = maxRows
		}
		return rows
	}

	targetH := srcH * cols / srcW
	if targetH%2 != 0 {
		targetH++
	}
	if targetH < 2 {
		targetH = 2
	}
	rows := targetH / 2
	if rows > maxRows {
		rows = maxRows
	}
	return rows
}

// ─── GIF picker rendering ───────────────────────────────────────────

func (m Model) renderGifPicker(width int) string {
	h := m.height
	innerW := width - 1
	gp := m.gifPicker

	var b strings.Builder

	// Header
	header := fmt.Sprintf("GIF Search: %s", gp.query)
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(primary).Render(truncate(header, innerW)))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render(strings.Repeat("\u2500", innerW)))
	b.WriteString("\n")

	linesUsed := 2

	if gp.loading {
		b.WriteString("\n")
		b.WriteString(connectingStyle.Render("  Searching Tenor..."))
		b.WriteString("\n")
		linesUsed += 2
	} else if gp.err != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("  " + gp.err))
		b.WriteString("\n")
		linesUsed += 2
	} else if len(gp.results) == 0 && gp.query != "" {
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("  No results found."))
		b.WriteString("\n")
		linesUsed += 2
	} else if len(gp.results) > 0 {
		gridCols := gifPickerGridCols
		cellW := (innerW - 2) / gridCols
		if cellW < 8 {
			cellW = 8
		}

		for rowStart := 0; rowStart < len(gp.results); rowStart += gridCols {
			if linesUsed+gifThumbH+2 > h-4 {
				break
			}

			rowEnd := rowStart + gridCols
			if rowEnd > len(gp.results) {
				rowEnd = len(gp.results)
			}

			// Render thumbnails for this row
			var thumbLines [][]string // [thumbIdx][lineIdx]
			maxLines := 0
			for idx := rowStart; idx < rowEnd; idx++ {
				if thumb, ok := gp.thumbs[idx]; ok {
					lines, _ := renderThumb(thumb, cellW-2, gifThumbH)
					thumbLines = append(thumbLines, lines)
					if len(lines) > maxLines {
						maxLines = len(lines)
					}
				} else {
					// Not loaded yet — placeholder
					placeholder := make([]string, gifThumbH)
					for i := range placeholder {
						if i == gifThumbH/2 {
							txt := helpStyle.Render("loading...")
							pad := cellW - 2 - 10
							if pad < 0 {
								pad = 0
							}
							placeholder[i] = txt + strings.Repeat(" ", pad)
						} else {
							placeholder[i] = strings.Repeat(" ", cellW-2)
						}
					}
					thumbLines = append(thumbLines, placeholder)
					if gifThumbH > maxLines {
						maxLines = gifThumbH
					}
				}
			}

			// Render rows side-by-side
			for line := 0; line < maxLines && linesUsed < h-4; line++ {
				var lineStr strings.Builder
				lineStr.WriteString(" ")
				for ti, tl := range thumbLines {
					if line < len(tl) {
						lineStr.WriteString(tl[line])
					} else {
						lineStr.WriteString(strings.Repeat(" ", cellW-2))
					}
					if ti < len(thumbLines)-1 {
						lineStr.WriteString("  ") // gap between cells
					}
				}
				b.WriteString(lineStr.String())
				b.WriteString("\n")
				linesUsed++
			}

			// Title row with selection indicator
			var titleStr strings.Builder
			titleStr.WriteString(" ")
			for idx := rowStart; idx < rowEnd; idx++ {
				title := ""
				if idx < len(gp.results) {
					title = gp.results[idx].Title
					if title == "" {
						title = fmt.Sprintf("GIF %d", idx+1)
					}
				}
				maxTitleW := cellW - 3
				if maxTitleW < 3 {
					maxTitleW = 3
				}
				if len(title) > maxTitleW {
					title = title[:maxTitleW-1] + "\u2026"
				}

				if idx == gp.cursor {
					titleStr.WriteString(gifPickerSelected.Render(fmt.Sprintf(" %s ", title)))
				} else {
					titleStr.WriteString(helpStyle.Render(fmt.Sprintf(" %s ", title)))
				}
				// Pad to cell width
				pad := cellW - len(title) - 3
				if pad > 0 {
					titleStr.WriteString(strings.Repeat(" ", pad))
				}
			}
			b.WriteString(titleStr.String())
			b.WriteString("\n")
			linesUsed++

			// Spacer
			b.WriteString("\n")
			linesUsed++
		}
	}

	// Pad remaining
	for linesUsed < h-3 {
		b.WriteString("\n")
		linesUsed++
	}

	// Input for re-search
	m.input.Width = innerW - 3
	b.WriteString(promptStyle.Render("> ") + m.input.View())
	b.WriteString("\n")

	// Help
	b.WriteString(helpStyle.Render(" \u2190\u2192\u2191\u2193 navigate \u2022 enter send \u2022 type to search \u2022 esc cancel"))

	return chatPane.Width(innerW).Height(h).Render(b.String())
}

// ─── GIF picker key handling ────────────────────────────────────────

func (m Model) handleGifPickerKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	gp := &m.gifPicker

	switch key {
	case "esc":
		if m.activeChatKey != "" {
			m.view = ViewChat
			m.focusPane = PaneChat
			m.input.Placeholder = "Type a message... (:emoji: tab \u2022 /file /gif)"
			m.input.SetValue("")
			m.input.Focus()
		} else {
			m.view = ViewEmpty
			m.focusPane = PaneSidebar
			m.input.Blur()
		}
		return m, nil

	case "left", "h":
		if gp.cursor > 0 {
			gp.cursor--
		}
		return m, nil

	case "right", "l":
		if gp.cursor < len(gp.results)-1 {
			gp.cursor++
		}
		return m, nil

	case "up", "k":
		if gp.cursor >= gifPickerGridCols {
			gp.cursor -= gifPickerGridCols
		}
		return m, nil

	case "down", "j":
		if gp.cursor+gifPickerGridCols < len(gp.results) {
			gp.cursor += gifPickerGridCols
		}
		return m, nil

	case "enter":
		// If input has text, run a new search
		query := strings.TrimSpace(m.input.Value())
		if query != "" {
			gp.query = query
			gp.loading = true
			gp.results = nil
			gp.thumbs = make(map[int]image.Image)
			gp.cursor = 0
			gp.err = ""
			m.input.SetValue("")
			return m, gifSearchCmd(query)
		}

		// Otherwise, send the selected GIF
		url := gp.selectedSendURL()
		if url == "" {
			return m, nil
		}

		chatKey := m.activeChatKey
		chatMgr := m.chatMgr

		// Return to chat view
		m.view = ViewChat
		m.focusPane = PaneChat
		m.input.Placeholder = "Type a message... (:emoji: tab \u2022 /file /gif)"
		m.input.SetValue("")
		m.input.Focus()

		return m, func() tea.Msg {
			var err error
			if strings.HasPrefix(chatKey, "group:") {
				groupID := strings.TrimPrefix(chatKey, "group:")
				err = chatMgr.SendGroupMessage(groupID, url)
			} else {
				err = chatMgr.SendMessage(chatKey, url)
			}
			return MessageSentMsg{ChatKey: chatKey, Err: err}
		}
	}

	// Forward other keys to input for search query
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}
