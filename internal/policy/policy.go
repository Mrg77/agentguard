// Package policy is the deterministic decision engine at the heart of
// agentguard: given an action an AI agent wants to take — a tool call with a
// target, in some context — it decides allow / ask / deny by matching the
// action against a versioned, testable set of rules.
//
// The design borrows the lesson the industry learned the hard way in 2025-26:
// telling a model "NEVER delete production" in its prompt does not hold (the
// Replit incident deleted a prod database despite eleven capitalized refusals;
// OWASP LLM06 says the same — enforce authorization downstream, not inside the
// LLM). So agentguard does not ask the model to behave. It intercepts the
// action and applies a rule the same way a firewall applies an ACL: the same
// input always yields the same decision, independent of what the model "meant".
//
// This is a containment tool, not a guarantee. It shrinks the blast radius of a
// misbehaving or prompt-injected agent; it does not claim to detect intent or
// stop prompt injection (there is no universal defense for that). Honest
// scope is a feature, not a disclaimer.
package policy

import (
	"regexp"
	"strings"
)

// Action is one thing an agent wants to do: call a tool, against a target,
// while a given context is active. All three are plain strings the proxy
// extracts from a tool call; the engine never interprets them semantically.
type Action struct {
	// Tool is the tool/function the agent is invoking, e.g. "shell",
	// "kubectl", "http.post", or an MCP tool name.
	Tool string
	// Target is what the tool acts on, e.g. "kubectl delete namespace payments"
	// or "https://api.internal/…". Free-form; matched as a string.
	Target string
	// Context is the environment the action runs in, e.g. "prod", "gke_prod-eu",
	// "staging". Mirrors opsforge's kube/cloud/tf context idea.
	Context string
}

// Decision is what the engine decides for an action.
type Decision string

const (
	// Allow: the action proceeds without interruption.
	Allow Decision = "allow"
	// Ask: the action is held until a human approves it (the confirm gate).
	Ask Decision = "ask"
	// Deny: the action is blocked outright.
	Deny Decision = "deny"
)

// Verdict is the full result of evaluating an action: the decision, which rule
// produced it, and the message to surface. A zero Rule means the default
// applied (no rule matched).
type Verdict struct {
	Decision Decision
	Rule     string // name of the matching rule, "" for the default
	Message  string
}

// Rule matches actions and assigns a decision. Tool, Target and Context are
// RE2 regexes; an empty pattern matches anything. A plain string like
// "kubectl delete" is itself a valid regex that behaves like a substring
// match, so simple rules stay readable. All present fields must match (AND).
type Rule struct {
	Name    string   `yaml:"name"`
	Tool    string   `yaml:"tool,omitempty"`
	Target  string   `yaml:"target,omitempty"`
	Context string   `yaml:"context,omitempty"`
	Action  Decision `yaml:"action"`
	Message string   `yaml:"message,omitempty"`

	// compiled patterns, filled by Compile; nil means "match anything".
	toolRe, targetRe, contextRe *regexp.Regexp
}

// Policy is an ordered set of rules plus a default decision. Rules are tried in
// order; the FIRST match wins (so specific rules go above broad ones). If none
// match, Default applies.
type Policy struct {
	// Default is the decision when no rule matches. It defaults to Allow when
	// unset, but a security-minded policy sets it to "ask" or "deny" — the
	// fail-safe posture agentguard recommends for anything touching prod.
	Default Decision `yaml:"default,omitempty"`
	Rules   []Rule   `yaml:"rules"`
}

// Evaluate returns the verdict for an action: the first matching rule's
// decision, else the policy default. It is pure and deterministic — no I/O, no
// clock, no model — so it is trivially testable and the same input always maps
// to the same output.
func (p *Policy) Evaluate(a Action) Verdict {
	for i := range p.Rules {
		if p.Rules[i].matches(a) {
			r := p.Rules[i]
			return Verdict{Decision: r.Action, Rule: r.Name, Message: r.Message}
		}
	}
	return Verdict{Decision: p.defaultDecision(), Message: "no rule matched — default policy applied"}
}

func (p *Policy) defaultDecision() Decision {
	if p.Default == "" {
		return Allow
	}
	return p.Default
}

// matches reports whether an action satisfies every present field of the rule.
// A nil compiled pattern (empty field) matches anything.
func (r *Rule) matches(a Action) bool {
	return matchField(r.toolRe, a.Tool) &&
		matchField(r.targetRe, a.Target) &&
		matchField(r.contextRe, a.Context)
}

func matchField(re *regexp.Regexp, v string) bool {
	return re == nil || re.MatchString(v)
}

// looksProd reports whether a context string looks production-like. It mirrors
// opsforge's fail-safe view: anything containing "prod" is treated as prod, so
// a rule author who forgets an exact cluster name still gets covered. Exposed
// for the proxy/audit layers.
func LooksProd(context string) bool {
	return strings.Contains(strings.ToLower(context), "prod")
}
