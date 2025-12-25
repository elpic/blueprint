package handlers

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

func TestInstallHandlerGetCommand(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "install single package on mac",
			rule: parser.Rule{
				Action:  "install",
				Packages: []parser.Package{{Name: "curl"}},
				OSList:  []string{"mac"},
			},
			expected: "sudo brew install curl",
		},
		{
			name: "install multiple packages on mac",
			rule: parser.Rule{
				Action: "install",
				Packages: []parser.Package{
					{Name: "git"},
					{Name: "curl"},
					{Name: "wget"},
				},
				OSList: []string{"mac"},
			},
			expected: "sudo brew install git curl wget",
		},
		{
			name: "install on linux",
			rule: parser.Rule{
				Action:  "install",
				Packages: []parser.Package{{Name: "curl"}},
				OSList:  []string{"linux"},
			},
			expected: "sudo apt-get install -y curl",
		},
		{
			name: "uninstall package on mac",
			rule: parser.Rule{
				Action:  "uninstall",
				Packages: []parser.Package{{Name: "curl"}},
				OSList:  []string{"mac"},
			},
			expected: "sudo brew uninstall -y curl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewInstallHandler(tt.rule, "")
			cmd := handler.GetCommand()
			if cmd != tt.expected {
				t.Errorf("GetCommand() = %q, want %q", cmd, tt.expected)
			}
		})
	}
}

func TestInstallHandlerBuildCommand(t *testing.T) {
	tests := []struct {
		name     string
		packages []parser.Package
		osName   string
		expected string
	}{
		{
			name: "brew install",
			packages: []parser.Package{
				{Name: "vim"},
				{Name: "git"},
			},
			osName:   "mac",
			expected: "brew install vim git",
		},
		{
			name: "apt-get install",
			packages: []parser.Package{
				{Name: "curl"},
			},
			osName:   "linux",
			expected: "apt-get install -y curl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := parser.Rule{
				Packages: tt.packages,
				OSList:   []string{tt.osName},
			}
			handler := NewInstallHandler(rule, "")
			cmd := handler.buildCommand()
			if cmd != tt.expected {
				t.Errorf("buildCommand() = %q, want %q", cmd, tt.expected)
			}
		})
	}
}

func TestInstallHandlerNeedsSudo(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		expected bool
	}{
		{
			name:     "brew command needs sudo",
			cmd:      "brew install curl",
			expected: true,
		},
		{
			name:     "apt-get needs sudo",
			cmd:      "apt-get install -y curl",
			expected: true,
		},
		{
			name:     "random command doesn't need sudo",
			cmd:      "echo hello",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := needsSudo(tt.cmd)
			if result != tt.expected {
				t.Errorf("needsSudo(%q) = %v, want %v", tt.cmd, result, tt.expected)
			}
		})
	}
}

func TestInstallHandlerUpdateStatus(t *testing.T) {
	tests := []struct {
		name              string
		rule              parser.Rule
		records           []ExecutionRecord
		initialStatus     Status
		expectedPackages  int
		shouldContainPkg  bool
		expectedPkgName   string
	}{
		{
			name: "add installed package to status",
			rule: parser.Rule{
				Action:  "install",
				OSList:  []string{"linux"},
				Packages: []parser.Package{
					{Name: "curl"},
				},
			},
			records: []ExecutionRecord{
				{
					Status:  "success",
					Command: "sudo apt-get install -y curl",
				},
			},
			initialStatus:    Status{},
			expectedPackages: 1,
			shouldContainPkg: true,
			expectedPkgName:  "curl",
		},
		{
			name: "remove package from status on uninstall",
			rule: parser.Rule{
				Action: "uninstall",
				Packages: []parser.Package{
					{Name: "curl"},
				},
			},
			records: []ExecutionRecord{
				{
					Status:  "success",
					Command: "sudo apt-get remove -y curl",
				},
			},
			initialStatus: Status{
				Packages: []PackageStatus{
					{Name: "curl", OS: "linux", Blueprint: "/tmp/test.bp"},
				},
			},
			expectedPackages: 0,
			shouldContainPkg: false,
		},
		{
			name: "multiple packages",
			rule: parser.Rule{
				Action:  "install",
				OSList:  []string{"linux"},
				Packages: []parser.Package{
					{Name: "git"},
					{Name: "curl"},
				},
			},
			records: []ExecutionRecord{
				{
					Status:  "success",
					Command: "sudo apt-get install -y git curl",
				},
			},
			initialStatus:    Status{},
			expectedPackages: 2,
			shouldContainPkg: true,
			expectedPkgName:  "git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewInstallHandler(tt.rule, "")
			status := tt.initialStatus

			err := handler.UpdateStatus(&status, tt.records, "/tmp/test.bp", "linux")
			if err != nil {
				t.Errorf("UpdateStatus() error = %v", err)
			}

			if len(status.Packages) != tt.expectedPackages {
				t.Errorf("UpdateStatus() packages count = %d, want %d", len(status.Packages), tt.expectedPackages)
			}

			if tt.shouldContainPkg && len(status.Packages) > 0 {
				found := false
				for _, pkg := range status.Packages {
					if pkg.Name == tt.expectedPkgName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("UpdateStatus() did not find package %q", tt.expectedPkgName)
				}
			}
		})
	}
}

func TestInstallHandlerDisplayInfo(t *testing.T) {
	tests := []struct {
		name             string
		rule             parser.Rule
		expectedContains []string
	}{
		{
			name: "install action with single package",
			rule: parser.Rule{
				Action: "install",
				Packages: []parser.Package{
					{Name: "git"},
				},
			},
			expectedContains: []string{"Packages:", "git"},
		},
		{
			name: "install action with multiple packages",
			rule: parser.Rule{
				Action: "install",
				Packages: []parser.Package{
					{Name: "git"},
					{Name: "vim"},
					{Name: "curl"},
				},
			},
			expectedContains: []string{"Packages:", "git", "vim", "curl"},
		},
		{
			name: "uninstall action with packages",
			rule: parser.Rule{
				Action: "uninstall",
				Packages: []parser.Package{
					{Name: "curl"},
					{Name: "wget"},
				},
			},
			expectedContains: []string{"Packages:", "curl", "wget"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewInstallHandler(tt.rule, "")

			// Capture stdout
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			handler.DisplayInfo()

			w.Close()
			os.Stdout = old

			// Read captured output
			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			// Verify expected content is present
			for _, expected := range tt.expectedContains {
				if !strings.Contains(output, expected) {
					t.Errorf("DisplayInfo() output missing expected content %q\nGot: %s", expected, output)
				}
			}
		})
	}
}
