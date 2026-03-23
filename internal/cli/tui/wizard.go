package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// SummaryRow represents a key-value pair for the summary table.
type SummaryRow struct {
	Key   string
	Value string
}

// WizardHeader prints a step counter combined with a section header.
func WizardHeader(w io.Writer, step, total int, title string) {
	counter := MutedStyle.Render(fmt.Sprintf("[%d/%d]", step, total))
	styled := lipgloss.NewStyle().
		Foreground(ColorViolet).
		Bold(true).
		Render(title)
	line := MutedStyle.Render(strings.Repeat("─", bannerWidth-len(title)-8))
	fmt.Fprintf(w, "\n%s %s %s\n", counter, styled, line)
}

// @AX:NOTE [AUTO]: SummaryTable uses bannerWidth (40) for box width — long values will wrap
// SummaryTable prints a key-value summary inside a branded box.
func SummaryTable(w io.Writer, rows []SummaryRow) {
	if len(rows) == 0 {
		return
	}

	// Find max key length for alignment
	maxKey := 0
	for _, r := range rows {
		if len(r.Key) > maxKey {
			maxKey = len(r.Key)
		}
	}

	var lines []string
	for _, r := range rows {
		key := MutedStyle.Render(fmt.Sprintf("%-*s", maxKey, r.Key))
		val := BrandStyle.Render(r.Value)
		lines = append(lines, fmt.Sprintf("  %s  %s", key, val))
	}

	content := strings.Join(lines, "\n")

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorViolet).
		Padding(0, 1).
		Width(bannerWidth)

	header := BrandStyle.Render("Summary")
	body := fmt.Sprintf("%s\n%s", header, content)
	fmt.Fprintln(w, style.Render(body))
}
