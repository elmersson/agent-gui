package tui

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/rasmuselmersson/opencode/pkg/agent"
)

// Commands available for autocomplete
var commands = []string{
	"start",
	"stop",
	"pause",
	"resume",
	"models",
	"model",
	"templates",
	"spawn-template",
	"pipelines",
	"run-pipeline",
	"pipeline-status",
	"clear",
	"help",
	"quit",
}

type Mode int

const (
	ModeNormal Mode = iota
	ModeCommand
	ModeModelSelect
	ModeTemplateSelect
	ModePipelineSelect
)

type Model struct {
	// Core state
	mode       Mode
	agentName  string
	modelName  string
	state      string
	tokenUsage agent.TokenUsage

	// UI components
	viewport    viewport.Model
	input       textinput.Model
	commandLine textinput.Model

	// Content
	output    strings.Builder
	rawOutput string

	// Dimensions
	width  int
	height int

	// Autocomplete
	suggestions      []string
	suggestionIdx    int
	suggestionScroll int
	showSuggestion   bool

	// Selection lists
	availableModels    []string
	modelCursor        int
	availableTemplates []string
	templateCursor     int
	availablePipelines []string
	pipelineCursor     int

	// Pending template/pipeline
	pendingTemplate string
	pendingPipeline string

	// Pipeline progress
	pipelineStatus    string
	pipelineStages    []string
	currentStageIndex int

	// External interfaces
	agentManager   interface{}
	sessionManager interface{}
	bus            interface{}

	// Markdown renderer
	renderer *glamour.TermRenderer
}

// Styles
var (
	// Colors
	primaryColor   = lipgloss.Color("#7C3AED") // Purple
	secondaryColor = lipgloss.Color("#10B981") // Green
	accentColor    = lipgloss.Color("#F59E0B") // Amber
	errorColor     = lipgloss.Color("#EF4444") // Red
	mutedColor     = lipgloss.Color("#6B7280") // Gray
	textColor      = lipgloss.Color("#F3F4F6") // Light gray

	// Box styles
	headerStyle = lipgloss.NewStyle().
			Background(primaryColor).
			Foreground(textColor).
			Bold(true).
			Padding(0, 1)

	statusRunning = lipgloss.NewStyle().
			Background(secondaryColor).
			Foreground(lipgloss.Color("#000")).
			Bold(true).
			Padding(0, 1)

	statusIdle = lipgloss.NewStyle().
			Background(mutedColor).
			Foreground(textColor).
			Padding(0, 1)

	statusPaused = lipgloss.NewStyle().
			Background(accentColor).
			Foreground(lipgloss.Color("#000")).
			Bold(true).
			Padding(0, 1)

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor)

	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(secondaryColor).
			Padding(0, 1)

	commandStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accentColor).
			Padding(0, 1)

	suggestionStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true)

	selectedSuggestion = lipgloss.NewStyle().
				Foreground(secondaryColor).
				Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	tokenStyle = lipgloss.NewStyle().
			Foreground(accentColor)

	costStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	listItemStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Padding(0, 2)

	listSelectedStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true).
				Padding(0, 1).
				SetString("> ")

	titleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			Underline(true)
)

type OutputMsg string
type StateMsg string
type TokenUsageMsg agent.TokenUsage
type PipelineStatusMsg struct {
	Status      string
	Stages      []string
	CurrentIdx  int
	StageName   string
	StageOutput string
}
type StageProgressMsg struct {
	StageName  string
	StageIndex int
	Total      int
	Status     string
	Output     string
}

