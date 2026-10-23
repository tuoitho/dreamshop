package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
)

var titlePattern = regexp.MustCompile(`^(feat|fix|docs|refactor|test|build|ci|chore|perf|revert)(\([a-z0-9]+(?:-[a-z0-9]+)*\))?!?: \S.*$`)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("validate-pr-title", flag.ContinueOnError)
	fs.SetOutput(errOut)
	var title string
	fs.StringVar(&title, "title", "", "pull-request title to validate")
	fs.StringVar(&title, "Title", "", "pull-request title to validate")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if title == "" && fs.NArg() > 0 {
		title = fs.Arg(0)
	}

	if !titlePattern.MatchString(title) {
		fmt.Fprintf(errOut, `Invalid pull-request title: %s
Expected: <type>(<optional-scope>)<optional-breaking-marker>: <description>
Allowed types: feat, fix, docs, refactor, test, build, ci, chore, perf, revert
Examples: feat(order): add checkout validation | fix!: reject reused refresh tokens
`, title)
		return 1
	}

	fmt.Fprintf(out, "Valid pull-request title: %s\n", title)
	return 0
}
