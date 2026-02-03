# AGENT.md — Builder Agent Operating Guide

This document defines how a builder agent must operate when working on this repository.
It is authoritative. If something is unclear elsewhere, follow this document.

---

## 1. Project Goal

Build a **terminal-based, framework-agnostic agent runtime** with:

- Streaming LLM agents (OpenCode first)
- Event-driven architecture
- Deterministic persistence
- Multi-agent orchestration
- Replayable sessions
- Optional remote execution

The system must feel like a **control cockpit**, not a log viewer.

---

## 2. Non-Negotiable Principles

You MUST follow these rules at all times:

1. **UI never talks directly to agents**
   - UI subscribes to events only
2. **Everything emits events**
   - State changes, output, tokens, messages, errors
3. **Streaming first**
   - No buffering entire responses
4. **Persistence before optimization**
   - If it happened, it must be replayable
5. **Framework-agnostic core**
   - LLM-specific logic stays in adapters
6. **One phase at a time**
   - Do NOT implement future phases early

Violating these principles is a failure.

---

## 3. Architecture Overview

High-level flow:

TUI
↓
Agent Manager
↓
Agent (via Adapter)
↓
Framework (OpenCode, etc.)


All communication flows through the **event bus**.

---

## 4. Phase-Based Execution Model

Work is divided into strict phases.
You may ONLY work on the currently assigned phase.

### Phases

| Phase | Name |
|------:|------|
| 0 | Baseline Runtime |
| 1 | Token & Cost Accounting |
| 2 | Agent Templates (YAML) |
| 3 | Agent Groups & Pipelines |
| 4 | Remote Agents |
| 5 | Session Replay |

Each phase has:
- A scope
- Required components
- Exit criteria

Do not proceed until exit criteria are met.

---

## 5. Phase Instructions

### PHASE 0 — Baseline Runtime

**Goal**
Establish a stable, event-driven agent runtime with one Claude agent.

**You must implement**
- Agent interface
- OpenCode adapter with streaming
- Event bus
- Agent manager
- Context-based cancel/pause
- Session persistence (JSON is fine)
- TUI with command palette (`:` mode)

**Exit Criteria**
- OpenCode agent streams output live
- Agent can be cancelled
- Session written to disk
- UI updates only via events

---

### PHASE 1 — Token & Cost Accounting

**Goal**
Track token usage and cost per agent/session.

**You must implement**
- TokenUsage model
- Input/output token counting
- Cost estimation
- Token update events
- Persistence of token usage

**Exit Criteria**
- Token usage visible live
- Cost persisted and replayable

---

### PHASE 2 — Agent Templates (YAML)

**Goal**
Create agents declaratively.

**You must implement**
- YAML schema
- Template loader + validator
- Agent instantiation from templates
- Limits enforcement
- `:spawn-template` command

**Exit Criteria**
- Agents spawn from YAML
- Limits are enforced

---

### PHASE 3 — Agent Groups & Pipelines

**Goal**
Enable structured multi-agent workflows.

**You must implement**
- Agent-to-agent messaging
- Pipeline schema
- Pipeline runner
- Pipeline lifecycle events
- Pipeline persistence

**Exit Criteria**
- Planner → coder → reviewer pipeline works
- Progress visible and replayable

---

### PHASE 4 — Remote Agents

**Goal**
Run agents outside the local process.

**You must implement**
- Remote agent protocol
- Remote adapter
- Streaming over network
- Heartbeats and reconnects
- Security (token-based)

**Exit Criteria**
- Remote and local agents behave identically

---

### PHASE 5 — Session Replay

**Goal**
Replay sessions deterministically.

**You must implement**
- Event recording with timestamps
- Replay engine
- Replay mode in TUI
- Timeline control (scrub/speed)

**Exit Criteria**
- Sessions replay exactly
- Replay is read-only
- Debugging possible via replay alone

---

## 6. Event Model (Mandatory)

All meaningful actions MUST emit events.

Minimum required events:

- agent.started
- agent.output
- agent.tokens.updated
- agent.finished
- agent.error
- agent.message
- pipeline.started
- pipeline.step.started
- pipeline.step.finished
- pipeline.failed

Persist events for replay.

---

## 7. Persistence Rules

- Persist **sessions**, not just output
- Persist **events**, not derived state
- Prefer explicit JSON over clever storage
- Replay must not invoke live agents

---

## 8. TUI Rules

The TUI:
- Subscribes to the event bus
- Sends commands to the manager
- Never mutates agent state directly

Required TUI features:
- Agent list
- Live output pane
- Token/cost display
- Command palette (`:`)
- Replay mode (Phase 5)

---

## 9. What NOT to Do

The agent MUST NOT:

- Skip phases
- Implement future phases early
- Hardcode Claude logic outside adapters
- Add autonomous self-spawning agents
- Add a web UI
- Optimize prematurely
- Hide errors instead of emitting events

---

## 10. Definition of Done (Global)

The project is successful when:

- Multi-agent pipelines are deterministic
- Sessions are fully replayable
- Cost is visible before overruns happen
- Remote agents feel local
- The TUI feels like a control cockpit

---

## 11. Execution Instructions for the Builder Agent

When working on this repo:

1. Identify the current phase
2. Read only the relevant phase section
3. Implement the minimum required functionality
4. Emit events for everything
5. Persist all meaningful state
6. Stop when exit criteria are met

Do not “improve” unrelated code.
Do not refactor across phases unless required.

This document is the source of truth.
