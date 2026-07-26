# agentguard demo — regenerate from the demo/ directory with: vhs demo.tape
# Requires on the host: the agentguard binary on PATH and python3.
#
# Narrative: the deterministic engine (policy test) → a real agent asking the
# guard before it acts, over MCP (agent-sim) → the audit trail (log) → the
# supply-chain scan. Everything runs for real; nothing destructive is executed.
Output demo.gif

Set Shell "zsh"
Set FontSize 16
Set Width 1200
Set Height 740
Set Padding 24
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms
Set PlaybackSpeed 1.0

# Set up a throwaway workdir with the sim + the sample MCP config beside it, and
# a fresh state dir for the audit log.
Hide
Type "setopt interactive_comments" Enter
Type "export XDG_STATE_HOME=$(mktemp -d)" Enter
Type "WORK=$(mktemp -d); cp $PWD/agent-sim.py $PWD/sample-mcp.json $WORK/; cd $WORK" Enter
Type "clear" Enter
Show
Sleep 800ms

# --- the engine: one action, a deterministic decision ------------------
Type "# The engine: telling a model 'never delete prod' fails. So we enforce it."  Enter
Sleep 1500ms
Type "agentguard policy test --tool shell --target 'kubectl delete namespace payments' --context prod"  Sleep 400ms  Enter
Sleep 3.5s
Type "clear" Enter
Sleep 500ms

# --- the policy: a fail-safe, versionable file -------------------------
Type "agentguard init"  Sleep 400ms  Enter
Sleep 2.5s
Type "clear" Enter
Sleep 500ms

# === THE MOMENT: a real agent asks the guard before acting, over MCP ====
Type "# A real agent, over MCP, asks the guard BEFORE each action:"  Enter
Sleep 1500ms
Type "python3 agent-sim.py agentguard 2>/dev/null"  Sleep 400ms  Enter
Sleep 5s
Type "clear" Enter
Sleep 500ms

# --- the receipts: what the agent tried on prod ------------------------
Type "# The receipts: what did the agent try against prod?"  Enter
Sleep 1200ms
Type "agentguard log --prod"  Sleep 400ms  Enter
Sleep 4s
Type "clear" Enter
Sleep 500ms

# --- supply chain: audit the agent's MCP servers -----------------------
Type "# And audit where the agent's tool servers come from:"  Enter
Sleep 1200ms
Type "agentguard scan sample-mcp.json"  Sleep 400ms  Enter
Sleep 5s
