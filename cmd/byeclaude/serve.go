package main

import (
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/IamAngusU/ByeClaude/internal/demo"
)

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	listen := fs.String("listen", "127.0.0.1:8080", "listen address; keep loopback when using a reverse proxy")
	maxInFlight := fs.Int("max-inflight", 2, "maximum concurrent repository audits (1-32)")
	timeout := fs.Duration("timeout", 60*time.Second, "timeout for one repository audit")
	rulesFile := rulesFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	matcher, err := resolveMatcher(*rulesFile)
	if err != nil {
		return err
	}
	app, err := demo.New(matcher, *maxInFlight, *timeout)
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              *listen,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      *timeout + 10*time.Second,
		IdleTimeout:       60 * time.Second,
	}
	fmt.Printf("ByeClaude demo listening on http://%s\n", *listen)
	fmt.Println("public GitHub repositories only · read-only scan/plan")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
