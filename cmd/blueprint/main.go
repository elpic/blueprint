package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/elpic/blueprint/internal/engine"
	"github.com/elpic/blueprint/internal/logging"
)

// version and commit are set at build time via -ldflags.
// They default to "dev" / "none" for local development builds.
var version = "dev"
var commit = "none"

// parseFlags extracts --skip-group, --skip-id, --skip-decrypt, --only, --prefer-ssh, --no-status, and --debug flags from arguments
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
		case "--debug":
			logging.SetLogLevel(logging.DEBUG)
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

// isHelpFlag returns true if the argument is --help or -h.
func isHelpFlag(arg string) bool {
	return arg == "--help" || arg == "-h"
}

// hasHelpFlag returns true if any argument in args is --help or -h.
func hasHelpFlag(args []string) bool {
	for _, a := range args {
		if isHelpFlag(a) {
			return true
		}
	}
	return false
}

func printGlobalHelp() {
	fmt.Print(`blueprint - declarative machine setup tool

Usage:
  blueprint <command> [arguments]

Commands:
  plan      <file.bp>   Dry-run: show what would be applied
  apply     <file.bp>   Apply a blueprint (with automatic cleanup)
  validate  <file.bp>   Parse and semantically check a blueprint
  diff      <file.bp>   Show rules that differ from current status
  export    <file.bp>   Generate a shell script or Dockerfile from a blueprint
  render    <file.bp>   Render Go templates using blueprint data
  check     <file.bp>   Compare rendered output against existing files
  get       <file.bp>   Extract a value from a blueprint
  encrypt   <file>      Encrypt a file with AES-256-GCM
  status                Show installed resource state
  history               View execution history
  ps                    Show progress summary
  slow                  Show slowest rules from history
  doctor                Diagnose and optionally fix issues
  version               Show version information

Run 'blueprint <command> --help' for usage details on a specific command.
`)
}

func printPlanHelp() {
	fmt.Print(`blueprint plan - dry-run preview of what would be applied

Usage:
  blueprint plan <file.bp> [flags]

Arguments:
  <file.bp>           Path to the blueprint file

Flags:
  --skip-group <name> Skip all rules in the given group
  --skip-id <name>    Skip the rule with the given id
  --only <id>         Only run the rule with the given id
  --skip-decrypt      Skip encrypted rules (useful when no password is available)
  --prefer-ssh        Prefer SSH over HTTPS for git operations
  --debug             Enable debug logging (printed to stderr)
  --help, -h          Show this help message

Examples:
  blueprint plan setup.bp
  blueprint plan setup.bp --skip-group expensive
  blueprint plan setup.bp --only my-rule
`)
}

func printApplyHelp() {
	fmt.Print(`blueprint apply - apply a blueprint with automatic cleanup

Usage:
  blueprint apply <file.bp> [flags]

Arguments:
  <file.bp>           Path to the blueprint file

Flags:
  --skip-group <name> Skip all rules in the given group
  --skip-id <name>    Skip the rule with the given id
  --only <id>         Only run the rule with the given id
  --skip-decrypt      Skip encrypted rules (useful when no password is available)
  --prefer-ssh        Prefer SSH over HTTPS for git operations
  --no-status         Do not write to ~/.blueprint/status.json
  --debug             Enable debug logging (printed to stderr)
  --help, -h          Show this help message

Examples:
  blueprint apply setup.bp
  blueprint apply setup.bp --skip-group expensive --prefer-ssh
  blueprint apply setup.bp --only my-rule
  blueprint apply setup.bp --debug
`)
}

func printEncryptHelp() {
	fmt.Print(`blueprint encrypt - encrypt a file with AES-256-GCM

Usage:
  blueprint encrypt <file> [flags]

Arguments:
  <file>              Path to the file to encrypt

Flags:
  --password-id <id>  Named password identifier (default: "default")
  --help, -h          Show this help message

Examples:
  blueprint encrypt secrets.yaml
  blueprint encrypt secrets.yaml --password-id mypassword
`)
}

