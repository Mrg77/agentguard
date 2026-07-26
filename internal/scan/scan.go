// Package scan is agentguard's supply-chain audit of the MCP servers an agent
// connects to. Before you let an agent load a tool server, it's worth knowing:
// where does its code come from, is it pinned, does it ship a secret in clear
// text, does it talk over plain HTTP?
//
// It is strictly READ-ONLY and passive: it parses the MCP client config and
// inspects each server's declared launch command or URL. It NEVER runs a
// server, never installs anything, never touches the network — the same posture
// opsforge's `verify` takes with credentials. Findings say *what* is risky and
// *why*, never a secret's value.
//
// Honest scope: this catches configuration-level red flags (unpinned code, a
// token in the config, plain HTTP). It does NOT execute the server, so it can't
// detect malicious behavior that only appears at runtime, and it is not a
// substitute for reviewing a server's source. See README "Honest scope".
package scan

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Severity is a coarse, sortable risk level.
type Severity int

const (
	SevOK Severity = iota
	SevLow
	SevMedium
	SevHigh
)

func (s Severity) String() string {
	switch s {
	case SevHigh:
		return "HIGH"
	case SevMedium:
		return "MEDIUM"
	case SevLow:
		return "LOW"
	default:
		return "OK"
	}
}

// Finding is one supply-chain observation about one MCP server. It carries no
// secret value — only the server name, the issue, and how to fix it.
type Finding struct {
	Server        string   `json:"server"`
	Severity      Severity `json:"-"`
	SeverityLabel string   `json:"severity"`
	Title         string   `json:"title"`
	Detail        string   `json:"detail,omitempty"`
}

func mk(server string, sev Severity, title, detail string) Finding {
	return Finding{Server: server, Severity: sev, SeverityLabel: sev.String(), Title: title, Detail: detail}
}

// Report is the full scan result.
type Report struct {
	Servers  int       `json:"servers_scanned"`
	Findings []Finding `json:"findings"`
}

// TopSeverity returns the highest severity among findings (SevOK if none).
func (r Report) TopSeverity() Severity {
	top := SevOK
	for _, f := range r.Findings {
		if f.Severity > top {
			top = f.Severity
		}
	}
	return top
}

// mcpServer is the subset of an MCP server entry we inspect. Both stdio
// (command/args/env) and remote (type/url/headers) shapes are covered.
type mcpServer struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type mcpConfig struct {
	MCPServers map[string]mcpServer `json:"mcpServers"`
}

// File scans an MCP client config file (Claude/Cursor style: {"mcpServers": …})
// and returns the findings, most-severe first.
func File(path string) (Report, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Report{}, err
	}
	var cfg mcpConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Report{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	return Servers(cfg.MCPServers), nil
}

// Servers audits a set of MCP servers. Exported so tests and other callers can
// scan an in-memory config without a file.
func Servers(servers map[string]mcpServer) Report {
	rep := Report{Servers: len(servers), Findings: []Finding{}}
	// Deterministic order for stable output.
	names := make([]string, 0, len(servers))
	for n := range servers {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		rep.Findings = append(rep.Findings, auditServer(name, servers[name])...)
	}
	sortFindings(rep.Findings)
	return rep
}

// unpinned matches package runners that fetch and execute code at launch time
// (npx, uvx, bunx, pipx run). Without a pinned version this is arbitrary,
// mutable remote code running on every start.
var unpinned = regexp.MustCompile(`^(npx|uvx|bunx|pnpm\s+dlx|pipx)$`)

// pinnedArg detects a version pin in the args (name@1.2.3, --version, a digest).
var pinnedArg = regexp.MustCompile(`@\d|@sha256:|--version|==\d`)

// secretKey matches env/header keys that hold a credential.
var secretKey = regexp.MustCompile(`(?i)(token|secret|key|password|passwd|auth|apikey|api_key|bearer)`)

// looksLikePlaceholder recognizes an env-var reference or obvious placeholder,
// which is NOT a hard-coded secret.
func looksLikePlaceholder(v string) bool {
	v = strings.TrimSpace(v)
	return v == "" || strings.HasPrefix(v, "${") || strings.HasPrefix(v, "$") ||
		strings.HasPrefix(v, "<") || strings.EqualFold(v, "changeme")
}

// auditServer produces the findings for one server.
func auditServer(name string, s mcpServer) []Finding {
	var out []Finding

	// --- stdio servers: command + args + env -------------------------------
	if s.Command != "" {
		base := lastPathElement(s.Command)
		if unpinned.MatchString(base) && !argsPinned(s.Args) {
			out = append(out, mk(name, SevHigh,
				"runs unpinned remote code at launch ("+base+")",
				"`"+base+"` fetches and executes a package on every start with no version pin, so the "+
					"code can change under you. Pin an exact version (name@1.2.3) or a digest, or vendor the server."))
		}
		if base == "docker" && !argsHaveDigest(s.Args) {
			out = append(out, mk(name, SevLow,
				"runs a container image without a pinned digest",
				"Pin the image by @sha256:… so the server can't silently change."))
		}
		out = append(out, secretFindings(name, "env", s.Env)...)
	}

	// --- remote servers: url + headers -------------------------------------
	if s.URL != "" {
		if strings.HasPrefix(strings.ToLower(s.URL), "http://") {
			out = append(out, mk(name, SevMedium,
				"connects over plain HTTP (no TLS)",
				"Tool calls and any header credentials travel unencrypted. Use https://."))
		}
		out = append(out, secretFindings(name, "headers", s.Headers)...)
	}

	return out
}

// secretFindings flags any credential-looking key whose value is a hard-coded
// secret (not an env reference or placeholder).
func secretFindings(server, where string, m map[string]string) []Finding {
	var out []Finding
	// Deterministic key order.
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if secretKey.MatchString(k) && !looksLikePlaceholder(m[k]) {
			out = append(out, mk(server, SevMedium,
				"a secret is hard-coded in "+where+" ("+k+")",
				"Reference it from the environment (${"+k+"}) instead of storing the value in the MCP config."))
		}
	}
	return out
}

func argsPinned(args []string) bool {
	return pinnedArg.MatchString(strings.Join(args, " "))
}

func argsHaveDigest(args []string) bool {
	return strings.Contains(strings.Join(args, " "), "@sha256:")
}

func lastPathElement(cmd string) string {
	if i := strings.LastIndexAny(cmd, "/\\"); i >= 0 {
		return cmd[i+1:]
	}
	return cmd
}

func sortFindings(fs []Finding) {
	sort.SliceStable(fs, func(a, b int) bool {
		if fs[a].Severity != fs[b].Severity {
			return fs[a].Severity > fs[b].Severity
		}
		return fs[a].Server < fs[b].Server
	})
}
