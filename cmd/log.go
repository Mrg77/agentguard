package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Mrg77/agentguard/internal/audit"
	"github.com/Mrg77/agentguard/internal/ui"
)

var (
	logProd     bool
	logDecision string
	logSince    string
)

// logCmd replays the audit trail: every action an agent proposed through the
// proxy and what the guard decided. It answers "what did my agent try against
// prod, and did the guard stop it?".
var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Replay what agents proposed and how the guard decided",
	Long: `Replay the audit trail the proxy records — one line per guarded action.

  agentguard log                 # everything, oldest first
  agentguard log --prod          # only production-like contexts
  agentguard log --decision deny # only what was blocked
  agentguard log --since 7d      # the last week
  agentguard log --json          # machine-readable`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		f := audit.Filter{ProdOnly: logProd, DecisionOnly: logDecision}
		if logSince != "" {
			d, err := parseSince(logSince)
			if err != nil {
				return err
			}
			f.Since = time.Now().Add(-d)
		}
		entries, err := audit.Read(f)
		if err != nil {
			return err
		}

		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(entries)
		}

		fmt.Println(ui.Header("agentguard log", "actions agents proposed, oldest first"))
		fmt.Println()
		if len(entries) == 0 {
			fmt.Println(ui.Dim.Render("  Nothing recorded yet — run `agentguard proxy` and let an agent call the guard."))
			return nil
		}
		var denied int
		for _, e := range entries {
			fmt.Printf("  %s  %s  %s\n",
				ui.Faint.Render(e.Time.Local().Format("2006-01-02 15:04")),
				decisionBadge(e.Decision),
				e.Target)
			meta := "tool: " + orDash(e.Tool) + "  ·  context: " + orDash(e.Context)
			if e.Tokens > 0 {
				meta += "  ·  ~" + strconv.Itoa(e.Tokens) + " tok"
			}
			fmt.Printf("             %s\n", ui.Faint.Render(meta))
			if e.Decision == "deny" {
				denied++
			}
		}
		fmt.Println()
		fmt.Println(ui.Faint.Render(fmt.Sprintf("  %d action(s) · %d denied · filter with --prod / --decision deny / --since 7d",
			len(entries), denied)))
		return nil
	},
}

func decisionBadge(d string) string {
	switch d {
	case "deny":
		return ui.Err.Render(ui.MarkDeny + " deny ")
	case "ask":
		return ui.Warn.Render(ui.MarkAsk + " ask  ")
	default:
		return ui.OK.Render(ui.MarkAllow + " allow")
	}
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// parseSince accepts "7d", "24h", "30m". Go's ParseDuration doesn't know "d".
func parseSince(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		var days int
		if _, err := fmt.Sscanf(s, "%dd", &days); err != nil || days < 0 {
			return 0, fmt.Errorf("invalid --since %q (try 7d, 24h, 30m)", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid --since %q (try 7d, 24h, 30m)", s)
	}
	return d, nil
}

func init() {
	logCmd.Flags().BoolVar(&logProd, "prod", false, "only production-like contexts")
	logCmd.Flags().StringVar(&logDecision, "decision", "", "only this decision (allow, ask, deny)")
	logCmd.Flags().StringVar(&logSince, "since", "", "only entries newer than a duration (7d, 24h, 30m)")
	rootCmd.AddCommand(logCmd)
}
