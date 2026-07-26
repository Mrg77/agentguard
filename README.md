<div align="center">

# agentguard 🛡️

**A deterministic policy-as-code firewall for an AI agent's tool calls.**

Telling a model *"never delete production"* in its prompt does not hold — the
[Replit incident](https://www.theregister.com/2025/07/21/replit_saastr_ai_vibe_coding/)
deleted a production database despite eleven capitalized refusals. agentguard
doesn't ask the model to behave. It **intercepts the action** — which tool,
against which target, in which context — and applies a versioned rule, the way a
firewall applies an ACL: the same input always yields the same decision.

Local-first and Kubernetes-native. It's a **containment tool that shrinks the
blast radius** of a misbehaving or prompt-injected agent — a safety net, not a
guarantee.

**English** · [Français](README.fr.md)

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Made with Go](https://img.shields.io/badge/made%20with-Go-00ADD8?logo=go&logoColor=white)](https://go.dev)

![agentguard demo](demo/demo.gif)

**[Why](#why) · [Quickstart](#quickstart) · [How it works](#how-it-works) · [Two modes](#two-ways-to-enforce-advisory-vs-firewall) · [The policy](#the-policy) · [CI](#ci) · [Honest scope](#honest-scope) · [Roadmap](#roadmap)**

</div>

---

## Why

An AI agent in production is a **100% ops problem**: how do you sandbox what it
can touch, trace what it did, and cap what it costs? The industry learned the
hard way in 2025-26 that you cannot solve this inside the model:

- **Prompt-level rules fail.** [OWASP LLM06](https://genai.owasp.org/llmrisk/llm06-excessive-agency/)
  is explicit — *enforce authorization downstream, not in the LLM*. An agent that
  is asked not to do something will still do it under the right prompt.
- **The blast radius is real.** A single mislabeled or injected tool call can
  drop a namespace, run `terraform destroy`, or `rm -rf` a volume — and the tools
  happily comply.

agentguard sits **between the agent and the action** and enforces a rule you
wrote and can test, independent of what the model "meant". Same idea a network
firewall has used for decades, applied to an agent's tool calls.

> It is deliberately the sibling of [opsforge](https://github.com/Mrg77/opsforge)
> — the same policy-as-code guards, generalized from *a command you type* to *an
> action your agent takes*.

## Quickstart

```sh
# Build (Go 1.26+)
go build -o agentguard .

# Write a fail-safe starter policy you can read, edit and commit
./agentguard init

# Simulate an agent action — would the guard allow it?
./agentguard policy test \
  --tool shell --target "kubectl delete namespace payments" --context prod
```

```text
────────────────────────────────────────────────────────────
  ❯ agentguard policy test
  would this action be allowed?
────────────────────────────────────────────────────────────

  tool    shell
  target  kubectl delete namespace payments
  context prod

  ✗ DENY  Deleting a production namespace is forbidden by policy.  [no deleting prod namespaces]
```

Same action away from prod, or read-only, flows straight through:

```sh
./agentguard policy test --tool kubectl --target "kubectl get pods" --context prod
#   ✓ ALLOW  [allow read-only kubectl anywhere]
```

## How it works

An **action** is three plain strings the guard never interprets semantically:

| Field | What it is | Example |
|---|---|---|
| `tool` | the tool/function the agent invokes | `shell`, `kubectl`, `http.post`, an MCP tool name |
| `target` | what the tool acts on | `kubectl delete namespace payments` |
| `context` | the active environment | `prod`, `gke_prod-eu`, `staging` |

The engine tries the rules **top to bottom, first match wins**, and returns one
of three decisions:

- **`allow`** — the action proceeds.
- **`ask`** — it's held until a human approves (the confirm gate).
- **`deny`** — it's blocked outright.

If no rule matches, the policy's `default` applies (fail-safe: the shipped
default is `ask`). The engine is **pure** — no I/O, no clock, no model — so the
same input always maps to the same decision, and every rule is unit-testable.

## Two ways to enforce (advisory vs. firewall)

agentguard speaks **MCP** ([Model Context Protocol](https://modelcontextprotocol.io)),
the open standard that Claude Code, Cursor, VS Code, Windsurf, Zed and others all
speak — so it is **IDE-independent**: you register it once and it works with any
MCP client. There are two modes, and the difference matters:

| Mode | How it sits | Can the agent bypass it? |
|---|---|---|
| **`agentguard proxy`** (advisory) | exposes a `guard` tool the agent *asks* before acting | Yes — a misbehaving agent could ignore it and call a tool directly. Useful when you also want the receipts. |
| **`agentguard interpose`** (firewall) | sits **transparently in front of** a real MCP tool server | **No.** The agent only has agentguard in its config, not the upstream — every call is forced through the policy. |

```text
interpose:  client ──► agentguard ──► upstream MCP server (kubectl, filesystem, …)
                            │ allow → relay to the upstream, return its result
                            │ deny  → the upstream is NEVER called; the agent gets a refusal
```

```sh
# Put agentguard in front of a real tool server. Register THIS with your client,
# not the upstream — so the agent can't reach the tools except through the guard.
claude mcp add fs -- agentguard interpose --context prod -- \
  npx -y @modelcontextprotocol/server-filesystem /data
```

`interpose` mirrors the upstream's tools verbatim (same names, same schemas), so
the agent sees exactly the same server — it just reaches it through the guard.
Every decision, in either mode, lands in the audit trail (`agentguard log`).

## The policy

`agentguard init` writes a commented, **fail-safe** `agentguard.yaml`. Rules are
case-insensitive regexes; an empty field matches anything; a plain string
behaves like a substring match:

```yaml
default: ask            # no rule matched → a human decides

rules:
  - name: no deleting prod namespaces
    tool: "kubectl|shell"
    target: "delete\\s+(namespace|ns)\\b"
    context: "prod"
    action: deny
    message: "Deleting a production namespace is forbidden by policy."

  - name: confirm destructive kubectl on prod
    target: "kubectl\\s+(delete|drain|cordon|scale|rollout\\s+restart)"
    context: "prod"
    action: ask

  - name: allow read-only kubectl anywhere
    target: "kubectl\\s+(get|describe|logs|top)\\b"
    action: allow
```

Because it's a plain file, the policy lives in your repo, is reviewed in a PR,
and is tested in CI — not hand-tweaked per machine. `rm -rf` is matched on the
target alone (no `tool` constraint) so it can't be bypassed by mislabeling the
call — the kind of hardening the whole approach is for.

## CI

`policy test` encodes its decision in the exit code, so a denied action **fails
the job** — you can assert your guardrails hold on every commit:

```sh
# fails (exit 2) if the policy would let an agent nuke prod
agentguard policy test --target "terraform destroy" --context prod
```

```yaml
# .github/workflows/guardrails.yml
- run: |
    ! agentguard policy test --target "kubectl delete namespace payments" --context prod
    ! agentguard policy test --target "terraform destroy" --context prod
```

Add `--json` to any command for machine-readable output.

## Honest scope

The single most important thing this tool says about itself:

- **It contains, it does not guarantee.** agentguard shrinks the blast radius of
  a misbehaving agent. It **cannot** stop prompt injection — there is
  [no universal defense](https://simonwillison.net/2025/Apr/11/camel/) — and it
  does not claim to detect intent.
- **It enforces on the action, not the model.** The value is precisely that it
  ignores what the model "meant" and matches what it *tried to do*.
- **"Firewall" means `interpose`.** Only the interpose mode is unbypassable
  (the agent has no direct handle to the tool). The `proxy` mode is advisory — it
  relies on the agent asking first, and is best paired with the audit trail.
- **A guard you can't trust is worse than none.** An invalid policy is a loud
  error, never a silent misfire. The default fails safe (`ask`).

This honesty is a feature. A tool that over-promised "AI safety" would be the
less trustworthy one.

## Roadmap

The four-command MVP is complete — engine, local demo, runtime enforcement,
supply-chain scan — each validated end to end on a real machine:

- [x] The deterministic policy engine (`policy test`, `init`) — allow/ask/deny
- [x] `agentguard up` / `down` — a throwaway kind cluster + a local Ollama model
      in one command, on a laptop, CPU-only (its own isolated kubeconfig, never
      your default)
- [x] `agentguard proxy` — advisory MCP server the agent asks before it acts;
      records every decision to an audit trail (`agentguard log`)
- [x] `agentguard interpose` — **the firewall**: a transparent MCP proxy in front
      of a real tool server, so every call is forced through the policy and the
      agent cannot bypass it
- [x] `agentguard scan` — a read-only supply-chain audit of the MCP servers an
      agent connects to (unpinned remote code, hard-coded secrets, plain HTTP)

---

<div align="center">
Built by <a href="https://github.com/Mrg77">Mrg77</a> · sibling of <a href="https://github.com/Mrg77/opsforge">opsforge</a> · MIT
</div>
