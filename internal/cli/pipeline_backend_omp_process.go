package cli

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: physical frames are capped at 1 MiB and v2 reassembly at 64 MiB.
const pipelineOMPMaxPhysicalFrame = 1 << 20
const pipelineOMPMaxReassembledFrame = 64 << 20

type pipelineOMPRPCChunk struct {
	Type       string `json:"type"`
	ChunkID    string `json:"chunkId"`
	Index      int    `json:"index"`
	Count      int    `json:"count"`
	ByteLength int    `json:"byteLength"`
	Data       string `json:"data"`
}

type pipelineOMPChunkSequence struct {
	id          string
	data        []byte
	next, count int
	byteLength  int
}

type pipelineOMPProcess struct {
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	frames      <-chan pipelineOMPRPCFrame
	done        <-chan error
	runtimeRoot string
	runtimeInfo os.FileInfo
	stopFrames  context.CancelFunc
	closeOnce   sync.Once
	closeErr    error
}

func startPipelineOMPProcess(ctx context.Context, config pipelineOMPBackendConfig) (*pipelineOMPProcess, error) { // @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: isolated OMP process boundary.
	// @AX:REASON [AUTO]: Runtime ownership, argv, environment, process group, bounded frame reader, and readiness are established together.
	// @AX:WARN [AUTO]: OMP process startup has more than eight runtime, pipe, launch, and readiness branches.
	// @AX:REASON [AUTO]: Every partial startup path must release the private runtime and reject an unready child before RPC use.
	runtimeRoot, err := os.MkdirTemp(config.RuntimeBase, "pipeline-task-")
	if err != nil {
		return nil, fmt.Errorf("create OMP pipeline task runtime: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(runtimeRoot) }
	if err := os.Chmod(runtimeRoot, 0o700); err != nil {
		cleanup()
		return nil, fmt.Errorf("secure OMP pipeline task runtime: %w", err)
	}
	runtimeInfo, err := os.Lstat(runtimeRoot)
	if err != nil {
		cleanup()
		return nil, err
	}
	sessionDir := filepath.Join(runtimeRoot, "sessions")
	if err := os.Mkdir(sessionDir, 0o700); err != nil {
		cleanup()
		return nil, fmt.Errorf("create OMP pipeline session directory: %w", err)
	}
	privateExecutable, err := materializePipelineOMPExecutable(config.Executable, config.executableID, runtimeRoot)
	if err != nil {
		cleanup()
		return nil, err
	}
	cmd := exec.Command(privateExecutable, "--mode", "rpc", "--no-session", "--no-extensions", "--session-dir", sessionDir)
	cmd.Dir = config.ProjectDir
	cmd.Env = append([]string(nil), config.canonicalEnv...)
	cmd.Stderr = io.Discard
	cmd.WaitDelay = 500 * time.Millisecond
	if err := configureWorkflowContextManagedRPCProcessGroup(cmd); err != nil {
		cleanup()
		return nil, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cleanup()
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		cleanup()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		cleanup()
		return nil, fmt.Errorf("start OMP pipeline RPC: %w", err)
	}
	frameCtx, stopFrames := context.WithCancel(context.Background())
	frames, done := readPipelineOMPFrames(frameCtx, stdout)
	process := &pipelineOMPProcess{
		cmd: cmd, stdin: stdin, frames: frames, done: done,
		runtimeRoot: runtimeRoot, runtimeInfo: runtimeInfo, stopFrames: stopFrames,
	}
	readyCtx, cancel := context.WithTimeout(ctx, config.MaxTime)
	defer cancel()
	frame, err := process.next(readyCtx)
	if err != nil || frame.Type != "ready" {
		_ = process.Close()
		return nil, errors.New("OMP pipeline RPC readiness was not observed")
	}
	return process, nil
}

func readPipelineOMPFrames(ctx context.Context, stdout io.Reader) (<-chan pipelineOMPRPCFrame, <-chan error) {
	frames := make(chan pipelineOMPRPCFrame)
	done := make(chan error, 1)
	go func() {
		defer close(frames)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64<<10), pipelineOMPMaxPhysicalFrame+1)
		var chunks *pipelineOMPChunkSequence
		for scanner.Scan() {
			frame, nextChunks, complete, err := decodePipelineOMPPhysicalFrame(scanner.Bytes(), chunks)
			if err != nil {
				done <- err
				return
			}
			chunks = nextChunks
			if !complete {
				continue
			}
			select {
			case frames <- frame:
			case <-ctx.Done():
				done <- ctx.Err()
				return
			}
		}
		if scanner.Err() != nil {
			done <- scanner.Err()
		} else if chunks != nil {
			done <- errors.New("interrupted OMP pipeline RPC chunk sequence")
		} else {
			done <- nil
		}
	}()
	return frames, done
}

