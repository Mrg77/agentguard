// Package ui holds the small, shared presentation helpers for agentguard's
// terminal output: a color palette and a couple of glyphs. Deliberately tiny —
// the value of this tool is the policy engine, not a TUI.
package ui

import "github.com/charmbracelet/lipgloss"

var (
	// The palette leans on a deliberate accent (a guard's amber) rather than a
	// default grey, with green/red for the allow/deny poles.
	Accent = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	OK     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	Warn   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	Err    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	Dim    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	Faint  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	Rule   = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
)

// Glyphs for the three decisions.
const (
	MarkAllow = "✓"
	MarkAsk   = "?"
	MarkDeny  = "✗"
)

// Header renders a framed section title: a rule, the title, an optional
// subtitle, another rule.
func Header(title, subtitle string) string {
	rule := Rule.Render("────────────────────────────────────────────────────────────")
	out := rule + "\n" + Accent.Render("  ❯ "+title) + "\n"
	if subtitle != "" {
		out += Dim.Render("  "+subtitle) + "\n"
	}
	return out + rule
}
