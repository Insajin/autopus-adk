// Command generate-templates regenerates platform templates from content sources.
package main

import (
	"fmt"
	"os"

	"github.com/insajin/autopus-adk/pkg/content"
)

func main() {
	contentDir := "content"
	templateDir := "templates"

	if len(os.Args) >= 3 {
		contentDir = os.Args[1]
		templateDir = os.Args[2]
	}

	fmt.Printf("Generating templates: %s → %s\n", contentDir, templateDir)

	if err := content.GenerateAllTemplates(contentDir, templateDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Done.")
}