func NewModel(agentManager interface{}, sessionManager interface{}, bus interface{}, modelName string) Model {
	if modelName == "" {
		modelName = "auto"
	}

	// Create text input for normal input
	ti := textinput.New()
	ti.Placeholder = "Type your message..."
	ti.Focus()
	ti.CharLimit = 0
	ti.Width = 80

	// Create text input for command mode
	cmd := textinput.New()
	cmd.Placeholder = "command"
	cmd.CharLimit = 0
	cmd.Width = 80
	cmd.Prompt = ":"

	// Create viewport for output
	vp := viewport.New(80, 20)
	vp.SetContent("")

	// Create markdown renderer
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	return Model{
		mode:           ModeNormal,
		agentName:      "opencode",
		modelName:      modelName,
		state:          "idle",
		viewport:       vp,
		input:          ti,
		commandLine:    cmd,
		agentManager:   agentManager,
		sessionManager: sessionManager,
		bus:            bus,
		renderer:       renderer,
		width:          80,
		height:         24,
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update viewport size (leave room for header, input, and help)
		headerHeight := 3
		inputHeight := 3
		helpHeight := 1
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - headerHeight - inputHeight - helpHeight - 4

		// Update input widths
		m.input.Width = msg.Width - 6
		m.commandLine.Width = msg.Width - 6

		// Re-render content
		m.updateViewportContent()

	case tea.KeyMsg:
		switch m.mode {
		case ModeNormal:
			return m.handleNormalMode(msg)
		case ModeCommand:
			return m.handleCommandMode(msg)
		case ModeModelSelect:
			return m.handleModelSelectMode(msg)
		case ModeTemplateSelect:
			return m.handleTemplateSelectMode(msg)
		case ModePipelineSelect:
			return m.handlePipelineSelectMode(msg)
		}

	case OutputMsg:
		m.rawOutput += string(msg)
		m.updateViewportContent()
		m.viewport.GotoBottom()

	case StateMsg:
		m.state = string(msg)

	case TokenUsageMsg:
		m.tokenUsage = agent.TokenUsage(msg)

	case PipelineStatusMsg:
		m.pipelineStatus = msg.Status
		m.pipelineStages = msg.Stages
		m.currentStageIndex = msg.CurrentIdx
		if msg.StageName != "" {
			m.rawOutput += fmt.Sprintf("\n**[Pipeline Stage: %s]**\n", msg.StageName)
			m.updateViewportContent()
		}

	case StageProgressMsg:
		statusIcon := "..."
		switch msg.Status {
		case "completed":
			statusIcon = "[ok]"
		case "failed":
			statusIcon = "[X]"
		case "running":
			statusIcon = "[>]"
		case "pending":
			statusIcon = "[ ]"
		}
		m.rawOutput += fmt.Sprintf("\n%s Stage %d/%d: **%s** %s\n", statusIcon, msg.StageIndex+1, msg.Total, msg.StageName, msg.Status)
		if msg.Output != "" && msg.Status == "completed" {
			m.rawOutput += fmt.Sprintf("Output preview: %.100s...\n", msg.Output)
		}
		m.updateViewportContent()
		m.viewport.GotoBottom()
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateViewportContent() {
	// Try to render as markdown, fall back to plain text
	content := m.rawOutput
	if m.renderer != nil && content != "" {
		if rendered, err := m.renderer.Render(content); err == nil {
			content = rendered
		}
	}
	m.viewport.SetContent(content)
}

func (m Model) handleNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit

	case tea.KeyEsc:
		if m.pendingPipeline != "" {
			m.rawOutput += fmt.Sprintf("\n*Cancelled pipeline: %s*\n", m.pendingPipeline)
			m.pendingPipeline = ""
			m.input.Placeholder = "Type your message..."
			m.updateViewportContent()
		} else if m.pendingTemplate != "" {
			m.rawOutput += fmt.Sprintf("\n*Cancelled template: %s*\n", m.pendingTemplate)
			m.pendingTemplate = ""
			m.input.Placeholder = "Type your message..."
			m.updateViewportContent()
		}
		return m, nil

	case tea.KeyEnter:
		value := m.input.Value()
		if value != "" {
			if m.pendingPipeline != "" {
				// Run pipeline
				m.rawOutput += fmt.Sprintf("\n**Running pipeline %s** with input: %s\n", m.pendingPipeline, value)
				m.updateViewportContent()

				if loader, ok := m.agentManager.(interface{ LoadPipelines() error }); ok {
					loader.LoadPipelines()
				}
				if am, ok := m.agentManager.(interface{ RunPipeline(string, string) error }); ok {
					if err := am.RunPipeline(m.pendingPipeline, value); err != nil {
						m.rawOutput += fmt.Sprintf("\n**Error:** %v\n", err)
						m.updateViewportContent()
					}
				}
				m.pendingPipeline = ""
				m.input.Placeholder = "Type your message..."
			} else if m.pendingTemplate != "" {
				// Spawn from template
				m.rawOutput += fmt.Sprintf("\n**Spawning %s** with task: %s\n", m.pendingTemplate, value)
				m.updateViewportContent()

				if loader, ok := m.agentManager.(interface{ LoadTemplates() error }); ok {
					loader.LoadTemplates()
				}
				if am, ok := m.agentManager.(interface{ SpawnFromTemplate(string, string) error }); ok {
					if err := am.SpawnFromTemplate(m.pendingTemplate, value); err != nil {
						m.rawOutput += fmt.Sprintf("\n**Error:** %v\n", err)
						m.updateViewportContent()
					}
				}
				m.pendingTemplate = ""
				m.input.Placeholder = "Type your message..."
			} else {
				// Normal message
				m.rawOutput += fmt.Sprintf("\n**You:** %s\n\n", value)
				m.updateViewportContent()

				if sm, ok := m.sessionManager.(interface{ StartSession(string, string) }); ok {
					sm.StartSession(m.agentName, value)
				}
				if am, ok := m.agentManager.(interface{ Start(string) error }); ok {
					am.Start(value)
				}
			}
			m.input.SetValue("")
		}
		return m, nil

	case tea.KeyRunes:
		if len(msg.Runes) == 1 && msg.Runes[0] == ':' && m.input.Value() == "" {
			m.mode = ModeCommand
			m.commandLine.SetValue("")
			m.commandLine.Focus()
			m.suggestions = nil
			m.showSuggestion = false
			return m, nil
		}
	}

	// Update text input
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	// Handle viewport scrolling with mouse or page keys
	switch msg.Type {
	case tea.KeyPgUp:
		m.viewport.LineUp(10)
	case tea.KeyPgDown:
		m.viewport.LineDown(10)
	}

	return m, cmd
}

