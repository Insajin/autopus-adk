package omp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var ompContextCoverageOwnedPaths = []string{
	"internal/cli/doctor_omp_context.go",
	"internal/cli/doctor_omp_context_capabilities.go",
	"internal/cli/doctor_omp_context_probe.go",
	"internal/cli/doctor_omp_context_projection.go",
	"internal/cli/doctor_omp_context_receipt.go",
	"internal/cli/doctor_omp_readiness.go",
	"internal/cli/workflow.go",
	"internal/cli/workflow_context_runtime_canary.go",
	"internal/cli/workflow_context_runtime_command.go",
	"internal/cli/workflow_context_runtime_helpers.go",
	"internal/cli/workflow_context_runtime_managed.go",
	"internal/cli/workflow_context_runtime_managed_rpc.go",
	"internal/cli/workflow_context_runtime_managed_rpc_admission.go",
	"internal/cli/workflow_context_runtime_managed_rpc_discovery.go",
	"internal/cli/workflow_context_runtime_managed_rpc_environment.go",
	"internal/cli/workflow_context_runtime_managed_rpc_identity.go",
	"internal/cli/workflow_context_runtime_managed_rpc_options.go",
	"internal/cli/workflow_context_runtime_managed_rpc_preflight.go",
	"internal/cli/workflow_context_runtime_managed_rpc_process.go",
	"internal/cli/workflow_context_runtime_managed_rpc_process_other.go",
	"internal/cli/workflow_context_runtime_managed_rpc_process_posix.go",
	"internal/cli/workflow_context_runtime_managed_rpc_product.go",
	"internal/cli/workflow_context_runtime_managed_rpc_protocol.go",
	"internal/cli/workflow_context_runtime_managed_rpc_root.go",
	"internal/cli/workflow_context_runtime_managed_rpc_run.go",
	"internal/cli/workflow_context_runtime_managed_rpc_sandbox_darwin.go",
	"internal/cli/workflow_context_runtime_managed_rpc_sandbox_other.go",
	"internal/cli/workflow_context_runtime_managed_rpc_surface.go",
	"internal/cli/workflow_context_runtime_overlay.go",
	"internal/cli/workflow_context_runtime_pipeline.go",
	"internal/cli/workflow_context_runtime_promotion.go",
	"internal/cli/workflow_context_runtime_product.go",
	"internal/cli/workflow_context_runtime_product_overlay.go",
	"internal/cli/workflow_context_runtime_receipt.go",
	"internal/cli/workflow_context_runtime_security.go",
	"internal/cli/workflow_context_runtime_supervisor.go",
	"internal/cli/workflow_context_runtime_types.go",
	"pkg/adapter/omp/omp.go",
	"pkg/adapter/omp/omp_ownership.go",
	"pkg/adapter/omp/omp_context_artifact.go",
	"pkg/adapter/omp/omp_context_artifact_fs.go",
	"pkg/adapter/omp/omp_context_bridge.go",
	"pkg/adapter/omp/omp_context_capability.go",
	"pkg/adapter/omp/omp_context_capability_eval.go",
	"pkg/adapter/omp/omp_rooted_workspace.go",
	"pkg/config/omp_context_policy.go",
	"pkg/config/omp_context_policy_validate.go",
	"pkg/config/schema.go",
	"pkg/promptlayer/omp_context_binding.go",
	"pkg/promptlayer/omp_context_canary.go",
	"pkg/promptlayer/omp_context_canary_attestation.go",
	"pkg/promptlayer/omp_context_canary_promotion.go",
	"pkg/promptlayer/omp_context_canary_reduce.go",
	"pkg/promptlayer/omp_context_evidence_store.go",
	"pkg/promptlayer/omp_context_evidence_verify.go",
	"pkg/promptlayer/omp_context_memory.go",
	"pkg/promptlayer/omp_context_memory_security.go",
	"pkg/promptlayer/omp_context_security.go",
	"pkg/promptlayer/omp_context_store.go",
	"pkg/promptlayer/omp_context_types.go",
}

var ompContextCoverageSharedPaths = map[string]bool{
	"internal/cli/doctor_omp_readiness.go":    true,
	"internal/cli/workflow.go":                true,
	"pkg/adapter/omp/omp.go":                  true,
	"pkg/adapter/omp/omp_ownership.go":        true,
	"pkg/adapter/omp/omp_rooted_workspace.go": true,
	"pkg/config/schema.go":                    true,
}

var ompContextCoverageDeclarationOnlyPaths = map[string]bool{
	"internal/cli/workflow_context_runtime_managed_rpc_process_other.go": true,
	"internal/cli/workflow_context_runtime_managed_rpc_sandbox_other.go": true,
	"pkg/promptlayer/omp_context_canary.go":                              true,
}

type ompCoverageTotals struct{ total, covered int }

func TestOMPContextChangedFileCoverage(t *testing.T) {
	base, profile, baseline := os.Getenv("AUTOPUS_COVER_BASE"), os.Getenv("AUTOPUS_COVER_PROFILE"), os.Getenv("AUTOPUS_COVER_BASELINE")
	present := 0
	for _, value := range []string{base, profile, baseline} {
		if value != "" {
			present++
		}
	}
	if present == 0 {
		t.Skip("set AUTOPUS_COVER_BASE, AUTOPUS_COVER_PROFILE, and AUTOPUS_COVER_BASELINE")
	}
	if present != 3 {
		t.Fatal("coverage gate environment is partial")
	}

	root := ompCoverageGit(t, "rev-parse", "--show-toplevel")
	_ = ompCoverageGit(t, "cat-file", "-e", base+"^{commit}")
	changed := ompContextChangedProductionPaths(t, root, base)
	want := append([]string(nil), ompContextCoverageOwnedPaths...)
	sort.Strings(want)
	if strings.Join(changed, "\n") != strings.Join(want, "\n") {
		t.Fatalf("context-owned path set drift: got=%v want=%v", changed, want)
	}

	fileTotals, packageTotals := parseOMPContextCoverProfile(t, profile)
	var aggregate ompCoverageTotals
	for _, path := range want {
		totals, ok := fileTotals[path]
		if !ok && ompContextCoverageDeclarationOnlyPaths[path] {
			continue
		}
		if !ok || totals.total == 0 {
			t.Fatalf("coverage row missing: %s", path)
		}
		aggregate.total += totals.total
		aggregate.covered += totals.covered
	}
	percent := 100 * float64(aggregate.covered) / float64(aggregate.total)
	if percent < 85 {
		t.Fatalf("context-owned aggregate coverage %.2f is below 85.00", percent)
	}
	verifyOMPContextPackageBaseline(t, baseline, packageTotals)
	_ = root
}

