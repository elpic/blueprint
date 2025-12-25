package parser

import (
	"os"
	"testing"
)

// TestParseGPGKeyBasic tests basic GPG key rule parsing
func TestParseGPGKeyBasic(t *testing.T) {
	tests := []struct {
		name    string
		input   string // input WITH the "gpg-key " prefix
		want    *Rule
		wantErr bool
	}{
		{
			name:  "basic gpg-key rule",
			input: "gpg-key https://apt.fury.io/wez/gpg.key keyring: wezterm-fury deb-url: https://apt.fury.io/wez/ on: [linux]",
			want: &Rule{
				Action:     "gpg-key",
				GPGKeyURL:  "https://apt.fury.io/wez/gpg.key",
				GPGKeyring: "wezterm-fury",
				GPGDebURL:  "https://apt.fury.io/wez/",
				OSList:     []string{"linux"},
			},
			wantErr: false,
		},
		{
			name:  "gpg-key with multiple OS platforms",
			input: "gpg-key https://example.com/gpg.key keyring: example-repo deb-url: https://example.com/apt on: [linux, ubuntu, debian]",
			want: &Rule{
				Action:     "gpg-key",
				GPGKeyURL:  "https://example.com/gpg.key",
				GPGKeyring: "example-repo",
				GPGDebURL:  "https://example.com/apt",
				OSList:     []string{"linux", "ubuntu", "debian"},
			},
			wantErr: false,
		},
		{
			name:  "gpg-key without on clause (applies to all OS)",
			input: "gpg-key https://apt.fury.io/wez/gpg.key keyring: wezterm-fury deb-url: https://apt.fury.io/wez/",
			want: &Rule{
				Action:     "gpg-key",
				GPGKeyURL:  "https://apt.fury.io/wez/gpg.key",
				GPGKeyring: "wezterm-fury",
				GPGDebURL:  "https://apt.fury.io/wez/",
				OSList:     []string{},
			},
			wantErr: false,
		},
		{
			name:  "gpg-key with id parameter",
			input: "gpg-key https://apt.fury.io/wez/gpg.key keyring: wezterm-fury deb-url: https://apt.fury.io/wez/ id: wezterm-setup on: [linux]",
			want: &Rule{
				ID:         "wezterm-setup",
				Action:     "gpg-key",
				GPGKeyURL:  "https://apt.fury.io/wez/gpg.key",
				GPGKeyring: "wezterm-fury",
				GPGDebURL:  "https://apt.fury.io/wez/",
				OSList:     []string{"linux"},
			},
			wantErr: false,
		},
		{
			name:  "gpg-key with after dependency",
			input: "gpg-key https://apt.fury.io/wez/gpg.key keyring: wezterm-fury deb-url: https://apt.fury.io/wez/ after: curl-setup on: [linux]",
			want: &Rule{
				Action:     "gpg-key",
				GPGKeyURL:  "https://apt.fury.io/wez/gpg.key",
				GPGKeyring: "wezterm-fury",
				GPGDebURL:  "https://apt.fury.io/wez/",
				After:      []string{"curl-setup"},
				OSList:     []string{"linux"},
			},
			wantErr: false,
		},
		{
			name:  "gpg-key with multiple dependencies",
			input: "gpg-key https://apt.fury.io/wez/gpg.key keyring: wezterm-fury deb-url: https://apt.fury.io/wez/ after: curl-setup, base-tools on: [linux]",
			want: &Rule{
				Action:     "gpg-key",
				GPGKeyURL:  "https://apt.fury.io/wez/gpg.key",
				GPGKeyring: "wezterm-fury",
				GPGDebURL:  "https://apt.fury.io/wez/",
				After:      []string{"curl-setup", "base-tools"},
				OSList:     []string{"linux"},
			},
			wantErr: false,
		},
		{
			name:  "gpg-key with id and after",
			input: "gpg-key https://apt.fury.io/wez/gpg.key keyring: wezterm-fury deb-url: https://apt.fury.io/wez/ id: wezterm-setup after: curl-setup on: [linux]",
			want: &Rule{
				ID:         "wezterm-setup",
				Action:     "gpg-key",
				GPGKeyURL:  "https://apt.fury.io/wez/gpg.key",
				GPGKeyring: "wezterm-fury",
				GPGDebURL:  "https://apt.fury.io/wez/",
				After:      []string{"curl-setup"},
				OSList:     []string{"linux"},
			},
			wantErr: false,
		},
		{
			name:    "gpg-key missing URL",
			input:   "gpg-key keyring: wezterm-fury deb-url: https://apt.fury.io/wez/ on: [linux]",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "gpg-key missing keyring",
			input:   "gpg-key https://apt.fury.io/wez/gpg.key deb-url: https://apt.fury.io/wez/ on: [linux]",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "gpg-key missing deb-url",
			input:   "gpg-key https://apt.fury.io/wez/gpg.key keyring: wezterm-fury on: [linux]",
			want:    nil,
			wantErr: true,
		},
		{
			name:  "gpg-key with complex URL",
			input: "gpg-key https://example.com/path/to/gpg.key keyring: my-repo deb-url: https://example.com/repository/apt/ on: [linux]",
			want: &Rule{
				Action:     "gpg-key",
				GPGKeyURL:  "https://example.com/path/to/gpg.key",
				GPGKeyring: "my-repo",
				GPGDebURL:  "https://example.com/repository/apt/",
				OSList:     []string{"linux"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGPGKeyRule(tt.input)

			if (got == nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("parseGPGKeyRule() = %v, want error", got)
				} else {
					t.Errorf("parseGPGKeyRule() got nil, want %v", tt.want)
				}
				return
			}

			if got == nil {
				return
			}

			// Check all fields
			if got.Action != tt.want.Action {
				t.Errorf("Action: got %q, want %q", got.Action, tt.want.Action)
			}
			if got.GPGKeyURL != tt.want.GPGKeyURL {
				t.Errorf("GPGKeyURL: got %q, want %q", got.GPGKeyURL, tt.want.GPGKeyURL)
			}
			if got.GPGKeyring != tt.want.GPGKeyring {
				t.Errorf("GPGKeyring: got %q, want %q", got.GPGKeyring, tt.want.GPGKeyring)
			}
			if got.GPGDebURL != tt.want.GPGDebURL {
				t.Errorf("GPGDebURL: got %q, want %q", got.GPGDebURL, tt.want.GPGDebURL)
			}
			if got.ID != tt.want.ID {
				t.Errorf("ID: got %q, want %q", got.ID, tt.want.ID)
			}

			// Check OSList
			if len(got.OSList) != len(tt.want.OSList) {
				t.Errorf("OSList length: got %d, want %d", len(got.OSList), len(tt.want.OSList))
			} else {
				for i, os := range got.OSList {
					if os != tt.want.OSList[i] {
						t.Errorf("OSList[%d]: got %q, want %q", i, os, tt.want.OSList[i])
					}
				}
			}

			// Check After dependencies
			if len(got.After) != len(tt.want.After) {
				t.Errorf("After length: got %d, want %d", len(got.After), len(tt.want.After))
			} else {
				for i, dep := range got.After {
					if dep != tt.want.After[i] {
						t.Errorf("After[%d]: got %q, want %q", i, dep, tt.want.After[i])
					}
				}
			}
		})
	}
}

