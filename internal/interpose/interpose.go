// Package interpose is agentguard's transparent MCP proxy — the mode that makes
// the word "firewall" earned. Instead of exposing an advisory `guard` tool the
// agent may choose to call, it sits ON THE PATH between the MCP client (Claude
// Code, Cursor, VS Code, …) and a real upstream tool server:
//
//	client  ──►  agentguard (interpose)  ──►  upstream MCP server (kubectl, fs, …)
//	                    │ policy: allow → relay
//	                    │         deny  → blocked, upstream never reached
//
// The agent no longer has the real server in its config — only agentguard — so
// every tool call it makes MUST pass through the policy. It cannot bypass the
// guard by calling the tool directly, because it has no direct handle to the
// tool anymore. That is the difference between advice and enforcement.
//
// Honest scope still holds: agentguard bounds WHAT tools may do and WHERE. It
// does not inspect a tool's side effects beyond the call it relays, and it
// can't stop prompt injection from making the agent *want* a bad action — it
// stops that action from reaching the tool. Containment, not a guarantee.
package interpose

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Mrg77/agentguard/internal/audit"
	"github.com/Mrg77/agentguard/internal/policy"
)

// Upstream describes the real MCP server agentguard fronts: the command to
// launch it (stdio) and its args.
type Upstream struct {
	Command string
	Args    []string
}

// Proxy fronts one upstream server with a policy. It connects to the upstream,
// mirrors its tools onto a downstream server, and gates every call.
type Proxy struct {
	pol      *policy.Policy
	context  string // the active context tag applied to every action (e.g. "prod")
	upstream *sdk.ClientSession
	now      func() time.Time
}

// New connects to the upstream server and returns a Proxy ready to Serve. The
// caller provides the compiled policy and the context tag to evaluate against.
func New(ctx context.Context, pol *policy.Policy, contextTag string, up Upstream) (*Proxy, error) {
	client := sdk.NewClient(&sdk.Implementation{Name: "agentguard-interpose", Version: "dev"}, nil)
	transport := &sdk.CommandTransport{Command: exec.CommandContext(ctx, up.Command, up.Args...)}
	sess, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connecting to upstream %q: %w", up.Command, err)
	}
	return &Proxy{pol: pol, context: contextTag, upstream: sess, now: time.Now}, nil
}

// Serve mirrors every upstream tool onto downstream and runs the downstream
// server over stdio (the transport the real MCP client speaks). It blocks until
// the client disconnects or ctx is cancelled.
func (p *Proxy) Serve(ctx context.Context, downstream *sdk.Server) error {
	tools, err := p.upstream.ListTools(ctx, nil)
	if err != nil {
		return fmt.Errorf("listing upstream tools: %w", err)
	}
	for _, t := range tools.Tools {
		p.mirror(downstream, t)
	}
	return downstream.Run(ctx, &sdk.StdioTransport{})
}

// mirror re-exposes one upstream tool on the downstream server. The downstream
// tool keeps the upstream's name/description/schema so the agent sees exactly
// the same tool — it just reaches it through the guard.
func (p *Proxy) mirror(downstream *sdk.Server, t *sdk.Tool) {
	name := t.Name
	downstream.AddTool(&sdk.Tool{
		Name:        name,
		Description: t.Description,
		InputSchema: t.InputSchema,
		Annotations: t.Annotations,
	}, func(ctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		return p.gateAndRelay(ctx, name, req)
	})
}

// gateAndRelay is the choke point every tool call flows through: evaluate the
// policy, record it, and either relay to the upstream (allow) or refuse
// (ask/deny) so the upstream is never reached.
func (p *Proxy) gateAndRelay(ctx context.Context, tool string, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
	target := argsToTarget(req)
	v := p.pol.Evaluate(policy.Action{Tool: tool, Target: target, Context: p.context})

	audit.Append(audit.Entry{
		Time: p.clock().UTC(), Tool: tool, Target: target, Context: p.context,
		Decision: string(v.Decision), Rule: v.Rule,
	})

	if v.Decision != policy.Allow {
		// The agent receives a tool error; the upstream is never called. We use
		// IsError rather than a transport error so the agent gets a clean,
		// model-readable refusal it can reason about.
		msg := v.Message
		if msg == "" {
			msg = "blocked by agentguard policy"
		}
		verb := "denied"
		if v.Decision == policy.Ask {
			verb = "requires human approval (not granted in this session)"
		}
		return &sdk.CallToolResult{
			IsError: true,
			Content: []sdk.Content{&sdk.TextContent{
				Text: fmt.Sprintf("agentguard %s this call: %s", verb, msg),
			}},
		}, nil
	}

	// Allowed: relay verbatim to the upstream and return its result.
	res, err := p.upstream.CallTool(ctx, &sdk.CallToolParams{
		Name:      tool,
		Arguments: req.Params.Arguments,
	})
	if err != nil {
		return nil, fmt.Errorf("relaying to upstream tool %q: %w", tool, err)
	}
	return res, nil
}

func (p *Proxy) clock() time.Time {
	if p.now != nil {
		return p.now()
	}
	return time.Now()
}

// Close shuts down the upstream session.
func (p *Proxy) Close() error {
	if p.upstream != nil {
		return p.upstream.Close()
	}
	return nil
}

// argsToTarget renders a tool call's arguments into a stable string the policy
// can match against — the same shape `policy test --target` expects. The
// wire arguments are already JSON (json.RawMessage), so we return them as-is.
func argsToTarget(req *sdk.CallToolRequest) string {
	if req == nil || req.Params == nil || len(req.Params.Arguments) == 0 {
		return ""
	}
	return string(req.Params.Arguments)
}
