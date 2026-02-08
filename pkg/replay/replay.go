package replay

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rasmuselmersson/opencode/pkg/agent"
	"github.com/rasmuselmersson/opencode/pkg/events"
)

// PlaybackState represents the current state of replay playback
type PlaybackState string

const (
	StateStopped PlaybackState = "stopped"
	StatePlaying PlaybackState = "playing"
	StatePaused  PlaybackState = "paused"
)

// Session represents a recorded session that can be replayed
type Session struct {
	ID         string           `json:"id"`
	AgentName  string           `json:"agent_name"`
	Input      string           `json:"input"`
	Output     string           `json:"output"`
	StartTime  time.Time        `json:"start_time"`
	EndTime    *time.Time       `json:"end_time,omitempty"`
	Events     []events.Event   `json:"events"`
	TokenUsage agent.TokenUsage `json:"token_usage"`
}

// PipelineStageConfig represents a stage configuration in a pipeline
type PipelineStageConfig struct {
	Name        string `json:"name"`
	Template    string `json:"template"`
	Description string `json:"description"`
}

// PipelineConfig represents the pipeline configuration
type PipelineConfig struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Stages      []PipelineStageConfig `json:"stages"`
	Metadata    map[string]string     `json:"metadata,omitempty"`
}

// PipelineStageExecution represents a single stage execution
type PipelineStageExecution struct {
	Stage     PipelineStageConfig `json:"stage"`
	Status    string              `json:"status"`
	Input     string              `json:"input"`
	Output    string              `json:"output"`
	Error     string              `json:"error,omitempty"`
	StartTime time.Time           `json:"start_time"`
	EndTime   time.Time           `json:"end_time"`
	Tokens    agent.TokenUsage    `json:"tokens"`
}

// PipelineExecution represents a complete pipeline execution that can be replayed
type PipelineExecution struct {
	ID           string                   `json:"id"`
	Pipeline     PipelineConfig           `json:"pipeline"`
	Status       string                   `json:"status"`
	Stages       []PipelineStageExecution `json:"stages"`
	InitialInput string                   `json:"initial_input"`
	StartTime    time.Time                `json:"start_time"`
	EndTime      time.Time                `json:"end_time"`
	TotalTokens  agent.TokenUsage         `json:"total_tokens"`
	FailedStage  int                      `json:"failed_stage,omitempty"`
	ErrorMessage string                   `json:"error_message,omitempty"`
}

// ReplayEvent is sent to observers during playback
type ReplayEvent struct {
	Type         string
	Position     time.Duration // Current position in the timeline
	TotalLength  time.Duration // Total length of the session
	OutputChunk  string        // For output events, the chunk of text
	FullOutput   string        // Full output up to this point
	Event        *events.Event // The underlying event being replayed
	State        PlaybackState
	Speed        float64
	TokenUsage   agent.TokenUsage
	SessionInfo  *Session
	PipelineInfo *PipelineExecution
	CurrentStage int // Index of current stage in pipeline replay
	IsPipeline   bool
}

// Observer is a function that receives replay events
type Observer func(ReplayEvent)

// Engine manages session replay
type Engine struct {
	mu sync.RWMutex

	// Current session
	session               *Session
	sessionsDir           string
	pipelineExecutionsDir string

	// Current pipeline execution
	pipelineExecution *PipelineExecution
	currentStage      int
	isPipeline        bool

	// Playback state
	state    PlaybackState
	speed    float64 // 1.0 = realtime, 2.0 = 2x speed, 0.5 = half speed
	position time.Duration

	// Control channels
	stopCh   chan struct{}
	seekCh   chan time.Duration
	speedCh  chan float64
	pauseCh  chan struct{}
	resumeCh chan struct{}

	// Observers
	observers []Observer
}

// NewEngine creates a new replay engine
func NewEngine(sessionsDir string) *Engine {
	return &Engine{
		sessionsDir:           sessionsDir,
		pipelineExecutionsDir: "pipeline-executions",
		state:                 StateStopped,
		speed:                 1.0,
		observers:             make([]Observer, 0),
	}
}

