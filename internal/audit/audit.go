// Package audit is agentguard's append-only record of every guarded action an
// agent proposed and what the guard decided. It answers the question a raw
// model transcript can't: what did my agent try to do against prod this week,
// and did the guard let it through?
//
// It is best-effort by design — a logging failure must never change a guard
// decision or wedge the proxy — and it never records a secret value, only the
// action's shape (tool, target, context) and the verdict.
package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Entry is one recorded decision.
type Entry struct {
	Time     time.Time `json:"time"`
	Tool     string    `json:"tool"`
	Target   string    `json:"target"`
	Context  string    `json:"context"`
	Decision string    `json:"decision"` // allow | ask | deny
	Rule     string    `json:"rule,omitempty"`
	Tokens   int       `json:"tokens,omitempty"` // rough token count of the action, when known
}

// Path returns the audit log location under XDG_STATE_HOME, alongside the
// cluster's kubeconfig, so all of agentguard's local state lives in one place.
func Path() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "agentguard", "audit.jsonl")
}

// Append records one entry. Best-effort: any error is swallowed so the proxy
// keeps enforcing even if the disk is full or read-only.
func Append(e Entry) {
	p := Path()
	if p == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	if b, err := json.Marshal(e); err == nil {
		f.Write(append(b, '\n'))
	}
}

// Filter narrows what Read returns.
type Filter struct {
	Since        time.Time // zero = no lower bound
	DecisionOnly string    // "" = any
	ProdOnly     bool
}

// Read returns entries matching the filter, oldest first. A missing log is not
// an error; corrupt lines are skipped rather than failing the whole read.
func Read(f Filter) ([]Entry, error) {
	p := Path()
	if p == "" {
		return nil, nil
	}
	file, err := os.Open(p)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var out []Entry
	sc := bufio.NewScanner(file)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e Entry
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		if f.matches(e) {
			out = append(out, e)
		}
	}
	return out, sc.Err()
}

func (f Filter) matches(e Entry) bool {
	if !f.Since.IsZero() && e.Time.Before(f.Since) {
		return false
	}
	if f.DecisionOnly != "" && e.Decision != f.DecisionOnly {
		return false
	}
	if f.ProdOnly && !strings.Contains(strings.ToLower(e.Context), "prod") {
		return false
	}
	return true
}
