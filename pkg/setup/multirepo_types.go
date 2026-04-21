package setup

// MultiRepoInfo describes a workspace composed of multiple Git repositories.
type MultiRepoInfo struct {
	IsMultiRepo   bool
	WorkspaceRoot string
	Components    []RepoComponent
	Dependencies  []RepoDependency
}

// RepoComponent represents a repository inside a multi-repo workspace.
type RepoComponent struct {
	Name            string
	Path            string
	AbsPath         string
	RemoteURL       string
	PrimaryLanguage string
	ModulePath      string
	PackageName     string
	Role            string
}

// RepoDependency represents a directed dependency between repositories.
type RepoDependency struct {
	Source  string
	Target  string
	Type    string
	Version string
}
