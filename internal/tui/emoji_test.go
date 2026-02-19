package tui

import "testing"

func TestExpandEmoji(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// Basic shortcode expansion
		{":fire:", "🔥"},
		{":thumbsup:", "👍"},
		{":heart:", "❤️"},
		{":rocket:", "🚀"},

		// Multiple emoji in one message
		{"hey :wave: how are you :smile:", "hey 👋 how are you 😊"},

		// Emoji at start and end
		{":fire: hot take :fire:", "🔥 hot take 🔥"},

		// Unknown shortcode — left as-is
		{":notanemoji:", ":notanemoji:"},

		// Single colon — no crash
		{"test : hello", "test : hello"},

		// Adjacent colons
		{"::fire::", ":🔥:"},

		// No emoji
		{"just plain text", "just plain text"},

		// Empty string
		{"", ""},

		// Mixed known and unknown
		{":fire: and :unknown: and :star:", "🔥 and :unknown: and ⭐"},
	}

	for _, tt := range tests {
		got := expandEmoji(tt.input)
		if got != tt.want {
			t.Errorf("expandEmoji(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsURL(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"https://example.com", true},
		{"http://example.com", true},
		{"https://example.com/path?q=1", true},
		{"ftp://example.com", false},
		{"not a url", false},
		{"", false},
		{"example.com", false},
	}

	for _, tt := range tests {
		got := isURL(tt.input)
		if got != tt.want {
			t.Errorf("isURL(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsImageURL(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		// Direct image extensions
		{"https://example.com/cat.gif", true},
		{"https://example.com/photo.png", true},
		{"https://example.com/photo.jpg", true},
		{"https://example.com/photo.jpeg", true},
		{"https://example.com/photo.webp", true},

		// With query params
		{"https://example.com/cat.gif?v=1", true},

		// GIF hosting sites
		{"https://media.giphy.com/media/abc/giphy.mp4", true},
		{"https://tenor.com/view/funny-123", true},
		{"https://i.imgur.com/abc123.jpg", true},

		// Case insensitive
		{"https://example.com/cat.GIF", true},
		{"https://example.com/photo.PNG", true},

		// Not an image
		{"https://example.com/page.html", false},
		{"https://example.com/doc.pdf", false},
		{"https://google.com", false},

		// Not even a URL
		{"cat.gif", false},
		{"just text", false},
	}

	for _, tt := range tests {
		got := isImageURL(tt.input)
		if got != tt.want {
			t.Errorf("isImageURL(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
