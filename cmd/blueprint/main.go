package main

import (
	"fmt"
	"os"

	"github.com/elpic/blueprint/internal/engine"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: blueprint <plan|apply> <file.bp>")
		os.Exit(1)
	}

	mode := os.Args[1]
	file := os.Args[len(os.Args)-1]

	switch mode {
	case "plan":
		engine.Run(file, true) // dry-run
	case "apply":
		engine.Run(file, false)
	default:
		// modo corto: blueprint setup.bp
		engine.Run(mode, false)
	}
}

