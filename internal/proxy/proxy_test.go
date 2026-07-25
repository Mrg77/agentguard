package proxy

import (
	"testing"
	"time"

	"github.com/Mrg77/agentguard/internal/audit"
	"github.com/Mrg77/agentguard/internal/policy"
)

func testGuard(t *testing.T) *Guard {
	t.Helper()
	p, err := policy.Parse([]byte(policy.DefaultYAML))
	if err != nil {
		t.Fatal(err)
	}
	g := NewGuard(p)
	g.now = func() time.Time { return time.Unix(0, 0).UTC() }
	return g
}

func TestGuardDecidesAndGuides(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir()) // isolate the audit log
	g := testGuard(t)

	deny := g.Evaluate("shell", "kubectl delete namespace payments", "prod")
	if deny.Decision != "deny" {
		t.Errorf("delete prod ns should deny, got %q", deny.Decision)
	}
	if deny.Guidance == "" || deny.Guidance[:6] != "Do NOT" {
		t.Errorf("a deny must guide the agent to stop, got %q", deny.Guidance)
	}

	allow := g.Evaluate("kubectl", "kubectl get pods", "prod")
	if allow.Decision != "allow" {
		t.Errorf("get pods should allow, got %q", allow.Decision)
	}
}

func TestGuardRecordsToAuditTrail(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	g := testGuard(t)

	g.Evaluate("shell", "terraform destroy", "prod")        // deny
	g.Evaluate("kubectl", "kubectl scale deploy x", "prod") // ask
	g.Evaluate("kubectl", "kubectl get pods", "staging")    // allow

	entries, err := audit.Read(audit.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("all three actions must be recorded, got %d", len(entries))
	}

	// The prod filter should catch the two prod actions.
	prod, _ := audit.Read(audit.Filter{ProdOnly: true})
	if len(prod) != 2 {
		t.Errorf("prod filter should return 2, got %d", len(prod))
	}
	// The deny filter should catch the one denial.
	denied, _ := audit.Read(audit.Filter{DecisionOnly: "deny"})
	if len(denied) != 1 || denied[0].Target != "terraform destroy" {
		t.Errorf("deny filter should return the terraform destroy, got %+v", denied)
	}
	// Token estimate is populated and positive.
	if entries[0].Tokens <= 0 {
		t.Errorf("token estimate should be > 0, got %d", entries[0].Tokens)
	}
}

func TestAuditIsBestEffortOnBadPath(t *testing.T) {
	// With no home and no XDG_STATE_HOME resolvable, Append must not panic and
	// Evaluate must still return a decision — enforcement never depends on the log.
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "")
	g := testGuard(t)
	if g.Evaluate("shell", "rm -rf /", "dev").Decision != "deny" {
		t.Error("guard must still decide even if the audit log can't be written")
	}
}
