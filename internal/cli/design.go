package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/design"
	templates "github.com/insajin/autopus-adk/templates"
)

func newDesignCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "design",
		Short: "Manage project DESIGN.md context",
	}
	cmd.AddCommand(newDesignInitCmd())
	cmd.AddCommand(newDesignContextCmd())
	cmd.AddCommand(newDesignImportCmd())
	return cmd
}

func newDesignInitCmd() *cobra.Command {
	var dir string
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a starter DESIGN.md",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveDir(dir)
			if err != nil {
				return err
			}
			target := filepath.Join(root, "DESIGN.md")
			if _, err := os.Stat(target); err == nil && !force {
				return fmt.Errorf("refusing to overwrite existing DESIGN.md; rerun with --force to replace it")
			} else if err != nil && !os.IsNotExist(err) {
				return err
			}
			data, err := templates.FS.ReadFile("shared/DESIGN.md.tmpl")
			if err != nil {
				return err
			}
			if err := os.WriteFile(target, data, 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created %s\n", target)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "project root directory")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing DESIGN.md")
	return cmd
}

func newDesignContextCmd() *cobra.Command {
	var dir string
	var maxLines int
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Print compact design prompt context",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveDir(dir)
			if err != nil {
				return err
			}
			ctx, err := design.LoadContext(root, design.Options{Enabled: true, MaxContextLines: maxLines})
			if err != nil {
				return err
			}
			if !ctx.Found {
				fmt.Fprintf(cmd.OutOrStdout(), "design context: skipped (%s)\n", ctx.SkipReason)
				fmt.Fprint(cmd.OutOrStdout(), ctx.DiagnosticsSummary())
				return nil
			}
			fmt.Fprint(cmd.OutOrStdout(), ctx.PromptSection())
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "project root directory")
	cmd.Flags().IntVar(&maxLines, "max-lines", design.DefaultMaxContextLines, "maximum design context lines")
	return cmd
}

func newDesignImportCmd() *cobra.Command {
	var dir string
	var trustLabel string
	var allowExternalImport bool
	cmd := &cobra.Command{
		Use:   "import URL",
		Short: "Import an external design reference as untrusted generated state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveDir(dir)
			if err != nil {
				return err
			}
			cfg, err := config.Load(root)
			if err != nil {
				return err
			}
			if !cfg.Design.ExternalImports && !allowExternalImport {
				return fmt.Errorf("design external imports are disabled; set design.external_imports: true or rerun with --allow-external-import")
			}
			result, err := design.ImportURL(context.Background(), root, args[0], design.ImportOptions{TrustLabel: trustLabel})
			if err != nil {
				return err
			}
			if result.Rejected {
				fmt.Fprintf(cmd.OutOrStdout(), "design import rejected: %v\nmetadata: %s\n", result.Reasons, filepath.Join(result.ArtifactDir, "metadata.json"))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "design import stored: %s\n", result.ArtifactDir)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "project root directory")
	cmd.Flags().StringVar(&trustLabel, "trust-label", "external-reference", "trust label for imported reference")
	cmd.Flags().BoolVar(&allowExternalImport, "allow-external-import", false, "explicitly allow this external import even when config disables external imports")
	return cmd
}