// TestParseGPGKeyRuleWithParse tests parsing GPG key rules through the main Parse function
func TestParseGPGKeyRuleWithParse(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
	}{
		{
			name: "single gpg-key rule",
			input: `# Test blueprint
gpg-key https://apt.fury.io/wez/gpg.key keyring: wezterm-fury deb-url: https://apt.fury.io/wez/ on: [linux]
`,
			wantCount: 1,
		},
		{
			name: "multiple rules including gpg-key",
			input: `mkdir /tmp/test on: [linux]
gpg-key https://apt.fury.io/wez/gpg.key keyring: wezterm-fury deb-url: https://apt.fury.io/wez/ on: [linux]
install wget curl on: [linux]
`,
			wantCount: 3,
		},
		{
			name: "gpg-key with comments and blank lines",
			input: `# Add GPG key for wezterm
gpg-key https://apt.fury.io/wez/gpg.key keyring: wezterm-fury deb-url: https://apt.fury.io/wez/ on: [linux]

# Another comment
install vim on: [linux, mac]
`,
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if len(rules) != tt.wantCount {
				t.Errorf("Parse() got %d rules, want %d", len(rules), tt.wantCount)
			}

			// Verify GPG key rules have correct structure
			for _, rule := range rules {
				if rule.Action == "gpg-key" {
					if rule.GPGKeyring == "" {
						t.Error("GPG key rule has empty keyring")
					}
					if rule.GPGKeyURL == "" {
						t.Error("GPG key rule has empty URL")
					}
					if rule.GPGDebURL == "" {
						t.Error("GPG key rule has empty deb-url")
					}
				}
			}
		})
	}
}

