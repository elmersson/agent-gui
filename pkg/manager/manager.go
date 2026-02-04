package manager

import (
	"context"
	"sync"

	"github.com/rasmuselmersson/opencode/pkg/agent"
	"github.com/rasmuselmersson/opencode/pkg/events"
)

type State string

const (
	StateIdle    State = "idle"
	StateRunning State = "running"
	StatePaused  State = "paused"
)

type Manager struct {
	agent      agent.Agent
	bus        events.Bus
	state      State
	stateMutex sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	paused     bool
	pauseCond  *sync.Cond
}

func NewManager(agent agent.Agent, bus events.Bus) *Manager {
	return &Manager{
		agent:     agent,
		bus:       bus,
		state:     StateIdle,
		pauseCond: sync.NewCond(&sync.Mutex{}),
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