func ompContextChangedProductionPaths(t *testing.T, root, base string) []string {
	t.Helper()
	tracked := ompCoverageGitAt(t, root, "diff", "--name-only", "--diff-filter=ACMR", base, "--", "*.go")
	untracked := ompCoverageGitAt(t, root, "ls-files", "--others", "--exclude-standard", "--", "*.go")
	set := make(map[string]bool)
	for _, path := range strings.Fields(tracked + "\n" + untracked) {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		if strings.HasPrefix(path, "pkg/adapter/omp/omp_context_") ||
			strings.HasPrefix(path, "pkg/promptlayer/omp_context_") ||
			strings.HasPrefix(path, "pkg/config/omp_context_") ||
			strings.HasPrefix(path, "internal/cli/doctor_omp_context") ||
			strings.HasPrefix(path, "internal/cli/workflow_context_runtime_") ||
			ompContextCoverageSharedPaths[path] {
			set[path] = true
		}
	}
	paths := make([]string, 0, len(set))
	for path := range set {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func parseOMPContextCoverProfile(t *testing.T, profile string) (map[string]ompCoverageTotals, map[string]ompCoverageTotals) {
	t.Helper()
	file, err := os.Open(profile)
	if err != nil {
		t.Fatalf("coverage profile unavailable")
	}
	defer file.Close()
	files, packages := make(map[string]ompCoverageTotals), make(map[string]ompCoverageTotals)
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() || !strings.HasPrefix(scanner.Text(), "mode: ") {
		t.Fatal("coverage profile malformed")
	}
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 3 {
			t.Fatal("coverage profile row malformed")
		}
		location := strings.SplitN(fields[0], ":", 2)[0]
		path := normalizeOMPContextCoveragePath(location)
		statements, err1 := strconv.Atoi(fields[1])
		count, err2 := strconv.Atoi(fields[2])
		if path == "" || err1 != nil || err2 != nil || statements < 0 || count < 0 {
			t.Fatal("coverage profile row invalid")
		}
		fileTotal := files[path]
		fileTotal.total += statements
		packagePath := filepath.ToSlash(filepath.Dir(path))
		packageTotal := packages[packagePath]
		packageTotal.total += statements
		if count > 0 {
			fileTotal.covered += statements
			packageTotal.covered += statements
		}
		files[path], packages[packagePath] = fileTotal, packageTotal
	}
	if err := scanner.Err(); err != nil {
		t.Fatal("coverage profile read failed")
	}
	return files, packages
}

func TestParseOMPContextCoverProfile_AcceptsGoZeroStatementRows(t *testing.T) {
	t.Parallel()

	profile := filepath.Join(t.TempDir(), "coverage.out")
	data := "mode: set\n" +
		"example/pkg/adapter/omp/omp_context_bridge.go:1.1,1.1 0 1\n" +
		"example/pkg/adapter/omp/omp_context_bridge.go:2.1,2.2 1 1\n"
	if err := os.WriteFile(profile, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	files, packages := parseOMPContextCoverProfile(t, profile)
	if files["pkg/adapter/omp/omp_context_bridge.go"] != (ompCoverageTotals{total: 1, covered: 1}) ||
		packages["pkg/adapter/omp"] != (ompCoverageTotals{total: 1, covered: 1}) {
		t.Fatalf("zero-statement row changed totals: files=%v packages=%v", files, packages)
	}
}

func normalizeOMPContextCoveragePath(path string) string {
	path = filepath.ToSlash(path)
	for _, marker := range []string{"pkg/", "internal/"} {
		if strings.HasPrefix(path, marker) {
			return path
		}
		if index := strings.Index(path, "/"+marker); index >= 0 {
			return path[index+1:]
		}
	}
	return ""
}

func verifyOMPContextPackageBaseline(t *testing.T, path string, current map[string]ompCoverageTotals) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("coverage baseline unavailable")
	}
	var baseline struct {
		Packages map[string]float64 `json:"packages"`
	}
	if json.Unmarshal(data, &baseline) != nil || len(baseline.Packages) == 0 {
		t.Fatal("coverage baseline malformed")
	}
	for packagePath, want := range baseline.Packages {
		totals, ok := current[packagePath]
		if !ok || totals.total == 0 {
			t.Fatalf("baseline package missing: %s", packagePath)
		}
		got := 100 * float64(totals.covered) / float64(totals.total)
		if got+0.000001 < want {
			t.Fatalf("package baseline regression: %s %.2f < %.2f", packagePath, got, want)
		}
	}
}

func ompCoverageGit(t *testing.T, args ...string) string { return ompCoverageGitAt(t, "", args...) }

func ompCoverageGitAt(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.Output()
	if err != nil {
		t.Fatalf("git coverage gate command failed: %s", fmt.Sprint(args[0]))
	}
	return strings.TrimSpace(string(output))
}
