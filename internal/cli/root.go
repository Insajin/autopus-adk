// Package cli는 Cobra 기반 CLI 커맨드를 정의한다.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/internal/cli/tui"
	"github.com/insajin/autopus-adk/pkg/version"
)

// NewRootCmd는 루트 커맨드를 생성한다.
// 패키지 수준 변수 대신 로컬 변수를 사용하여 병렬 테스트 시 데이터 레이스를 방지한다.
func NewRootCmd() *cobra.Command {
	// 플래그 변수를 로컬로 선언하여 각 호출마다 독립적인 상태를 가진다
	var (
		verbose    bool
		configPath string
	)

	root := &cobra.Command{
		Use:           "auto",
		Short:         "Autopus-ADK: Agentic Development Kit",
		Long:          "Autopus-ADK는 코딩 CLI에 하네스를 설치하는 Go 기반 셋업 도구입니다.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	root.PersistentFlags().StringVar(&configPath, "config", "", "Config file path (default: ./autopus.yaml)")

	root.AddCommand(newVersionCmd())
	root.AddCommand(newInitCmd())
	root.AddCommand(newUpdateCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newPlatformCmd())
	root.AddCommand(newArchCmd())
	root.AddCommand(newLoreCmd())
	root.AddCommand(newSpecCmd())
	root.AddCommand(newLSPCmd())
	root.AddCommand(newSearchCmd())
	root.AddCommand(newDocsCmd())
	root.AddCommand(newHashCmd())
	root.AddCommand(newSkillCmd())
	root.AddCommand(newOrchestraCmd())
	root.AddCommand(newSetupCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newVerifyCmd())

	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			out := cmd.OutOrStdout()
			tui.Banner(out)
			fmt.Fprintln(out, version.String())
		},
	}
}

// Execute는 CLI를 실행한다.
func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
