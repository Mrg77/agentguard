package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Mrg77/agentguard/internal/policy"
	"github.com/Mrg77/agentguard/internal/ui"
)

// DefaultPolicyPath is where agentguard looks for a policy and where `init`
// writes one: ./agentguard.yaml, so the policy lives in the repo, versioned
// next to the code it guards.
const DefaultPolicyPath = "agentguard.yaml"

var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Work with the guard policy (test, lint)",
}

var (
	testTool    string
	testTarget  string
	testContext string
	testFile    string
)

// policy test simulates one action against the policy and prints the decision.
// It is the fast feedback loop: prove a rule does what you think BEFORE an
// agent ever runs, and gate it in CI via --json + the exit code.
var policyTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Simulate an agent action and show the guard's decision",
	Long: `Evaluate a single action (tool × target × context) against the policy and
print allow / ask / deny, plus the matching rule.

  agentguard policy test --tool shell \
    --target "kubectl delete namespace payments" --context prod

Exit code encodes the decision so CI can gate on it: 0 = allow, 0 = ask,
2 = deny (a denied action fails the check). With --json the same result is
emitted as an object.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		p, loadedFrom, err := loadPolicyOrDefault(testFile)
		if err != nil {
			return err
		}
		act := policy.Action{Tool: testTool, Target: testTarget, Context: testContext}
		v := p.Evaluate(act)

		if jsonOut {
			if err := json.NewEncoder(os.Stdout).Encode(map[string]any{
				"tool": act.Tool, "target": act.Target, "context": act.Context,
				"decision": v.Decision, "rule": v.Rule, "message": v.Message,
				"policy": loadedFrom,
			}); err != nil {
				return err
			}
			return exitFor(v.Decision)
		}

		fmt.Println(ui.Header("agentguard policy test", "would this action be allowed?"))
		fmt.Println()
		fmt.Printf("  %s %s\n", ui.Dim.Render("tool   "), act.Tool)
		fmt.Printf("  %s %s\n", ui.Dim.Render("target "), act.Target)
		fmt.Printf("  %s %s\n", ui.Dim.Render("context"), orNone(act.Context))
		fmt.Println()
		fmt.Printf("  %s\n", decisionLine(v))
		return exitFor(v.Decision)
	},
}

// decisionLine renders the verdict in its themed style with a glyph.
func decisionLine(v policy.Verdict) string {
	rule := v.Rule
	if rule == "" {
		rule = "default policy"
	}
	switch v.Decision {
	case policy.Deny:
		return ui.Err.Render(ui.MarkDeny+" DENY") + "  " + v.Message + ui.Faint.Render("  ["+rule+"]")
	case policy.Ask:
		return ui.Warn.Render(ui.MarkAsk+" ASK ") + "  " + v.Message + ui.Faint.Render("  ["+rule+"]")
	default:
		return ui.OK.Render(ui.MarkAllow+" ALLOW") + ui.Faint.Render("  ["+rule+"]")
	}
}

// exitFor maps a decision to a process exit code so `policy test` can gate CI:
// a denied action is a failing check.
func exitFor(d policy.Decision) error {
	if d == policy.Deny {
		return &exitError{code: 2, msg: "action denied by policy"}
	}
	return nil
}

// loadPolicyOrDefault loads the given file, else DefaultPolicyPath, else falls
// back to the built-in default policy. The second return says where it came
// from, for transparency.
func loadPolicyOrDefault(file string) (*policy.Policy, string, error) {
	path := file
	if path == "" {
		path = DefaultPolicyPath
	}
	if _, err := os.Stat(path); err == nil {
		p, err := policy.Load(path)
		if err != nil {
			return nil, "", err
		}
		abs, _ := filepath.Abs(path)
		return p, abs, nil
	}
	if file != "" {
		// An explicit file that doesn't exist is an error, not a silent default.
		return nil, "", fmt.Errorf("policy file not found: %s", file)
	}
	p, err := policy.Parse([]byte(policy.DefaultYAML))
	if err != nil {
		return nil, "", err
	}
	return p, "built-in default policy", nil
}

func orNone(s string) string {
	if s == "" {
		return ui.Faint.Render("(none)")
	}
	return s
}

func init() {
	policyTestCmd.Flags().StringVar(&testTool, "tool", "", "the tool/function the agent is calling (e.g. shell, kubectl)")
	policyTestCmd.Flags().StringVar(&testTarget, "target", "", "what the tool acts on (e.g. \"kubectl delete namespace payments\")")
	policyTestCmd.Flags().StringVar(&testContext, "context", "", "the active context (e.g. prod, staging)")
	policyTestCmd.Flags().StringVarP(&testFile, "file", "f", "", "policy file to use (default: ./agentguard.yaml, else built-in)")
	policyCmd.AddCommand(policyTestCmd)
	rootCmd.AddCommand(policyCmd)
}
