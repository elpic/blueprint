package engine

import (
	"fmt"
	cryptopkg "github.com/elpic/blueprint/internal/crypto"
	handlerskg "github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
	"golang.org/x/term"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strconv"
)

func EncryptFile(filePath string, passwordID string) {
	// Check if file exists
	if _, err := os.Stat(filePath); err != nil {
		fmt.Printf("%s\n", ui.FormatError(fmt.Sprintf("File not found: %s", filePath)))
		os.Exit(1)
	}

	// Read file content
	plaintext, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("%s\n", ui.FormatError(fmt.Sprintf("Failed to read file: %v", err)))
		os.Exit(1)
	}

	// Prompt for password
	fmt.Printf("Enter password for %s: ", filePath)
	password, err := readPassword()
	if err != nil {
		fmt.Printf("%s\n", ui.FormatError(fmt.Sprintf("Failed to read password: %v", err)))
		os.Exit(1)
	}

	// Encrypt file
	encryptedData, err := cryptopkg.EncryptFile(plaintext, password)
	if err != nil {
		fmt.Printf("%s\n", ui.FormatError(fmt.Sprintf("Encryption failed: %v", err)))
		os.Exit(1)
	}

	// Write encrypted file with .enc extension
	encryptedPath := filePath + ".enc"
	if err := os.WriteFile(encryptedPath, encryptedData, 0600); err != nil {
		fmt.Printf("%s\n", ui.FormatError(fmt.Sprintf("Failed to write encrypted file: %v", err)))
		os.Exit(1)
	}

	fmt.Printf("%s\n", ui.FormatSuccess(fmt.Sprintf("File encrypted: %s -> %s", filePath, encryptedPath)))
}

// promptForDecryptPasswords collects all unique password-ids from decrypt rules and prompts for passwords upfront

func promptForDecryptPasswords(rules []parser.Rule) error {
	// Collect unique password-ids from decrypt rules
	passwordIDsMap := make(map[string]bool)
	var passwordIDs []string

	for _, rule := range rules {
		if rule.Action == "decrypt" {
			passwordID := rule.DecryptPasswordID
			if passwordID == "" {
				passwordID = "default"
			}

			// Only add if we haven't seen this password-id before
			if !passwordIDsMap[passwordID] {
				passwordIDsMap[passwordID] = true
				passwordIDs = append(passwordIDs, passwordID)
			}
		}
	}

	// If there are no decrypt rules, return early
	if len(passwordIDs) == 0 {
		return nil
	}

	// Prompt for each unique password-id
	for _, passwordID := range passwordIDs {
		fmt.Printf("Enter password for %s: ", ui.FormatHighlight(passwordID))
		password, err := readPassword()
		if err != nil {
			return fmt.Errorf("failed to read password for %s: %w", passwordID, err)
		}
		// Cache the password
		passwordCache[passwordID] = password
	}

	return nil
}

// readPassword reads a password from stdin without echoing using x/term

func readPassword() (string, error) {
	// Read password from stdin with terminal echo disabled
	bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	fmt.Println() // Print newline after password prompt
	return string(bytePassword), nil
}


func promptForSudoPasswordWithOS(rules []parser.Rule, currentOS string) error {
	// Check if we're on Linux and not root
	if runtime.GOOS != "linux" {
		return nil
	}

	currentUser, err := user.Current()
	if err == nil {
		uid, err := strconv.Atoi(currentUser.Uid)
		if err == nil && uid == 0 {
			// Already root, no sudo needed
			return nil
		}
	}

	// Check if user has passwordless sudo (sudo -n true)
	// If this succeeds, user can run sudo without password
	if cmd := exec.Command("sudo", "-n", "true"); cmd.Run() == nil {
		// User has passwordless sudo, no need to prompt
		return nil
	}

	// Check if any rule needs sudo by asking the handler
	// This delegates sudo requirement determination to each handler type
	needsSudoPassword := false
	for _, rule := range rules {
		handler := handlerskg.NewHandler(rule, "", make(map[string]string))
		if sudoAwareHandler, ok := handler.(handlerskg.SudoAwareHandler); ok {
			if sudoAwareHandler.NeedsSudo() {
				needsSudoPassword = true
				break
			}
		}
	}

	// If sudo is needed, prompt for password upfront
	if needsSudoPassword {
		fmt.Printf("Enter sudo password: ")
		password, err := readPassword()
		if err != nil {
			return fmt.Errorf("failed to read sudo password: %w", err)
		}
		// Cache the sudo password
		passwordCache["sudo"] = password
	}

	return nil
}
