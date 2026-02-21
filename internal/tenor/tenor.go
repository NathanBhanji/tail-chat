package tenor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

const (
	baseURL      = "https://tenor.googleapis.com/v2"
	defaultLimit = 9
	mediaFilter  = "gif,tinygif,nanogif"
)

// Client is a Tenor API v2 client.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// GIF represents a single Tenor GIF result.
type GIF struct {
	ID    string
	Title string
	URL   string // Tenor page URL
	Media MediaFormats
}

// MediaFormats contains the available media formats for a GIF.
type MediaFormats struct {
	GIF     MediaObject
	TinyGIF MediaObject
	NanoGIF MediaObject
}

// MediaObject contains the URL, dimensions and size for a specific format.
type MediaObject struct {
	URL  string
	Dims [2]int
	Size int
}

type searchResponse struct {
	Results []resultData `json:"results"`
	Next    string       `json:"next"`
}

type resultData struct {
	ID                 string                    `json:"id"`
	Title              string                    `json:"title"`
	URL                string                    `json:"url"`
	ItemURL            string                    `json:"itemurl"`
	H1Title            string                    `json:"h1_title"`
	ContentDescription string                    `json:"content_description"`
	MediaFormats       map[string]mediaFormatObj `json:"media_formats"`
}

type mediaFormatObj struct {
	URL  string `json:"url"`
	Dims []int  `json:"dims"`
	Size int    `json:"size"`
}

// New creates a new Tenor API client. Set the TAILCHAT_TENOR_KEY environment
// variable to enable GIF search. If unset, the client is created with an empty
// key and API calls will return errors.
func New() *Client {
	return &Client{
		apiKey:     os.Getenv("TAILCHAT_TENOR_KEY"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Search queries the Tenor search API.
func (c *Client) Search(query string, limit int) ([]GIF, error) {
	if limit <= 0 {
		limit = defaultLimit
	}
	u := fmt.Sprintf("%s/search?key=%s&q=%s&limit=%d&media_filter=%s&contentfilter=medium",
		baseURL, c.apiKey, url.QueryEscape(query), limit, mediaFilter)
	return c.fetch(u)
}

// Trending returns trending (featured) GIFs.
func (c *Client) Trending(limit int) ([]GIF, error) {
	if limit <= 0 {
		limit = defaultLimit
	}
	u := fmt.Sprintf("%s/featured?key=%s&limit=%d&media_filter=%s&contentfilter=medium",
		baseURL, c.apiKey, limit, mediaFilter)
	return c.fetch(u)
}

func (c *Client) fetch(u string) ([]GIF, error) {
	resp, err := c.httpClient.Get(u)
	if err != nil {
		return nil, fmt.Errorf("tenor: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tenor: status %d", resp.StatusCode)
	}

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("tenor decode: %w", err)
	}

	gifs := make([]GIF, len(sr.Results))
	for i, r := range sr.Results {
		title := r.H1Title
		if title == "" {
			title = r.ContentDescription
		}
		if title == "" {
			title = r.Title
		}
		gifs[i] = GIF{
			ID:    r.ID,
			Title: title,
			URL:   r.URL,
			Media: parseMediaFormats(r.MediaFormats),
		}
	}
	return gifs, nil
}

func parseMediaFormats(m map[string]mediaFormatObj) MediaFormats {
	return MediaFormats{
		GIF:     parseMediaObj(m["gif"]),
		TinyGIF: parseMediaObj(m["tinygif"]),
		NanoGIF: parseMediaObj(m["nanogif"]),
	}
}

func parseMediaObj(o mediaFormatObj) MediaObject {
	mo := MediaObject{
		URL:  o.URL,
		Size: o.Size,
	}
	if len(o.Dims) >= 2 {
		mo.Dims = [2]int{o.Dims[0], o.Dims[1]}
	}
	return mo
}
