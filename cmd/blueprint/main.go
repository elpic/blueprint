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
	"template": true,
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
  render    <file.bp>       Render Go templates using blueprint data
  check     <file.bp>       Compare rendered output against existing files
  get       <file.bp>       Extract a value from a blueprint
  template  <template-path>  Scaffold a project from a template directory (interactive)
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
  --no-status         Do not store status: skip writing ~/.blueprint/status.json.
                      Resources from this run go untracked, so a later apply will
                      not auto-uninstall them. History is still recorded.
  --var KEY=VALUE     Override or set a blueprint variable (can be repeated)
  --debug             Enable debug logging (printed to stderr)
  --help, -h          Show this help message

Examples:
  blueprint apply setup.bp
  blueprint apply setup.bp --skip-group expensive --prefer-ssh
  blueprint apply setup.bp --only my-rule
  blueprint apply setup.bp --no-status
  blueprint apply @github:elpic/blueprint --var WORKSPACE=~/other/path
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
  --verbose, -v       Show detailed progress for each check
  --help, -h          Show this help message

Examples:
  blueprint doctor
  blueprint doctor --fix
  blueprint doctor --verbose
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

func printTemplateHelp() {
	fmt.Print(`blueprint template - scaffold a project from a template directory

Usage:
  blueprint template <template-path> --output <output-dir> [flags]

Arguments:
  <template-path>      Path to a template directory (local path, @github: shorthand, or git URL)

Flags:
  --output <dir>      Output directory where rendered files are written (required)
  --var KEY=VALUE     Pre-set a template variable (repeatable) — skips the prompt for that variable
  --prefer-ssh        Prefer SSH over HTTPS for git operations
  --help, -h          Show this help message

Description:
  Scans the template directory for .tmpl files, discovers all required variables
  via {{ var "NAME" }} calls, prompts you interactively for each one, and renders
  the full template tree into the output directory.

  If the template directory contains a setup.bp with var rules, their default
  values are pre-filled and shown in the prompt. Variables without defaults
  are required.

  Use --var to skip prompting for known values. Useful for complex variables
  like JSON arrays or when automating from scripts.

Examples:
  blueprint template ./my-template --output ./my-project
  blueprint template @github:user/templates@main:python-service --output ./my-api
  blueprint template @github:user/templates@main:python-service --output ./my-api --var PORT=9000
  blueprint template ~/templates/web-app --output ./frontend --prefer-ssh
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
// It returns (nil, false) when a --var value is malformed (missing '='),
// having already printed the error to stderr. Callers must return their
// error exit code when ok is false.
func parseVarFlags(args []string) (map[string]string, bool) {
	vars := map[string]string{}
	for i := 0; i < len(args); i++ {
		if args[i] == "--var" && i+1 < len(args) {
			kv := args[i+1]
			i++
			idx := strings.Index(kv, "=")
			if idx < 0 {
				fmt.Fprintf(os.Stderr, "error: --var must be KEY=VALUE, got %q\n", kv)
				return nil, false
			}
			vars[kv[:idx]] = kv[idx+1:]
		}
	}
	return vars, true
}

func isKnownCommand(cmd string) bool {
	return knownCommands[cmd]
}

func unknownCommandMessage(cmd string) string {
	return fmt.Sprintf("unknown command: %q\nUsage: blueprint <plan|apply|encrypt|export|status|history|ps|slow|diff|doctor|validate|version|render|check|get|template> [<file>]", cmd)
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

	os.Exit(ExecuteCommand(os.Args[1:]))
}

// ExecuteCommand runs the blueprint CLI for the given arguments (the arguments
// after the program name, i.e. os.Args[1:]) and returns the process exit code
// instead of calling os.Exit. This makes the command dispatch testable in-process.
func ExecuteCommand(args []string) int {
	if len(args) < 1 || isHelpFlag(args[0]) {
		printGlobalHelp()
		if len(args) >= 1 {
			// --help or -h was explicitly passed — exit 0
			return 0
		}
		return 1
	}

	mode := args[0]

	switch mode {
	case "version":
		cmdArgs := args[1:]
		if hasHelpFlag(cmdArgs) {
			printVersionHelp()
			return 0
		}
		if len(cmdArgs) > 0 && cmdArgs[0] == "--commit" {
			fmt.Println(commit)
		} else if len(cmdArgs) > 0 && cmdArgs[0] == "--short" {
			fmt.Println(version)
		} else {
			fmt.Printf("Version: %s\nCommit:  %s\n", version, commit)
		}
	case "history":
		if hasHelpFlag(args[1:]) {
			printHistoryHelp()
			return 0
		}
		var since, blueprintFilter string
		var statsOnly bool
		cmdArgs := args[1:]
		var positional []string
		for i := 0; i < len(cmdArgs); i++ {
			switch {
			case cmdArgs[i] == "--stats":
				statsOnly = true
			case cmdArgs[i] == "--since" && i+1 < len(cmdArgs):
				i++
				since = cmdArgs[i]
			case strings.HasPrefix(cmdArgs[i], "--since="):
				since = strings.TrimPrefix(cmdArgs[i], "--since=")
			case cmdArgs[i] == "--blueprint" && i+1 < len(cmdArgs):
				i++
				blueprintFilter = cmdArgs[i]
			case strings.HasPrefix(cmdArgs[i], "--blueprint="):
				blueprintFilter = strings.TrimPrefix(cmdArgs[i], "--blueprint=")
			default:
				positional = append(positional, cmdArgs[i])
			}
		}
		runNumber := 0
		stepNumber := -1
		if len(positional) >= 1 {
			n, ok := parseNonNegativeInt(positional[0], "run_number")
			if !ok {
				return 1
			}
			runNumber = n
		}
		if len(positional) >= 2 {
			n, ok := parseNonNegativeInt(positional[1], "step_number")
			if !ok {
				return 1
			}
			stepNumber = n
		}
		if statsOnly {
			engine.PrintHistoryStats(since, blueprintFilter)
		} else {
			engine.PrintHistory(runNumber, stepNumber, since, blueprintFilter)
		}
	case "plan":
		if hasHelpFlag(args[1:]) {
			printPlanHelp()
			return 0
		}
		if len(args) < 2 {
			printPlanHelp()
			return 1
		}
		file := args[1]
		skipGroup, skipID, onlyID, skipDecrypt, preferSSH, _ := parseFlags(args[2:])
		cliVars, ok := parseVarFlags(args[2:])
		if !ok {
			return 1
		}
		return engine.RunWithSkip(file, true, skipGroup, skipID, onlyID, skipDecrypt, preferSSH, false, cliVars)
	case "apply":
		if hasHelpFlag(args[1:]) {
			printApplyHelp()
			return 0
		}
		if len(args) < 2 {
			printApplyHelp()
			return 1
		}
		file := args[1]
		skipGroup, skipID, onlyID, skipDecrypt, preferSSH, noStatus := parseFlags(args[2:])
		cliVars, ok := parseVarFlags(args[2:])
		if !ok {
			return 1
		}
		return engine.RunWithSkip(file, false, skipGroup, skipID, onlyID, skipDecrypt, preferSSH, noStatus, cliVars)
	case "encrypt":
		if hasHelpFlag(args[1:]) {
			printEncryptHelp()
			return 0
		}
		if len(args) < 2 {
			printEncryptHelp()
			return 1
		}
		file := args[1]
		passwordID := "default"
		// Check for --password-id flag
		for i := 2; i < len(args); i++ {
			if args[i] == "--password-id" && i+1 < len(args) {
				passwordID = args[i+1]
				break
			}
		}
		engine.EncryptFile(file, passwordID)
	case "export":
		if hasHelpFlag(args[1:]) {
			printExportHelp()
			return 0
		}
		if len(args) < 2 {
			printExportHelp()
			return 1
		}
		file := args[1]
		format := "bash"
		output := ""
		_, _, _, _, preferSSH, _ := parseFlags(args[2:])
		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--format":
				if i+1 < len(args) {
					format = args[i+1]
					i++
				}
			case "--output":
				if i+1 < len(args) {
					output = args[i+1]
					i++
				}
			}
		}
		if format != "bash" && format != "sh" {
			fmt.Fprintf(os.Stderr, "error: --format must be \"bash\" or \"sh\", got %q\n", format)
			return 1
		}
		engine.Export(file, format, output, preferSSH)
	case "status":
		if hasHelpFlag(args[1:]) {
			printStatusHelp()
			return 0
		}
		engine.PrintStatus()
	case "ps":
		if hasHelpFlag(args[1:]) {
			printPSHelp()
			return 0
		}
		engine.PrintPS()
	case "diff":
		if hasHelpFlag(args[1:]) {
			printDiffHelp()
			return 0
		}
		if len(args) < 2 {
			printDiffHelp()
			return 1
		}
		_, _, _, _, preferSSH, _ := parseFlags(args[2:])
		engine.PrintDiff(args[1], preferSSH)
	case "slow":
		if hasHelpFlag(args[1:]) {
			printSlowHelp()
			return 0
		}
		topN := 10
		for i := 1; i < len(args); i++ {
			if args[i] == "--top" && i+1 < len(args) {
				n, ok := parsePositiveInt(args[i+1], "--top")
				if !ok {
					return 1
				}
				topN = n
				i++
			}
		}
		engine.PrintSlow(topN)
	case "doctor":
		if hasHelpFlag(args[1:]) {
			printDoctorHelp()
			return 0
		}
		fix := false
		verbose := false
		for _, arg := range args[1:] {
			if arg == "--fix" {
				fix = true
			}
			if arg == "--verbose" || arg == "-v" {
				verbose = true
			}
		}
		engine.DoctorCheck(fix, verbose)
	case "validate":
		if hasHelpFlag(args[1:]) {
			printValidateHelp()
			return 0
		}
		if len(args) < 2 {
			printValidateHelp()
			return 1
		}
		_, _, _, _, preferSSH, _ := parseFlags(args[2:])
		engine.Validate(args[1], preferSSH)
	case "render":
		if hasHelpFlag(args[1:]) {
			printRenderHelp()
			return 0
		}
		if len(args) < 2 {
			printRenderHelp()
			return 1
		}
		file := args[1]
		tmplPath := ""
		output := ""
		_, _, _, _, preferSSH, _ := parseFlags(args[2:])
		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--template":
				if i+1 < len(args) {
					tmplPath = args[i+1]
					i++
				}
			case "--output":
				if i+1 < len(args) {
					output = args[i+1]
					i++
				}
			}
		}
		if tmplPath == "" {
			fmt.Fprintln(os.Stderr, "error: --template <file.tmpl|dir> is required")
			return 1
		}
		cliVars, ok := parseVarFlags(args[2:])
		if !ok {
			return 1
		}
		engine.Render(file, tmplPath, output, preferSSH, cliVars)
	case "check":
		if hasHelpFlag(args[1:]) {
			printCheckHelp()
			return 0
		}
		if len(args) < 2 {
			printCheckHelp()
			return 1
		}
		file := args[1]
		tmplPath := ""
		against := ""
		_, _, _, _, preferSSH, _ := parseFlags(args[2:])
		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--template":
				if i+1 < len(args) {
					tmplPath = args[i+1]
					i++
				}
			case "--against":
				if i+1 < len(args) {
					against = args[i+1]
					i++
				}
			}
		}
		if tmplPath == "" {
			fmt.Fprintln(os.Stderr, "error: --template <file.tmpl|dir> is required")
			return 1
		}
		cliVars, ok := parseVarFlags(args[2:])
		if !ok {
			return 1
		}
		engine.Check(file, tmplPath, against, preferSSH, cliVars)
	case "template":
		if hasHelpFlag(args[1:]) {
			printTemplateHelp()
			return 0
		}
		if len(args) < 2 {
			printTemplateHelp()
			return 1
		}
		tmplPath := args[1]
		output := ""
		_, _, _, _, preferSSH, _ := parseFlags(args[2:])
		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--output":
				if i+1 < len(args) {
					output = args[i+1]
					i++
				}
			}
		}
		if output == "" {
			fmt.Fprintln(os.Stderr, "error: --output <dir> is required")
			return 1
		}
		cliVars, ok := parseVarFlags(args[2:])
		if !ok {
			return 1
		}
		engine.Template(tmplPath, output, preferSSH, cliVars)
	case "get":
		if hasHelpFlag(args[1:]) {
			printGetHelp()
			return 0
		}
		if len(args) < 4 {
			printGetHelp()
			return 1
		}
		file := args[1]
		action := args[2]
		key := args[3]
		_, _, _, _, preferSSH, _ := parseFlags(args[4:])
		cliVars, ok := parseVarFlags(args[4:])
		if !ok {
			return 1
		}
		engine.Get(file, action, key, preferSSH, cliVars)
	default:
		// Short mode: treat as file path only if it looks like a path (not a known command typo).
		if !isKnownCommand(mode) {
			if _, err := os.Stat(mode); err == nil { // #nosec G703 -- user-supplied file path is intentional
				return engine.Run(mode, false)
			}
		}
		fmt.Fprintln(os.Stderr, unknownCommandMessage(mode))
		return 1
	}
	return 0
}
