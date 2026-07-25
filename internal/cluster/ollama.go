package cluster

import (
	"context"
	"fmt"
)

// OllamaManifest is the Deployment + Service for a CPU-only Ollama, the same
// shape validated by hand before this was automated. No GPU, modest resource
// requests, so it schedules on a laptop kind node.
const OllamaManifest = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ollama
  labels: {app: ollama}
spec:
  replicas: 1
  selector: {matchLabels: {app: ollama}}
  template:
    metadata: {labels: {app: ollama}}
    spec:
      containers:
      - name: ollama
        image: ollama/ollama:latest
        ports: [{containerPort: 11434}]
        resources:
          requests: {cpu: "500m", memory: "1Gi"}
          limits: {memory: "4Gi"}
---
apiVersion: v1
kind: Service
metadata: {name: ollama}
spec:
  selector: {app: ollama}
  ports: [{port: 11434, targetPort: 11434}]
`

// DeployOllama applies the Ollama manifest into our cluster and waits for the
// pod to become Ready. The image pull can take a while on first run, so the
// caller passes a context with a generous timeout.
func DeployOllama(ctx context.Context) error {
	if err := Apply(ctx, OllamaManifest); err != nil {
		return err
	}
	args := append(KubectlArgs(),
		"wait", "--for=condition=Available", "deployment/ollama", "--timeout=300s")
	if _, err := run(ctx, "", "kubectl", args...); err != nil {
		return fmt.Errorf("waiting for Ollama to be ready: %w", err)
	}
	return nil
}