func (m Model) handleCommandMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = ModeNormal
		m.input.Focus()
		m.suggestions = nil
		m.showSuggestion = false
		return m, nil

	case tea.KeyEnter:
		cmdStr := strings.TrimSpace(m.commandLine.Value())
		m.mode = ModeNormal
		m.input.Focus()
		m.suggestions = nil
		m.showSuggestion = false

		if cmdStr != "" {
			m = m.executeCommand(cmdStr)
		}
		return m, nil

	case tea.KeyTab:
		// Accept suggestion
		if m.showSuggestion && len(m.suggestions) > 0 {
			m.commandLine.SetValue(m.suggestions[m.suggestionIdx])
			m.commandLine.CursorEnd()
		}
		return m, nil

	case tea.KeyUp:
		if m.showSuggestion && m.suggestionIdx > 0 {
			m.suggestionIdx--
			// Scroll up if cursor goes above visible area
			if m.suggestionIdx < m.suggestionScroll {
				m.suggestionScroll = m.suggestionIdx
			}
		}
		return m, nil

	case tea.KeyDown:
		if m.showSuggestion && m.suggestionIdx < len(m.suggestions)-1 {
			m.suggestionIdx++
			// Scroll down if cursor goes below visible area
			maxVisible := 8
			if m.suggestionIdx >= m.suggestionScroll+maxVisible {
				m.suggestionScroll = m.suggestionIdx - maxVisible + 1
			}
		}
		return m, nil
	}

	// Update command input
	var cmd tea.Cmd
	m.commandLine, cmd = m.commandLine.Update(msg)

	// Update autocomplete suggestions
	m.updateSuggestions()

	return m, cmd
}