// TestParseGPGKeyFieldOrder tests that GPG key parsing works regardless of field order
func TestParseGPGKeyFieldOrder(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "standard order: url, keyring, deb-url, on",
			input: "gpg-key https://apt.fury.io/wez/gpg.key keyring: wezterm-fury deb-url: https://apt.fury.io/wez/ on: [linux]",
		},
		{
			name:  "different order: url, deb-url, keyring, on",
			input: "gpg-key https://apt.fury.io/wez/gpg.key deb-url: https://apt.fury.io/wez/ keyring: wezterm-fury on: [linux]",
		},
		{
			name:  "with id and after mixed in",
			input: "gpg-key https://apt.fury.io/wez/gpg.key id: setup-wez keyring: wezterm-fury deb-url: https://apt.fury.io/wez/ after: prep on: [linux]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGPGKeyRule(tt.input)

			if got == nil {
				t.Error("parseGPGKeyRule() got nil, want valid rule")
				return
			}

			// All variants should produce the same core values
			if got.GPGKeyURL != "https://apt.fury.io/wez/gpg.key" {
				t.Errorf("GPGKeyURL: got %q", got.GPGKeyURL)
			}
			if got.GPGKeyring != "wezterm-fury" {
				t.Errorf("GPGKeyring: got %q", got.GPGKeyring)
			}
			if got.GPGDebURL != "https://apt.fury.io/wez/" {
				t.Errorf("GPGDebURL: got %q", got.GPGDebURL)
			}
		})
	}
}

// TestParseGPGKeyEdgeCases tests edge cases in GPG key parsing
func TestParseGPGKeyEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValid bool
	}{
		{
			name:      "minimal valid rule",
			input:     "gpg-key https://example.com/key keyring: test deb-url: https://example.com/apt",
			wantValid: true,
		},
		{
			name:      "empty OS list is valid",
			input:     "gpg-key https://example.com/key keyring: test deb-url: https://example.com/apt on: []",
			wantValid: true,
		},
		{
			name:      "extra whitespace",
			input:     "gpg-key   https://example.com/key   keyring:   test   deb-url:   https://example.com/apt   on:   [linux]",
			wantValid: true,
		},
		{
			name:      "keyring with special characters",
			input:     "gpg-key https://example.com/key keyring: my-repo-2024 deb-url: https://example.com/apt",
			wantValid: true,
		},
		{
			name:      "URL with port number",
			input:     "gpg-key https://example.com:8443/gpg.key keyring: secure-repo deb-url: https://example.com:8443/apt",
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGPGKeyRule(tt.input)

			if (got != nil) != tt.wantValid {
				if tt.wantValid {
					t.Error("parseGPGKeyRule() got nil, want valid rule")
				} else {
					t.Error("parseGPGKeyRule() got valid rule, want nil")
				}
			}
		})
	}
}

