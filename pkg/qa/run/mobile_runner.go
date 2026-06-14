package run

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

// MobileDeviceRunner abstracts device resolution, app installation, and flow
// execution so tests can inject a fake while the real implementation reuses the
// single shared command engine (REQ-EXEC-01). The concrete handle returned by
// Resolve is runtime-only and is never written into published evidence.
type MobileDeviceRunner interface {
	Resolve(req mobileResolveRequest) (handle string, ok bool)
	InstallApp(ctx context.Context, req mobileInstallRequest) error
	RunFlow(ctx context.Context, req mobileFlowRequest) commandResult
}

type mobileResolveRequest struct{ DeviceRef, TargetRef, Platform, ProjectDir string }

type mobileInstallRequest struct{ Handle, AppPath, AppDigest, Platform string }

type mobileFlowRequest struct {
	ProjectDir  string
	Pack        journey.Pack
	Handle      string // concrete, runtime-only — NEVER published
	ArtifactDir string
}

type realMobileDeviceRunner struct{}

// @AX:NOTE: [AUTO] magic constant — env var name for device-ref-to-handle map; must match project operator docs
const mobileDeviceMapEnv = "AUTOPUS_QA_MOBILE_DEVICE_MAP"

// Resolve maps an opaque device ref to a concrete runtime handle. The env-based
// device map is consulted first, then the project-local devices.local.json map.
// The handle is runtime-only and must never be written into published evidence;
// devices.local.json is project-local config, never an artifact.
func (realMobileDeviceRunner) Resolve(req mobileResolveRequest) (string, bool) {
	if handle, ok := lookupDeviceHandle(os.Getenv(mobileDeviceMapEnv), req.DeviceRef); ok {
		return handle, true
	}
	if req.ProjectDir != "" {
		path := filepath.Join(req.ProjectDir, ".autopus", "qa", "mobile", "devices.local.json")
		if raw, err := os.ReadFile(path); err == nil {
			if handle, ok := lookupDeviceHandle(string(raw), req.DeviceRef); ok {
				return handle, true
			}
		}
	}
	return "", false
}

// InstallApp boots the target and installs the app artifact using the platform
// tool. context.Context carries the timeout; GNU timeout is never used.
func (realMobileDeviceRunner) InstallApp(ctx context.Context, req mobileInstallRequest) error {
	switch req.Platform {
	case "android":
		if _, err := exec.LookPath("adb"); err != nil {
			return err
		}
		return exec.CommandContext(ctx, "adb", "install", "-r", req.AppPath).Run()
	case "ios":
		if _, err := exec.LookPath("xcrun"); err != nil {
			return err
		}
		if err := exec.CommandContext(ctx, "xcrun", "simctl", "boot", req.Handle).Run(); err != nil {
			return err
		}
		return exec.CommandContext(ctx, "xcrun", "simctl", "install", req.Handle, req.AppPath).Run()
	default:
		return nil
	}
}

// RunFlow reuses the shared command engine, injecting the runtime handle through
// environment variables so the concrete handle never enters the journey pack or
// published evidence.
// @AX:WARN: [AUTO] security seam — handle injected via env vars; AUTOPUS_QA_DEVICE_HANDLE and MAESTRO_DEVICE must never appear in manifests or logs
// @AX:REASON: Concrete device handles are runtime-only; leaking them into published evidence violates the redaction contract (SPEC-QAMESH-008)
func (realMobileDeviceRunner) RunFlow(ctx context.Context, req mobileFlowRequest) commandResult {
	return runCommandWithEnv(req.ProjectDir, req.Pack, req.ArtifactDir, []string{
		"AUTOPUS_QA_DEVICE_HANDLE=" + req.Handle,
		"MAESTRO_DEVICE=" + req.Handle,
	})
}

func lookupDeviceHandle(raw, deviceRef string) (string, bool) {
	if raw == "" || deviceRef == "" {
		return "", false
	}
	var mapping map[string]string
	if err := json.Unmarshal([]byte(raw), &mapping); err != nil {
		return "", false
	}
	handle, ok := mapping[deviceRef]
	return handle, ok && handle != ""
}