func printExportHelp() {
	fmt.Print(`blueprint export - generate a shell script or Dockerfile from a blueprint

Usage:
  blueprint export <file.bp> [flags]

Arguments:
  <file.bp>           Path to the blueprint file

Flags:
  --format <fmt>      Output format: bash (default) or sh
  --output <path>     Write output to a file instead of stdout
  --prefer-ssh        Prefer SSH over HTTPS for git operations
  --help, -h          Show this help message

Examples:
  blueprint export setup.bp
  blueprint export setup.bp --format sh --output setup.sh
  blueprint export setup.bp --prefer-ssh
`)
}

func printStatusHelp() {
	fmt.Print(`blueprint status - show installed resource state

Usage:
  blueprint status

Description:
  Reads ~/.blueprint/status.json and prints all tracked resources:
  installed packages, cloned repos, symlinks, downloads, and commands.

Flags:
  --help, -h          Show this help message
`)
}

func printHistoryHelp() {
	fmt.Print(`blueprint history - view execution history

Usage:
  blueprint history [run_number [step_number]] [flags]

Arguments:
  run_number          Show details for a specific run (0 = latest)
  step_number         Show details for a specific step within the run

Flags:
  --since <prefix>    Filter records by timestamp prefix (e.g. 2025, 2025-05, 2025-05-01)
  --blueprint <name>  Filter records by blueprint name substring
  --stats             Show aggregate stats instead of run details
  --help, -h          Show this help message

Examples:
  blueprint history                          # show latest run
  blueprint history 0                        # show latest run explicitly
  blueprint history 0 3                      # show step 3 of the latest run
  blueprint history --since 2025-05          # runs from May 2025
  blueprint history --blueprint dotfiles     # runs for a specific blueprint
  blueprint history --stats                  # aggregate stats
  blueprint history --stats --since 2025     # stats for this year
`)
}

func printPSHelp() {
	fmt.Print(`blueprint ps - show progress summary

Usage:
  blueprint ps

Description:
  Prints a compact summary of which rules are installed, skipped, or pending
  based on ~/.blueprint/status.json.

Flags:
  --help, -h          Show this help message
`)
}

func printSlowHelp() {
	fmt.Print(`blueprint slow - show slowest rules from history

Usage:
  blueprint slow [flags]

Flags:
  --top <n>           Show the top N slowest rules (default: 10)
  --help, -h          Show this help message

Examples:
  blueprint slow
  blueprint slow --top 20
`)
}

func printDiffHelp() {
	fmt.Print(`blueprint diff - show rules that differ from current status

Usage:
  blueprint diff <file.bp> [flags]

Arguments:
  <file.bp>           Path to the blueprint file

Flags:
  --prefer-ssh        Prefer SSH over HTTPS for git operations
  --help, -h          Show this help message

Examples:
  blueprint diff setup.bp
  blueprint diff setup.bp --prefer-ssh
`)
}

func printDoctorHelp() {
	fmt.Print(`blueprint doctor - diagnose and optionally fix issues

Usage:
  blueprint doctor [flags]

Description:
  Checks for common problems such as stale symlinks, missing clones,
  and orphaned downloads in ~/.blueprint/status.json.

Flags:
  --fix               Automatically fix detected issues
  --help, -h          Show this help message

Examples:
  blueprint doctor
  blueprint doctor --fix
`)
}

func printValidateHelp() {
	fmt.Print(`blueprint validate - parse and semantically check a blueprint

Usage:
  blueprint validate <file.bp> [flags]

Arguments:
  <file.bp>           Path to the blueprint file

Flags:
  --prefer-ssh        Prefer SSH over HTTPS for git operations
  --help, -h          Show this help message

Examples:
  blueprint validate setup.bp
`)
}

func printRenderHelp() {
	fmt.Print(`blueprint render - render Go templates using blueprint data

Usage:
  blueprint render <file.bp> --template <file.tmpl|dir> [flags]

Arguments:
  <file.bp>           Path to the blueprint file

Flags:
  --template <path>   Template file or directory to render (required)
  --output <path>     Write output to a file or directory instead of stdout
  --var KEY=VALUE     Set a template variable (repeatable)
  --prefer-ssh        Prefer SSH over HTTPS for git operations
  --help, -h          Show this help message

Examples:
  blueprint render setup.bp --template Dockerfile.tmpl
  blueprint render setup.bp --template templates/ --output out/
  blueprint render setup.bp --template ci.yml.tmpl --var ENV=production
`)
}

