package main

import (
	"fmt"
	"os"

	"github.com/elpic/blueprint/internal/engine"
)

// parseSkipFlags extracts --skip-group and --skip-id flags from arguments
func parseSkipFlags(args []string) (string, string) {
	var skipGroup, skipID string
	for i := 0; i < len(args); i++ {
		if args[i] == "--skip-group" && i+1 < len(args) {
			skipGroup = args[i+1]
			i++ // Skip next arg
		} else if args[i] == "--skip-id" && i+1 < len(args) {
			skipID = args[i+1]
			i++ // Skip next arg
		}
	}
	return skipGroup, skipID
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: blueprint <plan|apply|encrypt|status|history> [<file|run_number>]")
		os.Exit(1)
	}

	mode := os.Args[1]

	switch mode {
	case "history":
		runNumber := 0
		stepNumber := -1
		if len(os.Args) >= 3 {
			_, _ = fmt.Sscanf(os.Args[2], "%d", &runNumber)
		}
		if len(os.Args) >= 4 {
			_, _ = fmt.Sscanf(os.Args[3], "%d", &stepNumber)
		}
		engine.PrintHistory(runNumber, stepNumber)
	case "plan":
		if len(os.Args) < 3 {
			fmt.Println("Usage: blueprint plan <file.bp> [--skip-group <name>] [--skip-id <name>]")
			os.Exit(1)
		}
		file := os.Args[2]
		skipGroup, skipID := parseSkipFlags(os.Args[3:])
		engine.RunWithSkip(file, true, skipGroup, skipID) // dry-run
	case "apply":
		if len(os.Args) < 3 {
			fmt.Println("Usage: blueprint apply <file.bp> [--skip-group <name>] [--skip-id <name>]")
			os.Exit(1)
		}
		file := os.Args[2]
		skipGroup, skipID := parseSkipFlags(os.Args[3:])
		engine.RunWithSkip(file, false, skipGroup, skipID)
	case "encrypt":
		if len(os.Args) < 3 {
			fmt.Println("Usage: blueprint encrypt <file> [--password-id <id>]")
			os.Exit(1)
		}
		file := os.Args[2]
		passwordID := "default"
		// Check for --password-id flag
		for i := 3; i < len(os.Args); i++ {
			if os.Args[i] == "--password-id" && i+1 < len(os.Args) {
				passwordID = os.Args[i+1]
				break
			}
		}
		engine.EncryptFile(file, passwordID)
	case "status":
		engine.PrintStatus()
	default:
		// Short mode: blueprint setup.bp
		engine.Run(mode, false)
	}
}

