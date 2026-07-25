package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Mrg77/agentguard/internal/cluster"
	"github.com/Mrg77/agentguard/internal/ui"
)

var upNoModel bool

// upCmd brings up a throwaway local cluster with a CPU-only LLM, so the guard
// can be demoed against a real agent on a laptop. It is the "make it tangible"
// command: everything else (policy, proxy) becomes something you can watch.
var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Bring up a throwaway kind cluster + a local CPU model to demo the guard",
	Long: `Create a dedicated, throwaway kind cluster and deploy a small CPU-only model
(Ollama) into it, so you can see the guard fire against a real agent — locally,
no cloud, no GPU.

It never touches your default kubeconfig: agentguard uses its own cluster and
its own kubeconfig file, so it can't collide with (or log into) a real cluster.

  agentguard up              # cluster + Ollama
  agentguard up --no-model   # just the cluster (faster; add the model later)
  agentguard down            # tear it all down`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// The image pull + first boot can be slow; give it real headroom.
		ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
		defer cancel()

		for _, t := range []cluster.Tool{cluster.Docker, cluster.Kind, cluster.Kubectl} {
			if err := cluster.Ensure(t); err != nil {
				return err
			}
		}

		fmt.Println(ui.Header("agentguard up", "a throwaway local cluster to watch the guard work"))
		fmt.Println()

		step("Creating the throwaway kind cluster")
		if err := cluster.Create(ctx); err != nil {
			return err
		}
		done("cluster \"" + cluster.Name + "\" ready")

		if !upNoModel {
			step("Deploying a CPU-only model (Ollama) — first run pulls the image, be patient")
			if err := cluster.DeployOllama(ctx); err != nil {
				return err
			}
			done("Ollama is running")
		}

		fmt.Println()
		fmt.Println(ui.OK.Render("  Ready.") + " Your isolated cluster is up.")
		fmt.Println(ui.Dim.Render("  Talk to it (agentguard's kubeconfig, never your default):"))
		fmt.Printf("    %s\n", ui.Faint.Render("kubectl "+strings.Join(cluster.KubectlArgs(), " ")+" get pods"))
		fmt.Println(ui.Dim.Render("  Tear it all down with `agentguard down`."))
		return nil
	},
}

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Tear down agentguard's throwaway cluster",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cluster.Ensure(cluster.Kind); err != nil {
			return err
		}
		fmt.Print(ui.Dim.Render("Removing the throwaway cluster… "))
		if err := cluster.Delete(cmd.Context()); err != nil {
			return err
		}
		fmt.Println(ui.OK.Render(ui.MarkAllow + " done"))
		return nil
	},
}

func step(msg string) { fmt.Printf("  %s %s\n", ui.Warn.Render("▸"), msg) }
func done(msg string) { fmt.Printf("  %s %s\n", ui.OK.Render(ui.MarkAllow), ui.Dim.Render(msg)) }

func init() {
	upCmd.Flags().BoolVar(&upNoModel, "no-model", false, "create the cluster only, skip deploying the model")
	rootCmd.AddCommand(upCmd, downCmd)
}