// TestParseGPGKeyIntegration tests GPG key parsing in realistic blueprints
func TestParseGPGKeyIntegration(t *testing.T) {
	blueprint := `# Setup development environment with multiple repositories

# Create cache directory
mkdir ~/.cache on: [linux, mac]

# Add wezterm repository
gpg-key https://apt.fury.io/wez/gpg.key keyring: wezterm-fury deb-url: https://apt.fury.io/wez/ id: wezterm-setup on: [linux]

# Add Docker repository with dependency
gpg-key https://download.docker.com/linux/ubuntu/gpg keyring: docker deb-url: https://download.docker.com/linux/ubuntu after: wezterm-setup id: docker-setup on: [linux]

# Install packages
install vim gcc on: [linux, mac]
`

	rules, err := Parse(blueprint)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Find GPG key rules
	gpgRules := make([]*Rule, 0)
	for i := range rules {
		if rules[i].Action == "gpg-key" {
			gpgRules = append(gpgRules, &rules[i])
		}
	}

	if len(gpgRules) != 2 {
		t.Errorf("Expected 2 GPG key rules, got %d", len(gpgRules))
		return
	}

	// Verify first GPG key rule
	if gpgRules[0].ID != "wezterm-setup" {
		t.Errorf("First rule ID: got %q, want 'wezterm-setup'", gpgRules[0].ID)
	}
	if gpgRules[0].GPGKeyring != "wezterm-fury" {
		t.Errorf("First rule keyring: got %q, want 'wezterm-fury'", gpgRules[0].GPGKeyring)
	}

	// Verify second GPG key rule with dependency
	if gpgRules[1].ID != "docker-setup" {
		t.Errorf("Second rule ID: got %q, want 'docker-setup'", gpgRules[1].ID)
	}
	if gpgRules[1].GPGKeyring != "docker" {
		t.Errorf("Second rule keyring: got %q, want 'docker'", gpgRules[1].GPGKeyring)
	}
	if len(gpgRules[1].After) != 1 || gpgRules[1].After[0] != "wezterm-setup" {
		t.Errorf("Second rule dependencies: got %v, want ['wezterm-setup']", gpgRules[1].After)
	}
}

// TestParseInstallRule tests install rule parsing
func TestParseInstallRule(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Rule
		wantErr bool
	}{
		{
			name:  "basic install single package",
			input: "install curl",
			want: &Rule{
				Action: "install",
				Packages: []Package{
					{Name: "curl", Version: "latest"},
				},
			},
			wantErr: false,
		},
		{
			name:  "install multiple packages",
			input: "install git vim curl wget",
			want: &Rule{
				Action: "install",
				Packages: []Package{
					{Name: "git", Version: "latest"},
					{Name: "vim", Version: "latest"},
					{Name: "curl", Version: "latest"},
					{Name: "wget", Version: "latest"},
				},
			},
			wantErr: false,
		},
		{
			name:  "install with OS filter",
			input: "install nodejs npm on: [linux, mac]",
			want: &Rule{
				Action: "install",
				Packages: []Package{
					{Name: "nodejs", Version: "latest"},
					{Name: "npm", Version: "latest"},
				},
				OSList: []string{"linux", "mac"},
			},
			wantErr: false,
		},
		{
			name:  "install with id",
			input: "install python3-pip id: python-setup on: [linux]",
			want: &Rule{
				ID:     "python-setup",
				Action: "install",
				Packages: []Package{
					{Name: "python3-pip", Version: "latest"},
				},
				OSList: []string{"linux"},
			},
			wantErr: false,
		},
		{
			name:  "install with after dependency",
			input: "install build-essential after: base-tools on: [linux]",
			want: &Rule{
				Action: "install",
				Packages: []Package{
					{Name: "build-essential", Version: "latest"},
				},
				After:  []string{"base-tools"},
				OSList: []string{"linux"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseInstallRule(tt.input)

			if (got == nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("parseInstallRule() = %v, want error", got)
				} else {
					t.Errorf("parseInstallRule() got nil, want %v", tt.want)
				}
				return
			}

			if got == nil {
				return
			}

			if got.Action != tt.want.Action {
				t.Errorf("Action: got %q, want %q", got.Action, tt.want.Action)
			}
			if len(got.Packages) != len(tt.want.Packages) {
				t.Errorf("Packages count: got %d, want %d", len(got.Packages), len(tt.want.Packages))
			}
		})
	}
}

