package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Mrg77/agentguard/internal/scan"
	"github.com/Mrg77/agentguard/internal/ui"
)

var (
	scanStrict bool
)

// scanCmd audits the MCP servers an agent is configured to connect to, for
// supply-chain red flags — unpinned remote code, secrets in the config, plain
// HTTP. Read-only and offline: it never launches a server.
var scanCmd = &cobra.Command{
	Use:   "scan <mcp-config.json>",
	Short: "Audit the MCP servers an agent connects to for supply-chain risks (read-only)",
	Long: `Parse an MCP client config (Claude/Cursor style: {"mcpServers": …}) and flag
supply-chain red flags in each server, without ever launching one:

  • unpinned remote code that runs at launch (npx/uvx/bunx with no version pin)
  • a container image with no pinned digest
  • a secret hard-coded in the server's env or headers
  • a remote server reached over plain HTTP

  agentguard scan ~/.cursor/mcp.json
  agentguard scan mcp.json --strict   # exit non-zero on ANY finding (CI gate)

Read-only and offline: it inspects the declared launch command / URL only, never
runs the server. It catches configuration red flags, not runtime behavior — not
a substitute for reviewing a server's source.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rep, err := scan.File(args[0])
		if err != nil {
			return err
		}

		if jsonOut {
			if err := json.NewEncoder(os.Stdout).Encode(rep); err != nil {
				return err
			}
			return scanExit(rep)
		}

		fmt.Println(ui.Header("agentguard scan", "supply-chain audit of the agent's MCP servers"))
		fmt.Println()
		if len(rep.Findings) == 0 {
			fmt.Println(ui.OK.Render("  ✓ No supply-chain red flags found."))
			fmt.Println()
			fmt.Println(ui.Faint.Render(fmt.Sprintf("  Scanned %d server(s). Config-level only — review a server's source before trusting it.", rep.Servers)))
			return nil
		}
		for _, f := range rep.Findings {
			fmt.Printf("  %s  %s\n", scanBadge(f.Severity), f.Title)
			fmt.Printf("      %s\n", ui.Faint.Render("server: "+f.Server))
			if f.Detail != "" {
				fmt.Printf("      %s\n", ui.Dim.Render(f.Detail))
			}
			fmt.Println()
		}
		fmt.Println(ui.Faint.Render(fmt.Sprintf("  %d finding(s) across %d server(s). No secret values are shown.",
			len(rep.Findings), rep.Servers)))
		return scanExit(rep)
	},
}

// scanExit fails the check on HIGH findings (or any finding under --strict), so
// scan can gate CI.
func scanExit(rep scan.Report) error {
	if len(rep.Findings) == 0 {
		return nil
	}
	if scanStrict {
		return &exitError{code: 2, msg: fmt.Sprintf("%d supply-chain finding(s)", len(rep.Findings))}
	}
	if rep.TopSeverity() >= scan.SevHigh {
		return &exitError{code: 2, msg: "HIGH supply-chain finding(s)"}
	}
	return nil
}

func scanBadge(s scan.Severity) string {
	switch s {
	case scan.SevHigh:
		return ui.Err.Render("HIGH  ")
	case scan.SevMedium:
		return ui.Warn.Render("MEDIUM")
	default:
		return ui.Dim.Render("LOW   ")
	}
}

func init() {
	scanCmd.Flags().BoolVar(&scanStrict, "strict", false, "exit non-zero on any finding, not just HIGH (CI gate)")
	rootCmd.AddCommand(scanCmd)
}