// NewEngineWithPipelines creates a new replay engine with custom directories
func NewEngineWithPipelines(sessionsDir, pipelineExecutionsDir string) *Engine {
	return &Engine{
		sessionsDir:           sessionsDir,
		pipelineExecutionsDir: pipelineExecutionsDir,
		state:                 StateStopped,
		speed:                 1.0,
		observers:             make([]Observer, 0),
	}
}

// AddObserver adds an observer to receive replay events
func (e *Engine) AddObserver(obs Observer) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.observers = append(e.observers, obs)
}

// notify sends an event to all observers
func (e *Engine) notify(event ReplayEvent) {
	e.mu.RLock()
	observers := make([]Observer, len(e.observers))
	copy(observers, e.observers)
	e.mu.RUnlock()

	for _, obs := range observers {
		obs(event)
	}
}

// ListSessions returns all available sessions
func (e *Engine) ListSessions() ([]Session, error) {
	entries, err := os.ReadDir(e.sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Session{}, nil
		}
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(e.sessionsDir, entry.Name())
		session, err := e.loadSessionFile(filePath)
		if err != nil {
			continue // Skip invalid files
		}
		sessions = append(sessions, *session)
	}

	// Sort by start time (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartTime.After(sessions[j].StartTime)
	})

	return sessions, nil
}

// loadSessionFile loads a session from a file
func (e *Engine) loadSessionFile(filePath string) (*Session, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session: %w", err)
	}

	return &session, nil
}

// ListPipelineExecutions returns all available pipeline executions
func (e *Engine) ListPipelineExecutions() ([]PipelineExecution, error) {
	entries, err := os.ReadDir(e.pipelineExecutionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []PipelineExecution{}, nil
		}
		return nil, fmt.Errorf("failed to read pipeline executions directory: %w", err)
	}

	var executions []PipelineExecution
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(e.pipelineExecutionsDir, entry.Name())
		execution, err := e.loadPipelineExecutionFile(filePath)
		if err != nil {
			continue // Skip invalid files
		}
		executions = append(executions, *execution)
	}

	// Sort by start time (newest first)
	sort.Slice(executions, func(i, j int) bool {
		return executions[i].StartTime.After(executions[j].StartTime)
	})

	return executions, nil
}

// loadPipelineExecutionFile loads a pipeline execution from a file
func (e *Engine) loadPipelineExecutionFile(filePath string) (*PipelineExecution, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read pipeline execution file: %w", err)
	}

	var execution PipelineExecution
	if err := json.Unmarshal(data, &execution); err != nil {
		return nil, fmt.Errorf("failed to parse pipeline execution: %w", err)
	}

	return &execution, nil
}

// LoadPipelineExecution loads a pipeline execution by ID
func (e *Engine) LoadPipelineExecution(executionID string) error {
	e.mu.Lock()

	// Stop any current playback
	if e.state != StateStopped {
		e.stopPlayback()
	}

	filePath := filepath.Join(e.pipelineExecutionsDir, executionID+".json")
	execution, err := e.loadPipelineExecutionFile(filePath)
	if err != nil {
		e.mu.Unlock()
		return err
	}

	e.pipelineExecution = execution
	e.session = nil // Clear any session
	e.isPipeline = true
	e.currentStage = 0
	e.position = 0
	e.state = StateStopped
	duration := e.pipelineDuration()
	state := e.state
	speed := e.speed

	e.mu.Unlock()

	// Notify observers (outside of lock)
	e.notify(ReplayEvent{
		Type:         "pipeline_loaded",
		Position:     0,
		TotalLength:  duration,
		PipelineInfo: execution,
		State:        state,
		Speed:        speed,
		TokenUsage:   execution.TotalTokens,
		IsPipeline:   true,
		CurrentStage: 0,
	})

	return nil
}