// TestParseMkdirRule tests mkdir rule parsing
func TestParseMkdirRule(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Rule
		wantErr bool
	}{
		{
			name:  "basic mkdir",
			input: "mkdir /tmp/test",
			want: &Rule{
				Action: "mkdir",
				Mkdir:  "/tmp/test",
			},
			wantErr: false,
		},
		{
			name:  "mkdir with permissions",
			input: "mkdir /home/user/.ssh perms: 0700",
			want: &Rule{
				Action:     "mkdir",
				Mkdir:      "/home/user/.ssh",
				MkdirPerms: "0700",
			},
			wantErr: false,
		},
		{
			name:  "mkdir with OS filter",
			input: "mkdir /var/lib/custom on: [linux]",
			want: &Rule{
				Action: "mkdir",
				Mkdir:  "/var/lib/custom",
				OSList: []string{"linux"},
			},
			wantErr: false,
		},
		{
			name:  "mkdir with id and after",
			input: "mkdir /opt/app id: app-dir after: base-setup on: [linux]",
			want: &Rule{
				ID:     "app-dir",
				Action: "mkdir",
				Mkdir:  "/opt/app",
				After:  []string{"base-setup"},
				OSList: []string{"linux"},
			},
			wantErr: false,
		},
		{
			name:    "mkdir with no path",
			input:   "mkdir on: [linux]",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseMkdirRule(tt.input)

			if (got == nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("parseMkdirRule() = %v, want error", got)
				} else {
					t.Errorf("parseMkdirRule() got nil, want %v", tt.want)
				}
				return
			}

			if got == nil {
				return
			}

			if got.Action != tt.want.Action {
				t.Errorf("Action: got %q, want %q", got.Action, tt.want.Action)
			}
			if got.Mkdir != tt.want.Mkdir {
				t.Errorf("Mkdir: got %q, want %q", got.Mkdir, tt.want.Mkdir)
			}
		})
	}
}

// TestParseCloneRule tests clone rule parsing
func TestParseCloneRule(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "basic clone",
			input:   "clone https://github.com/user/repo.git to: ~/projects/repo",
			wantErr: false,
		},
		{
			name:    "clone with branch",
			input:   "clone https://github.com/user/repo.git to: ~/projects/repo branch: develop",
			wantErr: false,
		},
		{
			name:    "clone with OS filter",
			input:   "clone https://github.com/user/repo.git to: ~/src on: [linux, mac]",
			wantErr: false,
		},
		{
			name:    "clone with id and after",
			input:   "clone https://github.com/user/dotfiles.git to: ~/.dotfiles id: dotfiles after: base-setup on: [linux]",
			wantErr: false,
		},
		{
			name:    "clone with missing path",
			input:   "clone https://github.com/user/repo.git",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCloneRule(tt.input)

			if (got == nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("parseCloneRule() = %v, want error", got)
				} else {
					t.Errorf("parseCloneRule() got nil, want valid rule")
				}
				return
			}

			if got == nil {
				return
			}

			if got.Action != "clone" {
				t.Errorf("Action: got %q, want %q", got.Action, "clone")
			}
			if got.CloneURL == "" {
				t.Error("CloneURL is empty")
			}
			if got.ClonePath == "" {
				t.Error("ClonePath is empty")
			}
		})
	}
}

// TestParseDecryptRule tests decrypt rule parsing
func TestParseDecryptRule(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "basic decrypt",
			input:   "decrypt ./secrets.enc to: ~/.ssh/id_rsa",
			wantErr: false,
		},
		{
			name:    "decrypt with OS filter",
			input:   "decrypt ./config.enc to: ~/.config/app.conf on: [linux, mac]",
			wantErr: false,
		},
		{
			name:    "decrypt with id",
			input:   "decrypt ./secrets.enc to: ~/.ssh/id_rsa id: ssh-key-setup on: [linux]",
			wantErr: false,
		},
		{
			name:    "decrypt with after dependency",
			input:   "decrypt ./certs.enc to: /etc/ssl/certs/app.crt after: base-setup on: [linux]",
			wantErr: false,
		},
		{
			name:    "decrypt with missing destination",
			input:   "decrypt ./secrets.enc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDecryptRule(tt.input)

			if (got == nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("parseDecryptRule() = %v, want error", got)
				} else {
					t.Errorf("parseDecryptRule() got nil, want valid rule")
				}
				return
			}

			if got == nil {
				return
			}

			if got.Action != "decrypt" {
				t.Errorf("Action: got %q, want %q", got.Action, "decrypt")
			}
			if got.DecryptFile == "" {
				t.Error("DecryptFile is empty")
			}
			if got.DecryptPath == "" {
				t.Error("DecryptPath is empty")
			}
		})
	}
}

