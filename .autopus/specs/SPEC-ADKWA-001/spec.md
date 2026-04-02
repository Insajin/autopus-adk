# SPEC: ADK Worker Approval Flow — input-required Status Integration

**SPEC-ID**: SPEC-ADKWA-001
**Status**: completed
**Created**: 2026-04-02
**Source**: SPEC-ADKWQ-001 deferred item (REQ-A2A-H01)
**Parent**: SPEC-ADKW-001

---

## Overview

Claude Code subprocess calls `request_approval` via the backend MCP SSE endpoint.
The backend relays the approval request to the worker via A2A WebSocket.
The worker surfaces the dialog in TUI, collects the decision, and sends it back.

```
Claude Code subprocess
  → MCP request_approval → Backend /mcp/sse
  → Backend relays → A2A WebSocket → Worker
  → Worker: status → input-required, show TUI dialog
  → User decision (approve/deny/skip)
  → Worker → A2A → Backend → MCP response → Claude Code resumes
```

## Domain: A2A Approval Handling

### REQ-APR-01 [P0]
WHEN the backend sends an A2A `tasks/approval` method over WebSocket,
THE SYSTEM SHALL parse the `ApprovalRequestParams` (task_id, action, risk_level, context),
update the task status to `input-required`, and emit an `ApprovalRequestMsg` to the TUI.

### REQ-APR-02 [P0]
WHEN the TUI user selects a decision (approve/deny/skip),
THE SYSTEM SHALL send an A2A `tasks/approvalResponse` notification to the backend with the decision
and update the task status back to `working`.

### REQ-APR-03 [P0]
THE SYSTEM SHALL define an `ApprovalCallback` function type on the Server struct,
settable via `ServerConfig`, so the TUI layer can receive approval requests without direct coupling.

## Domain: A2A Types

### REQ-APR-04 [P0]
THE SYSTEM SHALL add the following types to `a2a/types.go`:
- `MethodApproval = "tasks/approval"` constant
- `MethodApprovalResponse = "tasks/approvalResponse"` constant
- `ApprovalRequestParams { TaskID, Action, RiskLevel, Context string }`
- `ApprovalResponseParams { TaskID, Decision string }` (decision: "approve", "deny", "skip")

## Domain: TUI Approval Callback

### REQ-APR-05 [P0]
WHEN the TUI approval dialog receives a keypress (a/d/s),
THE SYSTEM SHALL invoke a registered callback `OnApprovalDecision(taskID, decision string)` instead of silently clearing the pointer.

### REQ-APR-06 [P1]
WHEN the TUI approval dialog receives `v` (view diff),
THE SYSTEM SHALL invoke a `OnViewDiff(taskID string)` callback to display the pending change detail.

## Domain: Integration Wiring

### REQ-APR-07 [P0]
THE SYSTEM SHALL wire the A2A Server's approval callback to the TUI's message channel:
- Server receives `tasks/approval` → calls `ApprovalCallback`
- `ApprovalCallback` sends `ApprovalRequestMsg` to TUI program
- TUI displays dialog → user decides → TUI calls `OnApprovalDecision`
- `OnApprovalDecision` calls `Server.SendApprovalResponse(taskID, decision)`

### REQ-APR-08 [P0]
THE SYSTEM SHALL add `SendApprovalResponse(taskID, decision string) error` to the Server,
which sends a `tasks/approvalResponse` JSON-RPC notification over the WebSocket.
