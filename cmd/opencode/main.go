package main

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/rasmuselmersson/opencode/pkg/adapter"
	"github.com/rasmuselmersson/opencode/pkg/agent"
	"github.com/rasmuselmersson/opencode/pkg/events"
	"github.com/rasmuselmersson/opencode/pkg/manager"
	"github.com/rasmuselmersson/opencode/pkg/session"
	"github.com/rasmuselmersson/opencode/pkg/tui"
)

func main() {
	bus := events.NewEventBus()

	openCodeAdapter := adapter.NewClaudeAdapter(agent.Config{
		Model: "", // Use default model
	})

	agentManager := manager.NewManager(openCodeAdapter, bus)

	sessionManager, err := session.NewManager("sessions")
	if err != nil {
		panic(err)
	}

	model := tui.NewModel(agentManager, sessionManager, bus)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
	)

	outputCh := bus.Subscribe(events.EventOutputChunk)
	startedCh := bus.Subscribe(events.EventAgentStarted)
	stoppedCh := bus.Subscribe(events.EventAgentStopped)
	pausedCh := bus.Subscribe(events.EventAgentPaused)
	resumedCh := bus.Subscribe(events.EventAgentResumed)

	go func() {
		for event := range outputCh {
			if chunk, ok := event.Data.(string); ok {
				sessionManager.AppendOutput(chunk)
				p.Send(tui.OutputMsg(chunk))
			}
		}
	}()

	go func() {
		for event := range startedCh {
			sessionManager.StartSession(event.AgentName, "")
			p.Send(tui.StateMsg("running"))
		}
	}()

	go func() {
		for range stoppedCh {
			sessionManager.EndSession()
			p.Send(tui.StateMsg("idle"))
		}
	}()

	go func() {
		for range pausedCh {
			p.Send(tui.StateMsg("paused"))
		}
	}()

	go func() {
		for range resumedCh {
			p.Send(tui.StateMsg("running"))
		}
	}()

	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
