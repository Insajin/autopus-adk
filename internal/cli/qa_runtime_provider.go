package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

const qaDesktopObservationAdapterID = "desktop-accessibility-observe"

var errQARuntimeProviderConflict = errors.New("exactly one --runtime-provider selection is allowed")

func parseQARuntimeProvider(
	cmd *cobra.Command,
	jsonMode bool,
	values []string,
) (desktopobserve.RuntimeProvider, error) {
	if len(values) > 1 {
		return "", qaCommandError(cmd, jsonMode, errQARuntimeProviderConflict, "qa_runtime_provider_conflict", nil)
	}
	if len(values) == 0 {
		return "", nil
	}
	provider := desktopobserve.RuntimeProvider(values[0])
	if provider != desktopobserve.RuntimeProviderLocal && provider != desktopobserve.RuntimeProviderOrca {
		return "", qaCommandError(cmd, jsonMode, desktopobserve.ErrRuntimeProviderInvalid, "qa_runtime_provider_invalid", nil)
	}
	return provider, nil
}

func requireQARuntimeProvider(
	cmd *cobra.Command,
	jsonMode bool,
	provider desktopobserve.RuntimeProvider,
	required bool,
) error {
	if provider != "" || !required {
		return nil
	}
	return qaCommandError(
		cmd,
		jsonMode,
		desktopobserve.ErrRuntimeProviderRequired,
		"qa_runtime_provider_required",
		nil,
	)
}

func projectRequiresQARuntimeProvider(projectDir string) bool {
	packs, err := journey.LoadDir(projectDir)
	if err != nil {
		return false
	}
	for _, pack := range packs {
		if pack.Adapter.ID == qaDesktopObservationAdapterID {
			return true
		}
	}
	return false
}

func runRequiresQARuntimeProvider(opts qaRunOptions) bool {
	if opts.AdapterID != "" {
		return opts.AdapterID == qaDesktopObservationAdapterID
	}
	packs, err := journey.LoadDir(opts.ProjectDir)
	if err != nil {
		return false
	}
	for _, pack := range packs {
		if pack.Adapter.ID != qaDesktopObservationAdapterID {
			continue
		}
		if opts.JourneyID != "" && pack.ID != opts.JourneyID {
			continue
		}
		if opts.Lane != "" && !journey.HasLane(pack, opts.Lane) {
			continue
		}
		return true
	}
	return false
}
