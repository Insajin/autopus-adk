// Package tui provides Autopus brand styling for terminal output.
//
// All styles are lazily initialized to prevent lipgloss v1.x OSC 11
// terminal background query that blocks in non-TTY environments.
package tui

import (
	"os"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// EnsureSafeEnv sets NO_COLOR when stdout is not a TTY, preventing lipgloss
// from sending OSC 11 terminal background queries that hang in non-interactive
// environments (e.g., Claude Code Bash tool, CI pipelines).
// Called automatically by style accessors; also callable from main.
var envOnce sync.Once

func EnsureSafeEnv() {
	envOnce.Do(func() {
		if !term.IsTerminal(int(os.Stdout.Fd())) {
			os.Setenv("NO_COLOR", "1")
		}
	})
}

// Brand colors from autopus.co — lipgloss.Color is just a string, no query.
var (
	ColorViolet     = lipgloss.Color("#7c3aed")
	ColorVioletDark = lipgloss.Color("#6D28D9")
	ColorPurple     = lipgloss.Color("#A855F7")

	ColorPink = lipgloss.Color("#F43F5E")
	ColorRose = lipgloss.Color("#FB7185")

	ColorSuccess = lipgloss.Color("#2ECC71")
	ColorError   = lipgloss.Color("#E74C3C")
	ColorWarning = lipgloss.Color("#F39C12")
	ColorInfo    = lipgloss.Color("#3498DB")

	ColorMuted = lipgloss.Color("#95A5A6")
	ColorDark  = lipgloss.Color("#1F2937")
	ColorWhite = lipgloss.Color("#FFFFFF")
)

// Lazy style initialization — deferred until first use.
var (
	styleOnce         sync.Once
	BrandStyle        lipgloss.Style
	AccentStyle       lipgloss.Style
	MutedStyle        lipgloss.Style
	BoldStyle         lipgloss.Style
	SuccessStyle      lipgloss.Style
	ErrorStyle        lipgloss.Style
	WarningStyle      lipgloss.Style
	InfoStyle         lipgloss.Style
	SuccessLabelStyle lipgloss.Style
	ErrorLabelStyle   lipgloss.Style
	WarningLabelStyle lipgloss.Style
	InfoLabelStyle    lipgloss.Style
)

// InitStyles initializes all styles. Safe to call multiple times.
// Automatically called by EnsureStyles().
func InitStyles() {
	styleOnce.Do(func() {
		EnsureSafeEnv()
		BrandStyle = lipgloss.NewStyle().Foreground(ColorViolet).Bold(true)
		AccentStyle = lipgloss.NewStyle().Foreground(ColorPink)
		MutedStyle = lipgloss.NewStyle().Foreground(ColorMuted)
		BoldStyle = lipgloss.NewStyle().Bold(true)
		SuccessStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
		ErrorStyle = lipgloss.NewStyle().Foreground(ColorError)
		WarningStyle = lipgloss.NewStyle().Foreground(ColorWarning)
		InfoStyle = lipgloss.NewStyle().Foreground(ColorInfo)
		SuccessLabelStyle = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
		ErrorLabelStyle = lipgloss.NewStyle().Foreground(ColorError).Bold(true)
		WarningLabelStyle = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
		InfoLabelStyle = lipgloss.NewStyle().Foreground(ColorInfo).Bold(true)
	})
}