func (m *Model) updateSuggestions() {
	input := m.commandLine.Value()
	if input == "" {
		m.suggestions = commands
		m.showSuggestion = true
		m.suggestionIdx = 0
		m.suggestionScroll = 0
		return
	}

	// Filter commands
	parts := strings.Fields(input)
	if len(parts) == 0 {
		m.suggestions = commands
		m.showSuggestion = true
		m.suggestionIdx = 0
		m.suggestionScroll = 0
		return
	}

	prefix := parts[0]
	var matches []string
	for _, cmd := range commands {
		if strings.HasPrefix(cmd, prefix) {
			matches = append(matches, cmd)
		}
	}

	sort.Strings(matches)
	m.suggestions = matches
	m.showSuggestion = len(matches) > 0
	if m.suggestionIdx >= len(matches) {
		m.suggestionIdx = 0
		m.suggestionScroll = 0
	}
}

func (m Model) handleModelSelectMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc, tea.KeyRunes:
		if msg.Type == tea.KeyEsc || (len(msg.Runes) == 1 && msg.Runes[0] == 'q') {
			m.mode = ModeNormal
			m.input.Focus()
			return m, nil
		}
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'j':
				if m.modelCursor < len(m.availableModels)-1 {
					m.modelCursor++
				}
			case 'k':
				if m.modelCursor > 0 {
					m.modelCursor--
				}
			}
		}

	case tea.KeyUp:
		if m.modelCursor > 0 {
			m.modelCursor--
		}

	case tea.KeyDown:
		if m.modelCursor < len(m.availableModels)-1 {
			m.modelCursor++
		}

	case tea.KeyEnter:
		if m.modelCursor < len(m.availableModels) {
			m.modelName = m.availableModels[m.modelCursor]
			if am, ok := m.agentManager.(interface{ SetModel(string) }); ok {
				am.SetModel(m.modelName)
			}
			m.rawOutput += fmt.Sprintf("\n**Model set to:** `%s`\n", m.modelName)
			m.updateViewportContent()
		}
		m.mode = ModeNormal
		m.input.Focus()

	case tea.KeyCtrlC:
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) handleTemplateSelectMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc, tea.KeyRunes:
		if msg.Type == tea.KeyEsc || (len(msg.Runes) == 1 && msg.Runes[0] == 'q') {
			m.mode = ModeNormal
			m.input.Focus()
			return m, nil
		}
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'j':
				if m.templateCursor < len(m.availableTemplates)-1 {
					m.templateCursor++
				}
			case 'k':
				if m.templateCursor > 0 {
					m.templateCursor--
				}
			}
		}

	case tea.KeyUp:
		if m.templateCursor > 0 {
			m.templateCursor--
		}

	case tea.KeyDown:
		if m.templateCursor < len(m.availableTemplates)-1 {
			m.templateCursor++
		}

	case tea.KeyEnter:
		if m.templateCursor < len(m.availableTemplates) {
			m.pendingTemplate = m.availableTemplates[m.templateCursor]
			m.input.Placeholder = fmt.Sprintf("Enter task for [%s]...", m.pendingTemplate)
			m.rawOutput += fmt.Sprintf("\n**Template selected:** `%s`\nEnter your task below.\n", m.pendingTemplate)
			m.updateViewportContent()
		}
		m.mode = ModeNormal
		m.input.Focus()

	case tea.KeyCtrlC:
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) handlePipelineSelectMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc, tea.KeyRunes:
		if msg.Type == tea.KeyEsc || (len(msg.Runes) == 1 && msg.Runes[0] == 'q') {
			m.mode = ModeNormal
			m.input.Focus()
			return m, nil
		}
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'j':
				if m.pipelineCursor < len(m.availablePipelines)-1 {
					m.pipelineCursor++
				}
			case 'k':
				if m.pipelineCursor > 0 {
					m.pipelineCursor--
				}
			}
		}

	case tea.KeyUp:
		if m.pipelineCursor > 0 {
			m.pipelineCursor--
		}

	case tea.KeyDown:
		if m.pipelineCursor < len(m.availablePipelines)-1 {
			m.pipelineCursor++
		}

	case tea.KeyEnter:
		if m.pipelineCursor < len(m.availablePipelines) {
			m.pendingPipeline = m.availablePipelines[m.pipelineCursor]
			m.input.Placeholder = fmt.Sprintf("Enter input for pipeline [%s]...", m.pendingPipeline)
			m.rawOutput += fmt.Sprintf("\n**Pipeline selected:** `%s`\nEnter your input below.\n", m.pendingPipeline)
			m.updateViewportContent()
		}
		m.mode = ModeNormal
		m.input.Focus()

	case tea.KeyCtrlC:
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) executeCommand(cmdStr string) Model {
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return m
	}

	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "start":
		if len(args) > 0 {
			input := strings.Join(args, " ")
			m.rawOutput += fmt.Sprintf("\n**Starting agent with:** %s\n\n", input)
			m.updateViewportContent()

			if sm, ok := m.sessionManager.(interface{ StartSession(string, string) }); ok {
				sm.StartSession(m.agentName, input)
			}
			if am, ok := m.agentManager.(interface{ Start(string) error }); ok {
				am.Start(input)
			}
		} else {
			m.rawOutput += "\n**Error:** `start` requires a prompt\n"
			m.updateViewportContent()
		}

	case "stop":
		if am, ok := m.agentManager.(interface{ Stop() }); ok {
			am.Stop()
		}
		m.rawOutput += "\n**Agent stopped**\n"
		m.updateViewportContent()

	case "pause":
		if am, ok := m.agentManager.(interface{ Pause() }); ok {
			am.Pause()
		}
		m.rawOutput += "\n**Agent paused**\n"
		m.updateViewportContent()

	case "resume":
		if am, ok := m.agentManager.(interface{ Resume() }); ok {
			am.Resume()
		}
		m.rawOutput += "\n**Agent resumed**\n"
		m.updateViewportContent()

	case "models":
		execCmd := exec.Command("opencode", "models")
		output, err := execCmd.Output()
		if err != nil {
			m.rawOutput += fmt.Sprintf("\n**Error listing models:** %v\n", err)
			m.updateViewportContent()
		} else {
			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			m.availableModels = make([]string, 0, len(lines))
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" {
					m.availableModels = append(m.availableModels, line)
				}
			}
			m.modelCursor = 0
			for i, model := range m.availableModels {
				if model == m.modelName {
					m.modelCursor = i
					break
				}
			}
			m.mode = ModeModelSelect
		}

	case "model":
		if len(args) > 0 {
			m.modelName = args[0]
			if am, ok := m.agentManager.(interface{ SetModel(string) }); ok {
				am.SetModel(m.modelName)
			}
			m.rawOutput += fmt.Sprintf("\n**Model set to:** `%s`\n", m.modelName)
		} else {
			m.rawOutput += fmt.Sprintf("\n**Current model:** `%s`\n", m.modelName)
			m.rawOutput += "Usage: `:model <provider/model-name>`\n"
		}
		m.updateViewportContent()

	case "templates":
		if am, ok := m.agentManager.(interface{ LoadTemplates() error }); ok {
			if err := am.LoadTemplates(); err != nil {
				m.rawOutput += fmt.Sprintf("\n**Error loading templates:** %v\n", err)
				m.updateViewportContent()
				return m
			}
		}

		if am, ok := m.agentManager.(interface{ ListTemplates() []string }); ok {
			m.availableTemplates = am.ListTemplates()
			if len(m.availableTemplates) == 0 {
				m.rawOutput += "\n**No templates found** in `templates/` directory\n"
				m.updateViewportContent()
				return m
			}
			m.templateCursor = 0
			m.mode = ModeTemplateSelect
		}

	case "spawn-template":
		if len(args) > 0 {
			templateName := args[0]
			input := ""
			if len(args) > 1 {
				input = strings.Join(args[1:], " ")
			}

			if input == "" {
				m.pendingTemplate = templateName
				m.input.Placeholder = fmt.Sprintf("Enter task for [%s]...", templateName)
				m.rawOutput += fmt.Sprintf("\n**Template:** `%s`\nEnter your task below.\n", templateName)
			} else {
				m.rawOutput += fmt.Sprintf("\n**Spawning %s** with: %s\n", templateName, input)
				if loader, ok := m.agentManager.(interface{ LoadTemplates() error }); ok {
					loader.LoadTemplates()
				}
				if am, ok := m.agentManager.(interface{ SpawnFromTemplate(string, string) error }); ok {
					if err := am.SpawnFromTemplate(templateName, input); err != nil {
						m.rawOutput += fmt.Sprintf("\n**Error:** %v\n", err)
					}
				}
			}
			m.updateViewportContent()
		} else {
			m.rawOutput += "\n**Usage:** `:spawn-template <name> [task]`\n"
			m.updateViewportContent()
		}

	case "pipelines":
		if am, ok := m.agentManager.(interface{ LoadPipelines() error }); ok {
			if err := am.LoadPipelines(); err != nil {
				m.rawOutput += fmt.Sprintf("\n**Error loading pipelines:** %v\n", err)
				m.updateViewportContent()
				return m
			}
		} else {
			m.rawOutput += "\n**Error:** Pipeline support not available\n"
			m.updateViewportContent()
			return m
		}

		if am, ok := m.agentManager.(interface{ ListPipelines() []string }); ok {
			m.availablePipelines = am.ListPipelines()
			if len(m.availablePipelines) == 0 {
				m.rawOutput += "\n**No pipelines found** in `pipelines/` directory\n"
				m.rawOutput += "Create a YAML file in `pipelines/` to define a pipeline.\n"
				m.updateViewportContent()
				return m
			}
			m.pipelineCursor = 0
			m.mode = ModePipelineSelect
		} else {
			m.rawOutput += "\n**Error:** Pipeline listing not available\n"
			m.updateViewportContent()
			return m
		}

	case "run-pipeline":
		if len(args) > 0 {
			pipelineName := args[0]
			input := ""
			if len(args) > 1 {
				input = strings.Join(args[1:], " ")
			}

			if input == "" {
				m.pendingPipeline = pipelineName
				m.input.Placeholder = fmt.Sprintf("Enter input for pipeline [%s]...", pipelineName)
				m.rawOutput += fmt.Sprintf("\n**Pipeline:** `%s`\nEnter your input below.\n", pipelineName)
			} else {
				m.rawOutput += fmt.Sprintf("\n**Running pipeline %s** with: %s\n", pipelineName, input)
				if loader, ok := m.agentManager.(interface{ LoadPipelines() error }); ok {
					loader.LoadPipelines()
				}
				if am, ok := m.agentManager.(interface{ RunPipeline(string, string) error }); ok {
					if err := am.RunPipeline(pipelineName, input); err != nil {
						m.rawOutput += fmt.Sprintf("\n**Error:** %v\n", err)
					}
				}
			}
			m.updateViewportContent()
		} else {
			m.rawOutput += "\n**Usage:** `:run-pipeline <name> [input]`\n"
			m.updateViewportContent()
		}

	case "pipeline-status":
		if am, ok := m.agentManager.(interface{ GetPipelineStatus() string }); ok {
			status := am.GetPipelineStatus()
			m.rawOutput += fmt.Sprintf("\n**Pipeline Status:**\n%s\n", status)
			m.updateViewportContent()
		} else {
			m.rawOutput += "\n**No pipeline running**\n"
			m.updateViewportContent()
		}

	case "clear":
		m.rawOutput = ""
		m.viewport.SetContent("")

	case "help":
		m.rawOutput += `
## Commands

| Command | Description |
|---------|-------------|
| **:start** <prompt> | Start agent with prompt |
| **:stop** | Stop running agent |
| **:pause** | Pause agent |
| **:resume** | Resume paused agent |
| **:models** | List/select models |
| **:model** <name> | Set model directly |
| **:templates** | List/select templates |
| **:spawn-template** <n> | Spawn from template |
| **:pipelines** | List/select pipelines |
| **:run-pipeline** <n> | Run a pipeline |
| **:pipeline-status** | Show pipeline progress |
| **:clear** | Clear output |
| **:help** | Show this help |
| **:quit** | Exit application |

## Shortcuts

- **:** - Enter command mode
- **Tab** - Accept autocomplete
- **Esc** - Cancel current mode
- **Ctrl+C** - Quit

`
		m.updateViewportContent()
		m.viewport.GotoBottom()

	case "quit", "q":
		// This will be handled by returning tea.Quit
		return m
	}

	return m
}

