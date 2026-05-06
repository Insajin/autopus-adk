package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/skillevolve"
)

var skillEvolveCandidateIDRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-SKILL-EVOLVE-001: public CLI namespace for quarantined skill evolution workflows.
// @AX:REASON: candidate generation, deterministic replay, human-approved promotion, and archive routes are coupled at this command boundary.
func newSkillEvolveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "evolve",
		Short: "Manage quarantined skill-evolution candidates",
	}
	cmd.AddCommand(newSkillEvolveCandidatesCmd())
	cmd.AddCommand(newSkillEvolveReplayCmd())
	cmd.AddCommand(newSkillEvolvePromoteCmd())
	cmd.AddCommand(newSkillEvolveArchiveCmd())
	return cmd
}

type skillEvolveCommonOptions struct {
	ProjectDir    string
	QuarantineDir string
	JSONOut       bool
	Format        string
}

func addSkillEvolveCommonFlags(cmd *cobra.Command, opts *skillEvolveCommonOptions) {
	cmd.Flags().StringVar(&opts.ProjectDir, "project-dir", ".", "Project directory")
	cmd.Flags().StringVar(&opts.QuarantineDir, "quarantine", "", "Candidate quarantine directory")
	addJSONFlags(cmd, &opts.JSONOut, &opts.Format)
}

func skillEvolveQuarantineDir(opts skillEvolveCommonOptions) string {
	if strings.TrimSpace(opts.QuarantineDir) != "" {
		return opts.QuarantineDir
	}
	return filepath.Join(opts.ProjectDir, ".autopus", "skill-evolve", "quarantine")
}

func newSkillEvolveCandidatesCmd() *cobra.Command {
	var (
		opts         skillEvolveCommonOptions
		qualityIndex string
		minCount     int
		creator      string
	)
	cmd := &cobra.Command{
		Use:   "candidates",
		Short: "Generate quarantined skill candidates from repeated failures",
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonMode, err := resolveJSONMode(opts.JSONOut, opts.Format)
			if err != nil {
				return err
			}
			result, err := skillevolve.GenerateCandidates(cmd.Context(), skillevolve.CandidateGenerationOptions{
				ProjectDir:       opts.ProjectDir,
				QualityIndexPath: qualityIndex,
				QuarantineDir:    skillEvolveQuarantineDir(opts),
				MinCount:         minCount,
				Creator:          creator,
			})
			if err != nil {
				if jsonMode {
					return writeJSONResultAndExit(cmd, jsonStatusError, err, "skill_evolve_candidates_failed", result, nil, nil)
				}
				return err
			}
			if jsonMode {
				return writeJSONResult(cmd, jsonStatusOK, skillevolve.SanitizeGenerationResult(result), nil, nil)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "candidates=%d\n", len(result.Candidates))
			return nil
		},
	}
	addSkillEvolveCommonFlags(cmd, &opts)
	cmd.Flags().StringVar(&qualityIndex, "quality-index", "", "Quality index JSON path")
	cmd.Flags().IntVar(&minCount, "min-count", 2, "Minimum repeated failures per fingerprint")
	cmd.Flags().StringVar(&creator, "creator", "auto-skill-evolve", "Candidate creator identity")
	_ = cmd.MarkFlagRequired("quality-index")
	return cmd
}

func newSkillEvolveReplayCmd() *cobra.Command {
	var opts skillEvolveCommonOptions
	cmd := &cobra.Command{
		Use:   "replay <candidate-id>",
		Short: "Replay deterministic evidence for a quarantined candidate",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonMode, err := resolveJSONMode(opts.JSONOut, opts.Format)
			if err != nil {
				return err
			}
			candidate, err := loadSkillEvolveCandidate(skillEvolveQuarantineDir(opts), args[0])
			if err != nil {
				if jsonMode {
					return writeJSONResultAndExit(cmd, jsonStatusError, err, "skill_evolve_replay_load_failed", skillevolve.ReplayResult{}, nil, nil)
				}
				return err
			}
			result, err := skillevolve.ReplayCandidate(cmd.Context(), skillevolve.ReplayOptions{
				ProjectDir: opts.ProjectDir,
				Candidate:  candidate,
			})
			if err != nil {
				if jsonMode {
					return writeJSONResultAndExit(cmd, jsonStatusError, err, "skill_evolve_replay_failed", result, nil, nil)
				}
				return err
			}
			if jsonMode {
				return writeJSONResult(cmd, jsonStatusOK, result, nil, nil)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "promotion_ready=%t\n", result.PromotionReady)
			return nil
		},
	}
	addSkillEvolveCommonFlags(cmd, &opts)
	return cmd
}

