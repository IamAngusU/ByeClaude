# Matching architecture

ByeClaude's public behavior is intentionally narrow: it removes Claude/Anthropic `Co-Authored-By` trailers. The Git history engine itself is no longer tied to that identity.

## Separation

The code is split into three layers:

```text
Git commit / ref rewrite
        ↑
generic trailer filtering
        ↑
identity matcher
        ↑
ByeClaude Claude/Anthropic preset
```

The rewrite layer only receives an `attribution.Matcher`. It does not know the words `Claude` or `Anthropic`.

The built-in product preset lives separately and currently requires:

- a `Co-Authored-By` display name containing `Claude`, case-insensitively;
- an email whose real domain is `anthropic.com`.

A body-text example is still ignored because only the final Git trailer block is considered.

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

An unrestricted pattern flag would make it easy to rewrite shared Git history because of a typo or overly broad regular expression. The current alpha keeps the public CLI opinionated while making the internal engine reusable.

If custom matching becomes a public feature later, it should use validated structured rules with a dry-run report, not an opaque one-line regex.

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
