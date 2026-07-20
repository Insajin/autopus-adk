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

const (
	orcaExecutableName = "orca"
	orcaAppBundleID    = "co.autopus.desktop"
	orcaAppName        = "Autopus Desktop"
	orcaWindowTitle    = "Autopus"
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