// GetPipelineExecution returns the currently loaded pipeline execution
func (e *Engine) GetPipelineExecution() *PipelineExecution {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.pipelineExecution
}

// IsPipelineReplay returns true if currently replaying a pipeline
func (e *Engine) IsPipelineReplay() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.isPipeline
}

// pipelineDuration returns the total duration of the loaded pipeline execution
func (e *Engine) pipelineDuration() time.Duration {
	if e.pipelineExecution == nil {
		return 0
	}
	return e.pipelineExecution.EndTime.Sub(e.pipelineExecution.StartTime)
}

// LoadSession loads a session by ID
func (e *Engine) LoadSession(sessionID string) error {
	e.mu.Lock()

	// Stop any current playback
	if e.state != StateStopped {
		e.stopPlayback()
	}

	filePath := filepath.Join(e.sessionsDir, sessionID+".json")
	session, err := e.loadSessionFile(filePath)
	if err != nil {
		e.mu.Unlock()
		return err
	}

	e.session = session
	e.pipelineExecution = nil // Clear any pipeline
	e.isPipeline = false
	e.currentStage = 0
	e.position = 0
	e.state = StateStopped
	duration := e.sessionDuration()
	state := e.state
	speed := e.speed

	e.mu.Unlock()

	// Notify observers (outside of lock)
	e.notify(ReplayEvent{
		Type:        "session_loaded",
		Position:    0,
		TotalLength: duration,
		SessionInfo: session,
		State:       state,
		Speed:       speed,
		TokenUsage:  session.TokenUsage,
		IsPipeline:  false,
	})

	return nil
}

// GetSession returns the currently loaded session
func (e *Engine) GetSession() *Session {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.session
}

// sessionDuration returns the total duration of the loaded session
func (e *Engine) sessionDuration() time.Duration {
	if e.session == nil || e.session.EndTime == nil {
		return 0
	}
	return e.session.EndTime.Sub(e.session.StartTime)
}

// GetDuration returns the total duration of the loaded session or pipeline
func (e *Engine) GetDuration() time.Duration {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.isPipeline {
		return e.pipelineDuration()
	}
	return e.sessionDuration()
}

// GetPosition returns the current playback position
func (e *Engine) GetPosition() time.Duration {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.position
}

// GetState returns the current playback state
func (e *Engine) GetState() PlaybackState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state
}

// GetSpeed returns the current playback speed
func (e *Engine) GetSpeed() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.speed
}

// Play starts or resumes playback
func (e *Engine) Play() error {
	e.mu.Lock()

	if e.session == nil && e.pipelineExecution == nil {
		e.mu.Unlock()
		return fmt.Errorf("no session or pipeline loaded")
	}

	if e.state == StatePlaying {
		e.mu.Unlock()
		return nil // Already playing
	}

	if e.state == StatePaused {
		// Resume from paused state
		if e.resumeCh != nil {
			e.resumeCh <- struct{}{}
		}
		e.state = StatePlaying
		e.mu.Unlock()
		return nil
	}

	// Start new playback
	e.state = StatePlaying
	e.stopCh = make(chan struct{})
	e.seekCh = make(chan time.Duration, 1)
	e.speedCh = make(chan float64, 1)
	e.pauseCh = make(chan struct{})
	e.resumeCh = make(chan struct{})

	e.mu.Unlock()

	go e.playbackLoop()

	return nil
}

// Pause pauses playback
func (e *Engine) Pause() {
	e.mu.Lock()

	if e.state != StatePlaying {
		e.mu.Unlock()
		return
	}

	e.state = StatePaused
	if e.pauseCh != nil {
		select {
		case e.pauseCh <- struct{}{}:
		default:
		}
	}

	position := e.position
	duration := e.sessionDuration()
	state := e.state
	speed := e.speed

	e.mu.Unlock()

	e.notify(ReplayEvent{
		Type:        "state_changed",
		Position:    position,
		TotalLength: duration,
		State:       state,
		Speed:       speed,
	})
}

