package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/lsp"
)

// newLSPCmd는 lsp 서브커맨드를 생성한다.
func newLSPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lsp",
		Short: "LSP-based code intelligence",
	}

	cmd.AddCommand(newLSPDiagnosticsCmd())
	cmd.AddCommand(newLSPRefsCmd())
	cmd.AddCommand(newLSPRenameCmd())
	cmd.AddCommand(newLSPSymbolsCmd())
	cmd.AddCommand(newLSPDefinitionCmd())
	return cmd
}

func newLSPDiagnosticsCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "diagnostics <file>",
		Short: "Get LSP diagnostics for a file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cleanup, err := createLSPClient(".")
			if err != nil {
				return err
			}
			defer cleanup()

			diags, err := client.Diagnostics(args[0])
			if err != nil {
				return fmt.Errorf("진단 조회 실패: %w", err)
			}

			if format == "json" {
				return json.NewEncoder(os.Stdout).Encode(diags)
			}

			if len(diags) == 0 {
				fmt.Println("진단 없음")
				return nil
			}

			for _, d := range diags {
				fmt.Printf("%s:%d:%d [%s] %s\n", d.File, d.Line, d.Col, d.Severity, d.Message)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "text", "Output format: text|json")
	return cmd
}

func newLSPRefsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "refs <symbol>",
		Short: "Find all references to a symbol",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cleanup, err := createLSPClient(".")
			if err != nil {
				return err
			}
			defer cleanup()

			refs, err := client.References(args[0])
			if err != nil {
				return fmt.Errorf("참조 조회 실패: %w", err)
			}

			for _, r := range refs {
				fmt.Printf("%s:%d:%d\n", r.File, r.Line, r.Col)
			}
			return nil
		},
	}
}

func newLSPRenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename <old> <new>",
		Short: "Rename a symbol",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cleanup, err := createLSPClient(".")
			if err != nil {
				return err
			}
			defer cleanup()

			if err := client.Rename(args[0], args[1]); err != nil {
				return fmt.Errorf("이름 변경 실패: %w", err)
			}

			fmt.Printf("%s -> %s 이름 변경 완료\n", args[0], args[1])
			return nil
		},
	}
}

func newLSPSymbolsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "symbols <file>",
		Short: "List symbols in a file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cleanup, err := createLSPClient(".")
			if err != nil {
				return err
			}
			defer cleanup()

			syms, err := client.Symbols(args[0])
			if err != nil {
				return fmt.Errorf("심볼 조회 실패: %w", err)
			}

			for _, s := range syms {
				fmt.Printf("%s (%s) %s:%d\n", s.Name, s.Kind, s.Location.File, s.Location.Line)
			}
			return nil
		},
	}
}

func newLSPDefinitionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "definition <symbol>",
		Short: "Find definition of a symbol",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cleanup, err := createLSPClient(".")
			if err != nil {
				return err
			}
			defer cleanup()

			loc, err := client.Definition(args[0])
			if err != nil {
				return fmt.Errorf("정의 조회 실패: %w", err)
			}
			if loc == nil {
				fmt.Println("정의를 찾을 수 없습니다")
				return nil
			}

			fmt.Printf("%s:%d:%d\n", loc.File, loc.Line, loc.Col)
			return nil
		},
	}
}

// createLSPClient는 현재 프로젝트에 맞는 LSP 클라이언트를 생성한다.
// 실제 LSP 서버가 없는 경우 모의 클라이언트를 반환한다.
func createLSPClient(projectDir string) (lsp.Commander, func(), error) {
	serverCmd, args, err := lsp.DetectServer(projectDir)
	if err != nil {
		return nil, nil, fmt.Errorf("LSP 서버 감지 실패: %w", err)
	}

	realClient, err := lsp.NewClient(serverCmd, args)
	if err != nil {
		return nil, nil, fmt.Errorf("LSP 클라이언트 생성 실패: %w", err)
	}

	// LSP 클라이언트를 Commander 어댑터로 래핑
	adapter := &lspClientAdapter{client: realClient}
	cleanup := func() {
		_ = realClient.Shutdown()
	}

	return adapter, cleanup, nil
}

// lspClientAdapter는 *lsp.Client를 Commander 인터페이스로 래핑한다.
type lspClientAdapter struct {
	client *lsp.Client
}

func (a *lspClientAdapter) Diagnostics(path string) ([]lsp.Diagnostic, error) {
	return nil, fmt.Errorf("LSP diagnostics: 실제 구현이 필요합니다 (path: %s)", path)
}

func (a *lspClientAdapter) References(symbol string) ([]lsp.Location, error) {
	return nil, fmt.Errorf("LSP references: 실제 구현이 필요합니다 (symbol: %s)", symbol)
}

func (a *lspClientAdapter) Rename(oldName, newName string) error {
	return fmt.Errorf("LSP rename: 실제 구현이 필요합니다 (%s -> %s)", oldName, newName)
}

func (a *lspClientAdapter) Symbols(path string) ([]lsp.Symbol, error) {
	return nil, fmt.Errorf("LSP symbols: 실제 구현이 필요합니다 (path: %s)", path)
}

func (a *lspClientAdapter) Definition(symbol string) (*lsp.Location, error) {
	return nil, fmt.Errorf("LSP definition: 실제 구현이 필요합니다 (symbol: %s)", symbol)
}