func (m Model) View() string {
	if m.width < 40 || m.height < 10 {
		return "Terminal too small"
	}

	var sections []string

	// Header
	sections = append(sections, m.renderHeader())

	// Main content area
	switch m.mode {
	case ModeModelSelect:
		sections = append(sections, m.renderModelSelect())
	case ModeTemplateSelect:
		sections = append(sections, m.renderTemplateSelect())
	case ModePipelineSelect:
		sections = append(sections, m.renderPipelineSelect())
	default:
		sections = append(sections, m.renderOutput())
		sections = append(sections, m.renderInput())
	}

	// Help footer
	sections = append(sections, m.renderHelp())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) renderHeader() string {
	// Status badge
	var statusBadge string
	switch m.state {
	case "running":
		statusBadge = statusRunning.Render(" RUNNING ")
	case "paused":
		statusBadge = statusPaused.Render(" PAUSED ")
	default:
		statusBadge = statusIdle.Render(" IDLE ")
	}

	// Agent info
	agentInfo := headerStyle.Render(fmt.Sprintf(" %s ", m.agentName))

	// Model info
	modelInfo := lipgloss.NewStyle().
		Foreground(mutedColor).
		Render(fmt.Sprintf(" model: %s ", m.modelName))

	// Token info
	var tokenInfo string
	if m.tokenUsage.TotalTokens > 0 {
		tokenInfo = tokenStyle.Render(fmt.Sprintf(" tokens: %d ", m.tokenUsage.TotalTokens))
		tokenInfo += costStyle.Render(fmt.Sprintf(" $%.4f ", m.tokenUsage.CostUSD))
	}

	// Build header line
	left := lipgloss.JoinHorizontal(lipgloss.Center, agentInfo, " ", statusBadge, modelInfo)
	right := tokenInfo

	// Calculate spacing
	spacer := strings.Repeat(" ", max(0, m.width-lipgloss.Width(left)-lipgloss.Width(right)-2))

	header := lipgloss.JoinHorizontal(lipgloss.Center, left, spacer, right)

	return lipgloss.NewStyle().
		Width(m.width).
		BorderBottom(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(primaryColor).
		Render(header)
}

func (m Model) renderOutput() string {
	outputBox := borderStyle.
		Width(m.width - 2).
		Height(m.viewport.Height + 2).
		Render(m.viewport.View())

	return outputBox
}

func (m Model) renderInput() string {
	var inputBox string

	if m.mode == ModeCommand {
		// Command mode with autocomplete
		content := m.commandLine.View()

		// Show suggestions with scrolling
		if m.showSuggestion && len(m.suggestions) > 0 {
			var suggestionLines []string
			maxVisible := 8

			startIdx := m.suggestionScroll
			endIdx := min(startIdx+maxVisible, len(m.suggestions))

			// Show scroll indicator at top if scrolled
			if startIdx > 0 {
				suggestionLines = append(suggestionLines, suggestionStyle.Render(fmt.Sprintf("  ... %d above ...", startIdx)))
			}

			for i := startIdx; i < endIdx; i++ {
				s := m.suggestions[i]
				if i == m.suggestionIdx {
					suggestionLines = append(suggestionLines, selectedSuggestion.Render("> "+s))
				} else {
					suggestionLines = append(suggestionLines, suggestionStyle.Render("  "+s))
				}
			}

			// Show scroll indicator at bottom if more items
			remaining := len(m.suggestions) - endIdx
			if remaining > 0 {
				suggestionLines = append(suggestionLines, suggestionStyle.Render(fmt.Sprintf("  ... %d below ...", remaining)))
			}

			content += "\n" + strings.Join(suggestionLines, "\n")
		}

		inputBox = commandStyle.
			Width(m.width - 4).
			Render(content)
	} else {
		// Normal input
		inputBox = inputStyle.
			Width(m.width - 4).
			Render(m.input.View())
	}

	return inputBox
}

func (m Model) renderModelSelect() string {
	var content strings.Builder

	content.WriteString(titleStyle.Render("Select Model") + "\n\n")

	visibleHeight := m.height - 10
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	startIdx := max(0, m.modelCursor-visibleHeight/2)
	endIdx := min(startIdx+visibleHeight, len(m.availableModels))

	for i := startIdx; i < endIdx; i++ {
		model := m.availableModels[i]
		if i == m.modelCursor {
			line := listSelectedStyle.Render(model)
			if model == m.modelName {
				line += lipgloss.NewStyle().Foreground(secondaryColor).Render(" (current)")
			}
			content.WriteString(line + "\n")
		} else {
			line := listItemStyle.Render(model)
			if model == m.modelName {
				line += lipgloss.NewStyle().Foreground(secondaryColor).Render(" (current)")
			}
			content.WriteString(line + "\n")
		}
	}

	content.WriteString(fmt.Sprintf("\n%s", helpStyle.Render(fmt.Sprintf("[%d/%d] j/k:navigate Enter:select q/Esc:cancel", m.modelCursor+1, len(m.availableModels)))))

	return borderStyle.
		Width(m.width - 2).
		Height(m.height - 6).
		Render(content.String())
}

func (m Model) renderTemplateSelect() string {
	var content strings.Builder

	content.WriteString(titleStyle.Render("Select Template") + "\n\n")

	visibleHeight := m.height - 10
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	startIdx := max(0, m.templateCursor-visibleHeight/2)
	endIdx := min(startIdx+visibleHeight, len(m.availableTemplates))

	for i := startIdx; i < endIdx; i++ {
		template := m.availableTemplates[i]
		if i == m.templateCursor {
			content.WriteString(listSelectedStyle.Render(template) + "\n")
		} else {
			content.WriteString(listItemStyle.Render(template) + "\n")
		}
	}

	content.WriteString(fmt.Sprintf("\n%s", helpStyle.Render(fmt.Sprintf("[%d/%d] j/k:navigate Enter:select q/Esc:cancel", m.templateCursor+1, len(m.availableTemplates)))))

	return borderStyle.
		Width(m.width - 2).
		Height(m.height - 6).
		Render(content.String())
}

func (m Model) renderPipelineSelect() string {
	var content strings.Builder

	content.WriteString(titleStyle.Render("Select Pipeline") + "\n\n")

	visibleHeight := m.height - 10
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	startIdx := max(0, m.pipelineCursor-visibleHeight/2)
	endIdx := min(startIdx+visibleHeight, len(m.availablePipelines))

	for i := startIdx; i < endIdx; i++ {
		pipeline := m.availablePipelines[i]
		if i == m.pipelineCursor {
			content.WriteString(listSelectedStyle.Render(pipeline) + "\n")
		} else {
			content.WriteString(listItemStyle.Render(pipeline) + "\n")
		}
	}

	content.WriteString(fmt.Sprintf("\n%s", helpStyle.Render(fmt.Sprintf("[%d/%d] j/k:navigate Enter:select q/Esc:cancel", m.pipelineCursor+1, len(m.availablePipelines)))))

	return borderStyle.
		Width(m.width - 2).
		Height(m.height - 6).
		Render(content.String())
}

func (m Model) renderHelp() string {
	var help string
	switch m.mode {
	case ModeCommand:
		help = "Tab:complete | Up/Down:suggestions | Enter:execute | Esc:cancel"
	case ModeModelSelect, ModeTemplateSelect, ModePipelineSelect:
		help = "j/k:navigate | Enter:select | q/Esc:cancel"
	default:
		if m.pendingPipeline != "" {
			help = fmt.Sprintf("Enter input for pipeline [%s] | Esc:cancel", m.pendingPipeline)
		} else if m.pendingTemplate != "" {
			help = fmt.Sprintf("Enter task for [%s] | Esc:cancel", m.pendingTemplate)
		} else {
			help = ":command | Enter:send | Ctrl+C:quit"
		}
	}

	return helpStyle.
		Width(m.width).
		Align(lipgloss.Center).
		Render(help)
}

func (m Model) GetCommand() string {
	return m.commandLine.Value()
}
