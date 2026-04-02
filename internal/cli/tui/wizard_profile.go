package tui

import (
	"fmt"

	"github.com/charmbracelet/huh"
)

// buildProfileStep creates the usage profile selection step.
// R1: developer = backend/infra, fullstack = frontend included.
// R2: Skip step when ExistingProfile is already set.
func buildProfileStep(num, total int, r *InitWizardResult) *huh.Form {
	title := fmt.Sprintf("[%d/%d] Usage Profile", num, total)

	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(title).
				Description("How will you use Autopus ADK?").
				Options(
					huh.NewOption("Developer — backend, infra, CLI tools", "developer"),
					huh.NewOption("Fullstack — includes frontend & UX verification", "fullstack"),
				).
				Value(&r.UsageProfile),
		),
	).WithTheme(AutopusTheme()).WithWidth(bannerWidth + 10)
}
