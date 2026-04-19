// Package lsp는 Language Server Protocol 클라이언트를 제공한다.
package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// Diagnostic는 LSP 진단 메시지이다.
type Diagnostic struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // error, warning, info, hint
}

// Location은 파일 내 위치이다.
type Location struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Col  int    `json:"col"`
}

// Symbol은 코드 심볼이다.
type Symbol struct {
	Name     string   `json:"name"`
	Kind     string   `json:"kind"` // function, class, variable, etc.
	Location Location `json:"location"`
}

// Client는 LSP 서버와 통신하는 클라이언트이다.
type Client struct {
	cmd    *exec.Cmd
	stdin  interface{ Write([]byte) (int, error) }
	stdout interface{ Read([]byte) (int, error) }
	ctx    context.Context
	cancel context.CancelFunc
	seq    int
}

// NewClient는 LSP 서버 프로세스를 시작하고 클라이언트를 생성한다.
func NewClient(serverCmd string, args []string) (*Client, error) {
	// 서버 명령 존재 여부 확인
	path, err := exec.LookPath(serverCmd)
	if err != nil {
		return nil, fmt.Errorf("LSP 서버 명령을 찾을 수 없습니다 %q: %w", serverCmd, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, path, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdin 파이프 생성 실패: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdout 파이프 생성 실패: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("LSP 서버 시작 실패: %w", err)
	}

	return &Client{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Initialize는 LSP 서버를 초기화한다.
func (c *Client) Initialize(rootURI string) error {
	c.seq++
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      c.seq,
		"method":  "initialize",
		"params": map[string]interface{}{
			"rootUri":      rootURI,
			"capabilities": map[string]interface{}{},
		},
	}
	return c.sendRequest(req)
}

// Shutdown는 LSP 서버를 종료한다.
func (c *Client) Shutdown() error {
	c.seq++
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      c.seq,
		"method":  "shutdown",
	}
	_ = c.sendRequest(req)
	c.cancel()
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Wait()
	}
	return nil
}

// sendRequest는 JSON-RPC 요청을 전송한다.
func (c *Client) sendRequest(req map[string]interface{}) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("요청 직렬화 실패: %w", err)
	}

	msg := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(data), data)
	_, err = c.stdin.Write([]byte(msg))
	return err
}
