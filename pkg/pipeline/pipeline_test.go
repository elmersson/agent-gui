package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rasmuselmersson/opencode/pkg/agent"
	"github.com/rasmuselmersson/opencode/pkg/events"
)

// MockAgent implements agent.Agent for testing
type MockAgent struct {
	name   string
	output string
}

func (m *MockAgent) Name() string {
	return m.name
}

func (m *MockAgent) Execute(ctx context.Context, input string) (<-chan string, <-chan error) {
	outputCh := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		defer close(outputCh)
		defer close(errCh)

		// Simulate processing
		time.Sleep(10 * time.Millisecond)

		// Return mock output based on agent name
		switch m.name {
		case "planner":
			outputCh <- "Plan: Step 1: Analyze, Step 2: Implement, Step 3: Test"
		case "coder":
			outputCh <- "Code: func main() { fmt.Println(\"Hello\") }"
		case "code-reviewer":
			outputCh <- "Review: Code looks good. No issues found."
		default:
			outputCh <- m.output
		}
	}()

	return outputCh, errCh
}

// MockAgentFactory creates mock agents
type MockAgentFactory struct{}

func (f *MockAgentFactory) CreateAgent(templateName string) (agent.Agent, error) {
	return &MockAgent{name: templateName, output: "mock output"}, nil
}

func TestPipelineLoader(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "pipeline-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test pipeline file
	pipelineYAML := `
name: test-pipeline
description: A test pipeline
stages:
  - name: stage1
    template: planner
  - name: stage2
    template: coder
`
	err = os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte(pipelineYAML), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Test loader
	loader := NewPipelineLoader(tmpDir)
	err = loader.Load()
	if err != nil {
		t.Fatalf("Failed to load pipelines: %v", err)
	}

	// Check pipeline was loaded
	names := loader.ListPipelines()
	if len(names) != 1 || names[0] != "test-pipeline" {
		t.Errorf("Expected ['test-pipeline'], got %v", names)
	}

	// Get pipeline details
	pipeline, err := loader.GetPipeline("test-pipeline")
	if err != nil {
		t.Fatalf("Failed to get pipeline: %v", err)
	}

	if len(pipeline.Stages) != 2 {
		t.Errorf("Expected 2 stages, got %d", len(pipeline.Stages))
	}
}

func TestMessageBus(t *testing.T) {
	bus := NewMessageBus()

	// Send messages
	bus.Send(AgentMessage{From: "planner", To: "coder", Content: "plan output"})
	bus.Send(AgentMessage{From: "coder", To: "reviewer", Content: "code output"})

	// Get messages
	messages := bus.GetMessages("planner", "coder")
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}

	// Get all messages
	allMessages := bus.GetAllMessages()
	if len(allMessages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(allMessages))
	}

	// Get last output
	lastOutput := bus.GetLastOutput()
	if lastOutput != "code output" {
		t.Errorf("Expected 'code output', got %s", lastOutput)
	}
}

