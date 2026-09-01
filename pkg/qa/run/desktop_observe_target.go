package run

import "strings"

// SPEC-QAMESH-013 REQ-3: the provider must observe the app the pack names.
//
// Before this existed, the Orca client asked the provider about a compiled-in
// bundle id and compared a compiled-in window title, so `desktop-native` could
// only ever observe Autopus's own desktop app. ADK ships to other projects and
// the lane is `must` in two release profiles, so a project with its own desktop
// app inherited a gate it could not pass.
//
// The identity is split deliberately. ProviderAppID addresses the provider and
// is never published; the published refs stay opaque aliases because the
// public-ref grammar (pkg/qa/desktopobserve/canonical.go) rejects the dots and
// spaces a real bundle id and window title contain. AppName and WindowTitle come
// from the pack's required_landmarks, which permit spaces.
type desktopProviderTarget struct {
	ProviderAppID string
	AppName       string
	WindowTitle   string
	AppRef        string
	WindowRef     string
	// Landmarks is the pack's required_landmarks, in declaration order.
	//
	// REQ-4 publishes a node only when the pack declared its name, so the
	// projection builder needs the whole list, not just the two canonical ones.
	// Synthesizing an application and a window landmark here instead made the
	// builder's deeper-landmark support unreachable from a provider: a pack's
	// third landmark reached the oracle's policy but never the projection, so the
	// oracle failed a landmark the harness had not looked for - a false negative
	// the harness manufactured about itself.
	//
	// AppName and WindowTitle stay as their own fields because the envelope
	// validator compares them directly and should not dig through a slice.
	Landmarks []declaredLandmark
}

// desktopTargetedClient is implemented only by clients that drive an external
// provider. The local and fake clients observe whatever they were constructed
// with, so requiring this on the shared client interface would force every test
// double to carry a field it never reads.
type desktopTargetedClient interface {
	applyTarget(target desktopProviderTarget)
}

// resolved answers whether the target can address a provider at all.
func (target desktopProviderTarget) resolved() bool {
	return strings.TrimSpace(target.ProviderAppID) != ""
}

// targetFromRequest lifts the identity the runner already validated.
//
// The landmarks come from Policy.MinimumLandmarks rather than a second request
// field, so the list the projection is built from and the list the oracle judges
// against are the same object. A parallel field could drift, and a drift there
// reproduces exactly the defect this threading fixes.
//
// The list passes through unfiltered and in order. buildDesktopProjection
// requires exactly one application and one window landmark and errors otherwise;
// the journey layer guarantees that, so padding or reordering here would only
// hide a pack that bypassed validation.
func targetFromRequest(request DesktopObservationRunRequest) desktopProviderTarget {
	landmarks := make([]declaredLandmark, 0, len(request.Policy.MinimumLandmarks))
	for _, requirement := range request.Policy.MinimumLandmarks {
		landmarks = append(landmarks, declaredLandmark{
			Role: requirement.Role, Name: requirement.Name,
			RequiredState: requirement.RequiredState,
		})
	}
	return desktopProviderTarget{
		ProviderAppID: strings.TrimSpace(request.ProviderAppID),
		AppName:       strings.TrimSpace(request.AppName),
		WindowTitle:   strings.TrimSpace(request.WindowTitle),
		AppRef:        request.AppRef,
		WindowRef:     request.WindowRef,
		Landmarks:     landmarks,
	}
}
