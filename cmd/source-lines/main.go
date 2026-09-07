// Command source-lines checks source paths against the shared comment-aware limit.
package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/linecount"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, out, errOut io.Writer) int {
	flags := flag.NewFlagSet("source-lines", flag.ContinueOnError)
	flags.SetOutput(errOut)
	max := flags.Int("max", 300, "Maximum source lines excluding comment-only lines")
	extension := flags.String("ext", "", "Optional source extension filter, such as .go")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *max < 0 || flags.NArg() == 0 {
		fmt.Fprintln(errOut, "usage: source-lines [--max 300] [--ext .go] PATH...")
		return 2
	}
	filter := strings.ToLower(strings.TrimSpace(*extension))
	if filter != "" && !strings.HasPrefix(filter, ".") {
		filter = "." + filter
	}
	checked, failures := 0, 0
	for _, root := range flags.Args() {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || (filter != "" && strings.ToLower(filepath.Ext(path)) != filter) {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			checked++
			// Physical is a byte scan; lexing only runs when the file could still be over.
			if linecount.Physical(data) <= *max {
				return nil
			}
			counts, err := linecount.Source(path, data)
			if err != nil {
				return err
			}
			if counts.Counted > *max {
				failures++
				fmt.Fprintf(out, "%s: %d counted lines > %d (%d comment-only lines excluded; %d physical)\n",
					path, counts.Counted, *max, counts.CommentOnly, counts.Physical)
			}
			return nil
		})
		if err != nil {
			fmt.Fprintf(errOut, "source-lines: %v\n", err)
			return 1
		}
	}
	if checked == 0 {
		fmt.Fprintln(errOut, "source-lines: no matching source files")
		return 1
	}
	if failures > 0 {
		return 1
	}
	fmt.Fprintf(out, "source-lines: %d files checked; limit %d excludes comment-only lines\n", checked, *max)
	return 0
}
