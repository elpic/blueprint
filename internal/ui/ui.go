package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Color styles
var (
	Success   = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)                 // Green
	Error     = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)                 // Red
	Info      = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)                 // Blue
	Highlight = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)                 // Yellow
	Dim       = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))                            // Gray
	Header    = lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true).Underline(true) // Magenta
)

// FormatHeader formats a header message
func FormatHeader(text string) string {
	return Header.Render(text)
}

// FormatSuccess formats a success message with checkmark
func FormatSuccess(text string) string {
	return Success.Render("✓ " + text)
}

// FormatError formats an error message with X
func FormatError(text string) string {
	return Error.Render("✗ " + text)
}

// FormatInfo formats an info message with info symbol
func FormatInfo(text string) string {
	return Info.Render(text)
}

// FormatHighlight formats a highlighted message
func FormatHighlight(text string) string {
	return Highlight.Render(text)
}

// FormatDim formats a dimmed message
func FormatDim(text string) string {
	return Dim.Render(text)
}

// PrintRuleSummary prints a formatted summary of rule execution
func PrintRuleSummary(index, total int, action, command string, status string) {
	var statusStr string
	switch status {
	case "success":
		statusStr = FormatSuccess("Done")
	case "error":
		statusStr = FormatError("Error")
	default:
		statusStr = FormatDim("Running")
	}

	fmt.Printf("[%d/%d] %s\n", index, total, FormatHighlight(action))
	fmt.Printf("       Command: %s\n", FormatDim(command))
	fmt.Printf("       %s\n\n", statusStr)
}

// PrintExecutionHeader prints the execution header with styling
func PrintExecutionHeader(isApplyMode bool, currentOS string, blueprintFile string, numRules, numAutoUninstall, numCleanups int) {
	if isApplyMode {
		fmt.Println(Header.Render("═══ [APPLY MODE] ═══") + "\n")
		fmt.Printf("OS: %s\n", FormatHighlight(currentOS))
		var executionInfo string
		// numCleanups already includes uninstall rules, so use it directly
		if numCleanups > 0 {
			executionInfo = fmt.Sprintf("Executing %s rules + %s cleanups from %s",
				FormatHighlight(fmt.Sprint(numRules)),
				FormatHighlight(fmt.Sprint(numCleanups)),
				FormatHighlight(blueprintFile))
		} else {
			executionInfo = fmt.Sprintf("Executing %s rules from %s",
				FormatHighlight(fmt.Sprint(numRules)),
				FormatHighlight(blueprintFile))
		}
		fmt.Printf("%s\n\n", executionInfo)
	} else {
		fmt.Println(Header.Render("═══ [PLAN MODE - DRY RUN] ═══") + "\n")
		fmt.Printf("Blueprint: %s\n", FormatHighlight(blueprintFile))
		fmt.Printf("Current OS: %s\n", FormatHighlight(currentOS))
		fmt.Printf("Applicable Rules: %s\n", FormatHighlight(fmt.Sprint(numRules)))
		// numCleanups already includes uninstall rules, so use it directly
		if numCleanups > 0 {
			fmt.Printf("Cleanups: %s\n", FormatHighlight(fmt.Sprint(numCleanups)))
		}
		fmt.Printf("\n")
	}
}

// PrintAutoUninstallSection prints the auto-uninstall section header
func PrintAutoUninstallSection() {
	fmt.Println(FormatDim("─── Auto-uninstall (removed from blueprint) ───") + "\n")
}

// PrintPlanFooter prints the footer message for plan mode
func PrintPlanFooter() {
	fmt.Println("\n" + FormatInfo("[No changes will be applied]"))
}

// PrintProgressBar prints a simple progress indicator
func PrintProgressBar(current, total int) {
	if total <= 0 {
		return
	}

	percentage := (current * 100) / total
	filled := (current * 50) / total
	empty := 50 - filled

	bar := "["
	for i := 0; i < filled; i++ {
		bar += "█"
	}
	for i := 0; i < empty; i++ {
		bar += "░"
	}
	bar += "]"

	fmt.Printf("\r%s %d%% (%d/%d)", bar, percentage, current, total)
}

// ClearProgressBar clears the progress bar line
func ClearProgressBar() {
	fmt.Print("\r" + FormatDim("                                                                        ") + "\r")
}
