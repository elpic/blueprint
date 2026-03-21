package git

import "testing"

// ---- #9: IsGitURL false positive on substring ".git" ----------------------
//
// strings.Contains(input, ".git") matches local paths that happen to contain
// the string ".git", causing blueprint to treat them as remote git URLs.

func TestIsGitURLFalsePositives(t *testing.T) {
	falsePositives := []string{
		"~/projects/dotfiles.git/setup.bp",  // local path with .git directory
		"/home/user/mygit-configs/setup.bp", // contains "git" but not .git
		"./setup.bp",                        // plain relative path
	}
	for _, input := range falsePositives {
		if IsGitURL(input) {
			t.Errorf("BUG: IsGitURL(%q) = true, want false — .git substring false positive", input)
		}
	}
}

func TestIsGitURLTruePositives(t *testing.T) {
	truePositives := []string{
		"git@github.com:user/repo.git",
		"https://github.com/user/repo.git",
		"http://github.com/user/repo.git",
		"git://github.com/user/repo.git",
		"https://github.com/user/repo",
	}
	for _, input := range truePositives {
		if !IsGitURL(input) {
			t.Errorf("IsGitURL(%q) = false, want true", input)
		}
	}
}
