package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/elpic/blueprint/internal/engine"
)

// version and commit are set at build time via -ldflags.
// They default to "dev" / "none" for local development builds.
var version = "dev"
var commit = "none"

// parseFlags extracts --skip-group, --skip-id, --skip-decrypt, --only, --prefer-ssh, and --no-status flags from arguments
func parseFlags(args []string) (skipGroup, skipID, onlyID string, skipDecrypt, preferSSH, noStatus bool) {
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
		case "--no-status":
			noStatus = true
		}
	}
	return
}

var knownCommands = map[string]bool{
	"plan": true, "apply": true, "encrypt": true, "export": true,
	"status": true, "history": true, "ps": true, "slow": true, "diff": true,
	"version": true, "doctor": true, "validate": true,
	"render": true, "check": true, "get": true,
}

// parseVarFlags extracts all --var KEY=VALUE pairs from args into a map.
func parseVarFlags(args []string) map[string]string {
	vars := map[string]string{}
	for i := 0; i < len(args); i++ {
		if args[i] == "--var" && i+1 < len(args) {
			kv := args[i+1]
			i++
			idx := strings.Index(kv, "=")
			if idx < 0 {
				fmt.Fprintf(os.Stderr, "error: --var must be KEY=VALUE, got %q\n", kv)
				os.Exit(1)
			}
			vars[kv[:idx]] = kv[idx+1:]
		}
	}
	return vars
}

func isKnownCommand(cmd string) bool {
	return knownCommands[cmd]
}

func unknownCommandMessage(cmd string) string {
	return fmt.Sprintf("unknown command: %q\nUsage: blueprint <plan|apply|encrypt|export|status|history|ps|slow|diff|doctor|validate|version> [<file>]", cmd)
}

// parseNonNegativeInt parses s as a non-negative integer. On any error it
// writes a human-readable message to stderr and returns -1, false.
func parseNonNegativeInt(s, flagName string) (int, bool) {
	n, err := strconv.Atoi(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s must be a valid integer, got %q\n", flagName, s)
		return -1, false
	}
	if n < 0 {
		fmt.Fprintf(os.Stderr, "error: %s must be a non-negative integer, got %d\n", flagName, n)
		return -1, false
	}
	return n, true
}

