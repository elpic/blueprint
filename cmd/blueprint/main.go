package main

import (
	"fmt"
	"os"

	"github.com/elpic/blueprint/internal/engine"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: blueprint <plan|apply|status> <file.bp>")
		os.Exit(1)
	}

	mode := os.Args[1]

	switch mode {
	case "plan":
		if len(os.Args) < 3 {
			fmt.Println("Usage: blueprint plan <file.bp>")
			os.Exit(1)
		}
		file := os.Args[2]
		engine.Run(file, true) // dry-run
	case "apply":
		if len(os.Args) < 3 {
			fmt.Println("Usage: blueprint apply <file.bp>")
			os.Exit(1)
		}
		file := os.Args[2]
		engine.Run(file, false)
	case "status":
		engine.PrintStatus()
	default:
		// Short mode: blueprint setup.bp
		engine.Run(mode, false)
	}
}

