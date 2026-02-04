package tui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rasmuselmersson/opencode/pkg/agent"
)

type Model struct {
	output         string
	input          string
	mode           Mode
	command        string
	agentName      string
	modelName      string
	state          string
	tokenUsage     agent.TokenUsage
	agentManager   interface{}
	sessionManager interface{}
	bus            interface{}
	scrollOffset   int
	terminalHeight int
	// Model selection
	availableModels []string
	modelCursor     int
	modelScroll     int
}

type Mode int

const (
	ModeView Mode = iota
	ModeCommand
	ModeModelSelect
)

type OutputMsg string
type StateMsg string
type CommandMsg string
type TokenUsageMsg agent.TokenUsage

func NewModel(agentManager interface{}, sessionManager interface{}, bus interface{}, modelName string) Model {
	if modelName == "" {
		modelName = "default"
	}
	return Model{
		output:         "",
		input:          "",
		mode:           ModeView,
		command:        "",
		agentName:      "opencode",
		modelName:      modelName,
		state:          "idle",
		agentManager:   agentManager,
		sessionManager: sessionManager,
		bus:            bus,
		scrollOffset:   0,
		terminalHeight: 24, // Default, will be updated
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.terminalHeight = msg.Height

	case tea.KeyMsg:
		switch m.mode {
		case ModeView:
			switch msg.Type {
			case tea.KeyRunes:
				if len(msg.Runes) == 1 && msg.Runes[0] == ':' && m.input == "" {
					m.mode = ModeCommand
					m.command = ""
				} else if len(msg.Runes) == 1 && msg.Runes[0] == 'j' && m.input == "" {
					// Scroll down
					m.scrollOffset++
				} else if len(msg.Runes) == 1 && msg.Runes[0] == 'k' && m.input == "" {
					// Scroll up
					if m.scrollOffset > 0 {
						m.scrollOffset--
					}
				} else if len(msg.Runes) == 1 && msg.Runes[0] == 'G' && m.input == "" {
					// Go to bottom
					lines := strings.Split(m.output, "\n")
					m.scrollOffset = max(0, len(lines)-m.terminalHeight+10)
				} else if len(msg.Runes) == 1 && msg.Runes[0] == 'g' && m.input == "" {
					// Go to top
					m.scrollOffset = 0
				} else {
					m.input += string(msg.Runes)
				}
			case tea.KeySpace:
				m.input += " "
			case tea.KeyUp:
				if m.scrollOffset > 0 {
					m.scrollOffset--
				}
			case tea.KeyDown:
				m.scrollOffset++
			case tea.KeyPgUp:
				m.scrollOffset = max(0, m.scrollOffset-10)
			case tea.KeyPgDown:
				m.scrollOffset += 10
			case tea.KeyEnter:
				if m.input != "" {
					if sm, ok := m.sessionManager.(interface{ StartSession(string, string) }); ok {
						sm.StartSession(m.agentName, m.input)
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
				cmdStr := strings.TrimSpace(m.command)
				m.command = ""
				m = m.handleCommand(cmdStr)
			case tea.KeyEsc:
				m.mode = ModeView
				m.command = ""
			case tea.KeyBackspace:
				if len(m.command) > 0 {
					m.command = m.command[:len(m.command)-1]
				}
			}

		case ModeModelSelect:
			switch msg.Type {
			case tea.KeyRunes:
				if len(msg.Runes) == 1 {
					switch msg.Runes[0] {
					case 'j':
						if m.modelCursor < len(m.availableModels)-1 {
							m.modelCursor++
							// Auto-scroll if needed
							visibleHeight := m.terminalHeight - 10
							if visibleHeight < 5 {
								visibleHeight = 5
							}
							if m.modelCursor >= m.modelScroll+visibleHeight {
								m.modelScroll++
							}
						}
					case 'k':
						if m.modelCursor > 0 {
							m.modelCursor--
							if m.modelCursor < m.modelScroll {
								m.modelScroll--
							}
						}
					case 'G':
						m.modelCursor = len(m.availableModels) - 1
						visibleHeight := m.terminalHeight - 10
						if visibleHeight < 5 {
							visibleHeight = 5
						}
						m.modelScroll = max(0, len(m.availableModels)-visibleHeight)
					case 'g':
						m.modelCursor = 0
						m.modelScroll = 0
					case 'q':
						m.mode = ModeView
					}
				}
			case tea.KeyUp:
				if m.modelCursor > 0 {
					m.modelCursor--
					if m.modelCursor < m.modelScroll {
						m.modelScroll--
					}
				}
			case tea.KeyDown:
				if m.modelCursor < len(m.availableModels)-1 {
					m.modelCursor++
					visibleHeight := m.terminalHeight - 10
					if visibleHeight < 5 {
						visibleHeight = 5
					}
					if m.modelCursor >= m.modelScroll+visibleHeight {
						m.modelScroll++
					}
				}
			case tea.KeyPgUp:
				m.modelCursor = max(0, m.modelCursor-10)
				m.modelScroll = max(0, m.modelScroll-10)
			case tea.KeyPgDown:
				m.modelCursor = min(len(m.availableModels)-1, m.modelCursor+10)
				visibleHeight := m.terminalHeight - 10
				if visibleHeight < 5 {
					visibleHeight = 5
				}
				maxScroll := max(0, len(m.availableModels)-visibleHeight)
				m.modelScroll = min(maxScroll, m.modelScroll+10)
			case tea.KeyEnter:
				// Select the model
				if m.modelCursor < len(m.availableModels) {
					selectedModel := m.availableModels[m.modelCursor]
					m.modelName = selectedModel
					if am, ok := m.agentManager.(interface{ SetModel(string) }); ok {
						am.SetModel(selectedModel)
					}
					m.output += fmt.Sprintf("Model set to: %s\n", selectedModel)
				}
				m.mode = ModeView
			case tea.KeyEsc:
				m.mode = ModeView
			case tea.KeyCtrlC:
				return m, tea.Quit
			}
		}

	case OutputMsg:
		m.output += string(msg)

	case StateMsg:
		m.state = string(msg)

	case TokenUsageMsg:
		m.tokenUsage = agent.TokenUsage(msg)
	}

	return m, nil
}

func (m Model) View() string {
	var b strings.Builder

	styles := lipgloss.NewStyle()

	// Build token/cost display string
	tokenInfo := ""
	if m.tokenUsage.TotalTokens > 0 {
		cacheInfo := ""
		if m.tokenUsage.CacheRead > 0 {
			cacheInfo = fmt.Sprintf(", cache: %d", m.tokenUsage.CacheRead)
		}
		tokenInfo = fmt.Sprintf(" | Tokens: %d (in: %d, out: %d%s) | Cost: $%.4f",
			m.tokenUsage.TotalTokens,
			m.tokenUsage.InputTokens,
			m.tokenUsage.OutputTokens,
			cacheInfo,
			m.tokenUsage.CostUSD)
	}

	header := styles.
		Foreground(lipgloss.Color("6")).
		Bold(true).
		Render(fmt.Sprintf(" Agent: %s (%s) | State: %s%s ", m.agentName, m.modelName, m.state, tokenInfo))

	b.WriteString(header + "\n\n")

	// Model selection mode
	if m.mode == ModeModelSelect {
		b.WriteString(styles.Foreground(lipgloss.Color("11")).Bold(true).Render("Select Model") + "\n")
		b.WriteString(styles.Foreground(lipgloss.Color("8")).Render("j/k or arrows to navigate | Enter to select | q/Esc to cancel") + "\n\n")

		visibleHeight := m.terminalHeight - 10
		if visibleHeight < 5 {
			visibleHeight = 5
		}

		startIdx := m.modelScroll
		endIdx := min(startIdx+visibleHeight, len(m.availableModels))

		for i := startIdx; i < endIdx; i++ {
			model := m.availableModels[i]
			prefix := "  "
			style := styles.Foreground(lipgloss.Color("7"))

			// Highlight current selection
			if i == m.modelCursor {
				prefix = "> "
				style = styles.Foreground(lipgloss.Color("14")).Bold(true)
			}

			// Mark currently active model
			suffix := ""
			if model == m.modelName {
				suffix = " (current)"
				if i != m.modelCursor {
					style = styles.Foreground(lipgloss.Color("10"))
				}
			}

			b.WriteString(style.Render(fmt.Sprintf("%s%s%s", prefix, model, suffix)) + "\n")
		}

		// Scroll indicator
		if len(m.availableModels) > visibleHeight {
			maxScroll := max(0, len(m.availableModels)-visibleHeight)
			b.WriteString(fmt.Sprintf("\n[%d/%d models]", m.modelCursor+1, len(m.availableModels)))
			if m.modelScroll > 0 {
				b.WriteString(" ↑")
			}
			if m.modelScroll < maxScroll {
				b.WriteString(" ↓")
			}
		}

		return b.String()
	}

	// Normal view mode
	// Apply scrolling to output
	outputLines := strings.Split(m.output, "\n")
	visibleHeight := m.terminalHeight - 8 // Reserve space for header, input, and footer
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	// Clamp scroll offset
	maxScroll := max(0, len(outputLines)-visibleHeight)
	scrollOffset := min(m.scrollOffset, maxScroll)

	// Get visible lines
	startLine := scrollOffset
	endLine := min(startLine+visibleHeight, len(outputLines))

	scrollIndicator := ""
	if len(outputLines) > visibleHeight {
		scrollIndicator = fmt.Sprintf(" [%d/%d]", scrollOffset+1, maxScroll+1)
	}

	b.WriteString(fmt.Sprintf("Output:%s\n", scrollIndicator))
	if startLine < len(outputLines) {
		visibleOutput := strings.Join(outputLines[startLine:endLine], "\n")
		b.WriteString(visibleOutput)
	}
	b.WriteString("\n\n")

	if m.mode == ModeView {
		b.WriteString(styles.Foreground(lipgloss.Color("12")).Render("Input: ") + m.input)
	} else {
		b.WriteString(styles.Foreground(lipgloss.Color("13")).Render("Command: ") + ":" + m.command)
	}

	b.WriteString("\n\n")
	b.WriteString(styles.Foreground(lipgloss.Color("8")).Render("Press : for commands | j/k or arrows to scroll | Ctrl+C to quit"))

	return b.String()
}

func (m Model) GetCommand() string {
	return m.command
}

func (m Model) handleCommand(cmd string) Model {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		m.mode = ModeView
		return m
	}
	switch parts[0] {
	case "start":
		if len(parts) > 1 {
			input := strings.Join(parts[1:], " ")
			if sm, ok := m.sessionManager.(interface{ StartSession(string, string) }); ok {
				sm.StartSession(m.agentName, input)
			}
			if am, ok := m.agentManager.(interface{ Start(string) error }); ok {
				am.Start(input)
			}
		} else {
			m.output += "Error: 'start' requires a prompt\n"
		}
		m.mode = ModeView
	case "stop":
		if am, ok := m.agentManager.(interface{ Stop() }); ok {
			am.Stop()
		}
		m.mode = ModeView
	case "pause":
		if am, ok := m.agentManager.(interface{ Pause() }); ok {
			am.Pause()
		}
		m.mode = ModeView
	case "resume":
		if am, ok := m.agentManager.(interface{ Resume() }); ok {
			am.Resume()
		}
		m.mode = ModeView
	case "models":
		// Load available models and enter selection mode
		execCmd := exec.Command("opencode", "models")
		output, err := execCmd.Output()
		if err != nil {
			m.output += fmt.Sprintf("Error listing models: %v\n", err)
			m.mode = ModeView
		} else {
			// Parse models into list
			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			m.availableModels = make([]string, 0, len(lines))
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" {
					m.availableModels = append(m.availableModels, line)
				}
			}

			// Find current model position
			m.modelCursor = 0
			for i, model := range m.availableModels {
				if model == m.modelName {
					m.modelCursor = i
					break
				}
			}

			// Set scroll to show cursor
			visibleHeight := m.terminalHeight - 10
			if visibleHeight < 5 {
				visibleHeight = 5
			}
			m.modelScroll = max(0, m.modelCursor-visibleHeight/2)

			m.mode = ModeModelSelect
		}
	case "model":
		if len(parts) > 1 {
			newModel := parts[1]
			m.modelName = newModel
			// Update the adapter's model
			if am, ok := m.agentManager.(interface{ SetModel(string) }); ok {
				am.SetModel(newModel)
			}
			m.output += fmt.Sprintf("Model set to: %s\n", newModel)
		} else {
			m.output += fmt.Sprintf("Current model: %s\n", m.modelName)
			m.output += "Usage: :model <provider/model-name>\n"
			m.output += "Example: :model opencode/claude-opus-4-5\n"
		}
		m.mode = ModeView
	case "help":
		m.output += "\n--- Commands ---\n"
		m.output += ":start <prompt>  - Start agent with prompt\n"
		m.output += ":stop            - Stop running agent\n"
		m.output += ":pause           - Pause agent\n"
		m.output += ":resume          - Resume paused agent\n"
		m.output += ":models          - List available models\n"
		m.output += ":model <name>    - Set model (e.g. :model opencode/claude-opus-4-5)\n"
		m.output += ":help            - Show this help\n"
		m.output += "----------------\n"
		m.mode = ModeView
	default:
		m.output += fmt.Sprintf("Unknown command: %s (try :help)\n", parts[0])
		m.mode = ModeView
	}
	return m
}
