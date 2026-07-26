# Testing agentguard from A to Z

Four levels, from "just Go" to "a real cluster". Each is self-contained; stop
wherever you like. Every command here was run for real before being written down.

## Prerequisites

| Level | Needs |
|---|---|
| 1 — the engine | Go 1.26+ |
| 2 — advisory proxy | + `python3` |
| 3 — the firewall | + `node` / `npx` |
| 4 — the cluster demo | + `docker`, `kind` |

```sh
git clone https://github.com/Mrg77/agentguard && cd agentguard
go build -o agentguard .
```

---

## Level 1 — the policy engine (30 seconds, no dependencies)

```sh
./agentguard init                     # writes a fail-safe agentguard.yaml

# a destructive prod action → DENY, exit code 2
./agentguard policy test --tool shell \
  --target "kubectl delete namespace payments" --context prod
echo "exit: $?"                        # → 2

# the same read-only action → ALLOW, exit 0
./agentguard policy test --tool kubectl --target "kubectl get pods" --context prod
```

You should see a red `✗ DENY` for the first and a green `✓ ALLOW` for the second.
Edit `agentguard.yaml`, re-run, and watch the decision change — that's the whole
engine. Add `--json` for machine output; the exit code (2 = deny) gates CI.

---

## Level 2 — an agent asking the guard over MCP (advisory `proxy`)

`demo/agent-sim.py` is a ~50-line stand-in for an AI agent. It speaks MCP to
`agentguard proxy` exactly as Claude Code / Cursor would, and asks the guard
before each action:

```sh
python3 demo/agent-sim.py ./agentguard
```

Expected: three lines — `✗ DENY`, `? ASK`, `✓ ALLOW` — then
*"the agent performed NONE of these — it asked, and the guard decided"*.

Now replay the audit trail (the receipts):

```sh
./agentguard log --prod
```

> Note: `proxy` is **advisory** — it relies on the agent choosing to ask. For an
> unbypassable guard, use `interpose` (Level 3).

---

## Level 3 — the real firewall (`interpose`, unbypassable)

This puts agentguard **in front of** a real MCP tool server (the official
filesystem server) and proves the agent cannot bypass it:

```sh
python3 demo/interpose-poc.py ./agentguard /tmp/agentguard-fsdata
```

Expected: `interpose firewall OK: tools mirrored, read relayed, write blocked,
upstream unreachable`. The script asserts that a denied write is refused **and
the file is never created** — i.e. the upstream is only reachable through the
guard.

To wire it to a real client instead (e.g. Claude Code), register agentguard —
**not** the upstream:

```sh
claude mcp add fs -- ./agentguard interpose --context prod -- \
  npx -y @modelcontextprotocol/server-filesystem /some/dir
```

---

## Level 4 — the local cluster demo (`up` / `down`)

Bring up a throwaway kind cluster with a CPU-only model, to see the guard in a
realistic setting. It uses its **own** kubeconfig — it never touches
`~/.kube/config`.

```sh
./agentguard up --no-model     # cluster only (~20s); drop --no-model for Ollama too
./agentguard down              # tears it all down
```

`up` (with the model) deploys Ollama and waits until it's Ready; you can then
port-forward `svc/ollama` and hit its API. `down` removes the cluster; your
default kubeconfig is left untouched throughout.

---

## Bonus — scan the MCP servers an agent connects to

```sh
./agentguard scan ~/.cursor/mcp.json          # or any {"mcpServers": …} file
./agentguard scan some-config.json --strict    # non-zero on ANY finding (CI)
```

It flags unpinned remote code (`npx` without a version), hard-coded secrets, and
plain-HTTP servers — read-only, without launching anything.
