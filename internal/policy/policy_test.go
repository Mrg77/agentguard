package policy

import "testing"

// The default policy must be valid and enforce its headline promises. This is
// the load-bearing test: if the shipped policy is wrong, the product is wrong.
func TestDefaultPolicyEnforcesPromises(t *testing.T) {
	p, err := Parse([]byte(DefaultYAML))
	if err != nil {
		t.Fatalf("default policy must compile: %v", err)
	}

	cases := []struct {
		name string
		act  Action
		want Decision
	}{
		{"delete prod ns is denied",
			Action{Tool: "kubectl", Target: "kubectl delete namespace payments", Context: "gke_prod-eu"}, Deny},
		{"terraform destroy on prod is denied",
			Action{Tool: "shell", Target: "terraform destroy -auto-approve", Context: "prod"}, Deny},
		{"rm -rf is denied in any context",
			Action{Tool: "shell", Target: "rm -rf /data", Context: "staging"}, Deny},
		{"rm -rf is denied even with no tool declared (can't bypass by mislabeling)",
			Action{Target: "rm -rf /", Context: "dev"}, Deny},
		{"scale on prod asks first",
			Action{Tool: "kubectl", Target: "kubectl scale deploy api --replicas=0", Context: "prod"}, Ask},
		{"read-only kubectl is allowed",
			Action{Tool: "kubectl", Target: "kubectl get pods -A", Context: "prod"}, Allow},
		{"unknown action falls back to the ask default",
			Action{Tool: "http.get", Target: "https://example.com", Context: "dev"}, Ask},
		{"case-insensitive: DELETE NAMESPACE still denied",
			Action{Tool: "KUBECTL", Target: "KUBECTL DELETE NAMESPACE payments", Context: "PROD"}, Deny},
	}
	for _, c := range cases {
		if got := p.Evaluate(c.act).Decision; got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestFirstMatchWins(t *testing.T) {
	// A specific deny above a broad allow must win.
	p, err := Parse([]byte(`
default: allow
rules:
  - name: deny prod deletes
    target: "delete"
    context: "prod"
    action: deny
  - name: allow everything else
    action: allow
`))
	if err != nil {
		t.Fatal(err)
	}
	v := p.Evaluate(Action{Target: "delete pod", Context: "prod"})
	if v.Decision != Deny || v.Rule != "deny prod deletes" {
		t.Errorf("first match should win: got %+v", v)
	}
}

func TestDefaultAppliesWhenNoRuleMatches(t *testing.T) {
	p, _ := Parse([]byte("default: deny\nrules: []\n"))
	if got := p.Evaluate(Action{Tool: "x"}).Decision; got != Deny {
		t.Errorf("empty policy must apply its default: got %q", got)
	}
	// Unset default is Allow (permissive only when the author says nothing).
	p2, _ := Parse([]byte("rules: []\n"))
	if got := p2.Evaluate(Action{Tool: "x"}).Decision; got != Allow {
		t.Errorf("unset default should be allow: got %q", got)
	}
}

func TestInvalidPolicyIsAnError(t *testing.T) {
	for _, bad := range []string{
		"rules:\n  - name: x\n    action: nope\n",                  // bad action
		"rules:\n  - action: deny\n",                               // no name
		"rules:\n  - name: x\n    tool: \"[\"\n    action: deny\n", // bad regex
		"default: maybe\nrules: []\n",                              // bad default
	} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Errorf("expected an error for invalid policy:\n%s", bad)
		}
	}
}
