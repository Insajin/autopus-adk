package config

const (
	SharedSurfaceAuto = "auto"
	SharedSurfaceFull = "full"
	SharedSurfaceCore = "core"
)

// EffectiveSharedSurface returns the normalized shared surface mode.
func (s SkillsConf) EffectiveSharedSurface() string {
	if s.SharedSurface == "" {
		return SharedSurfaceFull
	}
	return s.SharedSurface
}
