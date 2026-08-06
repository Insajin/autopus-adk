package cli

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

const workflowContextVerifiedExecRPCTimeout = 20 * time.Second

func runWorkflowContextVerifiedExecRPCSmoke(
	ctx context.Context,
	executable string,
	identity pipelineOMPExecutableIdentity,
) (int64, error) {
	return runWorkflowContextVerifiedExecRPCSmokeWithModel(ctx, executable, identity, "canary", "canary")
}

func runWorkflowContextVerifiedExecRPCSmokeWithModel(
	ctx context.Context,
	executable string,
	identity pipelineOMPExecutableIdentity,
	provider string,
	model string,
) (int64, error) {
	return runWorkflowContextVerifiedExecRPCSmokeWithCleanup(
		ctx, executable, identity, provider, model, os.RemoveAll,
	)
}

func runWorkflowContextVerifiedExecRPCSmokeWithCleanup(
	ctx context.Context,
	executable string,
	identity pipelineOMPExecutableIdentity,
	provider string,
	model string,
	cleanup func(string) error,
) (calls int64, resultErr error) {
	root, err := os.MkdirTemp("", "autopus-verified-exec-smoke-")
	if err != nil {
		return 0, err
	}
	defer func() { resultErr = errors.Join(resultErr, cleanup(root)) }()
	if err := os.Chmod(root, 0o700); err != nil {
		return 0, err
	}
	projectDir, runtimeBase := filepath.Join(root, "project"), filepath.Join(root, "runtime")
	if err := os.Mkdir(projectDir, 0o700); err != nil {
		return 0, err
	}
	if err := os.Mkdir(runtimeBase, 0o700); err != nil {
		return 0, err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	var providerCalls atomic.Int64
	server := &http.Server{
		ReadHeaderTimeout: time.Second,
		Handler: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			providerCalls.Add(1)
			writer.WriteHeader(http.StatusServiceUnavailable)
		}),
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	serverClosed := false
	closeServer := func() error {
		if serverClosed {
			return nil
		}
		serverClosed = true
		closeErr := server.Close()
		serveErr := <-serveDone
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		return errors.Join(closeErr, serveErr)
	}
	defer func() { resultErr = errors.Join(resultErr, closeServer()) }()
	select {
	case serveErr := <-serveDone:
		serverClosed = true
		return 0, errors.Join(errors.New("provider spy stopped before readiness probe"), serveErr)
	default:
	}
	token, err := workflowContextVerifiedExecDummyToken()
	if err != nil {
		return 0, err
	}
	nonce, err := newWorkflowContextRunNonceHash()
	if err != nil {
		return 0, err
	}
	binding := WorkflowContextBridgeBinding{
		SchemaVersion: workflowContextBridgeSchemaVersion,
		BindingHash:   workflowContextRuntimeHash("release-canary-binding:" + nonce),
		OptionsHash:   workflowContextRuntimeHash("release-canary-options:" + nonce),
		SessionHash:   workflowContextRuntimeHash("release-canary-session:" + nonce),
		NonceHash:     nonce,
	}
	selector := provider + "/" + model
	active := pipelineOMPActiveProcessConfig{
		backend: pipelineOMPBackendConfig{
			Executable: executable, executableID: identity,
			ProjectDir: projectDir, RuntimeBase: runtimeBase,
			PhaseModels:        map[pipeline.PhaseID]string{pipeline.PhasePlan: selector},
			ModelContextWindow: pipelineOMPActiveDefaultContextWindow,
			MaxTime:            workflowContextVerifiedExecRPCTimeout,
		},
		candidate: pipelineOMPManagedActiveCandidate{Provider: provider, Model: model},
		binding:   binding, endpoint: "http://" + listener.Addr().String(), credential: token,
	}
	probeCtx, cancel := context.WithTimeout(ctx, workflowContextVerifiedExecRPCTimeout)
	defer cancel()
	process, err := startPipelineOMPActiveProcess(probeCtx, active)
	if err != nil {
		return providerCalls.Load(), err
	}
	if err := process.Close(); err != nil {
		return providerCalls.Load(), err
	}
	if err := closeServer(); err != nil {
		return providerCalls.Load(), err
	}
	if calls := providerCalls.Load(); calls != 0 {
		return calls, errors.New("provider endpoint received a request before prompt dispatch")
	}
	return 0, nil
}

func workflowContextVerifiedExecDummyToken() (string, error) {
	var body [32]byte
	if _, err := rand.Read(body[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(body[:]), nil
}
