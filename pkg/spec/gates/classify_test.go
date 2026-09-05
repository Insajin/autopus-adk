package gates

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifyChange_Table(t *testing.T) {
	cases := []struct {
		name  string
		paths []string
		globs []string
		want  ChangeClass
	}{
		{name: "small ui change", paths: []string{"frontend/src/components/Button.tsx"}, want: ClassUIOnly},
		{name: "stylesheet only", paths: []string{"web/styles/main.scss", "web/styles/reset.css"}, want: ClassUIOnly},
		{name: "security db migration", paths: []string{"backend/db/migrations/001.sql"}, want: ClassSecurityOrData},
		{name: "auth handler", paths: []string{"pkg/server/auth_middleware.go"}, want: ClassSecurityOrData},
		{name: "multi domain", paths: []string{"pkg/a/x.go", "pkg/b/y.go"}, want: ClassMultiDomain},
		{name: "backend and frontend", paths: []string{"backend/main.go", "frontend/src/index.ts"}, want: ClassMultiDomain},
		{name: "doc only", paths: []string{"README.md", "docs/guide/intro.rst"}, want: ClassDocOnly},
		{name: "general single package", paths: []string{"pkg/a/x.go", "pkg/a/x_test.go"}, want: ClassGeneral},
		{name: "ui plus non-ui in one root", paths: []string{"frontend/src/components/Button.tsx", "frontend/src/api/client.ts"}, want: ClassGeneral},
		{name: "docs alongside code stay single domain", paths: []string{"pkg/a/x.go", "docs/a.md"}, want: ClassGeneral},
		{name: "top-level files do not add roots", paths: []string{"go.sum", "pkg/a/x.go"}, want: ClassGeneral},
		{name: "security wins over multi domain", paths: []string{"pkg/a/x.go", "pkg/b/schema.go"}, want: ClassSecurityOrData},
		{name: "configured ui glob", paths: []string{"lib/screens/home.view"}, globs: []string{"*.view"}, want: ClassUIOnly},
		{name: "empty change set is general", paths: nil, want: ClassGeneral},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ClassifyChange(tc.paths, tc.globs))
		})
	}
}

func TestClassify_UIPresenceIsIndependentOfClass(t *testing.T) {
	c := Classify([]string{"frontend/src/auth/Login.tsx", "backend/db/migrations/002.sql"}, nil)
	assert.Equal(t, ClassSecurityOrData, c.Class)
	assert.Equal(t, []string{"frontend/src/auth/Login.tsx"}, c.UIPaths)
	assert.Equal(t, []string{"backend/db/migrations/002.sql", "frontend/src/auth/Login.tsx"}, c.SecurityPaths)
	assert.Equal(t, []string{"backend", "frontend"}, c.Roots)
}

func TestClassify_NormalizesAndSortsPaths(t *testing.T) {
	c := Classify([]string{"./pkg/b/y.go", "pkg/a/x.go", "pkg/a/x.go", "", " pkg\\c\\z.go "}, nil)
	assert.Equal(t, []string{"pkg/a/x.go", "pkg/b/y.go", "pkg/c/z.go"}, c.Paths)
	assert.Equal(t, []string{"pkg/a", "pkg/b", "pkg/c"}, c.Roots)
}

func TestIsSecurityPath_TokenBoundaries(t *testing.T) {
	assert.True(t, IsSecurityPath("internal/auth/token.go"))
	assert.True(t, IsSecurityPath("services/rbac-policy/rules.yaml"))
	assert.False(t, IsSecurityPath("src/theme/tokens.css"), "design tokens are a UI surface, not a security one")
	assert.False(t, IsSecurityPath("pkg/author/bio.go"), "substring matches must not count")
}

func TestIsUIPath_DirectoryHintsOnlyApplyToDirectories(t *testing.T) {
	assert.True(t, IsUIPath("web/app/layout.ts", nil))
	assert.True(t, IsUIPath("src/design-system/button.go", nil))
	assert.False(t, IsUIPath("pkg/ui.go", nil), "a file named like a hint directory is not a UI surface")
	assert.True(t, IsUIPath("lib/screens/home.view", []string{"*.view"}))
	assert.True(t, IsUIPath("lib/screens/home.dart", []string{"lib/**/*.dart"}))
	assert.False(t, IsUIPath("lib/home.dart", []string{"lib/screens/*.dart"}))
}
