package audit

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendAndReadRoundTrip(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	now := time.Now().UTC().Truncate(time.Second)
	Append(Entry{Time: now, Tool: "shell", Target: "rm -rf /", Context: "prod", Decision: "deny", Rule: "r1"})
	Append(Entry{Time: now, Tool: "kubectl", Target: "kubectl get pods", Context: "dev", Decision: "allow"})

	got, err := Read(Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 entries, got %d", len(got))
	}
	if got[0].Decision != "deny" || got[0].Target != "rm -rf /" {
		t.Errorf("first entry wrong: %+v", got[0])
	}
	// File is private (it can hold command lines).
	if fi, err := os.Stat(Path()); err == nil && fi.Mode().Perm() != 0o600 {
		t.Errorf("audit log perms = %v, want 0600", fi.Mode().Perm())
	}
}

func TestReadMissingLogIsNotAnError(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	got, err := Read(Filter{})
	if err != nil || got != nil {
		t.Errorf("missing log should be (nil, nil), got (%v, %v)", got, err)
	}
}

func TestReadSkipsCorruptLine(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	p := Path()
	os.MkdirAll(filepath.Dir(p), 0o700)
	os.WriteFile(p, []byte(`{"decision":"deny","target":"ok"}`+"\n{not json\n"), 0o600)
	got, err := Read(Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Target != "ok" {
		t.Errorf("a corrupt line should be skipped: %+v", got)
	}
}

func TestSinceFilter(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	old := time.Now().Add(-48 * time.Hour).UTC()
	recent := time.Now().Add(-1 * time.Hour).UTC()
	Append(Entry{Time: old, Target: "a", Decision: "allow"})
	Append(Entry{Time: recent, Target: "b", Decision: "allow"})

	got, _ := Read(Filter{Since: time.Now().Add(-24 * time.Hour)})
	if len(got) != 1 || got[0].Target != "b" {
		t.Errorf("since filter should return only the recent entry, got %+v", got)
	}
}
