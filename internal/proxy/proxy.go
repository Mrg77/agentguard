// Package proxy is agentguard's runtime enforcement point: an MCP server an AI
// agent connects to instead of acting directly. Every action the agent
// proposes passes through the policy engine before anything happens — the same
// deterministic allow/ask/deny it can test offline, now applied live and
// recorded to the audit trail.
//
// This is the difference between `policy test` (a simulator) and the proxy (a
// firewall): the proxy is where the agent actually asks "may I?", and the guard
// answers before the action reaches a real tool.
//
// SCOPE: the proxy enforces on the ACTION the agent declares. It is a
// containment layer, not a sandbox escape detector — an agent that lies about
// its own tool/target is still bounded by what the downstream tool will let a
// declared action do, which is why the honest deployment pattern is to put the
// proxy in front of the tools, not to trust the agent's self-report alone. See
// README "Honest scope".
package proxy

import (
	"strings"
	"time"

	"github.com/Mrg77/agentguard/internal/audit"
	"github.com/Mrg77/agentguard/internal/policy"
)

// Result is what the proxy returns for a proposed action: the decision plus the
// human-readable reason, shaped for an agent to act on (proceed / stop / ask).
type Result struct {
	Decision string `json:"decision"` // allow | ask | deny
	Rule     string `json:"rule,omitempty"`
	Message  string `json:"message"`
	// Guidance tells the agent, in plain words, what to do with the decision —
	// so a well-behaved agent self-limits instead of retrying blindly.
	Guidance string `json:"guidance"`
}

// Guard wraps a compiled policy and records every decision. It is the reusable
// core the MCP handler (and tests) call; keeping it separate from the transport
// makes it unit-testable without a live MCP client.
type Guard struct {
	Policy *policy.Policy
	// now is injected for deterministic tests; defaults to time.Now.
	now func() time.Time
}

// NewGuard builds a Guard over a policy.
func NewGuard(p *policy.Policy) *Guard {
	return &Guard{Policy: p, now: time.Now}
}

// Evaluate runs one proposed action through the policy, records it, and returns
// the result. This is the single choke point every agent action flows through.
func (g *Guard) Evaluate(tool, target, context string) Result {
	act := policy.Action{Tool: tool, Target: target, Context: context}
	v := g.Policy.Evaluate(act)

	audit.Append(audit.Entry{
		Time:     g.clock().UTC(),
		Tool:     tool,
		Target:   target,
		Context:  context,
		Decision: string(v.Decision),
		Rule:     v.Rule,
		Tokens:   roughTokens(tool, target),
	})

	return Result{
		Decision: string(v.Decision),
		Rule:     v.Rule,
		Message:  v.Message,
		Guidance: guidance(v.Decision),
	}
}

func (g *Guard) clock() time.Time {
	if g.now != nil {
		return g.now()
	}
	return time.Now()
}

// guidance maps a decision to a plain instruction for the agent.
func guidance(d policy.Decision) string {
	switch d {
	case policy.Deny:
		return "Do NOT perform this action. It is blocked by policy — pick a safer approach or stop."
	case policy.Ask:
		return "Do NOT perform this action yet. It requires human approval first; surface it and wait."
	default:
		return "This action is permitted by policy. You may proceed."
	}
}

// roughTokens is a cheap, dependency-free estimate of an action's size (~4
// chars per token), so the audit trail can show relative cost without pulling a
// tokenizer. It is explicitly an estimate, not billing.
func roughTokens(parts ...string) int {
	n := 0
	for _, p := range parts {
		n += len(strings.TrimSpace(p))
	}
	if n == 0 {
		return 0
	}
	return n/4 + 1
}
