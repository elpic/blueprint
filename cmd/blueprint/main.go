package main

import (
	"fmt"
	"os"

	"github.com/elpic/blueprint/internal/engine"
)

// version and commit are set at build time via -ldflags.
// They default to "dev" / "none" for local development builds.
var version = "dev"
var commit = "none"

// parseFlags extracts --skip-group, --skip-id, --skip-decrypt, --only, and --prefer-ssh flags from arguments
func parseFlags(args []string) (skipGroup, skipID, onlyID string, skipDecrypt, preferSSH bool) {
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
		case "--prefer-ssh":
			preferSSH = true
		}
	}
	return
}

var knownCommands = map[string]bool{
	"plan": true, "apply": true, "encrypt": true,
	"status": true, "history": true, "ps": true, "slow": true, "diff": true,
	"version": true, "doctor": true, "validate": true,
}

func isKnownCommand(cmd string) bool {
	return knownCommands[cmd]
}

func unknownCommandMessage(cmd string) string {
	return fmt.Sprintf("unknown command: %q\nUsage: blueprint <plan|apply|encrypt|status|history|ps|slow|diff|doctor|validate|version> [<file>]", cmd)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: blueprint <plan|apply|encrypt|status|history|ps|slow|diff|doctor|validate|version> [<file|run_number>]")
		os.Exit(1)
	}

	mode := os.Args[1]

	switch mode {
	case "version":
		args := os.Args[2:]
		if len(args) > 0 && args[0] == "--commit" {
			fmt.Println(commit)
		} else if len(args) > 0 && args[0] == "--short" {
			fmt.Println(version)
		} else {
			fmt.Printf("Version: %s\nCommit:  %s\n", version, commit)
		}
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
			fmt.Println("Usage: blueprint plan <file.bp> [--skip-group <name>] [--skip-id <name>] [--only <id>] [--skip-decrypt] [--prefer-ssh]")
			os.Exit(1)
		}
		file := os.Args[2]
		skipGroup, skipID, onlyID, skipDecrypt, preferSSH := parseFlags(os.Args[3:])
		engine.RunWithSkip(file, true, skipGroup, skipID, onlyID, skipDecrypt, preferSSH) // dry-run
	case "apply":
		if len(os.Args) < 3 {
			fmt.Println("Usage: blueprint apply <file.bp> [--skip-group <name>] [--skip-id <name>] [--only <id>] [--skip-decrypt] [--prefer-ssh]")
			os.Exit(1)
		}
		file := os.Args[2]
		skipGroup, skipID, onlyID, skipDecrypt, preferSSH := parseFlags(os.Args[3:])
		engine.RunWithSkip(file, false, skipGroup, skipID, onlyID, skipDecrypt, preferSSH)
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
	case "diff":
		if len(os.Args) < 3 {
			fmt.Println("Usage: blueprint diff <file.bp> [--prefer-ssh]")
			os.Exit(1)
		}
		_, _, _, _, preferSSH := parseFlags(os.Args[3:])
		engine.PrintDiff(os.Args[2], preferSSH)
	case "slow":
		topN := 10
		for i := 2; i < len(os.Args); i++ {
			if os.Args[i] == "--top" && i+1 < len(os.Args) {
				_, _ = fmt.Sscanf(os.Args[i+1], "%d", &topN)
				i++
			}
		}
		engine.PrintSlow(topN)
	case "doctor":
		fix := false
		for _, arg := range os.Args[2:] {
			if arg == "--fix" {
				fix = true
			}
		}
		engine.DoctorCheck(fix)
	case "validate":
		if len(os.Args) < 3 {
			fmt.Println("Usage: blueprint validate <file.bp> [--prefer-ssh]")
			os.Exit(1)
		}
		_, _, _, _, preferSSH := parseFlags(os.Args[3:])
		engine.Validate(os.Args[2], preferSSH)
	default:
		// Short mode: treat as file path only if it looks like a path (not a known command typo).
		if !isKnownCommand(mode) {
			if _, err := os.Stat(mode); err == nil { // #nosec G703 -- user-supplied file path is intentional
				engine.Run(mode, false)
				return
			}
		}
		fmt.Fprintln(os.Stderr, unknownCommandMessage(mode))
		os.Exit(1)
	}
}