// parsePositiveInt parses s as a positive integer (>= 1). On any error it
// writes a human-readable message to stderr and returns -1, false.
func parsePositiveInt(s, flagName string) (int, bool) {
	n, err := strconv.Atoi(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s must be a valid integer, got %q\n", flagName, s)
		return -1, false
	}
	if n < 1 {
		fmt.Fprintf(os.Stderr, "error: %s must be a positive integer (>= 1), got %d\n", flagName, n)
		return -1, false
	}
	return n, true
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: blueprint <plan|apply|encrypt|export|status|history|ps|slow|diff|doctor|validate|version> [<file|run_number>]")
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
			n, ok := parseNonNegativeInt(os.Args[2], "run_number")
			if !ok {
				os.Exit(1)
			}
			runNumber = n
		}
		if len(os.Args) >= 4 {
			n, ok := parseNonNegativeInt(os.Args[3], "step_number")
			if !ok {
				os.Exit(1)
			}
			stepNumber = n
		}
		engine.PrintHistory(runNumber, stepNumber)
	case "plan":
		if len(os.Args) < 3 {
			fmt.Println("Usage: blueprint plan <file.bp> [--skip-group <name>] [--skip-id <name>] [--only <id>] [--skip-decrypt] [--prefer-ssh]")
			os.Exit(1)
		}
		file := os.Args[2]
		skipGroup, skipID, onlyID, skipDecrypt, preferSSH, _ := parseFlags(os.Args[3:])
		engine.RunWithSkip(file, true, skipGroup, skipID, onlyID, skipDecrypt, preferSSH, false) // dry-run
	case "apply":
		if len(os.Args) < 3 {
			fmt.Println("Usage: blueprint apply <file.bp> [--skip-group <name>] [--skip-id <name>] [--only <id>] [--skip-decrypt] [--prefer-ssh] [--no-status]")
			os.Exit(1)
		}
		file := os.Args[2]
		skipGroup, skipID, onlyID, skipDecrypt, preferSSH, noStatus := parseFlags(os.Args[3:])
		engine.RunWithSkip(file, false, skipGroup, skipID, onlyID, skipDecrypt, preferSSH, noStatus)
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
	case "export":
		if len(os.Args) < 3 {
			fmt.Println("Usage: blueprint export <file.bp> [--format bash|sh] [--output <path>] [--prefer-ssh]")
			os.Exit(1)
		}
		file := os.Args[2]
		format := "bash"
		output := ""
		_, _, _, _, preferSSH, _ := parseFlags(os.Args[3:])
		for i := 3; i < len(os.Args); i++ {
			switch os.Args[i] {
			case "--format":
				if i+1 < len(os.Args) {
					format = os.Args[i+1]
					i++
				}
			case "--output":
				if i+1 < len(os.Args) {
					output = os.Args[i+1]
					i++
				}
			}
		}
		if format != "bash" && format != "sh" {
			fmt.Fprintf(os.Stderr, "error: --format must be \"bash\" or \"sh\", got %q\n", format)
			os.Exit(1)
		}
		engine.Export(file, format, output, preferSSH)
	case "status":
		engine.PrintStatus()
	case "ps":
		engine.PrintPS()
	case "diff":
		if len(os.Args) < 3 {
			fmt.Println("Usage: blueprint diff <file.bp> [--prefer-ssh]")
			os.Exit(1)
		}
		_, _, _, _, preferSSH, _ := parseFlags(os.Args[3:])
		engine.PrintDiff(os.Args[2], preferSSH)
	case "slow":
		topN := 10
		for i := 2; i < len(os.Args); i++ {
			if os.Args[i] == "--top" && i+1 < len(os.Args) {
				n, ok := parsePositiveInt(os.Args[i+1], "--top")
				if !ok {
					os.Exit(1)
				}
				topN = n
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
		_, _, _, _, preferSSH, _ := parseFlags(os.Args[3:])
		engine.Validate(os.Args[2], preferSSH)
	case "render":
		if len(os.Args) < 3 {
			fmt.Println("Usage: blueprint render <file.bp> --template <file.tmpl|dir> [--output <path>] [--var KEY=VALUE] [--prefer-ssh]")
			os.Exit(1)
		}
		file := os.Args[2]
		tmplPath := ""
		output := ""
		_, _, _, _, preferSSH, _ := parseFlags(os.Args[3:])
		for i := 3; i < len(os.Args); i++ {
			switch os.Args[i] {
			case "--template":
				if i+1 < len(os.Args) {
					tmplPath = os.Args[i+1]
					i++
				}
			case "--output":
				if i+1 < len(os.Args) {
					output = os.Args[i+1]
					i++
				}
			}
		}
		if tmplPath == "" {
			fmt.Fprintln(os.Stderr, "error: --template <file.tmpl|dir> is required")
			os.Exit(1)
		}
		cliVars := parseVarFlags(os.Args[3:])
		engine.Render(file, tmplPath, output, preferSSH, cliVars)
	case "check":
		if len(os.Args) < 3 {
			fmt.Println("Usage: blueprint check <file.bp> --template <file.tmpl|dir> [--against <file>] [--output <dir>] [--var KEY=VALUE] [--prefer-ssh]")
			os.Exit(1)
		}
		file := os.Args[2]
		tmplPath := ""
		against := ""
		outputRoot := ""
		_, _, _, _, preferSSH, _ := parseFlags(os.Args[3:])
		for i := 3; i < len(os.Args); i++ {
			switch os.Args[i] {
			case "--template":
				if i+1 < len(os.Args) {
					tmplPath = os.Args[i+1]
					i++
				}
			case "--against":
				if i+1 < len(os.Args) {
					against = os.Args[i+1]
					i++
				}
			case "--output":
				if i+1 < len(os.Args) {
					outputRoot = os.Args[i+1]
					i++
				}
			}
		}
		if tmplPath == "" {
			fmt.Fprintln(os.Stderr, "error: --template <file.tmpl|dir> is required")
			os.Exit(1)
		}
		cliVars := parseVarFlags(os.Args[3:])
		engine.Check(file, tmplPath, against, outputRoot, preferSSH, cliVars)
	case "get":
		if len(os.Args) < 5 {
			fmt.Println("Usage: blueprint get <file.bp> <action> <key> [--var KEY=VALUE]")
			fmt.Println("Examples:")
			fmt.Println("  blueprint get setup.bp mise ruby")
			fmt.Println("  blueprint get setup.bp asdf nodejs")
			fmt.Println("  blueprint get setup.bp homebrew formula")
			fmt.Println("  blueprint get setup.bp var APP_NAME")
			os.Exit(1)
		}
		file := os.Args[2]
		action := os.Args[3]
		key := os.Args[4]
		_, _, _, _, preferSSH, _ := parseFlags(os.Args[5:])
		cliVars := parseVarFlags(os.Args[5:])
		engine.Get(file, action, key, preferSSH, cliVars)
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
