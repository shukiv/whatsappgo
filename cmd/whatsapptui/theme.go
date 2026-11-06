package main

import "github.com/charmbracelet/lipgloss"

// The desktop client's colours, as close as a terminal gets to them. Only the
// ones the shell needs are here; each later pane adds its own.
var (
	colourBrand   = lipgloss.Color("#25D366")
	colourMuted   = lipgloss.Color("#8696A0")
	colourDanger  = lipgloss.Color("#F15C6D")
	colourSelect  = lipgloss.Color("#2A3942")
	styleTitle    = lipgloss.NewStyle().Bold(true).Foreground(colourBrand)
	styleMuted    = lipgloss.NewStyle().Foreground(colourMuted)
	styleDanger   = lipgloss.NewStyle().Foreground(colourDanger)
	styleSelected = lipgloss.NewStyle().Background(colourRowKeys).Bold(true)
	// The line the desktop client draws across a conversation where the
	// messages nobody has read yet begin.
	styleUnread = lipgloss.NewStyle().Foreground(colourBrand)
	stylePane   = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(colourSelect)
	// A message is a bubble of colour: the account's own in WhatsApp's light
	// green with dark words, everybody else's in grey. The grey is a light one
	// on a light terminal and a dark one on a dark terminal, so a bubble stands
	// off the window either way.
	paintOwn = bubblePaint{
		fill:   lipgloss.Color("#D9FDD3"),
		words:  lipgloss.Color("#111B21"),
		muted:  lipgloss.Color("#54656F"),
		sender: lipgloss.Color("#008069"),
	}
	paintTheirs = bubblePaint{
		fill:   lipgloss.AdaptiveColor{Light: "#E9EDEF", Dark: "#202C33"},
		words:  lipgloss.AdaptiveColor{Light: "#111B21", Dark: "#E9EDEF"},
		muted:  lipgloss.AdaptiveColor{Light: "#54656F", Dark: "#8696A0"},
		sender: lipgloss.AdaptiveColor{Light: "#008069", Dark: "#06CF9C"},
	}
	// The account's disc at the top of the list, in the desktop client's
	// green, and red when the connection is down.
	colourDisc = lipgloss.Color("#00A884")
	// A conversation's disc is in one of these, the same one every time for
	// the same conversation.
	discColours = []lipgloss.Color{"#6B5CA5", "#2E7D6B", "#B3541E", "#1F6FB2", "#8E3B76",
		"#4F7A28", "#5A6B7B", "#A0522D", "#3A7CA5", "#7A6A2E"}
	// The open conversation's row is tinted the way WhatsApp tints it, and
	// the row the keys are on a shade further, on a light terminal and a dark.
	colourRowOpen = lipgloss.AdaptiveColor{Light: "#F0F2F5", Dark: "#202C33"}
	colourRowKeys = lipgloss.AdaptiveColor{Light: "#D9DEE2", Dark: "#2A3942"}
	// The message bar at the foot of a conversation, in the desktop client's
	// panel grey.
	colourBar = lipgloss.AdaptiveColor{Light: "#F0F2F5", Dark: "#202C33"}
	// How many messages are waiting, on a green pill; a grey one when the
	// conversation is muted. Read ticks are WhatsApp's blue.
	colourBadgeText = lipgloss.Color("#111B21")
	colourRead      = lipgloss.Color("#53BDEB")
)

// bubblePaint is the colours of one kind of bubble: its fill, its words, the
// clock and marks, and the name of who wrote it.
type bubblePaint struct {
	fill, words, muted, sender lipgloss.TerminalColor
}
