package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/design"
	"github.com/insajin/autopus-adk/pkg/detect"
)

// newVerifyCmd creates the "verify" subcommand for frontend UX verification.
func newVerifyCmd() *cobra.Command {
	var (
		fix              bool
		reportOnly       bool
		viewport         string
		visualGate       bool
		strictVisualGate bool
		visualCritic     string
	)

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Run frontend UX verification via Playwright screenshots",
		Long:  "Analyze git diff for changed frontend files and run Playwright-based screenshot verification.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVerifyWithOptions(cmd, fix, reportOnly, viewport, verifyVisualOptions{
				Enabled:    visualGate,
				Strict:     strictVisualGate,
				CriticPath: visualCritic,
			})
		},
	}

	cmd.Flags().BoolVar(&fix, "fix", true, "Enable auto-fix mode")
	cmd.Flags().BoolVar(&reportOnly, "report-only", false, "Skip auto-fix and report only")
	cmd.Flags().StringVar(&viewport, "viewport", "desktop", "Viewport size (desktop, mobile, tablet)")
	cmd.Flags().BoolVar(&visualGate, "visual-gate", true, "Write a metadata-only visual gate report for UI changes")
	cmd.Flags().BoolVar(&strictVisualGate, "strict-visual-gate", false, "Fail when the visual gate verdict is FAIL")
	cmd.Flags().StringVar(&visualCritic, "visual-critic-report", "", "Optional VLM critic JSON report to merge into the visual gate")

	return cmd
}

// runVerify executes the full frontend verification pipeline.
// cmd is used to detect whether --viewport was explicitly set by the user.
func runVerify(cmd *cobra.Command, fix, reportOnly bool, viewport string, visualArgs ...bool) error {
	visualGate, strictVisualGate := resolveVisualGateArgs(visualArgs)
	return runVerifyWithOptions(cmd, fix, reportOnly, viewport, verifyVisualOptions{Enabled: visualGate, Strict: strictVisualGate})
}

type verifyVisualOptions struct {
	Enabled    bool
	Strict     bool
	CriticPath string
}

func runVerifyWithOptions(cmd *cobra.Command, fix, reportOnly bool, viewport string, visual verifyVisualOptions) error {
	cfg, err := config.Load(".")
	if err != nil {
		return fmt.Errorf("설정 로드 실패: %w", err)
	}

	if !cfg.Verify.Enabled {
		fmt.Fprintln(os.Stderr, "경고: verify 기능이 비활성화되어 있습니다 (verify.enabled: false)")
		return nil
	}

	// Check prerequisites
	if !detect.IsInstalled("node") {
		// node.js is a proper noun but error strings must start lowercase per staticcheck ST1005.
		return fmt.Errorf("node.js가 설치되어 있지 않습니다. https://nodejs.org 를 통해 설치하세요")
	}
	if !detect.IsInstalled("playwright") {
		fmt.Fprintln(os.Stderr, "경고: playwright 바이너리를 찾을 수 없습니다. npx로 실행을 시도합니다")
	}

	// Resolve effective viewport: use config default only when the flag was not explicitly set.
	effectiveViewport := viewport
	if cmd != nil && !cmd.Flags().Changed("viewport") && cfg.Verify.DefaultViewport != "" {
		effectiveViewport = cfg.Verify.DefaultViewport
	}

	// Determine effective fix mode
	effectiveFix := fix && !reportOnly

	changed, err := analyzeGitDiff()
	if err != nil {
		fmt.Fprintf(os.Stderr, "경고: git diff 분석 실패: %v\n", err)
	}
	uiChanged := filterUIChangedFiles(changed, cfg.Design.UIFileGlobs)

	if len(uiChanged) == 0 {
		fmt.Print(buildVerifyDesignContextReport(".", changed, design.Options{
			Enabled:         cfg.Design.Enabled && cfg.Design.InjectOnVerify,
			Paths:           cfg.Design.Paths,
			MaxContextLines: cfg.Design.MaxContextLines,
			UIFileGlobs:     cfg.Design.UIFileGlobs,
		}))
		fmt.Println("변경된 프론트엔드 파일이 없습니다. verify를 건너뜁니다.")
		return nil
	}

	fmt.Fprintf(os.Stderr, "변경된 프론트엔드 파일 %d개 감지됨\n", len(uiChanged))
	for _, f := range uiChanged {
		fmt.Fprintf(os.Stderr, "  - %s\n", f)
	}
	designOpts := design.Options{
		Enabled:         cfg.Design.Enabled && cfg.Design.InjectOnVerify,
		Paths:           cfg.Design.Paths,
		MaxContextLines: cfg.Design.MaxContextLines,
		UIFileGlobs:     cfg.Design.UIFileGlobs,
	}
	fmt.Print(buildVerifyDesignContextReport(".", uiChanged, design.Options{
		Enabled:         cfg.Design.Enabled && cfg.Design.InjectOnVerify,
		Paths:           cfg.Design.Paths,
		MaxContextLines: cfg.Design.MaxContextLines,
		UIFileGlobs:     cfg.Design.UIFileGlobs,
	}))
	designCtx, designErr := design.LoadContext(".", designOpts)
	if designErr != nil {
		fmt.Fprintf(os.Stderr, "경고: design context 로드 실패: %v\n", designErr)
	}

	output, playwrightErr := runPlaywright(effectiveViewport)

	artifacts := collectVisualArtifacts(output)
	screenshots := collectScreenshotsFromArtifacts(artifacts)
	var visualGateErr error
	if visual.Enabled {
		visualGateErr = writeVerifyVisualGate(".", uiChanged, screenshots, artifacts, effectiveViewport, designCtx, cfg.Verify.MaxFixAttempts, playwrightErr, visual.Strict, visual.CriticPath)
	}

	fmt.Printf("verify 완료 (viewport: %s, auto-fix: %v)\n", effectiveViewport, effectiveFix)
	fmt.Printf("스크린샷 %d개 수집됨\n", len(screenshots))
	for _, s := range screenshots {
		fmt.Printf("  - %s\n", s)
	}

	if playwrightErr != nil {
		return fmt.Errorf("playwright 실행 실패: %w", playwrightErr)
	}
	if visualGateErr != nil {
		return visualGateErr
	}
	return nil
}

