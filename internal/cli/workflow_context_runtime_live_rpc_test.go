package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type workflowContextLiveDriver struct {
	executable    string
	layout        workflowContextLiveLayout
	endpoint      string
	beforePost    func() error
	sandboxed     bool
	nativeStart   int
	nativeEnd     int
	providerTurns int
}

func (d *workflowContextLiveDriver) ArtifactCount(context.Context) (int, error) {
	return workflowContextLiveRootCount(d.layout.runtime), nil
}

func (d *workflowContextLiveDriver) Cleanup(context.Context) error {
	if d.layout.runtime == "" || d.layout.base == "" || d.layout.runtime == d.layout.base {
		return errors.New("unsafe-runtime-root")
	}
	relative, err := filepathRel(d.layout.base, d.layout.runtime)
	if err != nil || relative != "omp-runtime" {
		return errors.New("runtime-root-not-owned")
	}
	if err := os.RemoveAll(d.layout.runtime); err != nil {
		return errors.New("runtime-cleanup")
	}
	return nil
}

func (d *workflowContextLiveDriver) Run(
	ctx context.Context, emit func(WorkflowContextRuntimeEvent) error,
) (runErr error) {
	cmd := exec.CommandContext(ctx, d.executable,
		"--mode", "rpc", "--no-session", "--session-dir", d.layout.sessions,
		"--cwd", d.layout.workspace, "--model", "contextfake/"+workflowContextLiveModel,
		"--config", d.layout.overlay, "--no-tools", "--no-lsp", "--no-pty",
		"--no-extensions", "--no-skills", "--no-rules", "--no-title", "--max-time", "30s",
	)
	cmd.Dir, cmd.Env, cmd.Stderr = d.layout.workspace, d.layout.env(), io.Discard
	cmd.WaitDelay = 500 * time.Millisecond
	sandboxed, err := configureWorkflowContextLiveSandbox(cmd, d.endpoint)
	if err != nil {
		return errors.New("network-sandbox-unavailable")
	}
	d.sandboxed = sandboxed
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return errors.New("rpc-stdin")
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.New("rpc-stdout")
	}
	processTree := newWorkflowContextLiveProcessTree(cmd)
	if err := processTree.Start(); err != nil {
		return errors.New("rpc-start")
	}
	defer func() {
		_ = stdin.Close()
		if closeErr := processTree.Close(); closeErr != nil {
			runErr = errors.New("rpc-process-tree-cleanup")
		}
	}()

	frames, done := workflowContextLiveFrames(stdout)
	observed := make([]string, 0, 16)
	if err := awaitWorkflowContextLiveReady(ctx, frames, done, &observed); err != nil {
		return err
	}
	encoder := json.NewEncoder(stdin)
	commands := []map[string]any{
		{"id": "retry-off", "type": "set_auto_retry", "enabled": false},
		{"id": "compaction-on", "type": "set_auto_compaction", "enabled": true},
	}
	for _, command := range commands {
		if encoder.Encode(command) != nil {
			return errors.New("rpc-command-write")
		}
	}
	if err := awaitWorkflowContextLiveResponse(ctx, frames, done, &observed, "compaction-on"); err != nil {
		return err
	}
	if encoder.Encode(map[string]any{
		"id": "seed-turn", "type": "prompt", "message": strings.Repeat("bounded seed context ", 320),
	}) != nil {
		return errors.New("rpc-seed-command-write")
	}
	if err := d.awaitCompaction(ctx, frames, done, &observed, encoder, emit); err != nil {
		return err
	}
	if err := stdin.Close(); err != nil {
		return errors.New("rpc-stdin-close")
	}
	if err := processTree.Close(); err != nil {
		return errors.New("rpc-process-tree-cleanup")
	}
	return nil
}

