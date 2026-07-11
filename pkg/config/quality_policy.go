package config

const (
	SupervisorModelPolicyInherit = "inherit"
	SupervisorModelPolicyQuality = "quality"
)

// EffectiveSupervisorModelPolicy keeps legacy configs quality-managed while
// new configs explicitly inherit the user's primary-session model.
func (q QualityConf) EffectiveSupervisorModelPolicy() string {
	if q.SupervisorModelPolicy == "" {
		return SupervisorModelPolicyQuality
	}
	return q.SupervisorModelPolicy
}

func (q QualityConf) ManagesSupervisorModel() bool {
	return q.EffectiveSupervisorModelPolicy() == SupervisorModelPolicyQuality
}

func (q QualityConf) IsValidSupervisorModelPolicy() bool {
	switch q.SupervisorModelPolicy {
	case "", SupervisorModelPolicyInherit, SupervisorModelPolicyQuality:
		return true
	default:
		return false
	}
}
