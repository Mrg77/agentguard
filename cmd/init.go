package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Mrg77/agentguard/internal/policy"
	"github.com/Mrg77/agentguard/internal/ui"
)

var initForce bool

// initCmd writes a starter agentguard.yaml so a user can see and edit the
// rules. It intentionally validates what it writes — a policy you can't trust
// is worse than none.
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Write a starter agentguard.yaml policy you can edit and commit",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := os.Stat(DefaultPolicyPath); err == nil && !initForce {
			return fmt.Errorf("%s already exists (use --force to overwrite)", DefaultPolicyPath)
		}
		// Belt and braces: never write a policy that doesn't compile.
		if _, err := policy.Parse([]byte(policy.DefaultYAML)); err != nil {
			return fmt.Errorf("built-in default policy is invalid: %w", err)
		}
		if err := os.WriteFile(DefaultPolicyPath, []byte(policy.DefaultYAML), 0o644); err != nil {
			return err
		}
		fmt.Printf("%s Wrote %s — a fail-safe starter policy.\n",
			ui.OK.Render(ui.MarkAllow), ui.Accent.Render(DefaultPolicyPath))
		fmt.Println(ui.Dim.Render("  Edit it, then `agentguard policy test …` to check a rule, or commit it and gate CI."))
		return nil
	},
}

func init() {
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite an existing agentguard.yaml")
	rootCmd.AddCommand(initCmd)
}
