package scan

import "testing"

func find(r Report, sub string) *Finding {
	for i := range r.Findings {
		if contains(r.Findings[i].Title, sub) {
			return &r.Findings[i]
		}
	}
	return nil
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestUnpinnedRunnerIsHigh(t *testing.T) {
	r := Servers(map[string]mcpServer{
		"x": {Command: "npx", Args: []string{"-y", "some-server"}},
	})
	f := find(r, "unpinned remote code")
	if f == nil || f.Severity != SevHigh {
		t.Fatalf("unpinned npx must be HIGH, got %+v", r.Findings)
	}
}

func TestPinnedRunnerIsClean(t *testing.T) {
	r := Servers(map[string]mcpServer{
		"x": {Command: "npx", Args: []string{"-y", "some-server@1.2.3"}},
	})
	if len(r.Findings) != 0 {
		t.Errorf("a pinned version must produce no finding, got %+v", r.Findings)
	}
}

func TestHardcodedSecretFlagged_EnvRefClean(t *testing.T) {
	r := Servers(map[string]mcpServer{
		"bad":  {Command: "node", Env: map[string]string{"API_TOKEN": "sk-real-123"}},
		"good": {Command: "node", Env: map[string]string{"API_TOKEN": "${API_TOKEN}"}},
	})
	if find(r, "secret is hard-coded") == nil {
		t.Error("a hard-coded secret must be flagged")
	}
	// The env-referenced one must not add a finding.
	count := 0
	for _, f := range r.Findings {
		if f.Server == "good" {
			count++
		}
	}
	if count != 0 {
		t.Errorf("an ${ENV} reference must not be flagged, got %d findings for it", count)
	}
}

func TestPlainHTTPFlagged_HTTPSClean(t *testing.T) {
	r := Servers(map[string]mcpServer{
		"insecure": {Type: "http", URL: "http://x/mcp"},
		"secure":   {Type: "http", URL: "https://x/mcp"},
	})
	if find(r, "plain HTTP") == nil {
		t.Error("plain HTTP must be flagged")
	}
	for _, f := range r.Findings {
		if f.Server == "secure" {
			t.Errorf("https must be clean, got %+v", f)
		}
	}
}

func TestDockerWithoutDigestIsLow(t *testing.T) {
	r := Servers(map[string]mcpServer{
		"c": {Command: "docker", Args: []string{"run", "some/image:latest"}},
	})
	if f := find(r, "pinned digest"); f == nil || f.Severity != SevLow {
		t.Errorf("undigested docker image should be LOW, got %+v", r.Findings)
	}
	// With a digest, clean.
	r2 := Servers(map[string]mcpServer{
		"c": {Command: "docker", Args: []string{"run", "some/image@sha256:abcd"}},
	})
	if len(r2.Findings) != 0 {
		t.Errorf("a digest-pinned image must be clean, got %+v", r2.Findings)
	}
}

func TestReportIsDeterministicAndSorted(t *testing.T) {
	// HIGH must sort before MEDIUM regardless of map order.
	r := Servers(map[string]mcpServer{
		"z-http": {Type: "http", URL: "http://x"},
		"a-npx":  {Command: "npx", Args: []string{"s"}},
	})
	if len(r.Findings) < 2 || r.Findings[0].Severity != SevHigh {
		t.Errorf("findings must be most-severe-first, got %+v", r.Findings)
	}
}
