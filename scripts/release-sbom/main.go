package main

import (
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path"
	"runtime/debug"
	"sort"
	"strings"
	"time"
)

type bom struct {
	BOMFormat   string      `json:"bomFormat"`
	SpecVersion string      `json:"specVersion"`
	Version     int         `json:"version"`
	Metadata    metadata    `json:"metadata"`
	Components  []component `json:"components,omitempty"`
}

type metadata struct {
	Timestamp  string     `json:"timestamp"`
	Component  component  `json:"component"`
	Properties []property `json:"properties,omitempty"`
}

type component struct {
	Type     string     `json:"type"`
	BOMRef   string     `json:"bom-ref"`
	Group    string     `json:"group,omitempty"`
	Name     string     `json:"name"`
	Version  string     `json:"version"`
	PURL     string     `json:"purl"`
	Hashes   []hash     `json:"hashes,omitempty"`
	Licenses []licenses `json:"licenses,omitempty"`
}

type hash struct {
	Algorithm string `json:"alg"`
	Content   string `json:"content"`
}

type licenses struct {
	License license `json:"license"`
}

type license struct {
	ID string `json:"id"`
}

type property struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func main() {
	binary := flag.String("binary", "", "release binary to inspect")
	version := flag.String("version", "", "v-prefixed release version")
	commit := flag.String("commit", "", "source commit")
	epoch := flag.Int64("epoch", -1, "source commit Unix timestamp")
	output := flag.String("output", "", "target CycloneDX JSON path")
	flag.Parse()
	if *binary == "" || *version == "" || *commit == "" || *epoch < 0 || *output == "" {
		fmt.Fprintln(os.Stderr, "usage: release-sbom --binary FILE --version VERSION --commit SHA --epoch UNIX --output FILE")
		os.Exit(2)
	}
	info, err := buildinfo.ReadFile(*binary)
	if err != nil {
		fatal(err)
	}
	data, err := os.ReadFile(*binary) // #nosec G304 -- explicit release-builder input
	if err != nil {
		fatal(err)
	}
	digest := sha256.Sum256(data)
	doc := buildBOM(info, *version, *commit, *epoch, hex.EncodeToString(digest[:]))
	encoded, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		fatal(err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(*output, encoded, 0644); err != nil { // #nosec G306 -- public release metadata
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func buildBOM(info *debug.BuildInfo, version, commit string, epoch int64, digest string) bom {
	mainPath := info.Path
	if mainPath == "" {
		mainPath = "github.com/IamAngusU/ByeClaude/cmd/byeclaude"
	}
	rootPURL := purl(mainPath, version)
	root := component{
		Type:     "application",
		BOMRef:   rootPURL,
		Group:    "github.com/IamAngusU",
		Name:     "ByeClaude",
		Version:  version,
		PURL:     rootPURL,
		Hashes:   []hash{{Algorithm: "SHA-256", Content: strings.ToLower(digest)}},
		Licenses: []licenses{{License: license{ID: "MIT"}}},
	}
	components := make([]component, 0, len(info.Deps))
	for _, dep := range info.Deps {
		module := dep
		if dep.Replace != nil {
			module = dep.Replace
		}
		moduleVersion := module.Version
		if moduleVersion == "" {
			moduleVersion = "(devel)"
		}
		ref := purl(module.Path, moduleVersion)
		components = append(components, component{
			Type:    "library",
			BOMRef:  ref,
			Name:    module.Path,
			Version: moduleVersion,
			PURL:    ref,
		})
	}
	sort.Slice(components, func(i, j int) bool { return components[i].BOMRef < components[j].BOMRef })
	return bom{
		BOMFormat:   "CycloneDX",
		SpecVersion: "1.5",
		Version:     1,
		Metadata: metadata{
			Timestamp: time.Unix(epoch, 0).UTC().Format(time.RFC3339),
			Component: root,
			Properties: []property{
				{Name: "byeclaude:source:repository", Value: "https://github.com/IamAngusU/ByeClaude"},
				{Name: "byeclaude:source:commit", Value: commit},
				{Name: "byeclaude:build:go-version", Value: info.GoVersion},
			},
		},
		Components: components,
	}
}

func purl(modulePath, version string) string {
	name := path.Clean(modulePath)
	return "pkg:golang/" + url.PathEscape(name) + "@" + url.QueryEscape(version)
}