// TestParseAsdfRule tests asdf rule parsing
func TestParseAsdfRule(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "basic asdf install",
			input:   "asdf install nodejs 18.0.0",
			wantErr: false,
		},
		{
			name:    "asdf install multiple tools",
			input:   "asdf install nodejs 18.0.0 ruby 3.1.0 python 3.11.0",
			wantErr: false,
		},
		{
			name:    "asdf install with OS filter",
			input:   "asdf install nodejs latest on: [linux, mac]",
			wantErr: false,
		},
		{
			name:    "asdf install with id and after",
			input:   "asdf install nodejs 18.0.0 id: nodejs-setup after: base-setup on: [linux]",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAsdfRule(tt.input)

			if (got == nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("parseAsdfRule() = %v, want error", got)
				} else {
					t.Errorf("parseAsdfRule() got nil, want valid rule")
				}
				return
			}

			if got == nil {
				return
			}

			if got.Action != "asdf" {
				t.Errorf("Action: got %q, want %q", got.Action, "asdf")
			}
		})
	}
}

// TestParseKnownHostsRule tests known_hosts rule parsing
func TestParseKnownHostsRule(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "basic known_hosts add",
			input:   "known_hosts github.com",
			wantErr: false,
		},
		{
			name:    "known_hosts with key type",
			input:   "known_hosts example.com key: rsa",
			wantErr: false,
		},
		{
			name:    "known_hosts with OS filter",
			input:   "known_hosts gitlab.example.com on: [linux, mac]",
			wantErr: false,
		},
		{
			name:    "known_hosts with id and after",
			input:   "known_hosts github.com id: github-setup after: base-setup on: [linux]",
			wantErr: false,
		},
		{
			name:    "known_hosts with ed25519 key type",
			input:   "known_hosts internal.company.com key: ed25519 on: [linux]",
			wantErr: false,
		},
		{
			name:    "known_hosts with no host",
			input:   "known_hosts on: [linux]",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseKnownHostsRule(tt.input)

			if (got == nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("parseKnownHostsRule() = %v, want error", got)
				} else {
					t.Errorf("parseKnownHostsRule() got nil, want valid rule")
				}
				return
			}

			if got == nil {
				return
			}

			if got.Action != "known_hosts" {
				t.Errorf("Action: got %q, want %q", got.Action, "known_hosts")
			}
			if got.KnownHosts == "" {
				t.Error("KnownHosts is empty")
			}
		})
	}
}

// TestComprehensiveBlueprintParsing tests parsing a realistic multi-handler blueprint
func TestComprehensiveBlueprintParsing(t *testing.T) {
	blueprint := `#!/usr/bin/env blueprint

# Development environment setup blueprint

# Create directory structure
mkdir ~/.config id: config-dir on: [linux, mac]
mkdir ~/.ssh perms: 0700 id: ssh-dir after: config-dir on: [linux, mac]

# Add repository keys and install base packages
gpg-key https://apt.fury.io/wez/gpg.key keyring: wezterm-fury deb-url: https://apt.fury.io/wez/ id: wezterm-key on: [linux]

# Install system packages
install curl wget git vim build-essential id: base-tools on: [linux]
install git vim curl on: [linux, mac]

# Decrypt sensitive files
decrypt ./secrets.enc to: ~/.ssh/id_rsa id: ssh-key after: ssh-dir on: [linux, mac]
decrypt ./config.enc to: ~/.config/app.conf id: app-config after: config-dir on: [linux, mac]

# Clone repositories
clone https://github.com/user/dotfiles.git to: ~/.dotfiles branch: main id: dotfiles-clone after: ssh-key on: [linux, mac]
clone https://github.com/user/projects.git to: ~/projects id: projects-clone on: [linux, mac]

# Install runtime versions
asdf install nodejs 18.0.0 id: nodejs-setup on: [linux, mac]
asdf install ruby 3.1.0 python 3.11.0 id: dev-tools after: nodejs-setup on: [linux, mac]

# Add SSH hosts to known_hosts
known_hosts github.com key: ed25519 id: github-hosts on: [linux, mac]
known_hosts gitlab.example.com key: rsa id: gitlab-hosts on: [linux, mac]
`

	rules, err := Parse(blueprint)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Verify total rule count (2 mkdir, 1 gpg, 2 install, 2 decrypt, 2 clone, 2 asdf, 2 known_hosts = 13)
	expectedCount := 13
	if len(rules) != expectedCount {
		t.Errorf("Parse() got %d rules, want %d", len(rules), expectedCount)
	}

	// Count rules by action
	actionCounts := make(map[string]int)
	for _, rule := range rules {
		actionCounts[rule.Action]++
	}

	expectedActions := map[string]int{
		"mkdir":       2,
		"gpg-key":     1,
		"install":     2,
		"decrypt":     2,
		"clone":       2,
		"asdf":        2,
		"known_hosts": 2,
	}

	for action, expectedCount := range expectedActions {
		if actualCount, exists := actionCounts[action]; !exists || actualCount != expectedCount {
			t.Errorf("Action %q: got %d rules, want %d", action, actualCount, expectedCount)
		}
	}

	// Verify some specific rules have correct dependencies
	var firstInstall, decryptRule, asdfRule *Rule
	for i := range rules {
		if rules[i].Action == "install" && firstInstall == nil {
			firstInstall = &rules[i]
		}
		if rules[i].Action == "decrypt" && rules[i].ID == "ssh-key" {
			decryptRule = &rules[i]
		}
		if rules[i].Action == "asdf" && rules[i].ID == "dev-tools" {
			asdfRule = &rules[i]
		}
	}

	// Check ssh-key decrypt has correct dependency
	if decryptRule != nil && (len(decryptRule.After) != 1 || decryptRule.After[0] != "ssh-dir") {
		t.Errorf("ssh-key decrypt After: got %v, want [ssh-dir]", decryptRule.After)
	}

	// Check asdf rule with dev-tools has correct dependency
	if asdfRule != nil && (len(asdfRule.After) != 1 || asdfRule.After[0] != "nodejs-setup") {
		t.Errorf("dev-tools asdf After: got %v, want [nodejs-setup]", asdfRule.After)
	}

	// Verify OS filters are correctly set
	for _, rule := range rules {
		if rule.ID == "config-dir" && len(rule.OSList) != 2 {
			t.Errorf("config-dir OSList: got %d, want 2", len(rule.OSList))
		}
	}
}