// Stop stops playback completely
func (e *Engine) Stop() {
	e.mu.Lock()

	if e.state == StateStopped {
		e.mu.Unlock()
		return
	}

	if e.stopCh != nil {
		close(e.stopCh)
		e.stopCh = nil
	}

	e.state = StateStopped
	e.position = 0
	duration := e.sessionDuration()
	speed := e.speed

	e.mu.Unlock()

	e.notify(ReplayEvent{
		Type:        "state_changed",
		Position:    0,
		TotalLength: duration,
		State:       StateStopped,
		Speed:       speed,
	})
}

// stopPlayback internal stop without lock - assumes lock is held but will be released for notify
func (e *Engine) stopPlayback() {
	if e.state == StateStopped {
		return
	}

	if e.stopCh != nil {
		close(e.stopCh)
		e.stopCh = nil
	}

	e.state = StateStopped
	e.position = 0
	duration := e.sessionDuration()
	speed := e.speed

	// Don't notify from here - caller should handle it
	// This avoids deadlock issues
	_ = duration
	_ = speed
}

// Seek jumps to a specific position in the timeline
func (e *Engine) Seek(position time.Duration) {
	e.mu.Lock()

	var duration time.Duration
	if e.isPipeline {
		duration = e.pipelineDuration()
	} else {
		duration = e.sessionDuration()
	}

	if position < 0 {
		position = 0
	}
	if position > duration {
		position = duration
	}

	e.position = position

	if e.seekCh != nil {
		select {
		case e.seekCh <- position:
		default:
		}
	}

	// Calculate output at this position
	output := e.outputAtPosition(position)
	state := e.state
	speed := e.speed
	isPipeline := e.isPipeline
	pipelineExec := e.pipelineExecution
	currentStage := 0
	if isPipeline {
		currentStage = e.currentStageAtPosition(position)
		e.currentStage = currentStage
	}

	e.mu.Unlock()

	e.notify(ReplayEvent{
		Type:         "seek",
		Position:     position,
		TotalLength:  duration,
		FullOutput:   output,
		State:        state,
		Speed:        speed,
		IsPipeline:   isPipeline,
		PipelineInfo: pipelineExec,
		CurrentStage: currentStage,
	})
}

// SeekPercent jumps to a percentage position (0.0 - 1.0)
func (e *Engine) SeekPercent(percent float64) {
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}

	duration := e.GetDuration()
	position := time.Duration(float64(duration) * percent)
	e.Seek(position)
}

// SetSpeed sets the playback speed
func (e *Engine) SetSpeed(speed float64) {
	e.mu.Lock()

	if speed < 0.1 {
		speed = 0.1
	}
	if speed > 10.0 {
		speed = 10.0
	}

	e.speed = speed

	if e.speedCh != nil {
		select {
		case e.speedCh <- speed:
		default:
		}
	}

	position := e.position
	duration := e.sessionDuration()
	state := e.state

	e.mu.Unlock()

	e.notify(ReplayEvent{
		Type:        "speed_changed",
		Position:    position,
		TotalLength: duration,
		State:       state,
		Speed:       speed,
	})
}

// outputAtPosition calculates what output should be visible at a given position
func (e *Engine) outputAtPosition(position time.Duration) string {
	if e.isPipeline {
		return e.pipelineOutputAtPosition(position)
	}

	if e.session == nil {
		return ""
	}

	// If we have events, use them to reconstruct output
	if len(e.session.Events) > 0 {
		var output strings.Builder
		for _, evt := range e.session.Events {
			eventOffset := evt.Timestamp.Sub(e.session.StartTime)
			if eventOffset > position {
				break
			}
			if evt.Type == events.EventOutputChunk {
				if chunk, ok := evt.Data.(string); ok {
					output.WriteString(chunk)
				}
			}
		}
		return output.String()
	}

	// Fallback: simulate progressive output based on position
	duration := e.sessionDuration()
	if duration == 0 {
		return e.session.Output
	}

	progress := float64(position) / float64(duration)
	if progress >= 1.0 {
		return e.session.Output
	}

	// Show output progressively
	totalLen := len(e.session.Output)
	showLen := int(float64(totalLen) * progress)
	if showLen > totalLen {
		showLen = totalLen
	}

	return e.session.Output[:showLen]
}