func printCheckHelp() {
	fmt.Print(`blueprint check - compare rendered output against existing files

Usage:
  blueprint check <file.bp> --template <file.tmpl|dir> [flags]

Arguments:
  <file.bp>           Path to the blueprint file

Flags:
  --template <path>   Template file or directory to render (required)
  --against <path>    File or directory to compare rendered output against
  --var KEY=VALUE     Set a template variable (repeatable)
  --prefer-ssh        Prefer SSH over HTTPS for git operations
  --help, -h          Show this help message

Examples:
  blueprint check setup.bp --template Dockerfile.tmpl --against Dockerfile
  blueprint check setup.bp --template templates/ --against out/
`)
}

func printGetHelp() {
	fmt.Print(`blueprint get - extract a value from a blueprint

Usage:
  blueprint get <file.bp> <action> <key> [flags]

Arguments:
  <file.bp>           Path to the blueprint file
  <action>            Rule action type (e.g. mise, asdf, homebrew, var)
  <key>               Key to look up within that action

Flags:
  --var KEY=VALUE     Set a variable (repeatable)
  --prefer-ssh        Prefer SSH over HTTPS for git operations
  --help, -h          Show this help message

Examples:
  blueprint get setup.bp mise ruby
  blueprint get setup.bp asdf nodejs
  blueprint get setup.bp homebrew formula
  blueprint get setup.bp var APP_NAME
`)
}

