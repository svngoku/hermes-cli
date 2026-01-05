package ui

import "github.com/charmbracelet/lipgloss"

var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED"))

	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981"))

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444"))

	WarnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B"))

	InfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3B82F6"))

	DimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	BoldStyle = lipgloss.NewStyle().Bold(true)
)

func Banner() string {
	banner := `
  _   _                                
 | | | | ___ _ __ _ __ ___   ___  ___  
 | |_| |/ _ \ '__| '_ ` + "`" + ` _ \ / _ \/ __| 
 |  _  |  __/ |  | | | | | |  __/\__ \ 
 |_| |_|\___|_|  |_| |_| |_|\___||___/ 
`
	return TitleStyle.Render(banner)
}

func HR() string {
	return DimStyle.Render("─────────────────────────────────────────────")
}

func Ok(msg string) string {
	return SuccessStyle.Render("✓ ") + msg
}

func Fail(msg string) string {
	return ErrorStyle.Render("✗ ") + msg
}

func Warn(msg string) string {
	return WarnStyle.Render("⚠ ") + msg
}

func Info(msg string) string {
	return InfoStyle.Render("ℹ ") + msg
}

func Step(msg string) string {
	return BoldStyle.Render("→ ") + msg
}
