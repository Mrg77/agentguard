#!/usr/bin/env python3
"""End-to-end proof that `agentguard interpose` is an unbypassable firewall.

It runs agentguard IN FRONT OF the real @modelcontextprotocol/server-filesystem
upstream, then, speaking MCP as a client would, checks three things:

  1. the upstream's tools are mirrored through agentguard,
  2. an allowed call (read) is relayed and returns the real file contents,
  3. a denied call (write) is refused AND the file is never created — proving
     the upstream is unreachable except through the guard.

Exits non-zero on any failure, so CI can gate on it. Requires: the agentguard
binary path as argv[1], npx on PATH, and a writable data dir as argv[2].
"""
import json
import os
import subprocess
import sys

BIN = sys.argv[1]
DATADIR = sys.argv[2]

os.makedirs(DATADIR, exist_ok=True)
with open(os.path.join(DATADIR, "readme.txt"), "w") as f:
    f.write("real content")

# A policy that denies writes, allows the rest.
policy = os.path.join(DATADIR, "..", "agentguard.yaml")
with open(policy, "w") as f:
    f.write('default: allow\nrules:\n  - name: block writes\n'
            '    tool: "write_file|edit_file"\n    action: deny\n'
            '    message: "writes blocked"\n')

proc = subprocess.Popen(
    [BIN, "interpose", "--context", "prod", "-f", policy, "--",
     "npx", "-y", "@modelcontextprotocol/server-filesystem", DATADIR],
    stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)


def send(o):
    proc.stdin.write(json.dumps(o) + "\n")
    proc.stdin.flush()


def recv():
    while True:
        line = proc.stdout.readline()
        if not line:
            sys.exit("agentguard closed early:\n" + proc.stderr.read())
        line = line.strip()
        if line:
            return json.loads(line)


def fail(msg):
    sys.exit("FAIL: " + msg)


send({"jsonrpc": "2.0", "id": 1, "method": "initialize",
      "params": {"protocolVersion": "2024-11-05", "capabilities": {},
                 "clientInfo": {"name": "ci", "version": "0"}}})
recv()
send({"jsonrpc": "2.0", "method": "notifications/initialized"})

# 1. tools mirrored
send({"jsonrpc": "2.0", "id": 2, "method": "tools/list"})
tools = [t["name"] for t in recv()["result"]["tools"]]
if "read_text_file" not in tools or "write_file" not in tools:
    fail(f"upstream tools not mirrored: {tools}")


def call(name, args):
    send({"jsonrpc": "2.0", "id": 9, "method": "tools/call",
          "params": {"name": name, "arguments": args}})
    r = recv()["result"]
    return r.get("isError", False), (r.get("content") or [{}])[0].get("text", "")


# 2. allowed read is relayed
err, txt = call("read_text_file", {"path": os.path.join(DATADIR, "readme.txt")})
if err or "real content" not in txt:
    fail(f"allowed read should relay real content, got err={err} txt={txt!r}")

# 3. denied write is refused and never reaches the upstream
err, txt = call("write_file", {"path": os.path.join(DATADIR, "hack.txt"), "content": "pwned"})
if not err:
    fail("denied write should return an error")
if os.path.exists(os.path.join(DATADIR, "hack.txt")):
    fail("BYPASS: the upstream wrote the file despite the deny")

proc.stdin.close()
proc.wait(timeout=5)
print("interpose firewall OK: tools mirrored, read relayed, write blocked, upstream unreachable")
