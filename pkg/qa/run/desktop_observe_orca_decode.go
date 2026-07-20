package run

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

var (
	orcaUUIDPattern    = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	orcaVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)
)

type orcaEnvelope struct {
	ID     string          `json:"id"`
	OK     *bool           `json:"ok"`
	Result json.RawMessage `json:"result"`
	Meta   orcaMeta        `json:"_meta"`
}

type orcaMeta struct {
	RuntimeID string `json:"runtimeId"`
}

type orcaCapabilitiesResult struct {
	Platform        string       `json:"platform"`
	Provider        string       `json:"provider"`
	Supports        orcaSupports `json:"supports"`
	ProtocolVersion *int         `json:"protocolVersion"`
	ProviderVersion string       `json:"providerVersion"`
}

type orcaSupports struct {
	Apps        orcaAppSupports         `json:"apps"`
	Observation orcaObservationSupports `json:"observation"`
	Windows     orcaWindowSupports      `json:"windows"`
	Actions     orcaActionSupports      `json:"actions"`
	Surfaces    orcaSurfaceSupports     `json:"surfaces"`
}

type orcaAppSupports struct {
	List      *bool `json:"list"`
	PIDs      *bool `json:"pids"`
	BundleIDs *bool `json:"bundleIds"`
}

type orcaObservationSupports struct {
	OCR                 *bool `json:"ocr"`
	AnnotatedScreenshot *bool `json:"annotatedScreenshot"`
	ElementFrames       *bool `json:"elementFrames"`
	Screenshot          *bool `json:"screenshot"`
}

type orcaWindowSupports struct {
	List          *bool `json:"list"`
	MoveResize    *bool `json:"moveResize"`
	Focus         *bool `json:"focus"`
	TargetByIndex *bool `json:"targetByIndex"`
	TargetByID    *bool `json:"targetById"`
}

type orcaActionSupports struct {
	PressKey      *bool `json:"pressKey"`
	Hotkey        *bool `json:"hotkey"`
	TypeText      *bool `json:"typeText"`
	Click         *bool `json:"click"`
	PasteText     *bool `json:"pasteText"`
	Scroll        *bool `json:"scroll"`
	SetValue      *bool `json:"setValue"`
	Drag          *bool `json:"drag"`
	PerformAction *bool `json:"performAction"`
}

type orcaSurfaceSupports struct {
	Dialogs *bool `json:"dialogs"`
	Menus   *bool `json:"menus"`
	Menubar *bool `json:"menubar"`
	Dock    *bool `json:"dock"`
}

func decodeOrcaCapabilities(raw []byte) (desktopobserve.ProviderIdentity, string, error) {
	var result orcaCapabilitiesResult
	runtimeID, err := decodeOrcaSuccess(raw, &result,
		"platform", "provider", "supports", "protocolVersion", "providerVersion")
	if err != nil || !validOrcaCapabilities(result) {
		return desktopobserve.ProviderIdentity{}, "", desktopobserve.ErrMalformedEnvelope
	}
	return desktopobserve.ProviderIdentity{
		Name: result.Provider, Version: result.ProviderVersion,
		ProtocolVersion: *result.ProtocolVersion,
	}, runtimeID, nil
}

func validOrcaCapabilities(result orcaCapabilitiesResult) bool {
	if result.Platform != "darwin" || result.Provider != "orca-computer-use-macos" ||
		result.ProtocolVersion == nil || *result.ProtocolVersion != desktopobserve.ProtocolVersion ||
		!orcaVersionPattern.MatchString(result.ProviderVersion) {
		return false
	}
	values := []*bool{
		result.Supports.Apps.List, result.Supports.Apps.PIDs, result.Supports.Apps.BundleIDs,
		result.Supports.Observation.OCR, result.Supports.Observation.AnnotatedScreenshot,
		result.Supports.Observation.ElementFrames, result.Supports.Observation.Screenshot,
		result.Supports.Windows.List, result.Supports.Windows.MoveResize, result.Supports.Windows.Focus,
		result.Supports.Windows.TargetByIndex, result.Supports.Windows.TargetByID,
		result.Supports.Actions.PressKey, result.Supports.Actions.Hotkey, result.Supports.Actions.TypeText,
		result.Supports.Actions.Click, result.Supports.Actions.PasteText, result.Supports.Actions.Scroll,
		result.Supports.Actions.SetValue, result.Supports.Actions.Drag, result.Supports.Actions.PerformAction,
		result.Supports.Surfaces.Dialogs, result.Supports.Surfaces.Menus,
		result.Supports.Surfaces.Menubar, result.Supports.Surfaces.Dock,
	}
	for _, value := range values {
		if value == nil {
			return false
		}
	}
	return *result.Supports.Apps.List && *result.Supports.Apps.BundleIDs &&
		*result.Supports.Windows.List && *result.Supports.Windows.TargetByIndex &&
		*result.Supports.Observation.ElementFrames
}

