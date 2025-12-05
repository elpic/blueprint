package engine

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strconv"
	"strings"
	"github.com/elpic/blueprint/internal/git"
	"github.com/elpic/blueprint/internal/parser"
)

func Run(file string, dry bool) {
	var setupPath string
	var err error

	// Check if input is a git URL
	if git.IsGitURL(file) {
		// Clone the repository (show progress in dry run mode, hide in apply mode)
		tempDir, setupFile, err := git.CloneRepository(file, dry)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer git.CleanupRepository(tempDir)

		// Find setup file in the cloned repo
		setupPath, err = git.FindSetupFile(tempDir, setupFile)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
	} else {
		// Local file
		setupPath = file
	}

	// Parse the setup file (with include support for local files)
	var rules []parser.Rule
	if git.IsGitURL(file) {
		// For git URLs, read content and parse with string parsing
		data, err := os.ReadFile(setupPath)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		rules, err = parser.Parse(string(data))
		if err != nil {
			fmt.Println("Parse error:", err)
			return
		}
	} else {
		// For local files, use ParseFile which supports includes
		rules, err = parser.ParseFile(setupPath)
		if err != nil {
			fmt.Println("Parse error:", err)
			return
		}
	}

	// Filter rules by current OS
	filteredRules := filterRulesByOS(rules)
	currentOS := getOSName()

	if dry {
		fmt.Println("=== [PLAN MODE - DRY RUN] ===\n")
		fmt.Printf("Blueprint: %s\n", file)
		fmt.Printf("Current OS: %s\n", currentOS)
		fmt.Printf("Applicable Rules: %d\n\n", len(filteredRules))
		displayRules(filteredRules)
		fmt.Println("\n[No changes will be applied]")
	} else {
		fmt.Println("=== [APPLY MODE] ===\n")
		fmt.Printf("OS: %s\n", currentOS)
		fmt.Printf("Executing %d rules from %s\n\n", len(filteredRules), file)
		executeRules(filteredRules)
	}
}

// getOSName returns the current operating system name
func getOSName() string {
	switch runtime.GOOS {
	case "darwin":
		return "mac"
	case "linux":
		return "linux"
	case "windows":
		return "windows"
	default:
		return runtime.GOOS
	}
}

// filterRulesByOS filters rules to only include those for the current OS
func filterRulesByOS(rules []parser.Rule) []parser.Rule {
	currentOS := getOSName()
	var filtered []parser.Rule

	for _, rule := range rules {
		// Check if rule applies to current OS
		for _, os := range rule.OSList {
			if strings.TrimSpace(os) == currentOS {
				filtered = append(filtered, rule)
				break
			}
		}
	}

	return filtered
}

func displayRules(rules []parser.Rule) {
	for i, rule := range rules {
		fmt.Printf("Rule #%d:\n", i+1)
		fmt.Printf("  Action: %s\n", rule.Action)

		if len(rule.Packages) > 0 {
			fmt.Print("  Packages: ")
			for j, pkg := range rule.Packages {
				if j > 0 {
					fmt.Print(", ")
				}
				fmt.Print(pkg.Name)
			}
			fmt.Println()
		}

		if len(rule.OSList) > 0 {
			fmt.Print("  On: ")
			for j, os := range rule.OSList {
				if j > 0 {
					fmt.Print(", ")
				}
				fmt.Print(os)
			}
			fmt.Println()
		}

		if rule.Tool != "" {
			fmt.Printf("  Tool: %s\n", rule.Tool)
		}

		// Display command that will be executed
		cmd := buildCommand(rule)
		fmt.Printf("  Command: %s\n", cmd)
		fmt.Println()
	}
}

func buildCommand(rule parser.Rule) string {
	if rule.Action == "install" && len(rule.Packages) > 0 {
		pkgNames := ""
		for i, pkg := range rule.Packages {
			if i > 0 {
				pkgNames += " "
			}
			pkgNames += pkg.Name
		}

		// Use brew for macOS, apt for Linux
		if len(rule.OSList) > 0 && rule.OSList[0] == "mac" {
			return fmt.Sprintf("brew install %s", pkgNames)
		}
		return fmt.Sprintf("apt-get install -y %s", pkgNames)
	}
	return fmt.Sprintf("%s %v", rule.Action, rule.Packages)
}

// needsSudo checks if a command requires sudo elevation
func needsSudo(command string) bool {
	// Only on Linux
	if runtime.GOOS != "linux" {
		return false
	}

	// Check if current user is root
	currentUser, err := user.Current()
	if err == nil {
		uid, err := strconv.Atoi(currentUser.Uid)
		if err == nil && uid == 0 {
			// Already root, no sudo needed
			return false
		}
	}

	// Package managers that require sudo on Linux
	sudoRequired := []string{
		"apt", "apt-get", "aptitude",
		"yum", "dnf",
		"pacman", "pamac",
		"zypper",
		"emerge",
		"opkg",
		"apk",
		"pkg",
	}

	cmdName := strings.Fields(command)[0]
	for _, pm := range sudoRequired {
		if cmdName == pm {
			return true
		}
	}

	return false
}

// executeCommand parses and executes a command string
func executeCommand(cmdStr string) (string, error) {
	// Check if sudo is needed
	if needsSudo(cmdStr) {
		cmdStr = "sudo " + cmdStr
	}

	// Parse command string into parts
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	// Create command
	cmd := exec.Command(parts[0], parts[1:]...)

	// Capture output
	output, err := cmd.CombinedOutput()

	return string(output), err
}

func executeRules(rules []parser.Rule) {
	for i, rule := range rules {
		fmt.Printf("[%d/%d] Executing: %s\n", i+1, len(rules), rule.Action)
		cmd := buildCommand(rule)

		// Show actual command including sudo if needed
		actualCmd := cmd
		if needsSudo(cmd) {
			actualCmd = "sudo " + cmd
		}
		fmt.Printf("       Command: %s\n", actualCmd)

		// Execute the command
		output, err := executeCommand(cmd)

		if err != nil {
			fmt.Printf("       ✗ Error: %v\n", err)
			if output != "" {
				fmt.Printf("       %s\n", strings.TrimSpace(output))
			}
		} else {
			if output != "" {
				fmt.Printf("       %s\n", strings.TrimSpace(output))
			}
			fmt.Println("       ✓ Done")
		}
		fmt.Println()
	}
}

