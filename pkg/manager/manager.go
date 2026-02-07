package manager

import (
	"context"
	"fmt"
	"sync"

	"github.com/rasmuselmersson/opencode/pkg/agent"
	"github.com/rasmuselmersson/opencode/pkg/events"
	"github.com/rasmuselmersson/opencode/pkg/pipeline"
	"github.com/rasmuselmersson/opencode/pkg/template"
)

type State string

const (
	StateIdle    State = "idle"
	StateRunning State = "running"
	StatePaused  State = "paused"
)

type Manager struct {
	agent          agent.Agent
	bus            events.Bus
	state          State
	stateMutex     sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	paused         bool
	pauseCond      *sync.Cond
	templateLoader *template.Loader
	pipelineLoader *pipeline.PipelineLoader
	pipelineRunner *pipeline.Runner
	pipelineStore  *pipeline.ExecutionStore
}

// TemplateAgentFactory implements pipeline.AgentFactory using templates
type TemplateAgentFactory struct {
	loader    *template.Loader
	baseAgent agent.Agent
}

func (f *TemplateAgentFactory) CreateAgent(templateName string) (agent.Agent, error) {
	tmpl, err := f.loader.GetTemplate(templateName)
	if err != nil {
		return nil, fmt.Errorf("template not found: %s", templateName)
	}

	// Create a template-aware agent wrapper
	return &TemplateAgent{
		baseAgent: f.baseAgent,
		template:  tmpl,
	}, nil
}

// TemplateAgent wraps an agent with template configuration
type TemplateAgent struct {
	baseAgent agent.Agent
	template  *template.Template
}

func (a *TemplateAgent) Name() string {
	return a.template.Name
}

func (a *TemplateAgent) Execute(ctx context.Context, input string) (<-chan string, <-chan error) {
	// Apply template system prompt
	fullInput := a.template.System + "\n\n" + input

	// Set model if supported
	if setter, ok := a.baseAgent.(interface{ SetModel(string) }); ok {
		setter.SetModel(a.template.Model)
	}

	return a.baseAgent.Execute(ctx, fullInput)
}

func (a *TemplateAgent) GetTokenUsageChan() <-chan agent.TokenUsage {
	if trackingAgent, ok := a.baseAgent.(agent.TokenTrackingAgent); ok {
		return trackingAgent.GetTokenUsageChan()
	}
	ch := make(chan agent.TokenUsage)
	close(ch)
	return ch
}

func NewManager(agentInstance agent.Agent, bus events.Bus) *Manager {
	templateLoader := template.NewLoader("templates")
	pipelineLoader := pipeline.NewPipelineLoader("pipelines")

	// Create execution store
	pipelineStore, _ := pipeline.NewExecutionStore("pipeline-executions")

	// Create agent factory
	factory := &TemplateAgentFactory{
		loader:    templateLoader,
		baseAgent: agentInstance,
	}

	// Create pipeline runner
	pipelineRunner := pipeline.NewRunner(bus, factory, pipelineStore)

	return &Manager{
		agent:          agentInstance,
		bus:            bus,
		state:          StateIdle,
		pauseCond:      sync.NewCond(&sync.Mutex{}),
		templateLoader: templateLoader,
		pipelineLoader: pipelineLoader,
		pipelineRunner: pipelineRunner,
		pipelineStore:  pipelineStore,
	}
}