func printVersionHelp() {
	fmt.Print(`blueprint version - show version information

Usage:
  blueprint version [flags]

Flags:
  --short             Print only the version number
  --commit            Print only the commit hash
  --help, -h          Show this help message

Examples:
  blueprint version
  blueprint version --short
  blueprint version --commit
`)
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
	// When invoked via `go run`, os.Args[0] is a temp binary like /tmp/go-build.../exe/blueprint.
	// Detect this and set the hint name so "Run to fix" suggestions are copy-pasteable.
	if strings.Contains(os.Args[0], "go-build") {
		engine.ExecutableName = "go run ./cmd/blueprint"
	}

	if len(os.Args) < 2 || isHelpFlag(os.Args[1]) {
		printGlobalHelp()
		if len(os.Args) >= 2 {
			// --help or -h was explicitly passed — exit 0
			os.Exit(0)
		}
		os.Exit(1)
	}

	mode := os.Args[1]

	switch mode {
	case "version":
		args := os.Args[2:]
		if hasHelpFlag(args) {
			printVersionHelp()
			os.Exit(0)
		}
		if len(args) > 0 && args[0] == "--commit" {
			fmt.Println(commit)
		} else if len(args) > 0 && args[0] == "--short" {
			fmt.Println(version)
		} else {
			fmt.Printf("Version: %s\nCommit:  %s\n", version, commit)
		}
	case "history":
		if hasHelpFlag(os.Args[2:]) {
			printHistoryHelp()
			os.Exit(0)
		}
		var since, blueprintFilter string
		var statsOnly bool
		args := os.Args[2:]
		var positional []string
		for i := 0; i < len(args); i++ {
			switch {
			case args[i] == "--stats":
				statsOnly = true
			case args[i] == "--since" && i+1 < len(args):
				i++
				since = args[i]
			case strings.HasPrefix(args[i], "--since="):
				since = strings.TrimPrefix(args[i], "--since=")
			case args[i] == "--blueprint" && i+1 < len(args):
				i++
				blueprintFilter = args[i]
			case strings.HasPrefix(args[i], "--blueprint="):
				blueprintFilter = strings.TrimPrefix(args[i], "--blueprint=")
			default:
				positional = append(positional, args[i])
			}
		}
		runNumber := 0
		stepNumber := -1
		if len(positional) >= 1 {
			n, ok := parseNonNegativeInt(positional[0], "run_number")
			if !ok {
				os.Exit(1)
			}
			runNumber = n
		}
		if len(positional) >= 2 {
			n, ok := parseNonNegativeInt(positional[1], "step_number")
			if !ok {
				os.Exit(1)
			}
			stepNumber = n
		}
		if statsOnly {
			engine.PrintHistoryStats(since, blueprintFilter)
		} else {
			engine.PrintHistory(runNumber, stepNumber, since, blueprintFilter)
		}
	case "plan":
		if hasHelpFlag(os.Args[2:]) {
			printPlanHelp()
			os.Exit(0)
		}
		if len(os.Args) < 3 {
			printPlanHelp()
			os.Exit(1)
		}
		file := os.Args[2]
		skipGroup, skipID, onlyID, skipDecrypt, preferSSH, _ := parseFlags(os.Args[3:])
		os.Exit(engine.RunWithSkip(file, true, skipGroup, skipID, onlyID, skipDecrypt, preferSSH, false))
	case "apply":
		if hasHelpFlag(os.Args[2:]) {
			printApplyHelp()
			os.Exit(0)
		}
		if len(os.Args) < 3 {
			printApplyHelp()
			os.Exit(1)
		}
		file := os.Args[2]
		skipGroup, skipID, onlyID, skipDecrypt, preferSSH, noStatus := parseFlags(os.Args[3:])
		os.Exit(engine.RunWithSkip(file, false, skipGroup, skipID, onlyID, skipDecrypt, preferSSH, noStatus))
	case "encrypt":
		if hasHelpFlag(os.Args[2:]) {
			printEncryptHelp()
			os.Exit(0)
		}
		if len(os.Args) < 3 {
			printEncryptHelp()
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
		if hasHelpFlag(os.Args[2:]) {
			printExportHelp()
			os.Exit(0)
		}
		if len(os.Args) < 3 {
			printExportHelp()
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
		if hasHelpFlag(os.Args[2:]) {
			printStatusHelp()
			os.Exit(0)
		}
		engine.PrintStatus()
	case "ps":
		if hasHelpFlag(os.Args[2:]) {
			printPSHelp()
			os.Exit(0)
		}
		engine.PrintPS()
	case "diff":
		if hasHelpFlag(os.Args[2:]) {
			printDiffHelp()
			os.Exit(0)
		}
		if len(os.Args) < 3 {
			printDiffHelp()
			os.Exit(1)
		}
		_, _, _, _, preferSSH, _ := parseFlags(os.Args[3:])
		engine.PrintDiff(os.Args[2], preferSSH)
	case "slow":
		if hasHelpFlag(os.Args[2:]) {
			printSlowHelp()
			os.Exit(0)
		}
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
		if hasHelpFlag(os.Args[2:]) {
			printDoctorHelp()
			os.Exit(0)
		}
		fix := false
		for _, arg := range os.Args[2:] {
			if arg == "--fix" {
				fix = true
			}
		}
		engine.DoctorCheck(fix)
	case "validate":
		if hasHelpFlag(os.Args[2:]) {
			printValidateHelp()
			os.Exit(0)
		}
		if len(os.Args) < 3 {
			printValidateHelp()
			os.Exit(1)
		}
		_, _, _, _, preferSSH, _ := parseFlags(os.Args[3:])
		engine.Validate(os.Args[2], preferSSH)
	case "render":
		if hasHelpFlag(os.Args[2:]) {
			printRenderHelp()
			os.Exit(0)
		}
		if len(os.Args) < 3 {
			printRenderHelp()
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
		if hasHelpFlag(os.Args[2:]) {
			printCheckHelp()
			os.Exit(0)
		}
		if len(os.Args) < 3 {
			printCheckHelp()
			os.Exit(1)
		}
		file := os.Args[2]
		tmplPath := ""
		against := ""
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
			}
		}
		if tmplPath == "" {
			fmt.Fprintln(os.Stderr, "error: --template <file.tmpl|dir> is required")
			os.Exit(1)
		}
		cliVars := parseVarFlags(os.Args[3:])
		engine.Check(file, tmplPath, against, preferSSH, cliVars)
	case "get":
		if hasHelpFlag(os.Args[2:]) {
			printGetHelp()
			os.Exit(0)
		}
		if len(os.Args) < 5 {
			printGetHelp()
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
				os.Exit(engine.Run(mode, false))
				return
			}
		}
		fmt.Fprintln(os.Stderr, unknownCommandMessage(mode))
		os.Exit(1)
	}
}
