package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"ecommerce.local/scripts/internal/boundaries"
	"ecommerce.local/scripts/internal/repo"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	fs := flag.NewFlagSet("check-boundaries", flag.ContinueOnError)
	fs.SetOutput(out)
	var root string
	fs.StringVar(&root, "root", "", "repository root directory")
	fs.StringVar(&root, "Root", "", "repository root directory")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if root == "" {
		r, err := repo.Root()
		if err != nil {
			fmt.Fprintf(out, "error locating repository root: %v\n", err)
			return 1
		}
		root = r
	}
	return boundaries.Check(root, out)
}
