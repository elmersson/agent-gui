package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	output         string
	input          string
	mode           Mode
	command        string
	agentName      string
	state          string
	agentManager   interface{}
	sessionManager interface{}
	bus            interface{}
}

type Mode int

const (
	ModeView Mode = iota
	ModeCommand
)

type OutputMsg string
type StateMsg string
type CommandMsg string

func NewModel(agentManager interface{}, sessionManager interface{}, bus interface{}) Model {
	return Model{
		output:         "",
		input:          "",
		mode:           ModeView,
		command:        "",
		agentName:      "claude",
		state:          "idle",
		agentManager:   agentManager,
		sessionManager: sessionManager,
		bus:            bus,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.mode {
		case ModeView:
			switch msg.Type {
			case tea.KeyRunes:
				if len(msg.Runes) == 1 && msg.Runes[0] == ':' && m.input == "" {
					m.mode = ModeCommand
					m.command = ""
				} else {
					m.input += string(msg.Runes)
				}
			case tea.KeySpace:
				m.input += " "
			case tea.KeyEnter:
				if m.input != "" {
					if sm, ok := m.sessionManager.(interface{ StartSession(string, string) }); ok {
						sm.StartSession("claude", m.input)
					}
					if am, ok := m.agentManager.(interface{ Start(string) error }); ok {
						am.Start(m.input)
					}
					m.input = ""
				}
			case tea.KeyBackspace:
				if len(m.input) > 0 {
					m.input = m.input[:len(m.input)-1]
				}
			case tea.KeyCtrlC:
				return m, tea.Quit
			}

		case ModeCommand:
			switch msg.Type {
			case tea.KeyRunes:
				m.command += string(msg.Runes)
			case tea.KeySpace:
				m.command += " "
			case tea.KeyEnter:
				cmd := strings.TrimSpace(m.command)
				if cmd != "" {
					m.handleCommand(cmd)
				}
				m.mode = ModeView
				m.command = ""
			case tea.KeyEsc:
				m.mode = ModeView
				m.command = ""
			case tea.KeyBackspace:
				if len(m.command) > 0 {
					m.command = m.command[:len(m.command)-1]
				}
			}
		}

	case OutputMsg:
		m.output += string(msg)

	case StateMsg:
		m.state = string(msg)
	}

	return m, nil
}

func (m Model) View() string {
	var b strings.Builder

	styles := lipgloss.NewStyle()

	header := styles.
		Foreground(lipgloss.Color("6")).
		Bold(true).
		Render(fmt.Sprintf(" Agent: %s | State: %s ", m.agentName, m.state))

	b.WriteString(header + "\n\n")

	b.WriteString("Output:\n")
	b.WriteString(m.output)
	b.WriteString("\n\n")

	if m.mode == ModeView {
		b.WriteString(styles.Foreground(lipgloss.Color("12")).Render("Input: ") + m.input)
	} else {
		b.WriteString(styles.Foreground(lipgloss.Color("13")).Render("Command: ") + ":" + m.command)
	}

	b.WriteString("\n\n")
	b.WriteString(styles.Foreground(lipgloss.Color("8")).Render("Press : for commands | Ctrl+C to quit"))

	return b.String()
}

func (m *Model) GetCommand() string {
	return m.command
}

func (m *Model) ClearCommand() {
	m.command = ""
}

func (m *Model) handleCommand(cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}
	switch parts[0] {
	case "start":
		if len(parts) > 1 {
			input := strings.Join(parts[1:], " ")
			if sm, ok := m.sessionManager.(interface{ StartSession(string, string) }); ok {
				sm.StartSession("claude", input)
			}
			if am, ok := m.agentManager.(interface{ Start(string) error }); ok {
				am.Start(input)
			}
		} else {
			m.output += "Error: 'start' requires a prompt\n"
		}
	case "stop":
		if am, ok := m.agentManager.(interface{ Stop() }); ok {
			am.Stop()
		}
	case "pause":
		if am, ok := m.agentManager.(interface{ Pause() }); ok {
			am.Pause()
		}
	case "resume":
		if am, ok := m.agentManager.(interface{ Resume() }); ok {
			am.Resume()
		}
	default:
		m.output += fmt.Sprintf("Unknown command: %s\n", parts[0])
	}
}