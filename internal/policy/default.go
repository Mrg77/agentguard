package policy

// DefaultYAML is the starter policy `agentguard init` writes and the built-in
// fallback the proxy uses when no policy file exists. It is fail-safe by
// design: destructive actions against a production-like context are denied,
// state-changing actions there need a human, and everything else flows.
//
// The rules read like a firewall for an agent's tool calls, which is exactly
// the mental model to promote — not "please behave", but "here is what you may
// touch, and where".
const DefaultYAML = `# agentguard policy — a deterministic firewall for an AI agent's tool calls.
#
# Each rule matches an action (tool × target × context) and assigns:
#   deny  — block it outright
#   ask   — hold it until a human approves
#   allow — let it through
# Rules are tried top to bottom; the FIRST match wins, so put specific rules
# above broad ones. Fields are case-insensitive regexes; an empty field matches
# anything. A plain string behaves like a substring match.
#
# Test it without an agent:  agentguard policy test --tool shell \
#   --target "kubectl delete namespace payments" --context prod

# When no rule matches, fail safe: a human decides.
default: ask

rules:
  # --- hard stops: irreversible destruction on prod is never automatic -------
  - name: no deleting prod namespaces
    tool: "kubectl|shell"
    target: "delete\\s+(namespace|ns)\\b"
    context: "prod"
    action: deny
    message: "Deleting a production namespace is forbidden by policy."

  - name: no terraform destroy on prod
    target: "terraform\\s+destroy"
    context: "prod"
    action: deny
    message: "terraform destroy against a production-like context is blocked."

  # No 'tool' constraint on purpose: 'rm -rf' is dangerous whatever tool
  # channel proposes it, so we match on the target alone — a rule that only
  # fired for tool="shell" would be trivially bypassed by mislabeling the call.
  - name: no recursive force-delete of the filesystem
    target: "rm\\s+-[a-z]*r[a-z]*f|rm\\s+-[a-z]*f[a-z]*r"
    action: deny
    message: "Recursive force-delete is blocked regardless of context or tool."

  # --- confirm gates: state changes on prod need a human ---------------------
  - name: confirm destructive kubectl on prod
    tool: "kubectl|shell"
    target: "kubectl\\s+(delete|drain|cordon|scale|rollout\\s+restart)"
    context: "prod"
    action: ask
    message: "This changes Kubernetes resources on a production-like context."

  - name: confirm writes to the internal network on prod
    tool: "http\\.(post|put|patch|delete)"
    context: "prod"
    action: ask
    message: "An outbound write to a production endpoint — confirm the target."

  # --- allow: read-only inspection is always fine ----------------------------
  - name: allow read-only kubectl anywhere
    tool: "kubectl|shell"
    target: "kubectl\\s+(get|describe|logs|top|explain)\\b"
    action: allow
`
