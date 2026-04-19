package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/config"
)

type globalFlags struct {
	Think      bool
	UltraThink bool
	AutoMode   bool
	LoopMode   bool
	MultiMode  bool
	Quality    string
	Effort     string
	TaskMode   string
}

type globalFlagsContextKey struct{}

func withGlobalFlags(ctx context.Context, flags globalFlags) context.Context {
	return context.WithValue(ctx, globalFlagsContextKey{}, flags)
}

func globalFlagsFromContext(ctx context.Context) globalFlags {
	if ctx == nil {
		return globalFlags{}
	}
	flags, ok := ctx.Value(globalFlagsContextKey{}).(globalFlags)
	if !ok {
		return globalFlags{}
	}
	return flags
}

func collectGlobalFlags(cmd *cobra.Command, configPath string) (globalFlags, error) {
	var flags globalFlags

	think, err := cmd.Flags().GetBool("think")
	if err != nil {
		return flags, err
	}
	ultraThink, err := cmd.Flags().GetBool("ultrathink")
	if err != nil {
		return flags, err
	}
	autoMode, err := cmd.Flags().GetBool("auto")
	if err != nil {
		return flags, err
	}
	loopMode, err := cmd.Flags().GetBool("loop")
	if err != nil {
		return flags, err
	}
	multiMode, err := cmd.Flags().GetBool("multi")
	if err != nil {
		return flags, err
	}
	quality, err := cmd.Flags().GetString("quality")
	if err != nil {
		return flags, err
	}
	effort, err := cmd.Flags().GetString("effort")
	if err != nil {
		return flags, err
	}
	taskMode, err := cmd.Flags().GetString("task-created-mode")
	if err != nil {
		return flags, err
	}

	flags = globalFlags{
		Think:      think,
		UltraThink: ultraThink,
		AutoMode:   autoMode,
		LoopMode:   loopMode,
		MultiMode:  multiMode,
		Quality:    strings.TrimSpace(quality),
		Effort:     strings.TrimSpace(effort),
		TaskMode:   normalizeTaskCreatedMode(taskMode),
	}
	if flags.UltraThink {
		flags.Think = true
	}
	if flags.Quality != "" {
		if err := validateQualityPreset(cmd, configPath, flags.Quality); err != nil {
			return globalFlags{}, err
		}
	}
	if err := validateTaskCreatedModeFlag(flags.TaskMode); err != nil {
		return globalFlags{}, err
	}

	return flags, nil
}

func validateQualityPreset(cmd *cobra.Command, configPath, preset string) error {
	configDir, err := resolveConfigDir(cmd, configPath)
	if err != nil {
		return fmt.Errorf("resolve config dir for --quality: %w", err)
	}

	cfg, err := config.Load(configDir)
	if err != nil {
		return fmt.Errorf("load config for --quality: %w", err)
	}
	if _, ok := cfg.Quality.Presets[preset]; ok {
		return nil
	}

	available := make([]string, 0, len(cfg.Quality.Presets))
	for name := range cfg.Quality.Presets {
		available = append(available, name)
	}
	sort.Strings(available)
	return fmt.Errorf("unknown quality preset %q (available: %s)", preset, strings.Join(available, ", "))
}

func resolveConfigDir(cmd *cobra.Command, configPath string) (string, error) {
	if cmd != nil && cmd.Name() == "init" {
		if dir, err := cmd.Flags().GetString("dir"); err == nil && strings.TrimSpace(dir) != "" {
			return dir, nil
		}
	}

	if strings.TrimSpace(configPath) != "" {
		info, err := os.Stat(configPath)
		if err == nil && info.IsDir() {
			return configPath, nil
		}
		return filepath.Dir(configPath), nil
	}

	return os.Getwd()
}
