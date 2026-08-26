package claude

import (
	"context"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

// Generate는 하네스 설정에 기반하여 Claude Code 파일을 생성한다.
func (a *Adapter) Generate(ctx context.Context, cfg *config.HarnessConfig) (*adapter.PlatformFiles, error) {
	return a.Update(ctx, cfg)
}
