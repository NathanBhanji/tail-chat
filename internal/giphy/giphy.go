package giphy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	betaAPIKey   = "dc6zaTOxFJmzC"
	baseURL      = "https://api.giphy.com/v1/gifs"
	defaultLimit = 9
)

// Client is a Giphy API client.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// GIF represents a single Giphy GIF result.
type GIF struct {
	ID     string
	Title  string
	URL    string // Giphy page URL
	Images Images
}

// Images contains the various image formats/sizes available.
type Images struct {
	Original         ImageData `json:"original"`
	FixedHeight      ImageData `json:"fixed_height"`
	FixedHeightStill ImageData `json:"fixed_height_still"`
	FixedHeightSmall ImageData `json:"fixed_height_small"`
	FixedWidthSmall  ImageData `json:"fixed_width_small"`
	DownsizedStill   ImageData `json:"downsized_still"`
}

// ImageData contains the URL and dimensions for a specific image format.
type ImageData struct {
	URL    string `json:"url"`
	Width  string `json:"width"`
	Height string `json:"height"`
}

type searchResponse struct {
	Data []gifData `json:"data"`
}

type gifData struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	Images Images `json:"images"`
}

// New creates a new Giphy API client using the public beta key.
func New() *Client {
	return &Client{
		apiKey:     betaAPIKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Search queries the Giphy search API.
func (c *Client) Search(query string, limit int) ([]GIF, error) {
	if limit <= 0 {
		limit = defaultLimit
	}
	u := fmt.Sprintf("%s/search?api_key=%s&q=%s&limit=%d&rating=g",
		baseURL, c.apiKey, url.QueryEscape(query), limit)
	return c.fetch(u)
}

// Trending returns trending GIFs.
func (c *Client) Trending(limit int) ([]GIF, error) {
	if limit <= 0 {
		limit = defaultLimit
	}
	u := fmt.Sprintf("%s/trending?api_key=%s&limit=%d&rating=g",
		baseURL, c.apiKey, limit)
	return c.fetch(u)
}

func (c *Client) fetch(u string) ([]GIF, error) {
	resp, err := c.httpClient.Get(u)
	if err != nil {
		return nil, fmt.Errorf("giphy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("giphy: status %d", resp.StatusCode)
	}

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("giphy decode: %w", err)
	}

	gifs := make([]GIF, len(sr.Data))
	for i, d := range sr.Data {
		gifs[i] = GIF{
			ID:     d.ID,
			Title:  d.Title,
			URL:    d.URL,
			Images: d.Images,
		}
	}
	return gifs, nil
}
