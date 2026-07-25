package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/Mrg77/agentguard/internal/proxy"
)

// proxyCmd runs the MCP enforcement server. An agent connects to it and, before
// taking any action, calls the `guard` tool to ask whether it's allowed. The
// guard applies the policy, records the decision, and answers allow/ask/deny.
//
// This is the runtime counterpart to `policy test`: same engine, but live, and
// driven by the agent itself over the Model Context Protocol.
var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Run the MCP enforcement server an agent asks before it acts",
	Long: `Start a Model Context Protocol (MCP) server on stdio that exposes a single
tool, "guard". An AI agent calls it before performing an action:

  guard(tool, target, context) -> { decision: allow | ask | deny, guidance }

The guard evaluates the action against your policy (the same rules 'policy test'
uses), records it to the audit trail, and tells the agent whether to proceed.
It never executes the action itself — enforcement is the agent honoring the
decision, plus (in a real deployment) the proxy sitting in front of the tools.

Register it with an MCP client, e.g. Claude Code:

  claude mcp add agentguard -- agentguard proxy

Review what agents proposed with:  agentguard log --prod`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		p, from, err := loadPolicyOrDefault(testFile)
		if err != nil {
			return err
		}
		// The policy source goes to stderr so it doesn't corrupt the stdio
		// JSON-RPC stream the agent speaks on stdout.
		fmt.Fprintln(os.Stderr, "agentguard proxy: enforcing "+from)

		guard := proxy.NewGuard(p)
		return serve(cmd.Context(), guard)
	},
}

// serve wires the guard onto an MCP server and blocks until the client
// disconnects or the process is interrupted.
func serve(parent context.Context, guard *proxy.Guard) error {
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := sdk.NewServer(&sdk.Implementation{
		Name:    "agentguard",
		Title:   "agentguard policy enforcement",
		Version: version,
	}, nil)

	type guardArgs struct {
		Tool    string `json:"tool" jsonschema:"the tool/function you want to call, e.g. shell, kubectl"`
		Target  string `json:"target" jsonschema:"what the tool would act on, e.g. 'kubectl delete namespace payments'"`
		Context string `json:"context" jsonschema:"the active environment, e.g. prod, staging (optional)"`
	}
	sdk.AddTool(server, &sdk.Tool{
		Name: "guard",
		Description: "Ask whether an action is permitted BEFORE performing it. Returns a decision " +
			"(allow, ask, or deny) and guidance. You must honor a deny (do not act) and an ask " +
			"(get human approval first). Call this for any state-changing or destructive action.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in guardArgs) (*sdk.CallToolResult, proxy.Result, error) {
		return nil, guard.Evaluate(in.Tool, in.Target, in.Context), nil
	})

	if err := server.Run(ctx, &sdk.StdioTransport{}); err != nil && ctx.Err() == nil {
		return fmt.Errorf("proxy server: %w", err)
	}
	return nil
}

func init() {
	proxyCmd.Flags().StringVarP(&testFile, "file", "f", "", "policy file to enforce (default: ./agentguard.yaml, else built-in)")
	rootCmd.AddCommand(proxyCmd)
}
