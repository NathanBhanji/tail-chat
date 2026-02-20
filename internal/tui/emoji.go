package tui

import "strings"

// shortcodes maps emoji shortcodes to their Unicode equivalents.
var shortcodes = map[string]string{
	// Smileys
	"smile":           "😊",
	"grin":            "😁",
	"laugh":           "😂",
	"rofl":            "🤣",
	"joy":             "😂",
	"wink":            "😉",
	"blush":           "😊",
	"heart_eyes":      "😍",
	"kissing":         "😘",
	"thinking":        "🤔",
	"neutral":         "😐",
	"expressionless":  "😑",
	"unamused":        "😒",
	"rolling_eyes":    "🙄",
	"grimacing":       "😬",
	"relieved":        "😌",
	"sweat":           "😅",
	"sob":             "😭",
	"cry":             "😢",
	"scream":          "😱",
	"angry":           "😠",
	"rage":            "🤬",
	"skull":           "💀",
	"clown":           "🤡",
	"poop":            "💩",
	"ghost":           "👻",
	"alien":           "👽",
	"robot":           "🤖",
	"eyes":            "👀",
	"brain":           "🧠",
	"mind_blown":      "🤯",
	"shush":           "🤫",
	"zipper_mouth":    "🤐",
	"nerd":            "🤓",
	"monocle":         "🧐",
	"sunglasses":      "😎",
	"disguise":        "🥸",
	"sick":            "🤢",
	"vomit":           "🤮",
	"hot":             "🥵",
	"cold":            "🥶",
	"party":           "🥳",
	"sleepy":          "😴",
	"yawn":            "🥱",

	// Gestures
	"wave":            "👋",
	"ok":              "👌",
	"ok_hand":         "👌",
	"thumbsup":        "👍",
	"+1":              "👍",
	"thumbsdown":      "👎",
	"-1":              "👎",
	"clap":            "👏",
	"raised_hands":    "🙌",
	"pray":            "🙏",
	"handshake":       "🤝",
	"point_up":        "☝️",
	"point_down":      "👇",
	"point_left":      "👈",
	"point_right":     "👉",
	"middle_finger":   "🖕",
	"fist":            "✊",
	"punch":           "👊",
	"muscle":          "💪",
	"crossed_fingers": "🤞",
	"v":               "✌️",
	"peace":           "✌️",
	"love_you":        "🤟",
	"metal":           "🤘",
	"salute":          "🫡",
	"shrug":           "🤷",
	"facepalm":        "🤦",

	// Hearts
	"heart":           "❤️",
	"red_heart":       "❤️",
	"orange_heart":    "🧡",
	"yellow_heart":    "💛",
	"green_heart":     "💚",
	"blue_heart":      "💙",
	"purple_heart":    "💜",
	"black_heart":     "🖤",
	"white_heart":     "🤍",
	"broken_heart":    "💔",
	"fire_heart":      "❤️‍🔥",
	"sparkling_heart": "💖",

	// Animals
	"dog":             "🐕",
	"cat":             "🐈",
	"bear":            "🐻",
	"panda":           "🐼",
	"fox":             "🦊",
	"lion":            "🦁",
	"unicorn":         "🦄",
	"bee":             "🐝",
	"butterfly":       "🦋",
	"snake":           "🐍",
	"dragon":          "🐉",
	"whale":           "🐋",
	"dolphin":         "🐬",
	"octopus":         "🐙",
	"crab":            "🦀",
	"lobster":         "🦞",
	"shrimp":          "🦐",

	// Objects & symbols
	"fire":            "🔥",
	"100":             "💯",
	"star":            "⭐",
	"sparkles":        "✨",
	"boom":            "💥",
	"lightning":       "⚡",
	"rainbow":         "🌈",
	"sun":             "☀️",
	"moon":            "🌙",
	"cloud":           "☁️",
	"rain":            "🌧️",
	"snow":            "❄️",
	"tada":            "🎉",
	"confetti":        "🎊",
	"balloon":         "🎈",
	"gift":            "🎁",
	"trophy":          "🏆",
	"medal":           "🏅",
	"crown":           "👑",
	"gem":             "💎",
	"money":           "💰",
	"dollar":          "💵",
	"lock":            "🔒",
	"key":             "🔑",
	"hammer":          "🔨",
	"wrench":          "🔧",
	"gear":            "⚙️",
	"bulb":            "💡",
	"bomb":            "💣",
	"pill":            "💊",
	"rocket":          "🚀",
	"satellite":       "🛰️",
	"phone":           "📱",
	"laptop":          "💻",
	"desktop":         "🖥️",
	"keyboard":        "⌨️",
	"printer":         "🖨️",
	"camera":          "📷",
	"mag":             "🔍",
	"link":            "🔗",
	"paperclip":       "📎",
	"scissors":        "✂️",
	"pen":             "🖊️",
	"memo":            "📝",
	"book":            "📖",
	"warning":         "⚠️",
	"no_entry":        "⛔",
	"check":           "✅",
	"x":               "❌",
	"question":        "❓",
	"exclamation":     "❗",

	// Food
	"pizza":           "🍕",
	"burger":          "🍔",
	"fries":           "🍟",
	"taco":            "🌮",
	"burrito":         "🌯",
	"sushi":           "🍣",
	"ramen":           "🍜",
	"beer":            "🍺",
	"wine":            "🍷",
	"coffee":          "☕",
	"tea":             "🍵",
	"cake":            "🍰",
	"cookie":          "🍪",
	"donut":           "🍩",
	"icecream":        "🍦",
	"chocolate":       "🍫",
	"popcorn":         "🍿",
	"avocado":         "🥑",
	"eggplant":        "🍆",
	"peach":           "🍑",
	"banana":          "🍌",
	"watermelon":      "🍉",
	"grapes":          "🍇",
	"apple":           "🍎",
	"lemon":           "🍋",
	"hot_pepper":      "🌶️",

	// Activities
	"soccer":          "⚽",
	"basketball":      "🏀",
	"football":        "🏈",
	"baseball":        "⚾",
	"tennis":          "🎾",
	"golf":            "⛳",
	"ski":             "⛷️",
	"surf":            "🏄",
	"gaming":          "🎮",
	"dice":            "🎲",
	"music":           "🎵",
	"guitar":          "🎸",
	"mic":             "🎤",
	"headphones":      "🎧",
	"art":             "🎨",
	"film":            "🎬",

	// Flags
	"flag_us":         "🇺🇸",
	"flag_gb":         "🇬🇧",
	"flag_fr":         "🇫🇷",
	"flag_de":         "🇩🇪",
	"flag_jp":         "🇯🇵",
	"flag_kr":         "🇰🇷",
	"flag_cn":         "🇨🇳",
	"flag_in":         "🇮🇳",
	"flag_br":         "🇧🇷",
	"flag_au":         "🇦🇺",
	"flag_ca":         "🇨🇦",
	"pirate_flag":     "🏴‍☠️",
	"checkered_flag":  "🏁",
	"white_flag":      "🏳️",
	"rainbow_flag":    "🏳️‍🌈",
}