func (m *Manager) Start(input string) error {
	m.stateMutex.Lock()
	defer m.stateMutex.Unlock()

	if m.state != StateIdle {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.ctx = ctx
	m.cancel = cancel
	m.state = StateRunning

	m.bus.Publish(events.Event{
		Type:      events.EventAgentStarted,
		AgentName: m.agent.Name(),
	})

	go m.run(input)

	return nil
}

func (m *Manager) run(input string) {
	outputCh, errCh := m.agent.Execute(m.ctx, input)

	// Check if agent supports token tracking
	var tokenCh <-chan agent.TokenUsage
	if trackingAgent, ok := m.agent.(agent.TokenTrackingAgent); ok {
		tokenCh = trackingAgent.GetTokenUsageChan()
	}

	for {
		m.stateMutex.RLock()
		paused := m.paused
		m.stateMutex.RUnlock()

		if paused {
			m.pauseCond.L.Lock()
			m.pauseCond.Wait()
			m.pauseCond.L.Unlock()
			continue
		}

		select {
		case <-m.ctx.Done():
			m.stateMutex.Lock()
			m.state = StateIdle
			m.stateMutex.Unlock()
			m.bus.Publish(events.Event{
				Type:      events.EventAgentStopped,
				AgentName: m.agent.Name(),
			})
			return

		case chunk, ok := <-outputCh:
			if !ok {
				m.stateMutex.Lock()
				m.state = StateIdle
				m.stateMutex.Unlock()
				m.bus.Publish(events.Event{
					Type:      events.EventAgentStopped,
					AgentName: m.agent.Name(),
				})
				return
			}

			m.bus.Publish(events.Event{
				Type:      events.EventOutputChunk,
				AgentName: m.agent.Name(),
				Data:      chunk,
			})

		case usage, ok := <-tokenCh:
			if ok {
				m.bus.Publish(events.Event{
					Type:      events.EventTokensUpdated,
					AgentName: m.agent.Name(),
					Data:      usage,
				})
			}

		case err, ok := <-errCh:
			if ok && err != nil {
				m.bus.Publish(events.Event{
					Type:      events.EventError,
					AgentName: m.agent.Name(),
					Data:      err.Error(),
				})
			}
		}
	}
}

func (m *Manager) Stop() {
	m.stateMutex.Lock()
	defer m.stateMutex.Unlock()

	if m.state == StateIdle {
		return
	}

	if m.cancel != nil {
		m.cancel()
	}
}

func (m *Manager) Pause() {
	m.stateMutex.Lock()
	defer m.stateMutex.Unlock()

	if m.state != StateRunning {
		return
	}

	m.paused = true
	m.state = StatePaused

	m.bus.Publish(events.Event{
		Type:      events.EventAgentPaused,
		AgentName: m.agent.Name(),
	})
}

func (m *Manager) Resume() {
	m.stateMutex.Lock()
	defer m.stateMutex.Unlock()

	if m.state != StatePaused {
		return
	}

	m.paused = false
	m.state = StateRunning
	m.pauseCond.Signal()

	m.bus.Publish(events.Event{
		Type:      events.EventAgentResumed,
		AgentName: m.agent.Name(),
	})
}

func (m *Manager) GetState() State {
	m.stateMutex.RLock()
	defer m.stateMutex.RUnlock()
	return m.state
}

// SetModel sets the model on the underlying agent if it supports it
func (m *Manager) SetModel(model string) {
	if setter, ok := m.agent.(interface{ SetModel(string) }); ok {
		setter.SetModel(model)
	}
}

// LoadTemplates loads all available templates
func (m *Manager) LoadTemplates() error {
	return m.templateLoader.Load()
}

// ListTemplates returns all available template names
func (m *Manager) ListTemplates() []string {
	return m.templateLoader.ListTemplates()
}

// GetTemplateDetails returns details for all templates
func (m *Manager) GetTemplateDetails() []*template.Template {
	return m.templateLoader.GetTemplateDetails()
}

// SpawnFromTemplate creates and starts an agent from a template
func (m *Manager) SpawnFromTemplate(templateName, input string) error {
	tmpl, err := m.templateLoader.GetTemplate(templateName)
	if err != nil {
		return err
	}

	// Set model from template
	if setter, ok := m.agent.(interface{ SetModel(string) }); ok {
		setter.SetModel(tmpl.Model)
	}

	// Start the agent with the system prompt from template
	fullInput := tmpl.System + "\n\n" + input
	return m.Start(fullInput)
}

// LoadPipelines loads all available pipelines
func (m *Manager) LoadPipelines() error {
	return m.pipelineLoader.Load()
}

// ListPipelines returns all available pipeline names
func (m *Manager) ListPipelines() []string {
	return m.pipelineLoader.ListPipelines()
}

// GetPipelineDetails returns details for all pipelines
func (m *Manager) GetPipelineDetails() []*pipeline.Pipeline {
	return m.pipelineLoader.GetPipelineDetails()
}

// RunPipeline executes a pipeline with the given input
func (m *Manager) RunPipeline(pipelineName, input string) error {
	// Check if already running
	m.stateMutex.Lock()
	if m.state == StateRunning {
		m.stateMutex.Unlock()
		return fmt.Errorf("a pipeline or agent is already running")
	}
	m.state = StateRunning
	m.stateMutex.Unlock()

	// Load templates first (needed for pipeline stages)
	if err := m.templateLoader.Load(); err != nil {
		m.stateMutex.Lock()
		m.state = StateIdle
		m.stateMutex.Unlock()
		return fmt.Errorf("failed to load templates: %w", err)
	}

	// Get the pipeline
	p, err := m.pipelineLoader.GetPipeline(pipelineName)
	if err != nil {
		m.stateMutex.Lock()
		m.state = StateIdle
		m.stateMutex.Unlock()
		return err
	}

	// Execute pipeline in background
	go func() {
		ctx := context.Background()
		_, err := m.pipelineRunner.Execute(ctx, p, input)

		m.stateMutex.Lock()
		m.state = StateIdle
		m.stateMutex.Unlock()

		if err != nil {
			m.bus.Publish(events.Event{
				Type: events.EventError,
				Data: err.Error(),
			})
		}
	}()

	return nil
}

// GetPipelineStatus returns the current pipeline execution status
func (m *Manager) GetPipelineStatus() string {
	exec := m.pipelineRunner.GetCurrentExecution()
	if exec == nil {
		return "No pipeline running"
	}

	status := fmt.Sprintf("Pipeline: %s\nStatus: %s\n", exec.Pipeline.Name, exec.Status)
	for i, stage := range exec.Stages {
		icon := "[ ]"
		switch stage.Status {
		case pipeline.StageStatusCompleted:
			icon = "[ok]"
		case pipeline.StageStatusRunning:
			icon = "[>]"
		case pipeline.StageStatusFailed:
			icon = "[X]"
		case pipeline.StageStatusSkipped:
			icon = "[-]"
		}
		status += fmt.Sprintf("  %s %d. %s (%s)\n", icon, i+1, stage.Stage.Name, stage.Status)
	}
	return status
}

// CancelPipeline stops the currently running pipeline
func (m *Manager) CancelPipeline() {
	m.pipelineRunner.Cancel()
}
