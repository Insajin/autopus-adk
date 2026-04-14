// Package cli는 doctor 커맨드를 구현한다.
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/internal/cli/tui"
	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/adapter/gemini"
	"github.com/insajin/autopus-adk/pkg/adapter/opencode"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/detect"
)

func newDoctorCmd() *cobra.Command {
	var dir string
	var fix bool
	var yes bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check autopus harness health",
		Long:  "하네스 설치 상태를 진단합니다. 누락된 파일, 의존성 상태, 플랫폼 상태를 확인합니다.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				var err error
				dir, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("현재 디렉터리를 가져올 수 없음: %w", err)
				}
			}

			out := cmd.OutOrStdout()
			tui.BannerWithInfo(out, "autopus-adk", "doctor")

			// 1. 설정 파일 확인
			cfg, err := config.Load(dir)
			if err != nil {
				tui.FAIL(out, fmt.Sprintf("autopus.yaml 로드 실패: %v", err))
				return nil
			}
			tui.OK(out, fmt.Sprintf("autopus.yaml (mode: %s)", cfg.Mode))

			// 2. 플랫폼 파일 검증
			ctx := context.Background()
			allOK := true
			for _, p := range cfg.Platforms {
				var validationErrs []adapter.ValidationError
				var validateErr error

				switch p {
				case "claude-code":
					a := claude.NewWithRoot(dir)
					validationErrs, validateErr = a.Validate(ctx)
				case "codex":
					a := codex.NewWithRoot(dir)
					validationErrs, validateErr = a.Validate(ctx)
				case "gemini-cli":
					a := gemini.NewWithRoot(dir)
					validationErrs, validateErr = a.Validate(ctx)
				case "opencode":
					a := opencode.NewWithRoot(dir)
					validationErrs, validateErr = a.Validate(ctx)
				default:
					tui.SKIP(out, fmt.Sprintf("알 수 없는 플랫폼: %s", p))
					continue
				}

				if validateErr != nil {
					tui.FAIL(out, fmt.Sprintf("%s 검증 실패: %v", p, validateErr))
					allOK = false
					continue
				}

				if len(validationErrs) == 0 {
					tui.OK(out, p)
				} else {
					for _, ve := range validationErrs {
						level := strings.ToUpper(ve.Level)
						switch level {
						case "ERROR":
							tui.FAIL(out, fmt.Sprintf("%s: %s", p, ve.Message))
							allOK = false
						case "WARN":
							tui.SKIP(out, fmt.Sprintf("%s: %s", p, ve.Message))
						default:
							tui.Info(out, fmt.Sprintf("%s: %s", p, ve.Message))
						}
					}
				}
			}

			// 3. 의존성 확인
			tui.SectionHeader(out, "Dependencies")
			statuses := detect.CheckDependencies(detect.FullModeDeps)
			for _, s := range statuses {
				if s.Installed {
					tui.OK(out, s.Name)
				} else if s.Required {
					tui.FAIL(out, fmt.Sprintf("%s not installed (install: %s)", s.Name, s.InstallCmd))
					allOK = false
				} else {
					tui.SKIP(out, fmt.Sprintf("%s not installed (optional, install: %s)", s.Name, s.InstallCmd))
				}
			}

			// Run auto-install if --fix flag is set
			if fix {
				missingDeps := filterMissing(statuses)
				if len(missingDeps) > 0 {
					if err := runDoctorFix(cmd.OutOrStdout(), missingDeps, yes); err != nil {
						tui.FAIL(out, fmt.Sprintf("Auto-install failed: %v", err))
					}
				}
			}

			// 4. 부모 디렉터리 규칙 충돌 검사
			conflicts := detect.CheckParentRuleConflicts(dir)
			if len(conflicts) > 0 {
				tui.SectionHeader(out, "Rule Conflicts")
				if cfg.IsolateRules {
					tui.OK(out, "isolate_rules: true (parent rules ignored)")
				}
				for _, c := range conflicts {
					if cfg.IsolateRules {
						tui.Info(out, fmt.Sprintf("%s/.claude/rules/%s/ (ignored)", c.ParentDir, c.Namespace))
					} else {
						tui.SKIP(out, fmt.Sprintf("Parent rules: %s/.claude/rules/%s/", c.ParentDir, c.Namespace))
						tui.Bullet(out, "Run 'auto init' or 'auto update' to configure rule isolation.")
						allOK = false
					}
				}
			}

			// 5. 코딩 CLI 감지 상태
			tui.SectionHeader(out, "Installed CLIs")
			detected := detect.DetectPlatforms()
			if len(detected) == 0 {
				tui.SKIP(out, "No coding CLIs detected in PATH")
			} else {
				for _, p := range detected {
					tui.OK(out, fmt.Sprintf("%s (%s)", p.Name, p.Version))
				}
			}

			// @AX:NOTE [AUTO]: Quality Gate complexity addressed — extracted to checkQualityGate() in doctor_checks.go
			// 6. Quality Gate diagnostics
			tui.SectionHeader(out, "Quality Gate")
			if !checkQualityGate(out, cfg) {
				allOK = false
			}

			// 7. Hooks & Permissions validation
			tui.SectionHeader(out, "Hooks & Permissions")
			if !checkHooksPermissions(out, dir) {
				allOK = false
			}

			fmt.Fprintln(out)
			tui.ResultBox(out, allOK, func() string {
				if allOK {
					return "All checks passed"
				}
				return "Issues found — run 'auto update' to fix"
			}())

			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "프로젝트 루트 디렉터리 (기본값: 현재 디렉터리)")
	cmd.Flags().BoolVar(&fix, "fix", false, "Auto-install missing dependencies")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip interactive prompts (use with --fix)")
	return cmd
}