func decodePipelineOMPPhysicalFrame(
	physical []byte,
	sequence *pipelineOMPChunkSequence,
) (pipelineOMPRPCFrame, *pipelineOMPChunkSequence, bool, error) { // @AX:WARN [AUTO]: Physical-frame and rpc_chunk admission has cyclomatic complexity 29.
	// @AX:REASON [AUTO]: Size, UTF-8, ordering, metadata, base64, and logical JSON checks prevent ambiguous or oversized RPC input.
	if len(physical) > pipelineOMPMaxPhysicalFrame {
		return pipelineOMPRPCFrame{}, nil, false, errors.New("OMP pipeline RPC physical frame exceeds limit")
	}
	if !utf8.Valid(physical) {
		return pipelineOMPRPCFrame{}, nil, false, errors.New("OMP pipeline RPC frame is not UTF-8")
	}
	var envelope struct {
		Type string `json:"type"`
	}
	if err := decodePipelineOMPJSONObject(physical, &envelope, false); err != nil || envelope.Type == "" {
		return pipelineOMPRPCFrame{}, nil, false, errors.New("invalid OMP pipeline RPC frame")
	}
	if envelope.Type != "rpc_chunk" {
		if sequence != nil {
			return pipelineOMPRPCFrame{}, nil, false, errors.New("interrupted OMP pipeline RPC chunk sequence")
		}
		var frame pipelineOMPRPCFrame
		if err := decodePipelineOMPJSONObject(physical, &frame, false); err != nil {
			return pipelineOMPRPCFrame{}, nil, false, err
		}
		return frame, nil, true, nil
	}
	var chunk pipelineOMPRPCChunk
	if err := decodePipelineOMPJSONObject(physical, &chunk, true); err != nil {
		return pipelineOMPRPCFrame{}, nil, false, err
	}
	if chunk.ChunkID == "" || chunk.Count <= 0 || chunk.Index < 0 || chunk.Index >= chunk.Count ||
		chunk.ByteLength <= 0 || chunk.ByteLength > pipelineOMPMaxReassembledFrame {
		return pipelineOMPRPCFrame{}, nil, false, errors.New("invalid OMP pipeline RPC chunk metadata")
	}
	if sequence == nil {
		if chunk.Index != 0 {
			return pipelineOMPRPCFrame{}, nil, false, errors.New("OMP pipeline RPC chunk sequence did not start at zero")
		}
		sequence = &pipelineOMPChunkSequence{id: chunk.ChunkID, count: chunk.Count, byteLength: chunk.ByteLength}
	}
	if chunk.ChunkID != sequence.id || chunk.Index != sequence.next || chunk.Count != sequence.count ||
		chunk.ByteLength != sequence.byteLength {
		return pipelineOMPRPCFrame{}, nil, false, errors.New("interleaved or invalid OMP pipeline RPC chunk sequence")
	}
	decoded, err := base64.StdEncoding.DecodeString(chunk.Data)
	if err != nil || len(decoded) == 0 || len(sequence.data)+len(decoded) > sequence.byteLength {
		return pipelineOMPRPCFrame{}, nil, false, errors.New("invalid OMP pipeline RPC chunk data")
	}
	sequence.data = append(sequence.data, decoded...)
	sequence.next++
	if sequence.next != sequence.count {
		return pipelineOMPRPCFrame{}, sequence, false, nil
	}
	logical := sequence.data
	if len(logical) != sequence.byteLength || !utf8.Valid(logical) {
		return pipelineOMPRPCFrame{}, nil, false, errors.New("OMP pipeline RPC reassembled frame is invalid")
	}
	var frame pipelineOMPRPCFrame
	if err := decodePipelineOMPJSONObject(logical, &frame, false); err != nil || frame.Type == "rpc_chunk" {
		return pipelineOMPRPCFrame{}, nil, false, errors.New("OMP pipeline RPC reassembled JSON is invalid")
	}
	return frame, nil, true, nil
}

func decodePipelineOMPJSONObject(data []byte, target any, strict bool) error {
	if err := rejectDuplicatePipelineOMPJSON(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	if strict {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("OMP pipeline RPC frame contains trailing JSON")
	}
	return nil
}

func (process *pipelineOMPProcess) send(command pipelineOMPRPCCommand) error {
	data, err := json.Marshal(command)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if _, err := process.stdin.Write(data); err != nil {
		return fmt.Errorf("write OMP pipeline RPC command: %w", err)
	}
	return nil
}

func (process *pipelineOMPProcess) next(ctx context.Context) (pipelineOMPRPCFrame, error) {
	select {
	case <-ctx.Done():
		return pipelineOMPRPCFrame{}, ctx.Err()
	case frame, ok := <-process.frames:
		if !ok {
			select {
			case err := <-process.done:
				if err != nil {
					return pipelineOMPRPCFrame{}, err
				}
			default:
			}
			return pipelineOMPRPCFrame{}, io.EOF
		}
		return frame, nil
	}
}

func (process *pipelineOMPProcess) Close() error {
	if process == nil {
		return nil
	}
	process.closeOnce.Do(func() {
		if process.stopFrames != nil {
			process.stopFrames()
		}
		_ = process.stdin.Close()
		killErr := terminateWorkflowContextManagedRPCProcessGroup(process.cmd)
		waitErr := process.cmd.Wait()
		var exitErr *exec.ExitError
		if waitErr != nil && errors.As(waitErr, &exitErr) {
			waitErr = nil
		}
		process.closeErr = errors.Join(killErr, waitErr, process.cleanupRuntime())
	})
	return process.closeErr
}

func (process *pipelineOMPProcess) cleanupRuntime() error {
	info, err := os.Lstat(process.runtimeRoot)
	baseRelative, relErr := filepath.Rel(filepath.Dir(process.runtimeRoot), process.runtimeRoot)
	if err != nil || relErr != nil || baseRelative == "." || strings.HasPrefix(baseRelative, "..") ||
		!os.SameFile(process.runtimeInfo, info) || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("OMP pipeline task runtime ownership changed")
	}
	return os.RemoveAll(process.runtimeRoot)
}
