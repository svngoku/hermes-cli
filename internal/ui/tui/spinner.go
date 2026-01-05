package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SpinnerModel struct {
	spinner  spinner.Model
	title    string
	quitting bool
	err      error
	done     bool
	result   string
}

func NewSpinner(title string) SpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED"))
	return SpinnerModel{
		spinner: s,
		title:   title,
	}
}

func (m SpinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m SpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case DoneMsg:
		m.done = true
		m.result = msg.Result
		m.err = msg.Err
		return m, tea.Quit
	}
	return m, nil
}

func (m SpinnerModel) View() string {
	if m.done {
		if m.err != nil {
			return fmt.Sprintf("✗ %s: %v\n", m.title, m.err)
		}
		return fmt.Sprintf("✓ %s: %s\n", m.title, m.result)
	}
	return fmt.Sprintf("%s %s\n", m.spinner.View(), m.title)
}

type DoneMsg struct {
	Result string
	Err    error
}

func RunWithSpinner(title string, task func() (string, error)) error {
	model := NewSpinner(title)

	p := tea.NewProgram(model)

	go func() {
		result, err := task()
		p.Send(DoneMsg{Result: result, Err: err})
	}()

	_, err := p.Run()
	return err
}
