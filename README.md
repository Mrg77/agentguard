<div align="center">

# agentguard 🛡️

**A safety net that stops an AI agent from doing something irreversible to your infrastructure.**

**English** · [Français](README.fr.md)

[![CI](https://github.com/Mrg77/agentguard/actions/workflows/ci.yml/badge.svg)](https://github.com/Mrg77/agentguard/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Made with Go](https://img.shields.io/badge/made%20with-Go-00ADD8?logo=go&logoColor=white)](https://go.dev)

![agentguard demo](demo/demo.gif)

</div>

---

## The problem, as a true story

In July 2025, a developer let an AI agent work on their project. In its instructions,
they had written — in capital letters, eleven times — to **never touch the production
database**. The agent deleted it anyway. ([the full story](https://www.theregister.com/2025/07/21/replit_saastr_ai_vibe_coding/))

That's the whole point. A modern AI agent doesn't just answer questions anymore — it
**acts**. It runs commands, calls tools, edits files. And we're handing it more and more
of the keys to our infrastructure. The catch is that **telling it not to do something in
its prompt isn't enough**: under the right phrasing, or after a malicious injection, it
will do it anyway. That's not an opinion — it's a lesson the security community has
written down (see [OWASP LLM06](https://genai.owasp.org/llmrisk/llm06-excessive-agency/),
which says it plainly: *authorization must be enforced outside the model, not inside it*).

**agentguard starts from a simple premise: don't try to convince the AI to behave. Stop it
from misbehaving, mechanically.**

The idea is as old as networking: a **firewall**. A network firewall doesn't politely ask
packets not to pass — it blocks them. agentguard does the same, but for an AI agent's
actions. Before each action, it checks a **rule you wrote** and decides: let it through,
ask a human first, or block it. The rule is the same every time, no matter what the model
"meant".

> **One thing to say up front, honestly:** agentguard does not make your AI agent "safe".
> It *shrinks the damage it can do*. It can't stop a prompt injection (nobody can do that
> reliably), and it doesn't read intentions. What it does, and does well: keep a dangerous
> action from reaching your infrastructure. A safety net, not a guarantee — and that's
> already a lot.

*(agentguard is the little sibling of [opsforge](https://github.com/Mrg77/opsforge): the
same idea of guardrails, applied there to a command **you** type, here to an action **your
agent** tries.)*

---

## Install

```sh
# macOS (Homebrew):
brew install mrg77/tap/agentguard

# Linux (Debian/Ubuntu/Alpine…) or macOS — the install script:
curl -fsSL https://raw.githubusercontent.com/Mrg77/agentguard/main/install.sh | sh

# or build from source (Go 1.26+):
git clone https://github.com/Mrg77/agentguard && cd agentguard && go build -o agentguard .
```

The script picks the right binary for your OS/arch into `~/.local/bin` (override
with `AGENTGUARD_INSTALL_DIR`, pin a version with `AGENTGUARD_VERSION=v0.1.0`).

## Try it in a minute

```sh
git clone https://github.com/Mrg77/agentguard && cd agentguard
go build -o agentguard .

# Generate a starter policy (a set of ready-to-use rules)
./agentguard init

# Simulate an action: "the agent wants to delete a prod namespace" — allowed or not?
./agentguard policy test \
  --tool shell --target "kubectl delete namespace payments" --context prod
```

What you see:

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

Blocked. The same action, but read-only or away from production, goes through fine:

```sh
./agentguard policy test --tool kubectl --target "kubectl get pods" --context prod
#   ✓ ALLOW
```

That's the whole idea: **the decision depends on what the action actually does and where it
runs — never on what the model claims to want.**

---

## How it works, concretely

To agentguard, an **action** boils down to three pieces of information — three bits of text
it never tries to "understand", just to match against rules:

| The field | What it is | Example |
|---|---|---|
| **the tool** (`tool`) | which tool or function the agent calls | `shell`, `kubectl`, `http.post`… |
| **the target** (`target`) | what the action acts on | `kubectl delete namespace payments` |
| **the context** (`context`) | the environment it runs in | `prod`, `staging`, `gke_prod-eu`… |

From there, the engine reads your rules **top to bottom and stops at the first match**. It
then returns one of three decisions:

- **`allow`** — the action goes through, nothing to flag.
- **`ask`** — pause and let a human decide.
- **`deny`** — block it, full stop.

If no rule matches, the default applies. And by default, agentguard picks caution (`ask`: a
human decides) over letting things through.

One detail that matters: this engine is **completely deterministic**. No network call, no
AI, no randomness. The same action *always* gives the same answer. That's what makes it
testable, predictable, and trustworthy — the opposite of a system that "guesses".

---

## The rules: a plain file you version

`agentguard init` writes you a commented, cautious-by-default `agentguard.yaml`. You read
it, adapt it, commit it to your repo — right next to the code it protects. Here's what a
rule looks like:

```yaml
default: ask            # if no rule matches → a human decides

rules:
  - name: no deleting prod namespaces
    tool: "kubectl|shell"
    target: "delete\\s+(namespace|ns)\\b"
    context: "prod"
    action: deny
    message: "Deleting a production namespace is forbidden by policy."

  - name: ask before a destructive kubectl on prod
    target: "kubectl\\s+(delete|drain|cordon|scale|rollout\\s+restart)"
    context: "prod"
    action: ask

  - name: allow read-only kubectl anywhere
    target: "kubectl\\s+(get|describe|logs|top)\\b"
    action: allow
```

The match patterns are regular expressions (case-insensitive), but you don't need to be an
expert: a plain word behaves like a "contains this text" search. An empty field means
"anything".

Because it's a file, it lives in your repo, gets reviewed in a pull request, and is tested
automatically (see the CI section below) — instead of being hand-tweaked differently on
every machine.

> A small example of care: the rule that blocks `rm -rf` doesn't specify a tool. Why?
> Because `rm -rf` is dangerous no matter which tool runs it. Had we required "tool =
> shell", the agent could slip past by mislabeling its call. That's exactly the kind of
> hole this sort of tool has to avoid.

---

## Two ways to wire agentguard in: advice, or a real barrier

This is **the** thing to understand, because the two modes don't protect you the same way.

First, a word on **MCP** (*Model Context Protocol*). It's the open standard, born in
2024-2025, that lets an AI agent connect to tools. Claude Code, Cursor, VS Code, Windsurf,
Zed… they all speak it. agentguard speaks MCP too, which means one practical thing: **it
works with any MCP-compatible editor/agent — you're locked into none of them.**

Now, the two modes:

| Mode | How it sits | Can the agent ignore it? |
|---|---|---|
| **`agentguard proxy`** *(advisory)* | offers a `guard` tool the agent **chooses** to call before acting | **Yes.** A misbehaving agent can ignore it and call the tool directly. Mostly useful for the audit trail. |
| **`agentguard interpose`** *(the real firewall)* | sits **on the path**, between the agent and the real tool | **No.** The agent only has agentguard to talk to. Every call *must* go through the rule. |

The `interpose` mode is the one that earns the word "firewall". Here's the idea:

```text
interpose:  the agent ──► agentguard ──► the real tool server (kubectl, files…)
                              │ allowed → relayed to the tool, the response comes back normally
                              │ denied  → the tool is NEVER called; the agent gets a refusal
```

Put plainly: you wire **agentguard** into your agent, *not* the real tool. The agent thinks
it's talking to the tool, but everything goes through the guardrail first. It can't bypass
it, simply because it no longer has the tool at hand — only agentguard.

```sh
# Put agentguard in front of a real tool server (here, filesystem access).
# You register AGENTGUARD with the agent, not the original server.
claude mcp add fs -- agentguard interpose --context prod -- \
  npx -y @modelcontextprotocol/server-filesystem /data
```

`interpose` mirrors the original server's tools exactly (same names, same parameters): the
agent sees the very same server, it just reaches it through the guardrail. And in both
modes, every decision is recorded to a log (`agentguard log`).

---

## The log: what did my agent try?

Every time agentguard makes a decision, it writes it to a local log. `agentguard log`
replays it for you:

```sh
agentguard log --prod            # only production contexts
agentguard log --decision deny   # only what was blocked
agentguard log --since 7d        # the last week
```

That's your trail: *what did my agent try to do against prod this week, and did the
guardrail let it through?* A question the raw conversation with the model can't answer. The
file is line-by-line JSON at a known path, so it's easy to ship to a team tool (Loki, a
SIEM…) if you want the fleet-wide view.

---

## Check where your tool servers' code comes from

A small bonus, in the same security spirit. Before you let an agent load an MCP tool
server, it's worth knowing where its code comes from. `agentguard scan` reads your agent's
MCP config (the file from Claude Code, Cursor…) and flags red flags — **without ever
launching any server**:

```sh
agentguard scan ~/.cursor/mcp.json
```

It catches, for example: code downloaded and run at every startup with no pinned version
(`npx` without a version number), a secret written in clear text in the config, or a server
reached over unencrypted HTTP. It's read-only and offline — it just looks at the declared
launch command.

---

## The local demo: a real cluster, a real model

To see the guardrail in a realistic setting, agentguard can spin up a whole throwaway
playground on your machine — a real Kubernetes cluster ([kind](https://kind.sigs.k8s.io))
with a small model running on CPU ([Ollama](https://ollama.com)), all in one command. No
cloud, no graphics card needed.

```sh
agentguard up        # brings up the cluster + the model
agentguard down      # tears it all down
```

**Reassuring to know:** agentguard creates its **own** cluster with its **own** Kubernetes
config, in an isolated corner. It never touches your `~/.kube/config`: no way for it to
accidentally connect to a real production cluster.

---

## Wiring it into your CI

`agentguard policy test` returns an exit code that encodes the decision (2 = denied). Which
means you can make a **build fail** if your policy would let something dangerous through — a
way to check, on every commit, that your guardrails still hold:

```yaml
# .github/workflows/guardrails.yml
- run: |
    # these lines fail (and break the build) if the policy NO LONGER blocks these actions
    ! agentguard policy test --target "kubectl delete namespace payments" --context prod
    ! agentguard policy test --target "terraform destroy" --context prod
```

Add `--json` to any command for machine-readable output.

---

## What it costs (spoiler: no tokens)

A fair question when AI is involved: **does this consume tokens?**

**No. agentguard makes no calls to a language model, so it spends no tokens of its own.**
Its decision is pure text matching — fast, local, free. That's a deliberate choice, and the
exact opposite of "let an LLM be the judge" approaches, which would bill a model call per
decision.

The only cost, on the agent's side:

- **In `interpose` mode**: negligible. Allowed actions are relayed as-is. The only spend is
  a short refusal message (a few dozen tokens) the agent reads when an action is blocked —
  far cheaper than the dangerous action you just avoided.
- **In `proxy` mode**: a small overhead, the agent asking a `guard(...)` question before
  each guarded action.

The `~N tok` you see in `agentguard log` is just an **estimate of the action's size** (for
cost tracking), not a bill, and definitely not tokens agentguard spent.

---

## The honest scope

The most important thing to remember about what this tool is — and isn't:

- **It contains, it doesn't guarantee.** It shrinks the possible damage from an agent that
  goes off the rails. It **cannot** stop a prompt injection — [there's no universal
  defense](https://simonwillison.net/2025/Apr/11/camel/) — and it doesn't claim to read
  intentions.
- **It acts on the action, not the model.** All its value is there: it ignores what the
  model "wanted" and looks at what it *tried to do*.
- **"Firewall" means `interpose`.** Only that mode is unbypassable. The `proxy` mode is
  advice: it assumes the agent plays along.
- **A guardrail you can't trust is worse than none.** An invalid policy raises a loud,
  visible error, never a misleading silence. And the default is cautious.

This honesty is a feature, not an admission of weakness. A tool that over-promised "AI
safety" would be exactly the one you shouldn't trust.

---

## Where the project stands

The core is complete, and every piece was tested end to end on a real machine:

- [x] **The decision engine** (`policy test`, `init`) — allow / ask / deny, deterministic
- [x] **`agentguard up` / `down`** — a throwaway cluster + a local model in one command (on
      a laptop, no GPU, with its own isolated config)
- [x] **`agentguard proxy`** — the advisory mode, with the audit trail
- [x] **`agentguard interpose`** — **the real firewall**: an unbypassable guardrail in
      front of a real tool server
- [x] **`agentguard scan`** — auditing where MCP servers' code comes from

Want to try each level yourself? The [TESTING.md](TESTING.md) guide walks you through it,
from the simplest (just Go) to the fullest (a real cluster).

---

<div align="center">
Built by <a href="https://github.com/Mrg77">Mrg77</a> · sibling of <a href="https://github.com/Mrg77/opsforge">opsforge</a> · MIT
</div>
