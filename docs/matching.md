# Matching architecture

ByeClaude's built-in behavior is intentionally narrow: it removes selected
Claude/Anthropic commit-message metadata. Saved blacklist entries and
structured rule files can select several other exact identities without
changing the Git history engine.

## Separation

The code is split into three layers:

```text
Git commit / ref rewrite
        ↑
generic message-metadata filtering
        ↑
identity matcher
        ↑
ByeClaude Claude/Anthropic preset
```

The rewrite layer only receives an `attribution.Matcher`. It does not know the words `Claude` or `Anthropic`.

The built-in product preset lives separately and currently requires:

- a `Co-Authored-By` display name containing `Claude`, case-insensitively;
- an email whose real domain is `anthropic.com`.
- one of four exact historical `Generated with Claude Code` end lines; or
- the exact `Claude-Session` trailer key.

A body-text example is still ignored because only the final attribution suffix
is considered. The parser also handles the exact `---------` separator GitHub
can insert between an original generated footer and its final co-author block,
but only when the final block already contains selected attribution. A
source-linked regression corpus lives in
`internal/clean/testdata/claude-message-corpus.json`.

## Generic rule

Internally, an attribution rule can match by:

- one or more case-insensitive name fragments;
- one or more exact email addresses;
- one or more email domains with a real `@` boundary.

For example, a different tool could use the same rewrite engine with a rule equivalent to:

```go
attribution.Rule{
    RuleID:       "example-bot",
    NameContains: []string{"build bot"},
    EmailDomains: []string{"example.dev"},
}
```

That custom rule is covered by integration tests: the generic engine can scan and rewrite a non-Claude bot without changing the ByeClaude preset.

## Why this is not a CLI regex flag

ByeClaude is safer when its default command has one obvious meaning.

An unrestricted pattern flag would make it easy to rewrite shared Git history because of a typo or overly broad regular expression. The current alpha uses validated saved blacklist entries and structured JSON rules instead; both are previewed through the same plan before a rewrite.

Use `byeclaude blacklist` for several exact tool identities, or `--rules` for a reviewed reusable policy. Neither accepts an opaque one-line regex.

## Adding another preset

A new preset should not require changes to:

- commit parsing;
- DAG traversal;
- parent remapping;
- tag rewriting;
- backup/result snapshots;
- restore;
- force-with-lease remote publication.

It should be a small identity rule plus tests that prove exactly what it matches and what it does not.

Provider presets should be added only with source-linked examples of the exact
metadata the provider emits, positive and negative corpus cases, and trademark
notes where needed. A vendor name appearing in documentation is not enough to
ship a rule that can rewrite history.
