package run

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"os"
	"os/exec"
	"sort"
	"sync"
	"time"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

// No compiled-in app identity lives here any more. SPEC-QAMESH-013 REQ-3: the
// bundle id, app name, and window title arrive per run from the pack through
// desktopProviderTarget, because a constant here made `desktop-native`
// observable only for Autopus's own desktop app.
const (
	orcaExecutableName = "orca"
	orcaMaxOutputBytes = 64 * 1024
)

type orcaCommandExecutor interface {
	Run(context.Context, string, []string) ([]byte, error)
}

type processOrcaCommandExecutor struct {
	executableInfo os.FileInfo
}

func (executor *processOrcaCommandExecutor) Run(
	ctx context.Context,
	path string,
	arguments []string,
) ([]byte, error) {
	if executor == nil || executor.executableInfo == nil {
		return nil, errDesktopProviderUnavailable
	}
	info, err := os.Stat(path)
	if err != nil || !os.SameFile(executor.executableInfo, info) || !desktopExecutableFile(path) {
		return nil, errDesktopProviderUnavailable
	}
	command := exec.CommandContext(ctx, path, arguments...)
	command.Env = orcaProviderEnvironment()
	command.WaitDelay = 250 * time.Millisecond
	stdout := &boundedOrcaOutput{}
	stderr := &boundedOrcaOutput{}
	command.Stdout = stdout
	command.Stderr = stderr
	runErr := command.Run()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if stdout.overflow || stderr.overflow {
		return nil, desktopobserve.ErrEnvelopeTooLarge
	}
	if stderr.buffer.Len() != 0 || runErr != nil || stdout.buffer.Len() == 0 {
		return nil, errDesktopProviderUnavailable
	}
	return append([]byte(nil), stdout.buffer.Bytes()...), nil
}

type boundedOrcaOutput struct {
	buffer   bytes.Buffer
	overflow bool
}

func (output *boundedOrcaOutput) Write(value []byte) (int, error) {
	remaining := orcaMaxOutputBytes - output.buffer.Len()
	if remaining > 0 {
		if remaining > len(value) {
			remaining = len(value)
		}
		_, _ = output.buffer.Write(value[:remaining])
	}
	if remaining < len(value) {
		output.overflow = true
	}
	return len(value), nil
}

func orcaProviderEnvironment() []string {
	values := make(map[string]string)
	for _, key := range []string{"HOME", "LANG", "LC_ALL", "LOGNAME", "PATH", "SHELL", "TMPDIR", "USER"} {
		if value, ok := os.LookupEnv(key); ok {
			values[key] = value
		}
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	environment := make([]string, 0, len(keys))
	for _, key := range keys {
		environment = append(environment, key+"="+values[key])
	}
	return environment
}

type orcaWindowBinding struct {
	id     int
	index  int
	pid    int
	x      int
	y      int
	width  int
	height int
}

type orcaDesktopClient struct {
	mu           sync.Mutex
	path         string
	executor     orcaCommandExecutor
	random       io.Reader
	runtimeID    string
	identity     desktopobserve.ProviderIdentity
	capabilities []desktopobserve.Operation
	targetPID    int
	window       orcaWindowBinding
	target       desktopProviderTarget
}

// applyTarget records which app this client observes. The runner calls it once
// before the handshake, so every provider request addresses the pack's app.
func (client *orcaDesktopClient) applyTarget(target desktopProviderTarget) {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.target = target
}

// appArgument is the value passed to the provider's --app flag. A pack with no
// provider_app_id cannot reach here: the journey layer rejects it, and the empty
// fallback below would address nothing rather than silently observing Autopus.
func (client *orcaDesktopClient) appArgument() string {
	return client.target.ProviderAppID
}

func (client *orcaDesktopClient) expectedAppName() string {
	return client.target.AppName
}

func (client *orcaDesktopClient) expectedWindowTitle() string {
	return client.target.WindowTitle
}

// appRefLabel and windowRefLabel are the aliases the harness speaks internally.
// They echo the pack's refs so the runner's request and the client's replies
// agree on identity without either side inventing one.
func (client *orcaDesktopClient) appRefLabel() string {
	return client.target.AppRef
}

func (client *orcaDesktopClient) windowRefLabel() string {
	return client.target.WindowRef
}

func newOrcaDesktopClient(path string) (desktopProviderClient, error) {
	info, err := os.Stat(path)
	if err != nil || !desktopExecutableFile(path) {
		return nil, errDesktopProviderUnavailable
	}
	return newOrcaDesktopClientWith(
		path,
		&processOrcaCommandExecutor{executableInfo: info},
		rand.Reader,
	)
}

func newOrcaDesktopClientWith(
	path string,
	executor orcaCommandExecutor,
	random io.Reader,
) (*orcaDesktopClient, error) {
	if path == "" || executor == nil || random == nil {
		return nil, errDesktopProviderUnavailable
	}
	return &orcaDesktopClient{path: path, executor: executor, random: random}, nil
}
