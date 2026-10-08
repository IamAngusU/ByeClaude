//go:build !windows && !linux && !darwin

package metrics

import "fmt"

func tryLock(string) (func(), error) {
	return nil, fmt.Errorf("local metrics locking is unsupported on this platform")
}
