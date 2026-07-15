package promptlayer

type ContextProfileName string

const (
	ProfileCore         ContextProfileName = "core"
	ProfileArchitecture ContextProfileName = "architecture"
	ProfileTest         ContextProfileName = "test"
	ProfileCanary       ContextProfileName = "canary"
	ProfileSignature    ContextProfileName = "signature"
	ProfileLearning     ContextProfileName = "learning"
)

type CommandContextProfile struct {
	Required     []ContextProfileName
	Conditional  []ContextProfileName
	RelevantSpec bool
}

var contextProfileDocuments = map[ContextProfileName][]string{
	ProfileCore: {
		"AGENTS.md",
		".autopus/project/workspace.md",
	},
	ProfileArchitecture: {
		"ARCHITECTURE.md",
		".autopus/project/product.md",
		".autopus/project/structure.md",
		".autopus/project/tech.md",
	},
	ProfileTest: {
		".autopus/project/scenarios.md",
	},
	ProfileCanary: {
		".autopus/project/canary.md",
	},
	ProfileSignature: {
		".autopus/context/signatures.md",
	},
	ProfileLearning: {
		".autopus/learnings/pipeline.jsonl",
	},
}

var commandContextProfiles = map[string]CommandContextProfile{
	"worker": {
		Required:    []ContextProfileName{ProfileCore},
		Conditional: []ContextProfileName{ProfileArchitecture, ProfileSignature, ProfileLearning},
	},
	"go": {
		Required:     []ContextProfileName{ProfileCore},
		Conditional:  []ContextProfileName{ProfileArchitecture, ProfileSignature, ProfileLearning},
		RelevantSpec: true,
	},
	"plan": {
		Required:     []ContextProfileName{ProfileCore, ProfileArchitecture},
		Conditional:  []ContextProfileName{ProfileSignature, ProfileLearning},
		RelevantSpec: true,
	},
	"review": {
		Required:     []ContextProfileName{ProfileCore},
		Conditional:  []ContextProfileName{ProfileArchitecture, ProfileSignature, ProfileLearning},
		RelevantSpec: true,
	},
	"test": {
		Required:    []ContextProfileName{ProfileCore, ProfileTest},
		Conditional: []ContextProfileName{ProfileSignature, ProfileLearning},
	},
	"canary": {
		Required:    []ContextProfileName{ProfileCore, ProfileCanary},
		Conditional: []ContextProfileName{ProfileLearning},
	},
}

func ResolveCommandContextProfile(command string) (CommandContextProfile, bool) {
	profile, ok := commandContextProfiles[command]
	if !ok {
		return CommandContextProfile{}, false
	}
	return cloneCommandContextProfile(profile), true
}

func (p CommandContextProfile) RequiredDocuments() []string {
	return documentsForProfiles(p.Required)
}

func (p CommandContextProfile) ConditionalDocuments() []string {
	return documentsForProfiles(p.Conditional)
}

func documentsForProfiles(profiles []ContextProfileName) []string {
	var documents []string
	for _, profile := range profiles {
		documents = append(documents, contextProfileDocuments[profile]...)
	}
	return append([]string(nil), documents...)
}

func cloneCommandContextProfile(profile CommandContextProfile) CommandContextProfile {
	profile.Required = append([]ContextProfileName(nil), profile.Required...)
	profile.Conditional = append([]ContextProfileName(nil), profile.Conditional...)
	return profile
}