// analyzeGitDiff runs git diff against HEAD~1 and returns changed files.
// @AX:NOTE [AUTO]: HEAD~1 mirrors the existing verify workflow; change if verify moves to staged or working-tree diffs.
func analyzeGitDiff() ([]string, error) {
	out, err := exec.Command("git", "diff", "--name-only", "HEAD~1").Output()
	if err != nil {
		return nil, fmt.Errorf("git diff 실행 실패: %w", err)
	}

	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		files = append(files, line)
	}

	return files, nil
}

func filterUIChangedFiles(files []string, globs []string) []string {
	var ui []string
	for _, file := range files {
		if design.IsUIRelatedFile(file, globs) {
			ui = append(ui, file)
		}
	}
	return ui
}

func buildVerifyDesignContextReport(root string, changed []string, opts design.Options) string {
	if !design.AnyUIRelatedFile(changed, opts.UIFileGlobs) {
		return "design context: skipped (non-ui changes)\n"
	}
	ctx, err := design.LoadContext(root, opts)
	if err != nil {
		return fmt.Sprintf("design context: skipped (%v)\n", err)
	}
	if !ctx.Found {
		return fmt.Sprintf("design context: skipped (%s)\n%s", ctx.SkipReason, ctx.DiagnosticsSummary())
	}
	label := ctx.SourcePath
	if ctx.BaselinePath != "" {
		label = label + " -> " + ctx.BaselinePath
	}
	return fmt.Sprintf("design context: %s\n%s\n%s", label, ctx.Summary, ctx.DiagnosticsSummary())
}

// runPlaywright executes Playwright tests with JSON reporter and returns raw output.
func runPlaywright(viewport string) ([]byte, error) {
	args := []string{
		"playwright", "test",
		"--reporter=json",
	}
	if viewport != "" && viewport != "desktop" {
		args = append(args, "--project="+viewport)
	}

	cmd := exec.Command("npx", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Playwright may return non-zero exit code on test failures;
		// return output regardless so screenshots can be collected.
		return out, fmt.Errorf("playwright 실행 오류 (종료 코드 포함): %w", err)
	}

	return out, nil
}
