package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/Mrg77/agentguard/internal/interpose"
)

var interposeContext string

// interposeCmd is the enforcing mode: agentguard sits transparently in front of
// a real upstream MCP server. The agent connects to agentguard, not to the
// upstream, so every tool call is forced through the policy — it cannot be
// bypassed. This is what makes "firewall" earned (vs. the advisory `proxy`).
var interposeCmd = &cobra.Command{
	Use:   "interpose -- <upstream-command> [args…]",
	Short: "Sit transparently in front of a real MCP server and gate every tool call",
	Long: `Run agentguard as a TRANSPARENT proxy in front of an upstream MCP tool server.

The agent connects to agentguard instead of the upstream server. agentguard
launches the upstream, mirrors its tools verbatim, and gates every call:

  client ──► agentguard (interpose) ──► upstream MCP server
                    │ allow → relay to upstream, return its result
                    │ deny  → the upstream is NEVER called; the agent gets a refusal

Because the agent no longer has the upstream in its config — only agentguard —
it cannot reach a tool except through the guard. That is enforcement, not
advice. Every decision is recorded (see 'agentguard log').

  # front a filesystem MCP server, tagging the session context as prod
  agentguard interpose --context prod -- npx -y @modelcontextprotocol/server-filesystem /data

Register agentguard (not the upstream) with your MCP client, e.g.:

  claude mcp add fs -- agentguard interpose --context prod -- <upstream…>`,
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: false,
	RunE: func(cmd *cobra.Command, args []string) error {
		pol, from, err := loadPolicyOrDefault(testFile)
		if err != nil {
			return err
		}
		// Everything diagnostic goes to stderr so stdout stays a clean JSON-RPC
		// stream for the downstream client.
		fmt.Fprintf(os.Stderr, "agentguard interpose: enforcing %s in front of: %v\n", from, args)

		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		up := interpose.Upstream{Command: args[0], Args: args[1:]}
		proxy, err := interpose.New(ctx, pol, interposeContext, up)
		if err != nil {
			return err
		}
		defer proxy.Close()

		downstream := sdk.NewServer(&sdk.Implementation{
			Name: "agentguard", Title: "agentguard (guarded)", Version: version,
		}, nil)

		if err := proxy.Serve(ctx, downstream); err != nil && ctx.Err() == nil {
			return fmt.Errorf("interpose: %w", err)
		}
		return nil
	},
}

func init() {
	interposeCmd.Flags().StringVar(&interposeContext, "context", "",
		"context tag applied to every call for policy matching (e.g. prod, staging)")
	interposeCmd.Flags().StringVarP(&testFile, "file", "f", "",
		"policy file to enforce (default: ./agentguard.yaml, else built-in)")
	rootCmd.AddCommand(interposeCmd)
}
