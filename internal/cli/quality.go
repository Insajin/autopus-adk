package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/config"
)

func newQualityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quality [preset]",
		Short: "Show or change quality mode",
		Long: "Show or change quality.default in autopus.yaml.\n\n" +
			"Run without arguments to choose a preset interactively.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				return runQualitySet(cmd, args[0])
			}
			return runQualityInteractive(cmd)
		},
	}
	cmd.AddCommand(newQualityShowCmd())
	cmd.AddCommand(newQualitySetCmd())
	return cmd
}

func newQualityShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current quality mode",
		Args:  cobra.NoArgs,
		RunE:  runQualityShow,
	}
}

func newQualitySetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <preset>",
		Short: "Set quality.default",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQualitySet(cmd, args[0])
		},
	}
}

func runQualityShow(cmd *cobra.Command, _ []string) error {
	dir, cfg, err := loadQualityConfig(cmd)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "quality.default = %s\n", cfg.Quality.Default)
	fmt.Fprintf(out, "config = %s\n", filepath.Join(dir, "autopus.yaml"))
	fmt.Fprintf(out, "available = %s\n", strings.Join(orderedQualityPresets(cfg), ", "))
	return nil
}

func runQualitySet(cmd *cobra.Command, preset string) error {
	dir, cfg, err := loadQualityConfig(cmd)
	if err != nil {
		return err
	}
	preset = strings.TrimSpace(preset)
	if err := validateQualityChoice(cfg, preset); err != nil {
		return err
	}
	if err := saveQualityDefault(dir, cfg, preset); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "quality.default = %s\n", preset)
	return nil
}

func runQualityInteractive(cmd *cobra.Command) error {
	dir, cfg, err := loadQualityConfig(cmd)
	if err != nil {
		return err
	}
	options := orderedQualityPresets(cfg)
	if len(options) == 0 {
		return fmt.Errorf("quality presets are not configured")
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Current quality: %s\n", cfg.Quality.Default)
	fmt.Fprintln(out, "Select quality mode:")
	for i, preset := range options {
		marker := " "
		if preset == cfg.Quality.Default {
			marker = "*"
		}
		desc := strings.TrimSpace(cfg.Quality.Presets[preset].Description)
		if desc != "" {
			fmt.Fprintf(out, "  %s %d) %s - %s\n", marker, i+1, preset, desc)
		} else {
			fmt.Fprintf(out, "  %s %d) %s\n", marker, i+1, preset)
		}
	}
	fmt.Fprint(out, "Choose: ")

	choice, err := readQualityChoice(cmd, options)
	if err != nil {
		return err
	}
	if err := saveQualityDefault(dir, cfg, choice); err != nil {
		return err
	}
	fmt.Fprintf(out, "quality.default = %s\n", choice)
	return nil
}

func loadQualityConfig(cmd *cobra.Command) (string, *config.HarnessConfig, error) {
	flags := globalFlagsFromContext(cmd.Context())
	dir, err := resolveConfigDir(cmd, flags.ConfigPath)
	if err != nil {
		return "", nil, fmt.Errorf("resolve config dir: %w", err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		return "", nil, fmt.Errorf("load config: %w", err)
	}
	return dir, cfg, nil
}

func validateQualityChoice(cfg *config.HarnessConfig, preset string) error {
	if _, ok := cfg.Quality.Presets[preset]; ok {
		return nil
	}
	return fmt.Errorf("unknown quality preset %q (available: %s)", preset, strings.Join(orderedQualityPresets(cfg), ", "))
}

func orderedQualityPresets(cfg *config.HarnessConfig) []string {
	seen := make(map[string]bool, len(cfg.Quality.Presets))
	var ordered []string
	for _, name := range []string{"ultra", "balanced"} {
		if _, ok := cfg.Quality.Presets[name]; ok {
			ordered = append(ordered, name)
			seen[name] = true
		}
	}
	var rest []string
	for name := range cfg.Quality.Presets {
		if !seen[name] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	return append(ordered, rest...)
}

func readQualityChoice(cmd *cobra.Command, options []string) (string, error) {
	in := cmd.InOrStdin()
	if f, ok := in.(*os.File); ok && f == os.Stdin && !isStdinTTY() {
		return "", fmt.Errorf("interactive quality selection requires a TTY; use `auto quality set <preset>`")
	}
	reader := bufio.NewReader(in)
	answer, err := reader.ReadString('\n')
	if err != nil && strings.TrimSpace(answer) == "" {
		return "", fmt.Errorf("read quality choice: %w", err)
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return "", fmt.Errorf("quality choice is required")
	}
	for i, preset := range options {
		if answer == fmt.Sprintf("%d", i+1) || strings.EqualFold(answer, preset) {
			return preset, nil
		}
	}
	return "", fmt.Errorf("invalid quality choice %q", answer)
}
