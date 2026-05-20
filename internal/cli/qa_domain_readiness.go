package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/qa/domainreadiness"
)

type qaDomainReadinessPlanOptions struct {
	ProjectDir string
	Catalog    string
	Lane       string
	JSONOut    bool
	Format     string
}

type qaDomainReadinessReportOptions struct {
	ProjectDir  string
	Catalog     string
	SuiteID     string
	RunID       string
	WorkspaceID string
	JSONOut     bool
	Format      string
}

type qaDomainReadinessInitOptions struct {
	ProjectDir string
	Catalog    string
	JSONOut    bool
	Format     string
}

func newQADomainReadinessCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domain-readiness",
		Short: "Initialize, plan, and report project-local domain readiness evidence",
	}
	cmd.AddCommand(newQADomainReadinessInitCmd())
	cmd.AddCommand(newQADomainReadinessPlanCmd())
	cmd.AddCommand(newQADomainReadinessReportCmd())
	return cmd
}

func newQADomainReadinessInitCmd() *cobra.Command {
	var opts qaDomainReadinessInitOptions
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a project-local domain readiness catalog starter",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQADomainReadinessInit(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.ProjectDir, "project-dir", ".", "Project directory")
	cmd.Flags().StringVar(&opts.Catalog, "catalog", domainreadiness.DefaultCatalogPath, "Project-local domain readiness catalog path")
	addJSONFlags(cmd, &opts.JSONOut, &opts.Format)
	return cmd
}

func newQADomainReadinessPlanCmd() *cobra.Command {
	var opts qaDomainReadinessPlanOptions
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Compile the domain readiness eval catalog without executing commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQADomainReadinessPlan(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.ProjectDir, "project-dir", ".", "Project directory")
	cmd.Flags().StringVar(&opts.Catalog, "catalog", domainreadiness.DefaultCatalogPath, "Project-local domain readiness catalog path")
	cmd.Flags().StringVar(&opts.Lane, "lane", "fast", "QAMESH lane")
	addJSONFlags(cmd, &opts.JSONOut, &opts.Format)
	return cmd
}

func newQADomainReadinessReportCmd() *cobra.Command {
	var opts qaDomainReadinessReportOptions
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Build a safe domain readiness setup-gap report",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQADomainReadinessReport(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.ProjectDir, "project-dir", ".", "Project directory")
	cmd.Flags().StringVar(&opts.Catalog, "catalog", domainreadiness.DefaultCatalogPath, "Project-local domain readiness catalog path")
	cmd.Flags().StringVar(&opts.SuiteID, "suite-id", "", "Domain readiness suite id; defaults to catalog suite_id")
	cmd.Flags().StringVar(&opts.RunID, "run-id", "domain-readiness-plan", "Domain readiness run id")
	cmd.Flags().StringVar(&opts.WorkspaceID, "workspace-id", "00000000-0000-4000-8000-000000000001", "Workspace id")
	addJSONFlags(cmd, &opts.JSONOut, &opts.Format)
	return cmd
}

func runQADomainReadinessInit(cmd *cobra.Command, opts qaDomainReadinessInitOptions) error {
	jsonMode, err := resolveJSONMode(opts.JSONOut, opts.Format)
	if err != nil {
		return err
	}
	path, err := domainreadiness.WriteStarterCatalog(opts.ProjectDir, opts.Catalog)
	if err != nil {
		return qaCommandError(cmd, jsonMode, err, "qa_domain_readiness_init_failed", map[string]any{"project_dir": opts.ProjectDir, "catalog": opts.Catalog})
	}
	result := map[string]any{
		"schema_version": domainreadiness.CatalogSchemaVersion,
		"catalog_path":   path,
		"created":        true,
	}
	if jsonMode {
		return writeJSONResult(cmd, jsonStatusOK, result, nil, nil)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "created %s\n", path)
	return nil
}

func runQADomainReadinessPlan(cmd *cobra.Command, opts qaDomainReadinessPlanOptions) error {
	jsonMode, err := resolveJSONMode(opts.JSONOut, opts.Format)
	if err != nil {
		return err
	}
	catalogPath := domainreadiness.ResolveCatalogPath(opts.ProjectDir, opts.Catalog)
	catalog, err := domainreadiness.LoadCatalogFile(catalogPath)
	if err != nil {
		return qaCommandError(cmd, jsonMode, err, "qa_domain_readiness_catalog_failed", map[string]any{"catalog": catalogPath})
	}
	plan, err := domainreadiness.CompileCatalog(catalog, domainreadiness.CompileOptions{
		ProjectDir: opts.ProjectDir,
		Lane:       opts.Lane,
	})
	if err != nil {
		return qaCommandError(cmd, jsonMode, err, "qa_domain_readiness_plan_failed", map[string]any{"project_dir": opts.ProjectDir})
	}
	if jsonMode {
		return writeJSONResult(cmd, jsonStatusOK, plan, nil, nil)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "domain_readiness scenarios=%d valid=%t lane=%s executed=%t\n", plan.ScenarioCount, plan.Validation.Valid, plan.SelectedLane, plan.CommandsExecuted)
	return nil
}

func runQADomainReadinessReport(cmd *cobra.Command, opts qaDomainReadinessReportOptions) error {
	jsonMode, err := resolveJSONMode(opts.JSONOut, opts.Format)
	if err != nil {
		return err
	}
	catalogPath := domainreadiness.ResolveCatalogPath(opts.ProjectDir, opts.Catalog)
	catalog, err := domainreadiness.LoadCatalogFile(catalogPath)
	if err != nil {
		return qaCommandError(cmd, jsonMode, err, "qa_domain_readiness_catalog_failed", map[string]any{"catalog": catalogPath})
	}
	report, err := domainreadiness.BuildSetupGapReport(catalog, domainreadiness.ReportOptions{
		SuiteID:     opts.SuiteID,
		RunID:       opts.RunID,
		WorkspaceID: opts.WorkspaceID,
	})
	if err != nil {
		return qaCommandError(cmd, jsonMode, err, "qa_domain_readiness_report_failed", map[string]any{"suite_id": opts.SuiteID})
	}
	if jsonMode {
		return writeJSONResult(cmd, jsonStatusOK, report, nil, nil)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "domain_readiness_report evidence=%d suite=%s run=%s\n", report.EvidenceCount, report.SuiteID, report.RunID)
	return nil
}
