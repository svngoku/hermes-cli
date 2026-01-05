package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Step struct {
	Name   string
	Status StepStatus
	Detail string
}

type StepStatus int

const (
	StepPending StepStatus = iota
	StepRunning
	StepDone
	StepFailed
	StepSkipped
)

type StepsModel struct {
	steps       []Step
	currentStep int
	spinner     spinner.Model
	quitting    bool
	done        bool
	width       int
}

func NewStepsModel(steps []string) StepsModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED"))

	stepList := make([]Step, len(steps))
	for i, name := range steps {
		stepList[i] = Step{Name: name, Status: StepPending}
	}

	return StepsModel{
		steps:       stepList,
		currentStep: 0,
		spinner:     s,
		width:       80,
	}
}

func (m StepsModel) Init() tea.Cmd {
	return m.spinner.Tick
}

type StepCompleteMsg struct {
	Index  int
	Status StepStatus
	Detail string
}

type AllDoneMsg struct{}

func (m StepsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case StepCompleteMsg:
		if msg.Index < len(m.steps) {
			m.steps[msg.Index].Status = msg.Status
			m.steps[msg.Index].Detail = msg.Detail
			if msg.Index < len(m.steps)-1 {
				m.currentStep = msg.Index + 1
				m.steps[m.currentStep].Status = StepRunning
			}
		}
		return m, nil
	case AllDoneMsg:
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

var (
	stepStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	pendingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	runningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3B82F6"))

	doneStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981"))

	failedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444"))

	skippedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B"))

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")).
			MarginBottom(1)
)

func (m StepsModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Hermes Pipeline"))
	b.WriteString("\n\n")

	for i, step := range m.steps {
		var icon string
		var style lipgloss.Style

		switch step.Status {
		case StepPending:
			icon = "○"
			style = pendingStyle
		case StepRunning:
			icon = m.spinner.View()
			style = runningStyle
		case StepDone:
			icon = "✓"
			style = doneStyle
		case StepFailed:
			icon = "✗"
			style = failedStyle
		case StepSkipped:
			icon = "⊘"
			style = skippedStyle
		}

		line := fmt.Sprintf("%s %s", icon, step.Name)
		if step.Detail != "" {
			line += fmt.Sprintf(" (%s)", step.Detail)
		}

		b.WriteString(stepStyle.Render(style.Render(line)))
		if i < len(m.steps)-1 {
			b.WriteString("\n")
		}
	}

	if m.quitting {
		b.WriteString("\n\nQuitting...")
	}

	return b.String()
}
