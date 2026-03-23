// Package cli는 테스트 헬퍼를 제공한다.
package cli_test

import (
	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/internal/cli"
	"github.com/insajin/autopus-adk/pkg/config"
)

// newTestRootCmd는 테스트용 루트 커맨드를 생성한다.
func newTestRootCmd() *cobra.Command {
	return cli.NewRootCmd()
}

// loadConfigFromDir loads HarnessConfig from the given directory for test assertions.
func loadConfigFromDir(dir string) (*config.HarnessConfig, error) {
	return config.Load(dir)
}
