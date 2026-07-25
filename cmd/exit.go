package cmd

import "os"

// exitError carries a specific process exit code out of a cobra RunE. Execute
// prints the message and exits 1 for a plain error; for an exitError it uses
// the carried code (e.g. 2 for a policy denial), so CI can distinguish "the
// tool broke" from "the guard said no".
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

// handleExit is called by Execute after rootCmd returns: it honors an
// exitError's code. Kept separate so root.go stays about wiring.
func handleExit(err error) {
	if err == nil {
		return
	}
	if ee, ok := err.(*exitError); ok {
		if ee.msg != "" {
			os.Stderr.WriteString("agentguard: " + ee.msg + "\n")
		}
		os.Exit(ee.code)
	}
	os.Stderr.WriteString("agentguard: " + err.Error() + "\n")
	os.Exit(1)
}
