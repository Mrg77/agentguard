#!/usr/bin/env python3
"""A tiny stand-in for an AI agent, for the demo and for manual testing.

It speaks MCP over stdio to `agentguard proxy` exactly as a real client (Claude
Code, Cursor) would: initialize, then call the `guard` tool before each action
it "wants" to take. It performs nothing — it just asks the guard and prints the
verdict, which is the whole point: a well-behaved agent asks first.

Usage:
    agentguard proxy | ...   # not like this; the proxy reads stdin
    python3 demo/agent-sim.py            # spawns `agentguard proxy` itself
    python3 demo/agent-sim.py /path/to/agentguard
"""
import json
import subprocess
import sys

BIN = sys.argv[1] if len(sys.argv) > 1 else "agentguard"

# The actions our "agent" considers taking, in order.
ACTIONS = [
    ("shell", "kubectl delete namespace payments", "prod"),
    ("kubectl", "kubectl scale deploy api --replicas=0", "prod"),
    ("kubectl", "kubectl get pods -A", "prod"),
]

SYMBOL = {"deny": "\033[38;5;203m✗ DENY\033[0m",
          "ask": "\033[38;5;214m? ASK \033[0m",
          "allow": "\033[38;5;42m✓ ALLOW\033[0m"}


def main():
    p = subprocess.Popen([BIN, "proxy"], stdin=subprocess.PIPE,
                         stdout=subprocess.PIPE, text=True)

    def send(obj):
        p.stdin.write(json.dumps(obj) + "\n")
        p.stdin.flush()

    def recv():
        while True:
            line = p.stdout.readline()
            if not line:
                sys.exit("proxy closed the connection")
            line = line.strip()
            if line:
                return json.loads(line)

    # MCP handshake.
    send({"jsonrpc": "2.0", "id": 1, "method": "initialize",
          "params": {"protocolVersion": "2024-11-05", "capabilities": {},
                     "clientInfo": {"name": "agent-sim", "version": "0"}}})
    recv()
    send({"jsonrpc": "2.0", "method": "notifications/initialized"})

    print("agent connected to agentguard — asking the guard before each action:\n")
    for i, (tool, target, ctx) in enumerate(ACTIONS, start=2):
        send({"jsonrpc": "2.0", "id": i, "method": "tools/call",
              "params": {"name": "guard",
                         "arguments": {"tool": tool, "target": target, "context": ctx}}})
        sc = recv()["result"]["structuredContent"]
        badge = SYMBOL.get(sc["decision"], sc["decision"])
        print(f"  {badge}  {target}  \033[38;5;240m[{ctx}]\033[0m")

    print("\n(the agent performed NONE of these — it asked, and the guard decided)")
    p.stdin.close()
    p.wait(timeout=5)


if __name__ == "__main__":
    main()
