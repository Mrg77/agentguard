package cluster

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestKubeconfigIsDedicatedNeverDefault(t *testing.T) {
	// The load-bearing safety property: agentguard's kubeconfig must live at its
	// own path, never ~/.kube/config, so it can't touch a real cluster.
	t.Setenv("XDG_STATE_HOME", "/tmp/agentguard-test-state")
	got := Kubeconfig()
	if !strings.Contains(got, filepath.Join("agentguard", "kubeconfig")) {
		t.Errorf("kubeconfig path should be agentguard-owned, got %q", got)
	}
	if strings.Contains(got, ".kube/config") {
		t.Errorf("kubeconfig must never be the default ~/.kube/config, got %q", got)
	}
}

func TestKubectlArgsPinToOurClusterOnly(t *testing.T) {
	args := KubectlArgs()
	joined := strings.Join(args, " ")
	// Every kubectl call must carry BOTH our context and our kubeconfig, so it
	// can never fall back to the user's current context.
	if !strings.Contains(joined, "--context "+kindContext) {
		t.Errorf("kubectl args must pin our context: %v", args)
	}
	if !strings.Contains(joined, "--kubeconfig") {
		t.Errorf("kubectl args must pin our kubeconfig: %v", args)
	}
}

func TestEnsureReportsMissingToolWithHint(t *testing.T) {
	err := Ensure(Tool{Name: "definitely-not-a-real-binary-xyz", Hint: "install it somehow"})
	if err == nil {
		t.Fatal("expected an error for a missing tool")
	}
	if !strings.Contains(err.Error(), "install it somehow") {
		t.Errorf("error should carry the install hint, got %q", err.Error())
	}
}

func TestOllamaManifestIsCPUOnly(t *testing.T) {
	// The demo must run on a laptop: no GPU requests, modest resources.
	if strings.Contains(OllamaManifest, "nvidia.com/gpu") {
		t.Error("the demo manifest must be CPU-only (no GPU requests)")
	}
	if !strings.Contains(OllamaManifest, "ollama/ollama") {
		t.Error("manifest should deploy the ollama image")
	}
}
