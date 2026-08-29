package mocks

import (
	"errors"
	"os"
	"testing"

	"github.com/elpic/blueprint/internal/platform"
)

// Compile-time interface compliance: the mock satisfies GitProvider.
var _ platform.GitProvider = (*MockGitProvider)(nil)

func TestMockGitProvider_CloneKeying(t *testing.T) {
	spec := platform.CloneSpec{URL: "https://github.com/u/r", Path: "~/src/r", Branch: "main", Mode: platform.ModeTwoStage}
	m := NewMockGitProvider().
		WithCloneResult(spec, platform.CloneResult{Status: platform.StatusSynced, NewSHA: "abc"})

	// Same mode+URL+branch, different path: same configured outcome.
	otherPath := spec
	otherPath.Path = "~/elsewhere/r"
	if res, err := m.Clone(otherPath); err != nil || res.Status != platform.StatusSynced {
		t.Errorf("Clone(same key, other path) = (%+v, %v), want StatusSynced", res, err)
	}

	// Different branch: falls through to the default clone result.
	otherBranch := spec
	otherBranch.Branch = "dev"
	if res, err := m.Clone(otherBranch); err != nil || res.Status != platform.StatusCloned {
		t.Errorf("Clone(other branch) = (%+v, %v), want default StatusCloned", res, err)
	}

	// Different mode: also falls through — mode is part of the key.
	otherMode := spec
	otherMode.Mode = platform.ModeDirect
	if res, err := m.Clone(otherMode); err != nil || res.Status != platform.StatusCloned {
		t.Errorf("Clone(other mode) = (%+v, %v), want default StatusCloned", res, err)
	}

	// Configured error wins over a configured result for the same key.
	m.WithCloneError(spec, errors.New("boom"))
	if _, err := m.Clone(spec); err == nil || err.Error() != "boom" {
		t.Errorf("Clone(error key) = %v, want boom", err)
	}
}

func TestMockGitProvider_PerMethodConfiguration(t *testing.T) {
	m := NewMockGitProvider()
	id := platform.RepoID{URL: "https://github.com/u/r", Branch: "main"}

	m.WithLocalSHA("/repo", "local111").
		WithRemoteSHA(id, "remote222").
		WithBareSHA("/bare", "main", "bare333").
		WithStorageSHA(id, "storage444").
		WithLayout("/repo", platform.LayoutStandard).
		WithCheckoutError(platform.CheckoutSpec{Path: "/repo", SHA: "local111"}, errors.New("dirty"))

	if got, err := m.LocalSHA("/repo"); err != nil || got != "local111" {
		t.Errorf("LocalSHA = (%q, %v), want (local111, nil)", got, err)
	}
	if got, err := m.RemoteSHA(id); err != nil || got != "remote222" {
		t.Errorf("RemoteSHA = (%q, %v), want (remote222, nil)", got, err)
	}
	if got, err := m.BareSHA("/bare", "main"); err != nil || got != "bare333" {
		t.Errorf("BareSHA = (%q, %v), want (bare333, nil)", got, err)
	}
	if got, err := m.StorageSHA(id); err != nil || got != "storage444" {
		t.Errorf("StorageSHA = (%q, %v), want (storage444, nil)", got, err)
	}
	if got := m.Layout("/repo"); got != platform.LayoutStandard {
		t.Errorf("Layout = %v, want LayoutStandard", got)
	}
	if err := m.Checkout(platform.CheckoutSpec{Path: "/repo", SHA: "local111"}); err == nil {
		t.Errorf("Checkout = nil error, want dirty")
	}
}

func TestMockGitProvider_ConvenienceConstructors(t *testing.T) {
	direct := platform.CloneSpec{URL: "https://github.com/u/r", Path: "~/src/r", Branch: "main", Mode: platform.ModeDirect}
	bare := direct
	bare.Mode = platform.ModeBare
	twoStage := direct
	twoStage.Mode = platform.ModeTwoStage

	// Direct: working copy on disk — standard layout + readable local SHA.
	m := NewMockGitProvider().WithSuccessfulClone(direct, "sha1")
	if res, err := m.Clone(direct); err != nil || res.Status != platform.StatusCloned || res.NewSHA != "sha1" {
		t.Errorf("Clone(direct) = (%+v, %v), want {Cloned sha1}", res, err)
	}
	if got := m.Layout("~/src/r"); got != platform.LayoutStandard {
		t.Errorf("Layout = %v, want LayoutStandard", got)
	}
	if got, err := m.LocalSHA("~/src/r"); err != nil || got != "sha1" {
		t.Errorf("LocalSHA = (%q, %v), want (sha1, nil)", got, err)
	}

	// Bare: bare layout + readable bare SHA.
	m = NewMockGitProvider().WithUpToDateRepository(bare, "sha7")
	if res, err := m.Clone(bare); err != nil || res.Status != platform.StatusUpToDate || res.NewSHA != "sha7" {
		t.Errorf("Clone(bare) = (%+v, %v), want {UpToDate sha7}", res, err)
	}
	if got := m.Layout("~/src/r"); got != platform.LayoutBare {
		t.Errorf("Layout = %v, want LayoutBare", got)
	}
	if got, err := m.BareSHA("~/src/r", "main"); err != nil || got != "sha7" {
		t.Errorf("BareSHA = (%q, %v), want (sha7, nil)", got, err)
	}

	// Two-stage: content mirror — no .git, SHA lives in clean storage.
	m = NewMockGitProvider().WithSuccessfulUpdate(twoStage, "old1", "new2")
	if res, err := m.Clone(twoStage); err != nil || res.Status != platform.StatusUpdated || res.OldSHA != "old1" || res.NewSHA != "new2" {
		t.Errorf("Clone(two-stage) = (%+v, %v), want {Updated old1 new2}", res, err)
	}
	if got := m.Layout("~/src/r"); got != platform.LayoutNone {
		t.Errorf("Layout = %v, want LayoutNone for a two-stage mirror", got)
	}
	if got, err := m.StorageSHA(platform.RepoID{URL: twoStage.URL, Branch: twoStage.Branch}); err != nil || got != "new2" {
		t.Errorf("StorageSHA = (%q, %v), want (new2, nil)", got, err)
	}
}

func TestMockGitProvider_UnreachableRemote(t *testing.T) {
	id := platform.RepoID{URL: "https://github.com/u/r", Branch: "main"}
	m := NewMockGitProvider().WithUnreachableRemote(id)
	if _, err := m.RemoteSHA(id); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("RemoteSHA = %v, want os.ErrNotExist", err)
	}
}
