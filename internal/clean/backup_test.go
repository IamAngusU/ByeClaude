package clean

import (
	"regexp"
	"testing"
)

func TestBackupIDIsUniqueAndRefSafe(t *testing.T) {
	pattern := regexp.MustCompile(`^[0-9]{8}T[0-9]{6}Z-[0-9a-f]{12}$`)
	seen := make(map[string]struct{}, 128)
	for i := 0; i < 128; i++ {
		id, err := backupID()
		if err != nil {
			t.Fatal(err)
		}
		if !pattern.MatchString(id) {
			t.Fatalf("backup ID %q does not match expected ref-safe format", id)
		}
		if _, exists := seen[id]; exists {
			t.Fatalf("duplicate backup ID %q", id)
		}
		seen[id] = struct{}{}
	}
}
