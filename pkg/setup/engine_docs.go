package setup

type renderedDoc struct {
	Key     string
	Name    string
	Content string
}

func renderDocContents(docSet *DocSet) map[string]string {
	docs := make(map[string]string, len(renderedDocs(docSet)))
	for _, doc := range renderedDocs(docSet) {
		docs[doc.Name] = doc.Content
	}
	return docs
}

func renderedDocs(docSet *DocSet) []renderedDoc {
	return []renderedDoc{
		{Key: "index", Name: "index.md", Content: docSet.Index},
		{Key: "commands", Name: "commands.md", Content: docSet.Commands},
		{Key: "structure", Name: "structure.md", Content: docSet.Structure},
		{Key: "conventions", Name: "conventions.md", Content: docSet.Conventions},
		{Key: "boundaries", Name: "boundaries.md", Content: docSet.Boundaries},
		{Key: "architecture", Name: "architecture.md", Content: docSet.Architecture},
		{Key: "testing", Name: "testing.md", Content: docSet.Testing},
	}
}

func docSourceFiles(info *ProjectInfo, docType string) []string {
	var files []string
	switch docType {
	case "index", "structure", "commands", "conventions", "boundaries", "architecture", "testing":
		for _, buildFile := range info.BuildFiles {
			files = append(files, buildFile.Path)
		}
	}
	return files
}
