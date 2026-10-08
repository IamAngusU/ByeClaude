package progress

import (
	"context"
	"sync"
	"testing"
)

func TestReportersStayScopedToTheirOperation(t *testing.T) {
	var a, b []Event
	first := WithReporter(context.Background(), func(e Event) { a = append(a, e) })
	second := WithReporter(context.Background(), func(e Event) { b = append(b, e) })
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		for i := 0; i <= 10; i++ {
			Report(first, "first", i, 10)
		}
	}()
	go func() {
		defer workers.Done()
		for i := 0; i <= 20; i++ {
			Report(second, "second", i, 20)
		}
	}()
	workers.Wait()
	if len(a) != 11 || len(b) != 21 || a[10].Stage != "first" || b[20].Stage != "second" {
		t.Fatal("crossed operation streams", a, b)
	}
	Report(context.Background(), "unobserved", 0, 0)
	Report(WithReporter(first, nil), "disabled", 0, 0)
	if len(a) != 11 {
		t.Fatal("nil override fell back to parent")
	}
}
