// Package progress carries optional, synchronous progress observations. It
// cannot change an operation's result and stores no global state.
package progress

import "context"

type Event struct {
	Stage       string
	Done, Total int
}

type key struct{}

func WithReporter(ctx context.Context, report func(Event)) context.Context {
	return context.WithValue(ctx, key{}, report)
}

func Report(ctx context.Context, stage string, done, total int) {
	if report, ok := ctx.Value(key{}).(func(Event)); ok && report != nil {
		report(Event{Stage: stage, Done: done, Total: total})
	}
}
