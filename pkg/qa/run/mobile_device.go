package run

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
	"github.com/insajin/autopus-adk/pkg/qa/mobile"
)

// mobileDeviceContext carries the resolved runtime handle alongside the opaque
// refs that are safe to publish. Handle is runtime-only.
type mobileDeviceContext struct {
	Handle    string
	Platform  string
	DeviceRef string
	TargetRef string
}

// prepareMobileDevice resolves the device, then (when managed) verifies the app
// digest BEFORE installing, probes platform tooling, and installs the app. The
// order is fixed so a digest mismatch yields zero install attempts. Every
// failure returns a *SetupGap and no flow runs (fail closed, no panic).
// @AX:WARN: [AUTO] multi-step conditional with fail-closed gate — digest check must precede install; reordering breaks the security invariant
// @AX:REASON: App artifact digest is verified before any install attempt; changing execution order could allow tampered artifacts to run (SPEC-QAMESH-008)
func prepareMobileDevice(ctx context.Context, opts Options, pack journey.Pack, runner MobileDeviceRunner) (mobileDeviceContext, *SetupGap) {
	readiness := mobile.Assess(opts.ProjectDir)
	platform := firstPlatform(readiness)
	req := mobileResolveRequest{
		DeviceRef:  pack.Mobile.DeviceTarget,
		TargetRef:  pack.Mobile.DeviceTarget,
		Platform:   platform,
		ProjectDir: opts.ProjectDir,
	}
	handle, gap := resolveDeviceHandle(runner, req, pack)
	if gap != nil {
		return mobileDeviceContext{}, gap
	}
	devCtx := mobileDeviceContext{
		Handle:    handle,
		Platform:  platform,
		DeviceRef: pack.Mobile.DeviceTarget,
		TargetRef: pack.Mobile.DeviceTarget,
	}
	if !opts.ManagedDevice {
		return devCtx, nil
	}
	declared := strings.TrimSpace(pack.Mobile.AppArtifactDigest)
	if declared == "" {
		declared = strings.TrimSpace(readiness.AppArtifact.Digest)
	}
	appPath := filepath.Join(opts.ProjectDir, filepath.FromSlash(readiness.AppArtifact.Path))
	got, err := computeFileDigest(appPath)
	if err != nil || got != declared {
		return mobileDeviceContext{}, &SetupGap{
			Adapter: laneMobileScripted,
			Reason:  mobile.ReasonAppArtifactDigestMismatch + ": computed " + got + " != declared " + declared,
		}
	}
	if gap := probeMobileTools(platform); gap != nil {
		return mobileDeviceContext{}, gap
	}
	if err := runner.InstallApp(ctx, mobileInstallRequest{
		Handle:    handle,
		AppPath:   appPath,
		AppDigest: declared,
		Platform:  platform,
	}); err != nil {
		return mobileDeviceContext{}, &SetupGap{
			Adapter: laneMobileScripted,
			Reason:  "device_boot_or_install_failed: " + err.Error(),
		}
	}
	return devCtx, nil
}

func resolveDeviceHandle(runner MobileDeviceRunner, req mobileResolveRequest, pack journey.Pack) (string, *SetupGap) {
	handle, ok := runner.Resolve(req)
	if !ok {
		return "", &SetupGap{
			Adapter:   laneMobileScripted,
			JourneyID: pack.ID,
			Reason:    mobile.ReasonDeviceRefUnresolved + ": no runtime handle for device ref " + req.DeviceRef,
		}
	}
	return handle, nil
}

func computeFileDigest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

// probeMobileTools verifies the Maestro CLI and the platform driver are present.
// Maestro is probed first so a missing toolchain surfaces "maestro" by default.
func probeMobileTools(platform string) *SetupGap {
	if _, err := exec.LookPath("maestro"); err != nil {
		return &SetupGap{Adapter: laneMobileScripted, Reason: "missing required device tool: maestro"}
	}
	driver := ""
	switch platform {
	case "android":
		driver = "adb"
	case "ios":
		driver = "xcrun"
	}
	if driver != "" {
		if _, err := exec.LookPath(driver); err != nil {
			return &SetupGap{Adapter: laneMobileScripted, Reason: "missing required device tool: " + driver}
		}
	}
	return nil
}

func firstPlatform(readiness mobile.Readiness) string {
	for _, device := range readiness.DeviceInventory.Devices {
		if strings.TrimSpace(device.Platform) != "" {
			return device.Platform
		}
	}
	for _, target := range readiness.SimulatorEmulator.Targets {
		if strings.TrimSpace(target.Platform) != "" {
			return target.Platform
		}
	}
	return ""
}
