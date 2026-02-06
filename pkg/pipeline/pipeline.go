package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rasmuselmersson/opencode/pkg/agent"
	"github.com/rasmuselmersson/opencode/pkg/events"
	"gopkg.in/yaml.v3"
)

// StageStatus represents the current state of a pipeline stage
type StageStatus string

const (
	StageStatusPending   StageStatus = "pending"
	StageStatusRunning   StageStatus = "running"
	StageStatusCompleted StageStatus = "completed"
	StageStatusFailed    StageStatus = "failed"
	StageStatusSkipped   StageStatus = "skipped"
)

// PipelineStatus represents the current state of a pipeline
type PipelineStatus string

const (
	PipelineStatusPending   PipelineStatus = "pending"
	PipelineStatusRunning   PipelineStatus = "running"
	PipelineStatusCompleted PipelineStatus = "completed"
	PipelineStatusFailed    PipelineStatus = "failed"
	PipelineStatusCancelled PipelineStatus = "cancelled"
)

// Stage represents a single stage in a pipeline
type Stage struct {
	Name        string `yaml:"name" json:"name"`
	Template    string `yaml:"template" json:"template"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	// ContinueOnError allows the pipeline to continue even if this stage fails
	ContinueOnError bool `yaml:"continue_on_error,omitempty" json:"continue_on_error,omitempty"`
}

// Pipeline defines a sequence of agents to execute
type Pipeline struct {
	Name        string  `yaml:"name" json:"name"`
	Description string  `yaml:"description,omitempty" json:"description,omitempty"`
	Stages      []Stage `yaml:"stages" json:"stages"`
	// Metadata for categorization
	Metadata map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

// StageExecution tracks the execution of a single stage
type StageExecution struct {
	Stage     Stage            `json:"stage"`
	Status    StageStatus      `json:"status"`
	Input     string           `json:"input"`
	Output    string           `json:"output"`
	Error     string           `json:"error,omitempty"`
	StartTime *time.Time       `json:"start_time,omitempty"`
	EndTime   *time.Time       `json:"end_time,omitempty"`
	Tokens    agent.TokenUsage `json:"tokens"`
}

// PipelineExecution tracks the execution of a complete pipeline
type PipelineExecution struct {
	ID           string           `json:"id"`
	Pipeline     Pipeline         `json:"pipeline"`
	Status       PipelineStatus   `json:"status"`
	Stages       []StageExecution `json:"stages"`
	InitialInput string           `json:"initial_input"`
	FinalOutput  string           `json:"final_output,omitempty"`
	StartTime    time.Time        `json:"start_time"`
	EndTime      *time.Time       `json:"end_time,omitempty"`
	TotalTokens  agent.TokenUsage `json:"total_tokens"`
	FailedStage  int              `json:"failed_stage,omitempty"`
	ErrorMessage string           `json:"error_message,omitempty"`
}

// AgentMessage represents a message passed between agents in a pipeline
type AgentMessage struct {
	From      string            `json:"from"`
	To        string            `json:"to"`
	Content   string            `json:"content"`
	Timestamp time.Time         `json:"timestamp"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// MessageBus enables agent-to-agent communication
type MessageBus struct {
	messages []AgentMessage
	mu       sync.RWMutex
}

// NewMessageBus creates a new message bus for agent communication
func NewMessageBus() *MessageBus {
	return &MessageBus{
		messages: make([]AgentMessage, 0),
	}
}

// Send adds a message to the bus
func (mb *MessageBus) Send(msg AgentMessage) {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	msg.Timestamp = time.Now()
	mb.messages = append(mb.messages, msg)
}

// GetMessages returns all messages between two agents
func (mb *MessageBus) GetMessages(from, to string) []AgentMessage {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	var result []AgentMessage
	for _, msg := range mb.messages {
		if msg.From == from && msg.To == to {
			result = append(result, msg)
		}
	}
	return result
}

// GetAllMessages returns all messages in the bus
func (mb *MessageBus) GetAllMessages() []AgentMessage {
	mb.mu.RLock()
	defer mb.mu.RUnlock()
	return append([]AgentMessage(nil), mb.messages...)
}

// GetLastOutput returns the most recent message output
func (mb *MessageBus) GetLastOutput() string {
	mb.mu.RLock()
	defer mb.mu.RUnlock()
	if len(mb.messages) == 0 {
		return ""
	}
	return mb.messages[len(mb.messages)-1].Content
}

// PipelineLoader loads pipeline definitions from YAML files
type PipelineLoader struct {
	pipelinesDir string
	pipelines    map[string]*Pipeline
	mu           sync.RWMutex
}

// NewPipelineLoader creates a new pipeline loader
func NewPipelineLoader(dir string) *PipelineLoader {
	return &PipelineLoader{
		pipelinesDir: dir,
		pipelines:    make(map[string]*Pipeline),
	}
}

// Load loads all pipelines from the pipelines directory
func (l *PipelineLoader) Load() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.pipelines = make(map[string]*Pipeline)

	entries, err := os.ReadDir(l.pipelinesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read pipelines directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		filePath := filepath.Join(l.pipelinesDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read pipeline %s: %w", entry.Name(), err)
		}

		var pipeline Pipeline
		if err := yaml.Unmarshal(data, &pipeline); err != nil {
			return fmt.Errorf("failed to parse pipeline %s: %w", entry.Name(), err)
		}

		l.pipelines[pipeline.Name] = &pipeline
	}

	return nil
}

