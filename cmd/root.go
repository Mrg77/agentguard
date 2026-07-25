package cmd

import (
	"github.com/spf13/cobra"
)

// version is injected at build time by GoReleaser via ldflags.
var version = "dev"

// jsonOut is the global --json flag: when set, commands emit machine-readable
// JSON instead of the human view, so agentguard drops into CI the same way
// opsforge does.
var jsonOut bool

var rootCmd = &cobra.Command{
	Use:   "agentguard",
	Short: "A deterministic policy-as-code firewall for an AI agent's tool calls",
	Long: `agentguard puts a deterministic guard between an AI agent and what it can
actually do — which tool, against which target, in which context — as versioned,
testable rules. It is local-first and Kubernetes-native: bring up a throwaway
cluster with a small local model, and gate the agent's dangerous actions the
same way a firewall gates packets.

It does NOT ask the model to behave (telling an LLM "never delete prod" in its
prompt is known to fail). It intercepts the action and applies a rule: the same
input always yields the same decision. It's a containment tool that shrinks the
blast radius of a misbehaving or prompt-injected agent — a safety net, not a
guarantee.`,
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command. It exits 0 on success, the exitError's code
// on a decision-carrying error (e.g. 2 when the guard denies), or 1 otherwise.
func Execute() {
	handleExit(rootCmd.Execute())
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false,
		"emit machine-readable JSON instead of the human view (for CI/scripts)")
}
