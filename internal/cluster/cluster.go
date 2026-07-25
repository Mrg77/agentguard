// Package cluster brings up a throwaway local Kubernetes cluster (via kind) and
// deploys a small CPU-only LLM (Ollama) into it, so a user can watch the guard
// fire against a real agent on their laptop — no cloud, no GPU.
//
// SAFETY: everything here targets a DEDICATED kind cluster and its OWN
// kubeconfig file. It never reads or writes the user's default kubeconfig and
// never runs against a context it did not create. That isolation is deliberate:
// on some machines the default kubeconfig points at a real (OIDC) cluster, and
// touching it could trigger a production login. agentguard's own cluster is the
// only thing it ever talks to.
package cluster

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Name is the fixed name of agentguard's throwaway cluster. A stable name means
// `up` is idempotent and `down` knows exactly what to remove.
const Name = "agentguard"

// kindContext is the kubectl context kind creates for our cluster.
const kindContext = "kind-" + Name

// Kubeconfig returns the path to agentguard's dedicated kubeconfig, kept under
// the user's state dir so it never collides with ~/.kube/config.
func Kubeconfig() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(os.TempDir(), "agentguard-kubeconfig")
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "agentguard", "kubeconfig")
}

// Tool is an external binary agentguard shells out to. We check presence up
// front and give an actionable error rather than a cryptic exec failure.
type Tool struct {
	Name string
	Hint string // how to install it
}

var (
	Docker  = Tool{"docker", "install Docker Desktop or the docker engine"}
	Kind    = Tool{"kind", "brew install kind  (or see https://kind.sigs.k8s.io)"}
	Kubectl = Tool{"kubectl", "brew install kubectl"}
)

// Ensure verifies a tool is on PATH, returning an actionable error if not.
func Ensure(t Tool) error {
	if _, err := exec.LookPath(t.Name); err != nil {
		return fmt.Errorf("%s is required but not found — %s", t.Name, t.Hint)
	}
	return nil
}

// Exists reports whether agentguard's cluster is already running.
func Exists(ctx context.Context) (bool, error) {
	out, err := run(ctx, "", "kind", "get", "clusters")
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == Name {
			return true, nil
		}
	}
	return false, nil
}

// Create brings up the kind cluster, writing its kubeconfig to our dedicated
// path. It is a no-op if the cluster already exists.
func Create(ctx context.Context) error {
	exists, err := Exists(ctx)
	if err != nil {
		return err
	}
	kubeconfig := Kubeconfig()
	if err := os.MkdirAll(filepath.Dir(kubeconfig), 0o700); err != nil {
		return err
	}
	if exists {
		// Refresh the kubeconfig in case it was removed, but don't recreate.
		_, err := run(ctx, "", "kind", "export", "kubeconfig",
			"--name", Name, "--kubeconfig", kubeconfig)
		return err
	}
	_, err = run(ctx, "", "kind", "create", "cluster",
		"--name", Name, "--kubeconfig", kubeconfig)
	return err
}

// Delete tears the cluster down. Missing cluster is not an error.
func Delete(ctx context.Context) error {
	_, err := run(ctx, "", "kind", "delete", "cluster", "--name", Name)
	return err
}

// Apply pipes a manifest to `kubectl apply`, always against OUR context and
// kubeconfig — never the user's default.
func Apply(ctx context.Context, manifest string) error {
	_, err := run(ctx, manifest, "kubectl",
		"--context", kindContext, "--kubeconfig", Kubeconfig(),
		"apply", "-f", "-")
	return err
}

// KubectlArgs returns the flags that pin any kubectl call to our cluster, so
// callers (and the printed instructions) never accidentally hit another
// context.
func KubectlArgs() []string {
	return []string{"--context", kindContext, "--kubeconfig", Kubeconfig()}
}

// run executes a command with optional stdin, returning combined output. It
// carries the caller's context so a hung tool can be cancelled/timed out.
func run(ctx context.Context, stdin string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, out)
	}
	return string(out), nil
}