// GetPipeline returns a pipeline by name
func (l *PipelineLoader) GetPipeline(name string) (*Pipeline, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	pipeline, ok := l.pipelines[name]
	if !ok {
		return nil, fmt.Errorf("pipeline not found: %s", name)
	}
	return pipeline, nil
}

// ListPipelines returns all available pipeline names
func (l *PipelineLoader) ListPipelines() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	names := make([]string, 0, len(l.pipelines))
	for name := range l.pipelines {
		names = append(names, name)
	}
	return names
}

// GetPipelineDetails returns all pipeline definitions
func (l *PipelineLoader) GetPipelineDetails() []*Pipeline {
	l.mu.RLock()
	defer l.mu.RUnlock()

	pipelines := make([]*Pipeline, 0, len(l.pipelines))
	for _, p := range l.pipelines {
		pipelines = append(pipelines, p)
	}
	return pipelines
}

// ExecutionStore handles persistence of pipeline executions
type ExecutionStore struct {
	storageDir string
}

// NewExecutionStore creates a new execution store
func NewExecutionStore(dir string) (*ExecutionStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &ExecutionStore{storageDir: dir}, nil
}

// Save persists a pipeline execution to disk
func (s *ExecutionStore) Save(exec *PipelineExecution) error {
	filePath := filepath.Join(s.storageDir, exec.ID+".json")
	data, err := json.MarshalIndent(exec, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal execution: %w", err)
	}
	return os.WriteFile(filePath, data, 0644)
}

// Load retrieves a pipeline execution from disk
func (s *ExecutionStore) Load(id string) (*PipelineExecution, error) {
	filePath := filepath.Join(s.storageDir, id+".json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var exec PipelineExecution
	if err := json.Unmarshal(data, &exec); err != nil {
		return nil, err
	}
	return &exec, nil
}

// List returns all stored execution IDs
func (s *ExecutionStore) List() ([]string, error) {
	entries, err := os.ReadDir(s.storageDir)
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) == ".json" {
			ids = append(ids, entry.Name()[:len(entry.Name())-5])
		}
	}
	return ids, nil
}

// AgentFactory creates agents for pipeline stages
type AgentFactory interface {
	CreateAgent(templateName string) (agent.Agent, error)
}

