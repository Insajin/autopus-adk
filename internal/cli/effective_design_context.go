package cli

import "github.com/insajin/autopus-adk/pkg/design"

func loadEffectiveDesignContext(effectiveCfg effectiveHarnessConfig, opts design.Options) (design.Context, error) {
	ctx, err := design.LoadContext(effectiveCfg.designRoot(), opts)
	if err != nil || ctx.Found || !canFallbackToParentDesignContext(effectiveCfg, ctx) {
		return ctx, err
	}
	parentCtx, parentErr := design.LoadContext(effectiveCfg.ParentDir, opts)
	if parentErr != nil || !parentCtx.Found {
		return ctx, err
	}
	return parentCtx, nil
}

func canFallbackToParentDesignContext(effectiveCfg effectiveHarnessConfig, ctx design.Context) bool {
	if effectiveCfg.ParentDir == "" || effectiveCfg.ParentDir == effectiveCfg.designRoot() {
		return false
	}
	return ctx.SkipReason == design.SkipMissing && len(ctx.Diagnostics) == 0
}
