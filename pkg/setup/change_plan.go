package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/insajin/autopus-adk/pkg/config"
)

type changeReasonSet struct {
	create   string
	update   string
	preserve string
}

// BuildGeneratePlan computes a no-write plan for setup generate.
func BuildGeneratePlan(projectDir string, opts *GenerateOptions) (*ChangePlan, error) {
	return buildGeneratePlanAt(projectDir, opts, time.Now().UTC())
}

// BuildUpdatePlan computes a no-write plan for setup update.
func BuildUpdatePlan(projectDir string, outputDir string) (*ChangePlan, error) {
	return buildUpdatePlanAt(projectDir, outputDir, time.Now().UTC())
}

func buildGeneratePlanAt(projectDir string, opts *GenerateOptions, builtAt time.Time) (*ChangePlan, error) {
	generateOpts := normalizeGenerateOptions(opts)
	absProjectDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, err
	}

	docsDir := resolveDocsDir(absProjectDir, generateOpts.OutputDir)
	if !generateOpts.Force {
		if _, err := os.Stat(docsDir); err == nil {
			return nil, fmt.Errorf("documentation already exists at %s. Use --force to overwrite", docsDir)
		}
	}

	return buildChangePlan(absProjectDir, docsDir, generateOpts, builtAt, ChangePlanModeGenerate, "", false)
}

func buildUpdatePlanAt(projectDir string, outputDir string, builtAt time.Time) (*ChangePlan, error) {
	absProjectDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, err
	}

	generateOpts := GenerateOptions{
		OutputDir: outputDir,
		Force:     true,
	}
	docsDir := resolveDocsDir(absProjectDir, outputDir)

	fullRegeneration := false
	note := ""
	if _, err := LoadMeta(docsDir); err != nil {
		fullRegeneration = true
		note = "missing or corrupted .meta.yaml requires full regeneration"
	}

	return buildChangePlan(absProjectDir, docsDir, generateOpts, builtAt, ChangePlanModeUpdate, note, fullRegeneration)
}

func buildChangePlan(
	projectDir string,
	docsDir string,
	opts GenerateOptions,
	builtAt time.Time,
	mode ChangePlanMode,
	note string,
	fullRegeneration bool,
) (*ChangePlan, error) {
	info, err := Scan(projectDir)
	if err != nil {
		return nil, fmt.Errorf("scan project: %w", err)
	}

	docSet := Render(info, opts.Render)
	meta := NewMetaAt(projectDir, builtAt)
	targets := make([]plannedTarget, 0, len(renderedDocs(docSet))+3)

	for _, doc := range renderedDocs(docSet) {
		meta.SetFileMeta(doc.Name, doc.Content, docSourceFiles(info, doc.Key), projectDir)
		target, err := buildContentTarget(
			projectDir,
			filepath.Join(docsDir, doc.Name),
			ChangeClassTrackedDocs,
			[]byte(doc.Content),
			changeReasonSet{
				create:   "tracked document will be created from the current project scan",
				update:   "tracked document content differs from the current project scan",
				preserve: "tracked document already matches the current project scan",
			},
		)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}

	scenarioContent, err := renderScenariosContent(projectDir, info)
	if err != nil {
		return nil, fmt.Errorf("render scenarios: %w", err)
	}
	scenarioTarget, err := buildContentTarget(
		projectDir,
		filepath.Join(projectDir, ".autopus", "project", "scenarios.md"),
		ChangeClassGeneratedSurface,
		scenarioContent,
		changeReasonSet{
			create:   "generated scenario catalog will be created from detected workflows",
			update:   "generated scenario catalog differs from detected workflows",
			preserve: "generated scenario catalog already matches detected workflows",
		},
	)
	if err != nil {
		return nil, err
	}
	targets = append(targets, scenarioTarget)

	signatureTarget, enabled, err := buildSignatureTarget(projectDir, opts.Config)
	if err != nil {
		return nil, err
	}
	if enabled {
		targets = append(targets, signatureTarget)
	} else {
		targets = append(
			targets,
			buildSkipTarget(
				projectDir,
				filepath.Join(projectDir, signaturesDir, signaturesFile),
				ChangeClassGeneratedSurface,
				"signature map generation is disabled by config",
			),
		)
	}

	metaContent, err := marshalMeta(meta)
	if err != nil {
		return nil, err
	}
	metaTarget, err := buildContentTarget(
		projectDir,
		filepath.Join(docsDir, metaFileName),
		ChangeClassRuntimeState,
		metaContent,
		changeReasonSet{
			create:   "runtime metadata will be created for this preview",
			update:   "runtime metadata will refresh hashes for this preview",
			preserve: "runtime metadata already matches this preview",
		},
	)
	if err != nil {
		return nil, err
	}
	targets = append(targets, metaTarget)

	sort.Slice(targets, func(i, j int) bool {
		return targets[i].change.Path < targets[j].change.Path
	})

	docSet.Meta = *meta
	changes := make([]PlannedChange, 0, len(targets))
	for _, target := range targets {
		changes = append(changes, target.change)
	}

	plan := &ChangePlan{
		Mode:                 mode,
		ProjectDir:           projectDir,
		DocsDir:              docsDir,
		BuiltAt:              builtAt.UTC(),
		Reason:               defaultPlanReason(mode, fullRegeneration, note),
		FullRegeneration:     fullRegeneration,
		FullRegenerationNote: note,
		Changes:              changes,
		WorkspaceHints:       buildWorkspaceHints(projectDir, info),
		docSet:               docSet,
		targets:              targets,
		generateOpts:         opts,
	}
	plan.Fingerprint = fingerprintPlan(mode, targets)
	return plan, nil
}

func normalizeGenerateOptions(opts *GenerateOptions) GenerateOptions {
	if opts == nil {
		return GenerateOptions{}
	}
	return *opts
}

func buildSignatureTarget(projectDir string, cfg *config.HarnessConfig) (plannedTarget, bool, error) {
	content, enabled, err := renderSignatureMapContent(projectDir, cfg)
	if err != nil {
		return plannedTarget{}, false, fmt.Errorf("render signature map: %w", err)
	}
	if !enabled {
		return plannedTarget{}, false, nil
	}

	target, err := buildContentTarget(
		projectDir,
		filepath.Join(projectDir, signaturesDir, signaturesFile),
		ChangeClassGeneratedSurface,
		content,
		changeReasonSet{
			create:   "generated signature map will be created from exported API analysis",
			update:   "generated signature map differs from exported API analysis",
			preserve: "generated signature map already matches exported API analysis",
		},
	)
	if err != nil {
		return plannedTarget{}, false, err
	}
	return target, true, nil
}

func buildContentTarget(
	projectDir string,
	absPath string,
	class ChangeClass,
	content []byte,
	reasons changeReasonSet,
) (plannedTarget, error) {
	action, reason, err := detectChangeAction(absPath, content, reasons)
	if err != nil {
		return plannedTarget{}, fmt.Errorf("inspect %s: %w", absPath, err)
	}

	return plannedTarget{
		change: PlannedChange{
			Path:   displayChangePath(projectDir, absPath),
			Action: action,
			Class:  class,
			Reason: reason,
		},
		absPath: absPath,
		content: append([]byte(nil), content...),
	}, nil
}

func buildSkipTarget(projectDir string, absPath string, class ChangeClass, reason string) plannedTarget {
	return plannedTarget{
		change: PlannedChange{
			Path:   displayChangePath(projectDir, absPath),
			Action: ChangeActionSkip,
			Class:  class,
			Reason: reason,
		},
		absPath: absPath,
	}
}
