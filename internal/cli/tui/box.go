package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/lipgloss"
)

// Box prints content inside a rounded branded box.
func Box(w io.Writer, title, content string) {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorViolet).
		Padding(0, 1).
		Width(bannerWidth)

	header := BrandStyle.Render(title)
	body := fmt.Sprintf("%s\n%s", header, content)

	_, _ = fmt.Fprintln(w, style.Render(body))
}

// InfoBox prints content inside a blue info box.
func InfoBox(w io.Writer, title, content string) {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorInfo).
		Padding(0, 1).
		Width(bannerWidth)

	header := InfoLabelStyle.Render(title)
	body := fmt.Sprintf("%s\n%s", header, content)

	fmt.Fprintln(w, style.Render(body))
}

// ErrorBox prints content inside a red error box.
func ErrorBox(w io.Writer, title, content string) {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorError).
		Padding(0, 1).
		Width(bannerWidth)

	header := ErrorLabelStyle.Render(title)
	body := fmt.Sprintf("%s\n%s", header, content)

	fmt.Fprintln(w, style.Render(body))
}

// ResultBox prints a summary result box (e.g., doctor diagnosis).
func ResultBox(w io.Writer, passed bool, message string) {
	var borderColor lipgloss.Color
	var icon, label string

	if passed {
		borderColor = ColorSuccess
		icon = SuccessLabelStyle.Render("✓")
		label = SuccessLabelStyle.Render("PASS")
	} else {
		borderColor = ColorError
		icon = ErrorLabelStyle.Render("✗")
		label = ErrorLabelStyle.Render("FAIL")
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(bannerWidth)

	body := fmt.Sprintf("%s %s  %s", icon, label, message)
	fmt.Fprintln(w, style.Render(body))
}
