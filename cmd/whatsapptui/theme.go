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
	styleSelected = lipgloss.NewStyle().Background(colourSelect).Bold(true)
	// The line the desktop client draws across a conversation where the
	// messages nobody has read yet begin.
	styleUnread = lipgloss.NewStyle().Foreground(colourBrand)
	stylePane   = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(colourSelect)
)
