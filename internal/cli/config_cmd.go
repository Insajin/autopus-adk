// Package cli defines Cobra-based CLI commands.
package cli

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/config"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage autopus.yaml configuration",
	}
	cmd.AddCommand(newConfigSetCmd())
	cmd.AddCommand(newConfigGetCmd())
	return cmd
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value in autopus.yaml",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, val := args[0], args[1]

			dir, err := os.Getwd()
			if err != nil {
				return err
			}

			cfg, err := config.Load(dir)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if err := applyConfigSet(cfg, key, val); err != nil {
				return err
			}

			if err := config.Save(dir, cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s = %s\n", key, val)
			return nil
		},
	}
}

// applyConfigSet maps dot-notation keys to HarnessConfig fields.
func applyConfigSet(cfg *config.HarnessConfig, key, val string) error {
	switch key {
	case "hints.platform":
		b, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("hints.platform must be true or false, got %q", val)
		}
		cfg.Hints.Platform = &b
	case "usage_profile":
		p := config.UsageProfile(val)
		if !p.IsValid() || p == "" {
			return fmt.Errorf("usage_profile must be 'developer' or 'fullstack', got %q", val)
		}
		cfg.UsageProfile = p
	default:
		return fmt.Errorf("unknown config key %q (supported: hints.platform, usage_profile)", key)
	}
	return nil
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value from autopus.yaml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			dir, err := os.Getwd()
			if err != nil {
				return err
			}

			cfg, err := config.Load(dir)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			val, err := getConfigValue(cfg, key)
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), val)
			return nil
		},
	}
}

// getConfigValue reads a config value by dot-notation key.
func getConfigValue(cfg *config.HarnessConfig, key string) (string, error) {
	switch key {
	case "hints.platform":
		return strconv.FormatBool(cfg.Hints.IsPlatformHintEnabled()), nil
	case "usage_profile":
		return string(cfg.UsageProfile.Effective()), nil
	default:
		return "", fmt.Errorf("unknown config key %q (supported: hints.platform, usage_profile)", key)
	}
}