func newSkillEvolvePromoteCmd() *cobra.Command {
	var (
		opts       skillEvolveCommonOptions
		apply      bool
		approvedBy string
	)
	cmd := &cobra.Command{
		Use:   "promote <candidate-id>",
		Short: "Promote a replay-approved candidate into ADK source paths",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonMode, err := resolveJSONMode(opts.JSONOut, opts.Format)
			if err != nil {
				return err
			}
			candidate, err := loadSkillEvolveCandidate(skillEvolveQuarantineDir(opts), args[0])
			if err != nil {
				if jsonMode {
					return writeJSONResultAndExit(cmd, jsonStatusError, err, "skill_evolve_promote_load_failed", skillevolve.PromotionResult{}, nil, nil)
				}
				return err
			}
			result, err := skillevolve.PromoteCandidate(cmd.Context(), skillevolve.PromotionOptions{
				ProjectDir: opts.ProjectDir,
				Candidate:  candidate,
				Approval:   skillevolve.HumanApproval{ApprovedBy: approvedBy},
				Apply:      apply,
			})
			if err != nil {
				if jsonMode {
					return writeJSONResultAndExit(cmd, jsonStatusError, err, "skill_evolve_promote_failed", result, nil, nil)
				}
				return err
			}
			if jsonMode {
				return writeJSONResult(cmd, jsonStatusOK, result, nil, nil)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "applied=%t paths=%d\n", result.Applied, len(result.AppliedPaths))
			return nil
		},
	}
	addSkillEvolveCommonFlags(cmd, &opts)
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply candidate files")
	cmd.Flags().StringVar(&approvedBy, "approved-by", "", "Human approver identity")
	return cmd
}

func newSkillEvolveArchiveCmd() *cobra.Command {
	var (
		opts   skillEvolveCommonOptions
		reason string
	)
	cmd := &cobra.Command{
		Use:   "archive <candidate-id>",
		Short: "Archive a stale, rejected, or superseded candidate",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonMode, err := resolveJSONMode(opts.JSONOut, opts.Format)
			if err != nil {
				return err
			}
			candidate, err := loadSkillEvolveCandidate(skillEvolveQuarantineDir(opts), args[0])
			if err != nil {
				if jsonMode {
					return writeJSONResultAndExit(cmd, jsonStatusError, err, "skill_evolve_archive_load_failed", skillevolve.ArchiveResult{}, nil, nil)
				}
				return err
			}
			result, err := skillevolve.ArchiveCandidate(cmd.Context(), skillevolve.ArchiveOptions{
				QuarantineDir: skillEvolveQuarantineDir(opts),
				Candidate:     candidate,
				Reason:        reason,
			})
			if err != nil {
				if jsonMode {
					return writeJSONResultAndExit(cmd, jsonStatusError, err, "skill_evolve_archive_failed", result, nil, nil)
				}
				return err
			}
			if jsonMode {
				return writeJSONResult(cmd, jsonStatusOK, result, nil, nil)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "archived=%s\n", result.ArchivePath)
			return nil
		},
	}
	addSkillEvolveCommonFlags(cmd, &opts)
	cmd.Flags().StringVar(&reason, "reason", "", "Archive reason code")
	_ = cmd.MarkFlagRequired("reason")
	return cmd
}

// @AX:WARN [AUTO] @AX:SPEC: SPEC-SKILL-EVOLVE-001: candidate id is used as a quarantine path segment.
// @AX:REASON: Keep ids generated by safeFileName/shortDigest or add validation before accepting broader user-provided id grammar.
func loadSkillEvolveCandidate(quarantineDir, id string) (skillevolve.CandidateBundle, error) {
	if !skillEvolveCandidateIDRe.MatchString(id) || strings.Contains(id, "..") {
		return skillevolve.CandidateBundle{}, fmt.Errorf("invalid candidate id %q", id)
	}
	path := filepath.Join(quarantineDir, id+".json")
	body, err := os.ReadFile(path)
	if err != nil {
		return skillevolve.CandidateBundle{}, err
	}
	var candidate skillevolve.CandidateBundle
	if err := json.Unmarshal(body, &candidate); err != nil {
		return skillevolve.CandidateBundle{}, err
	}
	candidate.BundlePath = path
	return candidate, nil
}