// TestParseFromFile tests parsing blueprint rules from actual blueprint files
func TestParseFromFile(t *testing.T) {
	tests := []struct {
		name           string
		filepath       string
		expectedCount  int
		expectedAction string
		validate       func(t *testing.T, rule Rule) // Custom validation for handler-specific properties
	}{
		{
			name:           "parse install blueprint file",
			filepath:       "test/data/install/single.bp",
			expectedCount:  1,
			expectedAction: "install",
			validate: func(t *testing.T, rule Rule) {
				if len(rule.Packages) == 0 {
					t.Error("install rule has no packages")
				}
				if len(rule.Packages) > 0 && rule.Packages[0].Name == "" {
					t.Error("install package name is empty")
				}
			},
		},
		{
			name:           "parse mkdir blueprint file",
			filepath:       "test/data/mkdir/basic.bp",
			expectedCount:  1,
			expectedAction: "mkdir",
			validate: func(t *testing.T, rule Rule) {
				if rule.Mkdir == "" {
					t.Error("mkdir path is empty")
				}
			},
		},
		{
			name:           "parse clone blueprint file",
			filepath:       "test/data/clone/basic.bp",
			expectedCount:  1,
			expectedAction: "clone",
			validate: func(t *testing.T, rule Rule) {
				if rule.CloneURL == "" {
					t.Error("clone URL is empty")
				}
				if rule.ClonePath == "" {
					t.Error("clone path is empty")
				}
			},
		},
		{
			name:           "parse decrypt blueprint file",
			filepath:       "test/data/decrypt/basic.bp",
			expectedCount:  1,
			expectedAction: "decrypt",
			validate: func(t *testing.T, rule Rule) {
				if rule.DecryptFile == "" {
					t.Error("decrypt source file is empty")
				}
				if rule.DecryptPath == "" {
					t.Error("decrypt destination path is empty")
				}
			},
		},
		{
			name:           "parse asdf blueprint file",
			filepath:       "test/data/asdf/single.bp",
			expectedCount:  1,
			expectedAction: "asdf",
			validate: func(t *testing.T, rule Rule) {
				if len(rule.AsdfPackages) == 0 {
					t.Error("asdf rule has no packages")
				}
				if len(rule.AsdfPackages) > 0 && rule.AsdfPackages[0] == "" {
					t.Error("asdf package is empty")
				}
			},
		},
		{
			name:           "parse known_hosts blueprint file",
			filepath:       "test/data/known_hosts/basic.bp",
			expectedCount:  1,
			expectedAction: "known_hosts",
			validate: func(t *testing.T, rule Rule) {
				if rule.KnownHosts == "" {
					t.Error("known_hosts hostname is empty")
				}
			},
		},
		{
			name:           "parse gpg-key blueprint file",
			filepath:       "test/data/gpg_key/basic.bp",
			expectedCount:  1,
			expectedAction: "gpg-key",
			validate: func(t *testing.T, rule Rule) {
				if rule.GPGKeyURL == "" {
					t.Error("gpg-key URL is empty")
				}
				if rule.GPGKeyring == "" {
					t.Error("gpg-key keyring is empty")
				}
				if rule.GPGDebURL == "" {
					t.Error("gpg-key deb-url is empty")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get the path to the test blueprint file
			content, err := os.ReadFile(tt.filepath)
			if err != nil {
				t.Fatalf("Failed to read %s: %v", tt.filepath, err)
			}

			// Parse the blueprint file
			rules, err := Parse(string(content))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			// Verify rule count
			if len(rules) != tt.expectedCount {
				t.Errorf("Parse() got %d rules, want %d", len(rules), tt.expectedCount)
			}

			// Verify rules have the expected action and handler-specific properties
			for _, rule := range rules {
				if rule.Action != tt.expectedAction {
					t.Errorf("Rule action: got %q, want %q", rule.Action, tt.expectedAction)
				}
				if tt.validate != nil {
					tt.validate(t, rule)
				}
			}
		})
	}
}

// TestParseComprehensiveFromFile tests parsing a comprehensive blueprint with all handler types
func TestParseComprehensiveFromFile(t *testing.T) {
	// Read the comprehensive blueprint file
	content, err := os.ReadFile("test/data/comprehensive/full-setup.bp")
	if err != nil {
		t.Fatalf("Failed to read comprehensive blueprint: %v", err)
	}

	// Parse the blueprint
	rules, err := Parse(string(content))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Verify total rule count
	expectedCount := 13
	if len(rules) != expectedCount {
		t.Errorf("Parse() got %d rules, want %d", len(rules), expectedCount)
	}

	// Count rules by action
	actionCounts := make(map[string]int)
	for _, rule := range rules {
		actionCounts[rule.Action]++
	}

	// Verify action counts
	expectedActions := map[string]int{
		"mkdir":       2,
		"gpg-key":     1,
		"install":     2,
		"decrypt":     2,
		"clone":       2,
		"asdf":        2,
		"known_hosts": 2,
	}

	for action, expectedCount := range expectedActions {
		if actualCount, exists := actionCounts[action]; !exists || actualCount != expectedCount {
			t.Errorf("Action %q: got %d rules, want %d", action, actualCount, expectedCount)
		}
	}

	// Verify specific rules have correct properties
	for _, rule := range rules {
		switch rule.ID {
		case "config-dir":
			if rule.Action != "mkdir" {
				t.Errorf("config-dir action: got %q, want mkdir", rule.Action)
			}
			if len(rule.OSList) != 2 {
				t.Errorf("config-dir OSList: got %d, want 2", len(rule.OSList))
			}
		case "ssh-dir":
			if rule.Action != "mkdir" {
				t.Errorf("ssh-dir action: got %q, want mkdir", rule.Action)
			}
			if len(rule.After) != 1 || rule.After[0] != "config-dir" {
				t.Errorf("ssh-dir dependencies: got %v, want [config-dir]", rule.After)
			}
		case "ssh-key":
			if rule.Action != "decrypt" {
				t.Errorf("ssh-key action: got %q, want decrypt", rule.Action)
			}
			if len(rule.After) != 1 || rule.After[0] != "ssh-dir" {
				t.Errorf("ssh-key dependencies: got %v, want [ssh-dir]", rule.After)
			}
		case "dev-tools":
			if rule.Action != "asdf" {
				t.Errorf("dev-tools action: got %q, want asdf", rule.Action)
			}
			if len(rule.After) != 1 || rule.After[0] != "nodejs-setup" {
				t.Errorf("dev-tools dependencies: got %v, want [nodejs-setup]", rule.After)
			}
		}
	}
}