func TestPipelineExecution(t *testing.T) {
	// Create event bus
	bus := events.NewEventBus()

	// Create execution store
	tmpDir, err := os.MkdirTemp("", "execution-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewExecutionStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create runner with mock factory
	factory := &MockAgentFactory{}
	runner := NewRunner(bus, factory, store)

	// Create test pipeline
	pipeline := &Pipeline{
		Name: "test-pipeline",
		Stages: []Stage{
			{Name: "planner", Template: "planner"},
			{Name: "coder", Template: "coder"},
			{Name: "reviewer", Template: "code-reviewer"},
		},
	}

	// Subscribe to events
	startedCh := bus.Subscribe(events.EventPipelineStarted)
	completedCh := bus.Subscribe(events.EventPipelineCompleted)
	stageStartedCh := bus.Subscribe(events.EventStageStarted)
	stageCompletedCh := bus.Subscribe(events.EventStageCompleted)

	// Execute pipeline
	ctx := context.Background()
	exec, err := runner.Execute(ctx, pipeline, "Create a hello world program")

	if err != nil {
		t.Fatalf("Pipeline execution failed: %v", err)
	}

	if exec.Status != PipelineStatusCompleted {
		t.Errorf("Expected status %s, got %s", PipelineStatusCompleted, exec.Status)
	}

	// Check events were emitted
	select {
	case <-startedCh:
		// Good
	case <-time.After(time.Second):
		t.Error("Did not receive pipeline started event")
	}

	select {
	case <-completedCh:
		// Good
	case <-time.After(time.Second):
		t.Error("Did not receive pipeline completed event")
	}

	// Count stage events
	stageStartCount := 0
	stageCompleteCount := 0
	timeout := time.After(time.Second)

countLoop:
	for {
		select {
		case <-stageStartedCh:
			stageStartCount++
		case <-stageCompletedCh:
			stageCompleteCount++
		case <-timeout:
			break countLoop
		default:
			if stageStartCount >= 3 && stageCompleteCount >= 3 {
				break countLoop
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	if stageStartCount < 3 {
		t.Errorf("Expected at least 3 stage started events, got %d", stageStartCount)
	}
	if stageCompleteCount < 3 {
		t.Errorf("Expected at least 3 stage completed events, got %d", stageCompleteCount)
	}

	// Check all stages completed
	for i, stage := range exec.Stages {
		if stage.Status != StageStatusCompleted {
			t.Errorf("Stage %d (%s) not completed: %s", i, stage.Stage.Name, stage.Status)
		}
	}

	// Check final output contains reviewer output
	if !strings.Contains(exec.FinalOutput, "Review:") {
		t.Errorf("Expected final output to contain 'Review:', got: %s", exec.FinalOutput)
	}

	// Check execution was persisted
	loadedExec, err := store.Load(exec.ID)
	if err != nil {
		t.Fatalf("Failed to load execution: %v", err)
	}

	if loadedExec.Status != PipelineStatusCompleted {
		t.Errorf("Loaded execution status wrong: %s", loadedExec.Status)
	}
}

func TestPipelineFailure(t *testing.T) {
	// Create a failing agent factory
	failingFactory := &FailingAgentFactory{failAt: "coder"}

	bus := events.NewEventBus()
	runner := NewRunner(bus, failingFactory, nil)

	pipeline := &Pipeline{
		Name: "test-failing",
		Stages: []Stage{
			{Name: "planner", Template: "planner"},
			{Name: "coder", Template: "coder"},
			{Name: "reviewer", Template: "code-reviewer"},
		},
	}

	ctx := context.Background()
	exec, err := runner.Execute(ctx, pipeline, "test input")

	if err == nil {
		t.Error("Expected pipeline to fail")
	}

	if exec.Status != PipelineStatusFailed {
		t.Errorf("Expected status %s, got %s", PipelineStatusFailed, exec.Status)
	}

	if exec.FailedStage != 1 {
		t.Errorf("Expected failed stage 1, got %d", exec.FailedStage)
	}
}

func TestPipelineContinueOnError(t *testing.T) {
	failingFactory := &FailingAgentFactory{failAt: "coder"}
	bus := events.NewEventBus()
	runner := NewRunner(bus, failingFactory, nil)

	// Test that without ContinueOnError, the pipeline stops immediately
	pipeline := &Pipeline{
		Name: "test-no-continue",
		Stages: []Stage{
			{Name: "planner", Template: "planner"},
			{Name: "coder", Template: "coder", ContinueOnError: false},
			{Name: "reviewer", Template: "code-reviewer"},
		},
	}

	ctx := context.Background()
	exec, err := runner.Execute(ctx, pipeline, "test input")

	// Pipeline should fail
	if err == nil {
		t.Error("Expected error when ContinueOnError is false")
	}
	if exec.Status != PipelineStatusFailed {
		t.Errorf("Expected status failed, got %s", exec.Status)
	}

	// Now test with ContinueOnError - remaining stages should still be skipped
	// because they depend on the failed stage's output
	pipelineWithContinue := &Pipeline{
		Name: "test-continue",
		Stages: []Stage{
			{Name: "planner", Template: "planner"},
			{Name: "coder", Template: "coder", ContinueOnError: true},
			{Name: "reviewer", Template: "code-reviewer"},
		},
	}

	exec2, _ := runner.Execute(ctx, pipelineWithContinue, "test input")

	// With continue_on_error, the remaining stages should be skipped
	// because they depend on the previous stage's output
	if exec2.Stages[2].Status != StageStatusSkipped {
		t.Errorf("Expected stage 2 to be skipped, got %s", exec2.Stages[2].Status)
	}
}

// FailingAgentFactory creates an agent that fails at a specific template
type FailingAgentFactory struct {
	failAt string
}

func (f *FailingAgentFactory) CreateAgent(templateName string) (agent.Agent, error) {
	if templateName == f.failAt {
		return &FailingAgent{}, nil
	}
	return &MockAgent{name: templateName}, nil
}

// FailingAgent always returns an error
type FailingAgent struct{}

func (a *FailingAgent) Name() string { return "failing" }

func (a *FailingAgent) Execute(ctx context.Context, input string) (<-chan string, <-chan error) {
	outputCh := make(chan string)
	errCh := make(chan error, 1)

	go func() {
		defer close(outputCh)
		defer close(errCh)
		errCh <- context.DeadlineExceeded
	}()

	return outputCh, errCh
}

func TestPipelineCancellation(t *testing.T) {
	bus := events.NewEventBus()
	factory := &SlowAgentFactory{}
	runner := NewRunner(bus, factory, nil)

	pipeline := &Pipeline{
		Name: "test-cancel",
		Stages: []Stage{
			{Name: "slow1", Template: "slow"},
			{Name: "slow2", Template: "slow"},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Start pipeline in goroutine
	done := make(chan *PipelineExecution)
	go func() {
		exec, _ := runner.Execute(ctx, pipeline, "test")
		done <- exec
	}()

	// Cancel after short delay
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Wait for completion
	exec := <-done

	if exec.Status != PipelineStatusCancelled {
		t.Errorf("Expected status %s, got %s", PipelineStatusCancelled, exec.Status)
	}
}

// SlowAgentFactory creates agents that take a long time
type SlowAgentFactory struct{}

func (f *SlowAgentFactory) CreateAgent(templateName string) (agent.Agent, error) {
	return &SlowAgent{}, nil
}

// SlowAgent takes a long time to complete
type SlowAgent struct{}

func (a *SlowAgent) Name() string { return "slow" }

func (a *SlowAgent) Execute(ctx context.Context, input string) (<-chan string, <-chan error) {
	outputCh := make(chan string)
	errCh := make(chan error, 1)

	go func() {
		defer close(outputCh)
		defer close(errCh)

		select {
		case <-ctx.Done():
			errCh <- ctx.Err()
		case <-time.After(5 * time.Second):
			outputCh <- "done"
		}
	}()

	return outputCh, errCh
}
