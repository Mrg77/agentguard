package interpose

import (
	"encoding/json"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestArgsToTargetSerializesArguments(t *testing.T) {
	req := &sdk.CallToolRequest{Params: &sdk.CallToolParamsRaw{
		Name:      "write_file",
		Arguments: json.RawMessage(`{"path":"/etc/passwd","content":"x"}`),
	}}
	got := argsToTarget(req)
	// The target must contain the argument values so a policy can match on them.
	if !strings.Contains(got, "/etc/passwd") {
		t.Errorf("target should include the path argument, got %q", got)
	}
}

func TestArgsToTargetHandlesNil(t *testing.T) {
	if got := argsToTarget(nil); got != "" {
		t.Errorf("nil request should yield empty target, got %q", got)
	}
	empty := &sdk.CallToolRequest{Params: &sdk.CallToolParamsRaw{Name: "x"}}
	if got := argsToTarget(empty); got != "" {
		t.Errorf("no arguments should yield empty target, got %q", got)
	}
}