type orcaPermissionsResult struct {
	Platform       string           `json:"platform"`
	HelperAppPath  string           `json:"helperAppPath"`
	OpenedSettings *bool            `json:"openedSettings"`
	LaunchedHelper *bool            `json:"launchedHelper"`
	Permissions    []orcaPermission `json:"permissions"`
	NextStep       json.RawMessage  `json:"nextStep"`
}

type orcaPermission struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func decodeOrcaPermissions(raw []byte, expectedRuntime string) (bool, error) {
	var result orcaPermissionsResult
	runtimeID, err := decodeOrcaSuccess(raw, &result,
		"platform", "helperAppPath", "openedSettings", "launchedHelper", "permissions", "nextStep")
	if err != nil || runtimeID != expectedRuntime || result.Platform != "darwin" ||
		result.HelperAppPath == "" || result.OpenedSettings == nil || *result.OpenedSettings ||
		result.LaunchedHelper == nil || *result.LaunchedHelper || len(result.Permissions) != 2 ||
		len(result.NextStep) == 0 {
		return false, desktopobserve.ErrMalformedEnvelope
	}
	statuses := make(map[string]string, 2)
	for _, permission := range result.Permissions {
		if (permission.ID != "accessibility" && permission.ID != "screenshots") ||
			(permission.Status != "granted" && permission.Status != "denied" &&
				permission.Status != "not_determined") || statuses[permission.ID] != "" {
			return false, desktopobserve.ErrMalformedEnvelope
		}
		statuses[permission.ID] = permission.Status
	}
	return statuses["accessibility"] == "granted", nil
}

func decodeOrcaSuccess(raw []byte, target any, resultKeys ...string) (string, error) {
	var envelope orcaEnvelope
	if err := decodeOrcaObject(raw, &envelope, "id", "ok", "result", "_meta"); err != nil ||
		envelope.OK == nil || !*envelope.OK || !orcaUUIDPattern.MatchString(envelope.ID) ||
		!orcaUUIDPattern.MatchString(envelope.Meta.RuntimeID) || len(envelope.Result) == 0 ||
		bytes.Equal(bytes.TrimSpace(envelope.Result), []byte("null")) {
		return "", desktopobserve.ErrMalformedEnvelope
	}
	if err := decodeOrcaObject(envelope.Result, target, resultKeys...); err != nil {
		return "", err
	}
	return envelope.Meta.RuntimeID, nil
}

func decodeOrcaObject(raw []byte, target any, keys ...string) error {
	if len(raw) == 0 || len(raw) > orcaMaxOutputBytes || !utf8.Valid(raw) {
		if len(raw) > orcaMaxOutputBytes {
			return desktopobserve.ErrEnvelopeTooLarge
		}
		return desktopobserve.ErrMalformedEnvelope
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := rejectDesktopV2DuplicateKeys(decoder); err != nil {
		return err
	}
	decoder = json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			return fmt.Errorf("%w: orca result", desktopobserve.ErrUnknownField)
		}
		return desktopobserve.ErrMalformedEnvelope
	}
	if err := desktopV2JSONEOF(decoder); err != nil {
		return err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil || len(object) != len(keys) {
		return desktopobserve.ErrMissingField
	}
	for _, key := range keys {
		if _, ok := object[key]; !ok {
			return desktopobserve.ErrMissingField
		}
	}
	return nil
}