// Runner executes pipelines deterministically
type Runner struct {
	bus        events.Bus
	factory    AgentFactory
	store      *ExecutionStore
	messageBus *MessageBus
	current    *PipelineExecution
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewRunner creates a new pipeline runner
func NewRunner(bus events.Bus, factory AgentFactory, store *ExecutionStore) *Runner {
	return &Runner{
		bus:        bus,
		factory:    factory,
		store:      store,
		messageBus: NewMessageBus(),
	}
}

// Execute runs a pipeline with the given input
func (r *Runner) Execute(ctx context.Context, pipeline *Pipeline, input string) (*PipelineExecution, error) {
	r.mu.Lock()

	ctx, cancel := context.WithCancel(ctx)
	r.ctx = ctx
	r.cancel = cancel

	// Initialize execution
	exec := &PipelineExecution{
		ID:           time.Now().Format("20060102-150405") + "-" + pipeline.Name,
		Pipeline:     *pipeline,
		Status:       PipelineStatusPending,
		Stages:       make([]StageExecution, len(pipeline.Stages)),
		InitialInput: input,
		StartTime:    time.Now(),
	}

	// Initialize stage executions
	for i, stage := range pipeline.Stages {
		exec.Stages[i] = StageExecution{
			Stage:  stage,
			Status: StageStatusPending,
		}
	}

	r.current = exec
	r.messageBus = NewMessageBus()
	r.mu.Unlock()

	// Emit pipeline started event
	r.bus.Publish(events.Event{
		Type: events.EventPipelineStarted,
		Data: PipelineEventData{
			ExecutionID:  exec.ID,
			PipelineName: pipeline.Name,
			StageCount:   len(pipeline.Stages),
		},
	})

	exec.Status = PipelineStatusRunning
	currentInput := input

	// Execute stages deterministically
	for i, stage := range pipeline.Stages {
		select {
		case <-ctx.Done():
			exec.Status = PipelineStatusCancelled
			r.finishExecution(exec)
			return exec, ctx.Err()
		default:
		}

		stageExec := &exec.Stages[i]
		stageExec.Input = currentInput

		// Emit stage started event
		r.bus.Publish(events.Event{
			Type: events.EventStageStarted,
			Data: StageEventData{
				ExecutionID: exec.ID,
				StageName:   stage.Name,
				StageIndex:  i,
				TotalStages: len(pipeline.Stages),
			},
		})

		now := time.Now()
		stageExec.StartTime = &now
		stageExec.Status = StageStatusRunning

		// Execute the stage
		output, tokens, err := r.executeStage(ctx, stage, currentInput)

		endTime := time.Now()
		stageExec.EndTime = &endTime
		stageExec.Tokens = tokens
		exec.TotalTokens.Add(tokens)

		if err != nil {
			// Check if this was a context cancellation (external cancel)
			if ctx.Err() == context.Canceled {
				stageExec.Status = StageStatusFailed
				stageExec.Error = err.Error()
				exec.Status = PipelineStatusCancelled
				r.finishExecution(exec)
				return exec, err
			}

			stageExec.Status = StageStatusFailed
			stageExec.Error = err.Error()
			exec.FailedStage = i
			exec.ErrorMessage = err.Error()

			// Emit stage failed event
			r.bus.Publish(events.Event{
				Type: events.EventStageFailed,
				Data: StageEventData{
					ExecutionID: exec.ID,
					StageName:   stage.Name,
					StageIndex:  i,
					TotalStages: len(pipeline.Stages),
					Error:       err.Error(),
				},
			})

			// Check if we should continue
			if !stage.ContinueOnError {
				exec.Status = PipelineStatusFailed
				r.finishExecution(exec)
				return exec, err
			}

			// Mark remaining stages as skipped and exit the loop
			for j := i + 1; j < len(pipeline.Stages); j++ {
				exec.Stages[j].Status = StageStatusSkipped
			}
			exec.Status = PipelineStatusFailed
			r.finishExecution(exec)
			return exec, nil // Return nil as this is an expected failure with continue_on_error
		} else {
			stageExec.Status = StageStatusCompleted
			stageExec.Output = output

			// Send message to message bus
			nextStageName := "output"
			if i < len(pipeline.Stages)-1 {
				nextStageName = pipeline.Stages[i+1].Name
			}
			r.messageBus.Send(AgentMessage{
				From:    stage.Name,
				To:      nextStageName,
				Content: output,
			})

			// Emit stage completed event
			r.bus.Publish(events.Event{
				Type: events.EventStageCompleted,
				Data: StageEventData{
					ExecutionID: exec.ID,
					StageName:   stage.Name,
					StageIndex:  i,
					TotalStages: len(pipeline.Stages),
					Output:      output,
				},
			})

			currentInput = output
		}
	}

	exec.Status = PipelineStatusCompleted
	exec.FinalOutput = currentInput
	r.finishExecution(exec)

	return exec, nil
}

// executeStage runs a single pipeline stage
func (r *Runner) executeStage(ctx context.Context, stage Stage, input string) (string, agent.TokenUsage, error) {
	var tokens agent.TokenUsage

	// Create agent from template
	agentInstance, err := r.factory.CreateAgent(stage.Template)
	if err != nil {
		return "", tokens, fmt.Errorf("failed to create agent for stage %s: %w", stage.Name, err)
	}

	// Execute the agent
	outputCh, errCh := agentInstance.Execute(ctx, input)

	// Track token usage if agent supports it
	var tokenCh <-chan agent.TokenUsage
	if trackingAgent, ok := agentInstance.(agent.TokenTrackingAgent); ok {
		tokenCh = trackingAgent.GetTokenUsageChan()
	}

	var output string
	for {
		select {
		case <-ctx.Done():
			return output, tokens, ctx.Err()
		case chunk, ok := <-outputCh:
			if !ok {
				return output, tokens, nil
			}
			output += chunk

			// Emit output chunk event
			r.bus.Publish(events.Event{
				Type:      events.EventOutputChunk,
				AgentName: stage.Name,
				Data:      chunk,
			})
		case usage, ok := <-tokenCh:
			if ok {
				tokens = usage
			}
		case err, ok := <-errCh:
			if ok && err != nil {
				return output, tokens, err
			}
		}
	}
}

// finishExecution completes a pipeline execution and persists it
func (r *Runner) finishExecution(exec *PipelineExecution) {
	now := time.Now()
	exec.EndTime = &now

	// Emit pipeline completed/failed event
	var eventType events.EventType
	if exec.Status == PipelineStatusCompleted {
		eventType = events.EventPipelineCompleted
	} else if exec.Status == PipelineStatusFailed {
		eventType = events.EventPipelineFailed
	} else {
		eventType = events.EventPipelineCancelled
	}

	r.bus.Publish(events.Event{
		Type: eventType,
		Data: PipelineEventData{
			ExecutionID:  exec.ID,
			PipelineName: exec.Pipeline.Name,
			StageCount:   len(exec.Pipeline.Stages),
			FinalOutput:  exec.FinalOutput,
			TotalTokens:  exec.TotalTokens,
			ErrorMessage: exec.ErrorMessage,
		},
	})

	// Persist execution
	if r.store != nil {
		r.store.Save(exec)
	}

	r.mu.Lock()
	r.current = nil
	r.cancel = nil
	r.mu.Unlock()
}

// Cancel stops the current pipeline execution
func (r *Runner) Cancel() {
	r.mu.RLock()
	cancel := r.cancel
	r.mu.RUnlock()

	if cancel != nil {
		cancel()
	}
}

// GetCurrentExecution returns the currently running execution
func (r *Runner) GetCurrentExecution() *PipelineExecution {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.current
}

// GetMessageBus returns the message bus for the current execution
func (r *Runner) GetMessageBus() *MessageBus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.messageBus
}

// PipelineEventData contains data for pipeline lifecycle events
type PipelineEventData struct {
	ExecutionID  string           `json:"execution_id"`
	PipelineName string           `json:"pipeline_name"`
	StageCount   int              `json:"stage_count"`
	FinalOutput  string           `json:"final_output,omitempty"`
	TotalTokens  agent.TokenUsage `json:"total_tokens,omitempty"`
	ErrorMessage string           `json:"error_message,omitempty"`
}

// StageEventData contains data for stage lifecycle events
type StageEventData struct {
	ExecutionID string `json:"execution_id"`
	StageName   string `json:"stage_name"`
	StageIndex  int    `json:"stage_index"`
	TotalStages int    `json:"total_stages"`
	Output      string `json:"output,omitempty"`
	Error       string `json:"error,omitempty"`
}
