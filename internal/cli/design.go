package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	cmd.AddCommand(newDesignPackCmd())
	cmd.AddCommand(newDesignFigmaCmd())
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
			target, err := createStarterDesignFile(root, force)
			if err != nil {
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

func newDesignPackCmd() *cobra.Command {
	var dir string
	var format string
	var output string
	var maxRefs int
	cmd := &cobra.Command{
		Use:   "pack",
		Short: "Build a compact design source pack",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := resolveDir(dir)
			if err != nil {
				return err
			}
			cfg, err := config.Load(root)
			if err != nil {
				return err
			}
			pack, err := design.BuildPack(root, design.PackOptions{
				ContextOptions: design.Options{
					Enabled:         cfg.Design.Enabled,
					Paths:           cfg.Design.Paths,
					MaxContextLines: cfg.Design.MaxContextLines,
					UIFileGlobs:     cfg.Design.UIFileGlobs,
				},
				MaxRefs: maxRefs,
			})
			if err != nil {
				return err
			}
			data, err := renderDesignOutput(format, pack)
			if err != nil {
				return err
			}
			return writeOrPrint(cmd, output, data)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "project root directory")
	cmd.Flags().StringVar(&format, "format", "markdown", "output format: markdown or json")
	cmd.Flags().StringVar(&output, "output", "", "write output to a file instead of stdout")
	cmd.Flags().IntVar(&maxRefs, "max-refs", 30, "maximum refs to include per category")
	return cmd
}

type markdownJSON interface {
	Markdown() string
	JSON() ([]byte, error)
}

func renderDesignOutput(format string, value markdownJSON) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "markdown", "md":
		return []byte(value.Markdown()), nil
	case "json":
		return value.JSON()
	default:
		return nil, fmt.Errorf("unsupported format %q; use markdown or json", format)
	}
}

func writeOrPrint(cmd *cobra.Command, output string, data []byte) error {
	if output == "" {
		_, err := cmd.OutOrStdout().Write(data)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(output, data, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", output)
	return nil
}

func createStarterDesignFile(root string, force bool) (string, error) {
	target := filepath.Join(root, "DESIGN.md")
	if _, err := os.Stat(target); err == nil && !force {
		return "", fmt.Errorf("refusing to overwrite existing DESIGN.md; rerun with --force to replace it")
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return target, writeStarterDesignTemplate(target)
}

func ensureStarterDesignFile(root string) (string, bool, error) {
	target := filepath.Join(root, "DESIGN.md")
	if _, err := os.Stat(target); err == nil {
		return target, false, nil
	} else if err != nil && !os.IsNotExist(err) {
		return "", false, err
	}
	if err := writeStarterDesignTemplate(target); err != nil {
		return "", false, err
	}
	return target, true, nil
}

func writeStarterDesignTemplate(target string) error {
	data, err := templates.FS.ReadFile("shared/DESIGN.md.tmpl")
	if err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o644)
}
