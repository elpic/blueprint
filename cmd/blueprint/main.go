package main

import (
	"fmt"
	"os"

	"github.com/elpic/blueprint/internal/engine"
)

// parseFlags extracts --skip-group, --skip-id, --skip-decrypt, and --only flags from arguments
func parseFlags(args []string) (skipGroup, skipID, onlyID string, skipDecrypt bool) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--skip-group":
			if i+1 < len(args) {
				skipGroup = args[i+1]
				i++
			}
		case "--skip-id":
			if i+1 < len(args) {
				skipID = args[i+1]
				i++
			}
		case "--only":
			if i+1 < len(args) {
				onlyID = args[i+1]
				i++
			}
		case "--skip-decrypt":
			skipDecrypt = true
		}
	}
	return
}

var knownCommands = map[string]bool{
	"plan": true, "apply": true, "encrypt": true,
	"status": true, "history": true, "ps": true,
}

func isKnownCommand(cmd string) bool {
	return knownCommands[cmd]
}

func unknownCommandMessage(cmd string) string {
	return fmt.Sprintf("unknown command: %q\nUsage: blueprint <plan|apply|encrypt|status|history|ps> [<file>]", cmd)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: blueprint <plan|apply|encrypt|status|history|ps> [<file|run_number>]")
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
			fmt.Println("Usage: blueprint plan <file.bp> [--skip-group <name>] [--skip-id <name>] [--only <id>] [--skip-decrypt]")
			os.Exit(1)
		}
		file := os.Args[2]
		skipGroup, skipID, onlyID, skipDecrypt := parseFlags(os.Args[3:])
		engine.RunWithSkip(file, true, skipGroup, skipID, onlyID, skipDecrypt) // dry-run
	case "apply":
		if len(os.Args) < 3 {
			fmt.Println("Usage: blueprint apply <file.bp> [--skip-group <name>] [--skip-id <name>] [--only <id>] [--skip-decrypt]")
			os.Exit(1)
		}
		file := os.Args[2]
		skipGroup, skipID, onlyID, skipDecrypt := parseFlags(os.Args[3:])
		engine.RunWithSkip(file, false, skipGroup, skipID, onlyID, skipDecrypt)
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
	case "ps":
		engine.PrintPS()
	default:
		// Short mode: treat as file path only if it looks like a path (not a known command typo).
		if !isKnownCommand(mode) {
			if _, err := os.Stat(mode); err == nil {
				engine.Run(mode, false)
				return
			}
		}
		fmt.Fprintln(os.Stderr, unknownCommandMessage(mode))
		os.Exit(1)
	}
}
