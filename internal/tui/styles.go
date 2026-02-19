package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primary   = lipgloss.Color("#7C3AED") // violet
	secondary = lipgloss.Color("#06B6D4") // cyan
	success   = lipgloss.Color("#10B981") // green
	warning   = lipgloss.Color("#F59E0B") // amber
	danger    = lipgloss.Color("#EF4444") // red
	muted     = lipgloss.Color("#6B7280") // gray
	surface   = lipgloss.Color("#1F2937") // dark gray
	text      = lipgloss.Color("#F9FAFB") // near white
	dimText   = lipgloss.Color("#9CA3AF") // light gray

	// App frame
	appStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// Title bar
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(text).
			Background(primary).
			Padding(0, 2).
			MarginBottom(1)

	// Sidebar
	sidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(muted).
			Padding(0, 1)

	sidebarTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(secondary).
			MarginBottom(1)

	// Peer list
	peerOnline = lipgloss.NewStyle().
			Foreground(success)

	peerOffline = lipgloss.NewStyle().
			Foreground(muted)

	peerSelected = lipgloss.NewStyle().
			Bold(true).
			Foreground(text).
			Background(primary).
			Padding(0, 1)

	peerNormal = lipgloss.NewStyle().
			Foreground(text).
			Padding(0, 1)

	// Chat area
	chatStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(muted).
			Padding(0, 1)

	chatHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(primary).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(muted).
			MarginBottom(1).
			Width(100)

	// Messages
	ownMsgStyle = lipgloss.NewStyle().
			Foreground(secondary)

	peerMsgStyle = lipgloss.NewStyle().
			Foreground(success)

	msgTimeStyle = lipgloss.NewStyle().
			Foreground(dimText)

	msgContentStyle = lipgloss.NewStyle().
			Foreground(text)

	// Input
	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primary).
			Padding(0, 1)

	// Status bar
	statusBarStyle = lipgloss.NewStyle().
			Foreground(dimText).
			MarginTop(1)

	// Help
	helpStyle = lipgloss.NewStyle().
			Foreground(dimText)

	// Group badge
	groupBadge = lipgloss.NewStyle().
			Foreground(warning).
			Bold(true)

	// Connecting indicator
	connectingStyle = lipgloss.NewStyle().
			Foreground(warning).
			Italic(true)

	// Error style
	errorStyle = lipgloss.NewStyle().
			Foreground(danger).
			Bold(true)

	// Encryption badge
	encryptedBadge = lipgloss.NewStyle().
			Foreground(success).
			Bold(true)

	// Unread badge
	unreadStyle = lipgloss.NewStyle().
			Foreground(danger).
			Bold(true)

	// System message
	systemMsgStyle = lipgloss.NewStyle().
			Foreground(warning).
			Italic(true)

	// URL style
	urlStyle = lipgloss.NewStyle().
			Foreground(secondary).
			Underline(true)

	// GIF/image label
	gifLabel = lipgloss.NewStyle().
			Foreground(warning).
			Bold(true)

	// Tailchat online (running tailchat but not yet connected)
	tailchatOnline = lipgloss.NewStyle().
			Foreground(secondary)

	// Tailchat badge label
	tailchatBadge = lipgloss.NewStyle().
			Foreground(secondary).
			Italic(true)

	// Typing indicator
	typingStyle = lipgloss.NewStyle().
			Foreground(dimText).
			Italic(true)

	// Reactions
	reactionStyle = lipgloss.NewStyle().
			Foreground(warning)

	// Delivery ticks
	deliveryStyle = lipgloss.NewStyle().
			Foreground(success)

	deliveryPending = lipgloss.NewStyle().
			Foreground(dimText)

	// Status indicators
	statusAway = lipgloss.NewStyle().
			Foreground(warning)

	statusBusy = lipgloss.NewStyle().
			Foreground(danger)

	statusDnd = lipgloss.NewStyle().
			Foreground(danger).
			Bold(true)

	// Emoji completion
	completionStyle = lipgloss.NewStyle().
			Foreground(dimText)

	completionSelected = lipgloss.NewStyle().
				Foreground(text).
				Background(primary)

	// Scroll indicator
	scrollIndicator = lipgloss.NewStyle().
			Foreground(dimText).
			Italic(true)
)
