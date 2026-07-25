package policy

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Load reads and compiles a policy file. A compiled policy is ready to
// Evaluate; an invalid one (bad YAML, bad regex, unknown action) is a clear
// error rather than a silent misfire — a guard you can't trust is worse than
// no guard.
func Load(path string) (*Policy, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(raw)
}

// Parse compiles a policy from YAML bytes. Separated from Load so tests and the
// `policy test` command can work in-memory.
func Parse(raw []byte) (*Policy, error) {
	var p Policy
	if err := yaml.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("parsing policy: %w", err)
	}
	if err := p.Compile(); err != nil {
		return nil, err
	}
	return &p, nil
}

// Compile validates the policy and compiles every rule's regexes. It is called
// by Parse but is exported so a hand-built Policy can be validated too.
func (p *Policy) Compile() error {
	if p.Default != "" && !validDecision(p.Default) {
		return fmt.Errorf("invalid default %q (want allow, ask or deny)", p.Default)
	}
	for i := range p.Rules {
		r := &p.Rules[i]
		if r.Name == "" {
			return fmt.Errorf("rule %d has no name", i+1)
		}
		if !validDecision(r.Action) {
			return fmt.Errorf("rule %q: invalid action %q (want allow, ask or deny)", r.Name, r.Action)
		}
		var err error
		if r.toolRe, err = compileField("tool", r.Name, r.Tool); err != nil {
			return err
		}
		if r.targetRe, err = compileField("target", r.Name, r.Target); err != nil {
			return err
		}
		if r.contextRe, err = compileField("context", r.Name, r.Context); err != nil {
			return err
		}
	}
	return nil
}

// compileField compiles one pattern, or returns nil for an empty field (match
// anything). Patterns are matched case-insensitively: an agent asking for
// "KUBECTL Delete" must not slip past a rule written in lowercase.
func compileField(field, rule, pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, nil
	}
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return nil, fmt.Errorf("rule %q: bad %s pattern %q: %w", rule, field, pattern, err)
	}
	return re, nil
}

func validDecision(d Decision) bool {
	switch d {
	case Allow, Ask, Deny:
		return true
	default:
		return false
	}
}