// pipelineOutputAtPosition calculates pipeline output at a given position
func (e *Engine) pipelineOutputAtPosition(position time.Duration) string {
	if e.pipelineExecution == nil {
		return ""
	}

	var output strings.Builder
	currentTime := e.pipelineExecution.StartTime.Add(position)

	for i, stage := range e.pipelineExecution.Stages {
		// Check if this stage has started
		if stage.StartTime.After(currentTime) {
			break
		}

		// Stage header
		statusIcon := "[ ]"
		switch stage.Status {
		case "completed":
			statusIcon = "[OK]"
		case "failed":
			statusIcon = "[X]"
		case "running":
			statusIcon = "[>]"
		}

		output.WriteString(fmt.Sprintf("\n## Stage %d: %s %s\n", i+1, stage.Stage.Name, statusIcon))
		output.WriteString(fmt.Sprintf("*%s*\n\n", stage.Stage.Description))

		// Check if stage has completed or is in progress
		if stage.EndTime.Before(currentTime) || stage.EndTime.Equal(currentTime) {
			// Stage completed - show full output
			if stage.Output != "" {
				output.WriteString(stage.Output)
				output.WriteString("\n")
			}
			if stage.Error != "" {
				output.WriteString(fmt.Sprintf("\n**Error:** %s\n", stage.Error))
			}
		} else if !stage.StartTime.After(currentTime) {
			// Stage in progress - show partial output
			stageDuration := stage.EndTime.Sub(stage.StartTime)
			stageProgress := currentTime.Sub(stage.StartTime)
			if stageDuration > 0 && stage.Output != "" {
				progress := float64(stageProgress) / float64(stageDuration)
				if progress > 1 {
					progress = 1
				}
				showLen := int(float64(len(stage.Output)) * progress)
				if showLen > len(stage.Output) {
					showLen = len(stage.Output)
				}
				output.WriteString(stage.Output[:showLen])
			}
		}
	}

	return output.String()
}

// GetCurrentStage returns the current stage index in pipeline replay
func (e *Engine) GetCurrentStage() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.currentStage
}

// currentStageAtPosition determines which stage is active at a given position
func (e *Engine) currentStageAtPosition(position time.Duration) int {
	if e.pipelineExecution == nil {
		return 0
	}

	currentTime := e.pipelineExecution.StartTime.Add(position)
	for i, stage := range e.pipelineExecution.Stages {
		if stage.StartTime.After(currentTime) {
			if i > 0 {
				return i - 1
			}
			return 0
		}
		if !stage.EndTime.After(currentTime) {
			continue
		}
		return i
	}
	return len(e.pipelineExecution.Stages) - 1
}

