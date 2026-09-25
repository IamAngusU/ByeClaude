package main

import (
	"runtime/debug"
	"testing"
)

func TestBuildBOMIsDeterministicAndIdentifiesRelease(t *testing.T) {
	info := &debug.BuildInfo{
		GoVersion: "go1.27.0",
		Path:      "github.com/IamAngusU/ByeClaude/cmd/byeclaude",
		Deps: []*debug.Module{
			{Path: "example.com/z", Version: "v1.0.0"},
			{Path: "example.com/a", Version: "v2.0.0", Replace: &debug.Module{Path: "example.com/a-fork", Version: "v2.0.1"}},
		},
	}
	doc := buildBOM(info, "v0.1.0-alpha.1", "abc123", 1_700_000_000, "AABB")
	if doc.BOMFormat != "CycloneDX" || doc.SpecVersion != "1.5" {
		t.Fatalf("unexpected document header: %+v", doc)
	}
	if doc.Metadata.Component.Name != "ByeClaude" || doc.Metadata.Component.Version != "v0.1.0-alpha.1" {
		t.Fatalf("unexpected root component: %+v", doc.Metadata.Component)
	}
	if got := doc.Metadata.Component.Hashes[0].Content; got != "aabb" {
		t.Fatalf("digest=%q", got)
	}
	if len(doc.Components) != 2 || doc.Components[0].Name != "example.com/a-fork" || doc.Components[1].Name != "example.com/z" {
		t.Fatalf("components are missing or unsorted: %+v", doc.Components)
	}
	if doc.Metadata.Properties[1].Value != "abc123" {
		t.Fatalf("source commit not recorded: %+v", doc.Metadata.Properties)
	}
}
