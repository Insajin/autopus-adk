// Package content provides content data helpers for harness file generation.
package content

// FileSizeExclusion represents a generated file pattern excluded from the file size limit.
type FileSizeExclusion struct {
	Pattern     string // glob pattern, e.g. "*_generated.go"
	Description string // brief description
}

// commonExclusions returns exclusion patterns applicable to all stacks.
func commonExclusions() []FileSizeExclusion {
	return []FileSizeExclusion{
		{Pattern: "*.md", Description: "Documentation files"},
		{Pattern: "*.txt", Description: "Documentation files"},
		{Pattern: "*.rst", Description: "Documentation files"},
		{Pattern: "*.yaml", Description: "Configuration files"},
		{Pattern: "*.yml", Description: "Configuration files"},
		{Pattern: "*.json", Description: "Configuration files"},
		{Pattern: "*.toml", Description: "Configuration files"},
	}
}

// goExclusions returns exclusion patterns for the Go stack.
func goExclusions() []FileSizeExclusion {
	return []FileSizeExclusion{
		{Pattern: "*_generated.go", Description: "Generated files"},
		{Pattern: "*.pb.go", Description: "Generated files"},
		{Pattern: "*_gen.go", Description: "Generated files"},
		{Pattern: "*_string.go", Description: "Generated files"},
		{Pattern: "mock_*.go", Description: "Generated mock files"},
		{Pattern: "go.sum", Description: "Lock files"},
	}
}

// typescriptExclusions returns exclusion patterns for the TypeScript stack.
func typescriptExclusions() []FileSizeExclusion {
	return []FileSizeExclusion{
		{Pattern: "*.generated.ts", Description: "Generated files"},
		{Pattern: "*.d.ts", Description: "Generated type declaration files"},
		{Pattern: "*.min.js", Description: "Minified files"},
		{Pattern: "*.min.css", Description: "Minified files"},
		{Pattern: "dist/**", Description: "Build output"},
		{Pattern: "node_modules/**", Description: "Vendor dependencies"},
		{Pattern: "package-lock.json", Description: "Lock files"},
	}
}

// pythonExclusions returns exclusion patterns for the Python stack.
func pythonExclusions() []FileSizeExclusion {
	return []FileSizeExclusion{
		{Pattern: "*_pb2.py", Description: "Generated files"},
		{Pattern: "*_pb2_grpc.py", Description: "Generated files"},
		{Pattern: "*.pyc", Description: "Compiled Python files"},
		{Pattern: "__pycache__/**", Description: "Python cache directories"},
		{Pattern: "poetry.lock", Description: "Lock files"},
		{Pattern: "Pipfile.lock", Description: "Lock files"},
	}
}

// rustExclusions returns exclusion patterns for the Rust stack.
func rustExclusions() []FileSizeExclusion {
	return []FileSizeExclusion{
		{Pattern: "*.generated.rs", Description: "Generated files"},
		{Pattern: "build.rs", Description: "Build scripts"},
		{Pattern: "Cargo.lock", Description: "Lock files"},
	}
}

// frameworkExclusions returns extra exclusion patterns specific to a framework.
func frameworkExclusions(framework string) []FileSizeExclusion {
	switch framework {
	case "nextjs":
		return []FileSizeExclusion{
			{Pattern: ".next/**", Description: "Next.js build output"},
			{Pattern: "next-env.d.ts", Description: "Next.js generated types"},
		}
	case "nuxtjs":
		return []FileSizeExclusion{
			{Pattern: ".nuxt/**", Description: "Nuxt.js build output"},
			{Pattern: ".output/**", Description: "Nuxt.js output directory"},
		}
	case "django":
		return []FileSizeExclusion{
			{Pattern: "*/migrations/*.py", Description: "Django migration files"},
		}
	case "react":
		return []FileSizeExclusion{
			{Pattern: "build/**", Description: "React build output"},
		}
	case "vue":
		return []FileSizeExclusion{
			{Pattern: "dist/**", Description: "Vue build output"},
		}
	case "svelte":
		return []FileSizeExclusion{
			{Pattern: ".svelte-kit/**", Description: "SvelteKit build output"},
		}
	case "nestjs":
		return []FileSizeExclusion{
			{Pattern: "dist/**", Description: "NestJS build output"},
		}
	}
	// gin, echo, chi, fastapi, flask, axum — no extra exclusions
	return nil
}

// FileSizeExclusions returns exclusion patterns for the given stack and framework.
// Common exclusions are always included first, followed by stack-specific patterns,
// then framework-specific patterns.
func FileSizeExclusions(stack, framework string) []FileSizeExclusion {
	result := commonExclusions()

	switch stack {
	case "go":
		result = append(result, goExclusions()...)
	case "typescript":
		result = append(result, typescriptExclusions()...)
	case "python":
		result = append(result, pythonExclusions()...)
	case "rust":
		result = append(result, rustExclusions()...)
	}

	result = append(result, frameworkExclusions(framework)...)
	return result
}