// playbackLoop runs the main playback loop
func (e *Engine) playbackLoop() {
	e.mu.RLock()
	session := e.session
	pipelineExec := e.pipelineExecution
	isPipeline := e.isPipeline
	startPos := e.position
	speed := e.speed
	e.mu.RUnlock()

	var duration time.Duration
	var finalOutput string
	var tokenUsage agent.TokenUsage

	if isPipeline {
		if pipelineExec == nil {
			return
		}
		duration = pipelineExec.EndTime.Sub(pipelineExec.StartTime)
		finalOutput = e.pipelineOutputAtPosition(duration)
		tokenUsage = pipelineExec.TotalTokens
	} else {
		if session == nil {
			return
		}
		if session.EndTime == nil {
			duration = 0
		} else {
			duration = session.EndTime.Sub(session.StartTime)
		}
		finalOutput = session.Output
		tokenUsage = session.TokenUsage
	}

	if duration <= 0 {
		// Session/pipeline with no duration - just show final state
		e.notify(ReplayEvent{
			Type:         "output",
			Position:     0,
			TotalLength:  0,
			FullOutput:   finalOutput,
			State:        StatePlaying,
			Speed:        speed,
			TokenUsage:   tokenUsage,
			IsPipeline:   isPipeline,
			PipelineInfo: pipelineExec,
			SessionInfo:  session,
		})
		e.mu.Lock()
		e.state = StateStopped
		e.mu.Unlock()
		return
	}

	// Start playback
	e.notify(ReplayEvent{
		Type:         "playback_started",
		Position:     startPos,
		TotalLength:  duration,
		State:        StatePlaying,
		Speed:        speed,
		SessionInfo:  session,
		PipelineInfo: pipelineExec,
		IsPipeline:   isPipeline,
	})

	// Calculate tick interval for smooth playback
	tickInterval := 50 * time.Millisecond
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	currentPos := startPos
	paused := false

	for {
		select {
		case <-e.stopCh:
			return

		case <-e.pauseCh:
			paused = true
			ticker.Stop()

		case <-e.resumeCh:
			paused = false
			ticker.Reset(tickInterval)

		case newSpeed := <-e.speedCh:
			e.mu.Lock()
			speed = newSpeed
			e.mu.Unlock()

		case newPos := <-e.seekCh:
			currentPos = newPos
			e.mu.Lock()
			e.position = currentPos
			if isPipeline {
				e.currentStage = e.currentStageAtPosition(currentPos)
			}
			e.mu.Unlock()

		case <-ticker.C:
			if paused {
				continue
			}

			// Advance position
			e.mu.Lock()
			currentPos += time.Duration(float64(tickInterval) * speed)
			e.position = currentPos
			currentStage := 0
			if isPipeline {
				currentStage = e.currentStageAtPosition(currentPos)
				e.currentStage = currentStage
			}
			e.mu.Unlock()

			if currentPos >= duration {
				// Playback complete
				e.notify(ReplayEvent{
					Type:         "output",
					Position:     duration,
					TotalLength:  duration,
					FullOutput:   finalOutput,
					State:        StatePlaying,
					Speed:        speed,
					TokenUsage:   tokenUsage,
					IsPipeline:   isPipeline,
					PipelineInfo: pipelineExec,
					CurrentStage: currentStage,
				})

				e.notify(ReplayEvent{
					Type:         "playback_complete",
					Position:     duration,
					TotalLength:  duration,
					FullOutput:   finalOutput,
					State:        StateStopped,
					Speed:        speed,
					TokenUsage:   tokenUsage,
					IsPipeline:   isPipeline,
					PipelineInfo: pipelineExec,
					CurrentStage: currentStage,
				})

				e.mu.Lock()
				e.state = StateStopped
				e.mu.Unlock()
				return
			}

			// Emit current output
			output := e.outputAtPosition(currentPos)
			e.notify(ReplayEvent{
				Type:         "output",
				Position:     currentPos,
				TotalLength:  duration,
				FullOutput:   output,
				State:        StatePlaying,
				Speed:        speed,
				IsPipeline:   isPipeline,
				PipelineInfo: pipelineExec,
				CurrentStage: currentStage,
			})
		}
	}
}

// FormatDuration formats a duration as mm:ss.ms
func FormatDuration(d time.Duration) string {
	mins := int(d.Minutes())
	secs := int(d.Seconds()) % 60
	ms := (d.Milliseconds() % 1000) / 10
	return fmt.Sprintf("%02d:%02d.%02d", mins, secs, ms)
}

// FormatProgress returns a progress string like "01:23 / 05:00"
func FormatProgress(current, total time.Duration) string {
	return fmt.Sprintf("%s / %s", FormatDuration(current), FormatDuration(total))
}
