package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type NLFormModel struct {
	input    textinput.Model
	Quitting bool
	done     bool
}

type NLFormResult struct {
	NL       string
	Quitting bool
}

func RunNL2SQLForm() (NLFormResult, error) {
	m := NLFormModel{
		input: textinput.New(),
	}
	m.input.Placeholder = "Describe what you want to query..."
	m.input.Focus()
	m.input.CharLimit = 500
	m.input.Prompt = ""

	prog := tea.NewProgram(m)
	res, err := prog.Run()
	if err != nil {
		return NLFormResult{}, err
	}
	model := res.(NLFormModel)
	return NLFormResult{NL: strings.TrimSpace(model.input.Value()), Quitting: model.Quitting}, nil
}

func (m NLFormModel) Init() tea.Cmd { return textinput.Blink }

func (m NLFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "ctrl+c", "esc":
			m.Quitting = true
			return m, tea.Quit
		}
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			m.done = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m NLFormModel) View() string {
	if m.Quitting || m.done {
		return ""
	}
	return "Enter NL query and press Enter to submit, Esc to cancel:\n\n" + m.input.View() + "\n"
}

// WaitForEsc creates a simple TUI that displays content and waits for Esc key
type waitForEscModel struct {
	content string
	done    bool
}

func WaitForEsc(content string) error {
	m := waitForEscModel{content: content}
	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}

func (m waitForEscModel) Init() tea.Cmd {
	return nil
}

func (m waitForEscModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "ctrl+c":
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m waitForEscModel) View() string {
	if m.done {
		return ""
	}
	return m.content + "\n\nPress Esc to return to menu..."
}
