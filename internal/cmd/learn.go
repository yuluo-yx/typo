package cmd

import (
	"fmt"
	"os"

	"github.com/yuluo-yx/typo/internal/config"
)

func cmdLearn(args []string) int {
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "Error: exactly two arguments required: <from> and <to>")
		return 1
	}

	cfg := config.Load()
	if err := requireConfigDirectory(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	eng := createEngine(cfg)

	if err := eng.Learn(args[0], args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Printf("Learned: %s -> %s\n", args[0], args[1])
	return 0
}
