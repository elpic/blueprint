package parser

import (
	"reflect"
	"testing"
)

func TestParseFields(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantKV     map[string]string
		wantTokens []string
		wantOS     []string
	}{
		{
			name:       "empty body",
			body:       "",
			wantKV:     map[string]string{},
			wantTokens: nil,
			wantOS:     nil,
		},
		{
			name:       "positional tokens only",
			body:       "git vim curl",
			wantKV:     map[string]string{},
			wantTokens: []string{"git", "vim", "curl"},
			wantOS:     nil,
		},
		{
			name:       "single keyword",
			body:       "id: my-rule",
			wantKV:     map[string]string{"id:": "my-rule"},
			wantTokens: nil,
			wantOS:     nil,
		},
		{
			name:       "on: filter extracted",
			body:       "curl on: [linux, mac]",
			wantKV:     map[string]string{},
			wantTokens: []string{"curl"},
			wantOS:     []string{"linux", "mac"},
		},
		{
			name:       "url containing on: is not split as os filter",
			body:       "https://example.com/path?x=on:yes to: /tmp/file",
			wantKV:     map[string]string{"to:": "/tmp/file"},
			wantTokens: []string{"https://example.com/path?x=on:yes"},
			wantOS:     nil,
		},
		{
			name:       "position-independent keywords",
			body:       "id: foo to: /bar branch: main",
			wantKV:     map[string]string{"id:": "foo", "to:": "/bar", "branch:": "main"},
			wantTokens: nil,
			wantOS:     nil,
		},
		{
			name:       "after: with comma-space list",
			body:       "after: a, b, c",
			wantKV:     map[string]string{"after:": "a, b, c"},
			wantTokens: nil,
			wantOS:     nil,
		},
		{
			name:       "after: single item",
			body:       "after: setup",
			wantKV:     map[string]string{"after:": "setup"},
			wantTokens: nil,
			wantOS:     nil,
		},
		{
			name:       "quoted cron expression",
			body:       `cron: "0 9 * * *" source: ~/setup.bp`,
			wantKV:     map[string]string{"cron:": "0 9 * * *", "source:": "~/setup.bp"},
			wantTokens: nil,
			wantOS:     nil,
		},
		{
			name:       "skip: bracket list",
			body:       "skip: [.git, node_modules, .DS_Store]",
			wantKV:     map[string]string{"skip:": ".git,node_modules,.DS_Store"},
			wantTokens: nil,
			wantOS:     nil,
		},
		{
			name:       "unless: multiword shell command",
			body:       "unless: which curl id: curl-install",
			wantKV:     map[string]string{"unless:": "which curl", "id:": "curl-install"},
			wantTokens: nil,
			wantOS:     nil,
		},
		{
			name:       "undo: multiword shell command",
			body:       "undo: rm -rf /tmp/thing id: cleanup",
			wantKV:     map[string]string{"undo:": "rm -rf /tmp/thing", "id:": "cleanup"},
			wantTokens: nil,
			wantOS:     nil,
		},
		{
			name:       "mixed positional and keywords",
			body:       "nodejs@18 python@3.11 id: tools after: base on: [linux]",
			wantKV:     map[string]string{"id:": "tools", "after:": "base"},
			wantTokens: []string{"nodejs@18", "python@3.11"},
			wantOS:     []string{"linux"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := parseFields(tt.body)

			if !reflect.DeepEqual(f.kv, tt.wantKV) {
				t.Errorf("kv: got %v, want %v", f.kv, tt.wantKV)
			}
			if !reflect.DeepEqual(f.tokens, tt.wantTokens) {
				t.Errorf("tokens: got %v, want %v", f.tokens, tt.wantTokens)
			}
			if !reflect.DeepEqual(f.osFilter, tt.wantOS) {
				t.Errorf("osFilter: got %v, want %v", f.osFilter, tt.wantOS)
			}
		})
	}
}

func TestLineFieldsMethods(t *testing.T) {
	t.Run("word returns first token of value", func(t *testing.T) {
		f := parseFields("id: my-id branch: main")
		if got := f.word("id:"); got != "my-id" {
			t.Errorf("word(id:) = %q, want %q", got, "my-id")
		}
		if got := f.word("missing:"); got != "" {
			t.Errorf("word(missing:) = %q, want %q", got, "")
		}
	})

	t.Run("list splits comma-separated values", func(t *testing.T) {
		f := parseFields("after: setup, base, tools")
		got := f.list("after:")
		want := []string{"setup", "base", "tools"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("list(after:) = %v, want %v", got, want)
		}
	})

	t.Run("list returns nil for absent key", func(t *testing.T) {
		f := parseFields("id: foo")
		if got := f.list("after:"); got != nil {
			t.Errorf("list(after:) = %v, want nil", got)
		}
	})

	t.Run("skipList parses bracket list", func(t *testing.T) {
		f := parseFields("skip: [.git, node_modules]")
		got := f.skipList()
		want := []string{".git", "node_modules"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("skipList() = %v, want %v", got, want)
		}
	})

	t.Run("rest returns positional tokens", func(t *testing.T) {
		f := parseFields("curl wget id: tools")
		if got := f.rest(); got != "curl wget" {
			t.Errorf("rest() = %q, want %q", got, "curl wget")
		}
	})

	t.Run("multiword returns full value", func(t *testing.T) {
		f := parseFields("unless: which curl id: install-curl")
		if got := f.multiword("unless:"); got != "which curl" {
			t.Errorf("multiword(unless:) = %q, want %q", got, "which curl")
		}
	})
}

func TestStripComment(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// full-line comments — stripped to empty string
		{"# full line comment", ""},
		{"// full line comment", ""},
		// inline # comment
		{"install curl # install curl tool", "install curl "},
		{"install curl#nospace", "install curl#nospace"}, // no preceding space → not a comment
		// inline // comment
		{"run echo hello // this is noise", "run echo hello "},
		// URLs must not be stripped
		{"clone https://github.com/user/repo.git to: ~/repo", "clone https://github.com/user/repo.git to: ~/repo"},
		{"run-sh https://example.com/setup.sh", "run-sh https://example.com/setup.sh"},
		// no comment
		{"install git vim curl", "install git vim curl"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stripComment(tt.input)
			if got != tt.want {
				t.Errorf("stripComment(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseContentComments(t *testing.T) {
	content := `# full line hash comment
// full line slash comment
install curl # inline hash comment
install git // inline slash comment
install vim
`
	rules, err := parseContent(content, "", make(map[string]bool))
	if err != nil {
		t.Fatalf("parseContent() error: %v", err)
	}
	if len(rules) != 3 {
		t.Fatalf("got %d rules, want 3", len(rules))
	}
	for _, r := range rules {
		if r.Action != "install" {
			t.Errorf("unexpected action %q", r.Action)
		}
		if len(r.Packages) != 1 {
			t.Errorf("rule %q: got %d packages, want 1", r.Packages, len(r.Packages))
		}
	}
}

func TestParseFieldsErrorMessages(t *testing.T) {
	// Verify line numbers appear in errors from parseContent
	content := "install curl\nfoobar something\nclone https://x"
	_, err := parseContent(content, "", make(map[string]bool))
	if err == nil {
		t.Fatal("parseContent() should return error for unknown directive")
	}
	errStr := err.Error()
	if !contains(errStr, "line 2") {
		t.Errorf("error should mention line 2, got: %q", errStr)
	}
	if !contains(errStr, "foobar something") {
		t.Errorf("error should include offending line, got: %q", errStr)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
