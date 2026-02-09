package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rasmuselmersson/opencode/pkg/adapter"
	"github.com/rasmuselmersson/opencode/pkg/agent"
	"github.com/rasmuselmersson/opencode/pkg/events"
	"github.com/rasmuselmersson/opencode/pkg/manager"
	"github.com/rasmuselmersson/opencode/pkg/pane"
	"github.com/rasmuselmersson/opencode/pkg/remote"
	"github.com/rasmuselmersson/opencode/pkg/replay"
	"github.com/rasmuselmersson/opencode/pkg/session"
	"github.com/rasmuselmersson/opencode/pkg/tui"
)

func main() {
	bus := events.NewEventBus()

	// Create agent based on environment configuration
	var agentInstance agent.Agent

	// Check if remote mode is enabled via environment variable
	remoteAddr := os.Getenv("OPENCODE_REMOTE_ADDRESS")
	remoteToken := os.Getenv("OPENCODE_REMOTE_TOKEN")

	if remoteAddr != "" {
		// Remote mode
		if remoteToken == "" {
			fmt.Fprintf(os.Stderr, "Error: OPENCODE_REMOTE_TOKEN must be set when using remote agents\n")
			os.Exit(1)
		}

		remoteConfig := remote.RemoteConfig{
			Address:              remoteAddr,
			AuthToken:            remoteToken,
			AgentName:            "opencode",
			TLSEnabled:           os.Getenv("OPENCODE_REMOTE_TLS") == "true",
			MaxReconnectAttempts: 5,
			ReconnectBackoffBase: 1000, // 1 second
		}

		agentInstance = adapter.NewRemoteAdapter(remoteConfig, bus)
		fmt.Printf("Using remote agent at: %s\n", remoteAddr)
	} else {
		// Local mode (default)
		agentInstance = adapter.NewClaudeAdapter(agent.Config{
			Model: "", // Use opencode's configured model
		})
	}

	agentManager := manager.NewManager(agentInstance, bus)

	sessionManager, err := session.NewManager("sessions")
	if err != nil {
		panic(err)
	}

	// Create replay engine for session replay functionality
	replayEngine := replay.NewEngine("sessions")

	// Create pane manager for multi-pane layout
	paneManager := pane.NewManager(bus)
	paneManager.Initialize("opencode")

	model := tui.NewModel(agentManager, sessionManager, bus, "auto", replayEngine, paneManager)

	// Note: replay observer will be set up after program is created

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
	)

	// Set up replay engine observer to forward events to TUI
	// Use goroutine to avoid deadlock when LoadSession is called from within Update
	replayEngine.AddObserver(func(event replay.ReplayEvent) {
		go p.Send(tui.ReplayEventMsg(event))
	})

	outputCh := bus.Subscribe(events.EventOutputChunk)
	startedCh := bus.Subscribe(events.EventAgentStarted)
	stoppedCh := bus.Subscribe(events.EventAgentStopped)
	pausedCh := bus.Subscribe(events.EventAgentPaused)
	resumedCh := bus.Subscribe(events.EventAgentResumed)
	tokensCh := bus.Subscribe(events.EventTokensUpdated)
	errorCh := bus.Subscribe(events.EventError)

	// Pipeline events
	pipelineStartedCh := bus.Subscribe(events.EventPipelineStarted)
	pipelineCompletedCh := bus.Subscribe(events.EventPipelineCompleted)
	pipelineFailedCh := bus.Subscribe(events.EventPipelineFailed)
	stageStartedCh := bus.Subscribe(events.EventStageStarted)
	stageCompletedCh := bus.Subscribe(events.EventStageCompleted)

	// Remote connection events
	remoteConnectingCh := bus.Subscribe(events.EventRemoteConnecting)
	remoteConnectedCh := bus.Subscribe(events.EventRemoteConnected)
	remoteDisconnectedCh := bus.Subscribe(events.EventRemoteDisconnected)
	remoteReconnectingCh := bus.Subscribe(events.EventRemoteReconnecting)
	remoteFailedCh := bus.Subscribe(events.EventRemoteFailed)

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

	go func() {
		for event := range tokensCh {
			if usage, ok := event.Data.(agent.TokenUsage); ok {
				sessionManager.UpdateTokenUsage(usage)
				p.Send(tui.TokenUsageMsg(usage))
			}
		}
	}()

	go func() {
		for event := range errorCh {
			if errMsg, ok := event.Data.(string); ok {
				p.Send(tui.OutputMsg("\n[ERROR] " + errMsg + "\n"))
			}
		}
	}()

	// Pipeline event handlers
	go func() {
		for range pipelineStartedCh {
			p.Send(tui.OutputMsg("\n**Pipeline started**\n"))
			p.Send(tui.StateMsg("running"))
		}
	}()

	go func() {
		for range pipelineCompletedCh {
			p.Send(tui.OutputMsg("\n**Pipeline completed**\n"))
			p.Send(tui.StateMsg("idle"))
		}
	}()

	go func() {
		for range pipelineFailedCh {
			p.Send(tui.OutputMsg("\n**Pipeline failed**\n"))
			p.Send(tui.StateMsg("idle"))
		}
	}()

	go func() {
		for range stageStartedCh {
			p.Send(tui.OutputMsg("\n[Stage started]\n"))
		}
	}()

	go func() {
		for range stageCompletedCh {
			p.Send(tui.OutputMsg("\n[Stage completed]\n"))
		}
	}()

	// Remote connection event handlers
	go func() {
		for range remoteConnectingCh {
			p.Send(tui.RemoteStatusMsg{State: "connecting"})
		}
	}()

	go func() {
		for event := range remoteConnectedCh {
			var address string
			if data, ok := event.Data.(map[string]interface{}); ok {
				if addr, ok := data["address"].(string); ok {
					address = addr
				}
			}
			p.Send(tui.RemoteStatusMsg{State: "connected", Address: address})
		}
	}()

	go func() {
		for range remoteDisconnectedCh {
			p.Send(tui.RemoteStatusMsg{State: "disconnected"})
		}
	}()

	go func() {
		for event := range remoteReconnectingCh {
			attempt := 0
			if data, ok := event.Data.(map[string]interface{}); ok {
				if a, ok := data["attempt"].(int); ok {
					attempt = a
				}
			}
			p.Send(tui.RemoteStatusMsg{State: "reconnecting", Attempt: attempt})
		}
	}()

	go func() {
		for event := range remoteFailedCh {
			var errMsg string
			if data, ok := event.Data.(map[string]interface{}); ok {
				if e, ok := data["error"].(string); ok {
					errMsg = e
				}
			}
			p.Send(tui.RemoteStatusMsg{State: "failed", Error: errMsg})
		}
	}()

	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