// expandEmoji replaces :shortcode: patterns with their emoji equivalents.
func expandEmoji(text string) string {
	var result strings.Builder
	i := 0
	for i < len(text) {
		if text[i] == ':' {
			// Look for closing colon
			end := strings.IndexByte(text[i+1:], ':')
			if end > 0 && end < 30 { // reasonable shortcode length
				code := text[i+1 : i+1+end]
				if emoji, ok := shortcodes[code]; ok {
					result.WriteString(emoji)
					i = i + 1 + end + 1
					continue
				}
			}
		}
		result.WriteByte(text[i])
		i++
	}
	return result.String()
}

// isImageURL checks if a URL points to an image/GIF.
func isImageURL(s string) bool {
	lower := strings.ToLower(s)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return false
	}
	// Strip query params for extension check
	path := lower
	if idx := strings.IndexByte(path, '?'); idx >= 0 {
		path = path[:idx]
	}
	return strings.HasSuffix(path, ".gif") ||
		strings.HasSuffix(path, ".png") ||
		strings.HasSuffix(path, ".jpg") ||
		strings.HasSuffix(path, ".jpeg") ||
		strings.HasSuffix(path, ".webp") ||
		// Common GIF hosting patterns
		strings.Contains(lower, "giphy.com") ||
		strings.Contains(lower, "tenor.com") ||
		strings.Contains(lower, "imgur.com")
}

// isURL checks if a string looks like a URL.
func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// renderContent styles message content, highlighting URLs and labeling GIFs/images.
func renderContent(content string) string {
	words := strings.Fields(content)
	if len(words) == 0 {
		return msgContentStyle.Render(content)
	}

	// Check if the entire message is a single URL (common for sharing links/GIFs)
	if len(words) == 1 && isURL(words[0]) {
		if isImageURL(words[0]) {
			return gifLabel.Render("[GIF] ") + urlStyle.Render(words[0])
		}
		return urlStyle.Render(words[0])
	}

	// Mixed content: style URLs inline
	var parts []string
	for _, word := range words {
		if isURL(word) {
			if isImageURL(word) {
				parts = append(parts, gifLabel.Render("[IMG]")+" "+urlStyle.Render(word))
			} else {
				parts = append(parts, urlStyle.Render(word))
			}
		} else {
			parts = append(parts, msgContentStyle.Render(word))
		}
	}
	return strings.Join(parts, " ")
}