func (d *workflowContextLiveDriver) awaitCompaction(
	ctx context.Context, frames <-chan []byte, done <-chan error, observed *[]string,
	encoder *json.Encoder,
	emit func(WorkflowContextRuntimeEvent) error,
) error {
	started := false
	thresholdTurn := false
	providerTurns := 0
	for {
		frame, err := nextWorkflowContextLiveFrame(ctx, frames, done)
		if err != nil {
			return fmt.Errorf("native-compaction-missing:start=%t observed=%s", started, joinLiveEvents(*observed))
		}
		var event struct {
			Type, Reason, Action, ErrorMessage string
			Aborted, Skipped                   bool
			Result                             json.RawMessage
			Success                            *bool `json:"success"`
			Command                            string
		}
		if json.Unmarshal(frame, &event) != nil {
			continue
		}
		appendLiveEvent(observed, event.Type)
		if event.Type == "response" && event.Success != nil && !*event.Success {
			return fmt.Errorf("rpc-command-failed:%s", event.Command)
		}
		switch event.Type {
		case "turn_end":
			providerTurns++
			d.providerTurns = providerTurns
			if providerTurns > 2 {
				return fmt.Errorf("provider-turn-budget-exceeded:%d", providerTurns)
			}
		case "agent_end":
			if thresholdTurn || providerTurns != 1 {
				continue
			}
			if encoder.Encode(map[string]any{
				"id": "threshold-turn", "type": "prompt",
				"message": strings.Repeat("bounded threshold context ", 320),
			}) != nil {
				return errors.New("rpc-threshold-command-write")
			}
			thresholdTurn = true
		case "auto_compaction_start":
			if !thresholdTurn || providerTurns != 2 || started ||
				event.Reason != "threshold" || event.Action != "snapcompact" {
				return fmt.Errorf("native-compaction-start-invalid:threshold=%t turns=%d started=%t reason=%s action=%s observed=%s",
					thresholdTurn, providerTurns, started, event.Reason, event.Action, joinLiveEvents(*observed))
			}
			if err := emit(WorkflowContextRuntimeEvent{Kind: WorkflowContextEventPreCompaction}); err != nil {
				return err
			}
			started = true
			d.nativeStart++
		case "auto_compaction_end":
			resultMissing := len(event.Result) == 0 || string(event.Result) == "null"
			if !started || event.Action != "snapcompact" || event.Aborted || event.Skipped ||
				event.ErrorMessage != "" || resultMissing {
				return fmt.Errorf("native-compaction-end-invalid:action=%s aborted=%t skipped=%t result=%t",
					event.Action, event.Aborted, event.Skipped, !resultMissing)
			}
			if err := emit(WorkflowContextRuntimeEvent{
				Kind: WorkflowContextEventCompacted, HistoryAfterTokens: map[string]int{"old-read": 2},
			}); err != nil {
				return err
			}
			if d.beforePost != nil {
				if err := d.beforePost(); err != nil {
					return err
				}
			}
			d.nativeEnd++
			return emit(WorkflowContextRuntimeEvent{Kind: WorkflowContextEventPostCompaction})
		}
	}
}

func awaitWorkflowContextLiveResponse(
	ctx context.Context, frames <-chan []byte, done <-chan error, observed *[]string, id string,
) error {
	for {
		frame, err := nextWorkflowContextLiveFrame(ctx, frames, done)
		if err != nil {
			return errors.New("rpc-command-response-missing")
		}
		var response struct {
			ID, Type, Command string
			Success           *bool
		}
		if json.Unmarshal(frame, &response) != nil {
			continue
		}
		appendLiveEvent(observed, response.Type)
		if response.Type != "response" || response.ID != id {
			continue
		}
		if response.Success == nil || !*response.Success {
			return fmt.Errorf("rpc-command-failed:%s", response.Command)
		}
		return nil
	}
}

func workflowContextLiveFrames(reader io.Reader) (<-chan []byte, <-chan error) {
	frames, done := make(chan []byte, 128), make(chan error, 1)
	go func() {
		defer close(frames)
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 64*1024), 2<<20)
		for scanner.Scan() {
			frames <- append([]byte(nil), scanner.Bytes()...)
		}
		done <- scanner.Err()
	}()
	return frames, done
}

func awaitWorkflowContextLiveReady(
	ctx context.Context, frames <-chan []byte, done <-chan error, observed *[]string,
) error {
	for {
		frame, err := nextWorkflowContextLiveFrame(ctx, frames, done)
		if err != nil {
			return errors.New("rpc-ready-missing")
		}
		var event struct{ Type string }
		if json.Unmarshal(frame, &event) == nil {
			appendLiveEvent(observed, event.Type)
			if event.Type == "ready" {
				return nil
			}
		}
	}
}

func nextWorkflowContextLiveFrame(ctx context.Context, frames <-chan []byte, done <-chan error) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case frame, ok := <-frames:
		if ok {
			return frame, nil
		}
		select {
		case err := <-done:
			return nil, err
		default:
			return nil, io.EOF
		}
	}
}

func appendLiveEvent(events *[]string, event string) {
	if event != "" && len(*events) < 32 {
		*events = append(*events, event)
	}
}

func joinLiveEvents(events []string) string { return strings.Join(events, ",") }

func filepathRel(base, target string) (string, error) {
	return filepath.Rel(base, target)
}
