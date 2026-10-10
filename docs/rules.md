# Multiple attribution rules

ByeClaude defaults to one built-in rule: Claude + the `anthropic.com` email domain.

## Saved blacklist

Use the terminal menu (`byeclaude` or `byeclaude tui`) or these commands inside
the repository you want to protect:

```sh
byeclaude blacklist list
byeclaude blacklist add --id helper --email helper@example.org
byeclaude blacklist add --id internal-bot --name "Build Helper" --domain example.org
byeclaude blacklist test --name Helper --email helper@example.org
byeclaude scan --include-identities
byeclaude blacklist remove --id helper
byeclaude blacklist reset
```

Each `add` keeps existing entries, including Claude. Repeat `--email`, `--name`
or `--domain` for alternatives within one rule. Prefer exact emails observed
in your history. A name-only or domain-only rule has a broader scope: inspect
`plan` before rewriting. `remove` requires the exact rule ID and refuses to
remove the final rule; add a replacement first. `reset` restores only Claude.

The policy is stored under `byeclaude.blacklist` in **local Git config**, outside
tracked files. Git locks and replaces the configuration when saving. Cloning a
repository does not import another person's cleanup policy. Linked worktrees
share it and policy edits require `--shared-worktrees` there.

`scan`, `check`, `plan`, `clean`, `setup`, `push`, default Git hooks, and
`verify` with autodetected origin use the saved blacklist. `--rules FILE`
explicitly replaces that selection. Hooks installed with an explicit rules
file keep that file; remove those managed hooks and rerun default setup before
switching to a saved blacklist. Invalid saved rules stop the operation; they
never silently fall back to Claude.

Batch scans, the demo server, server-side ruleset commands and `verify --repo
OWNER/REPO` use their explicit rule file or the Claude default. To share the
same policy with those commands or CI, export it:

```sh
byeclaude blacklist export > rules.json
byeclaude batch scan --owner YOUR_NAME --public --rules rules.json
```

In Windows PowerShell 5.1, use `byeclaude blacklist export | Set-Content
-Encoding UTF8 rules.json`; the loader accepts a UTF-8 BOM. PowerShell 7 and
cmd redirection produce compatible UTF-8 output directly.

## Explicit rules files

For audits that need several declared AI co-authors or internal bots, pass a structured JSON rule file:

```sh
byeclaude scan --rules ./rules.json
byeclaude batch scan --owner IamAngusU --public --rules ./rules.json
```

The same rule file can be used for rewriting one reviewed repository:

```sh
byeclaude clean --rules ./rules.json --apply
byeclaude push --rules ./rules.json --backup BACKUP_ID
```

And for future local commits:

```sh
byeclaude hook install --rules ./rules.json
```

The hook stores the absolute path to the validated rules file. If that file later disappears or becomes invalid, the commit hook fails closed instead of silently stopping enforcement.

## Schema

```json
{
  "rules": [
    {
      "id": "provider-or-policy-id",
      "name_contains": ["display name fragment"],
      "email_domains": ["example.com"],
      "exact_emails": ["bot@example.com"],
      "message_lines": ["Generated with Example Tool"],
      "trailer_keys": ["Example-Session"]
    }
  ]
}
```

Within one rule:

- multiple `name_contains` values are OR conditions;
- `email_domains` and `exact_emails` are alternative allowed email conditions;
- if both a name constraint and an email constraint are present, both sides must match;
- email-domain matching requires a real `@domain` boundary;
- `message_lines` are exact, case-sensitive lines at the end of the message or immediately before its final trailer block;
- `trailer_keys` match a complete Git trailer key, case-insensitively, and also remove its folded continuation lines;
- rule IDs must be unique, 1-64 letters/digits/dots/underscores/hyphens, starting with a letter or digit;
- empty constraints, invalid emails/domains, unknown JSON fields and files larger than 1 MiB are rejected.

No arbitrary regular expressions are executed.

## Example provider set

See [`examples/rules/multi-ai.example.json`](../examples/rules/multi-ai.example.json).

The example includes conservative rules for Claude, Codex and a synthetic internal bot. Treat it as an editable starting point, not a permanent provider registry: attribution formats and opt-in settings can change over time.

Without a saved blacklist or explicit rule file, ByeClaude uses the narrower built-in Claude rule.

## Scope

These rules classify **`Co-Authored-By`** trailers, explicitly listed message lines and trailer keys, and Git author/committer identities. The built-in Claude rule includes exact historical `Generated with Claude Code` variants and `Claude-Session`; it does not use fuzzy prose matching. `scan` and `check` include actual identities when `--include-identities` is selected; the menu audit and pre-push guard include them. Cleanup removes matching message metadata by default. Actual author/committer replacement always requires [explicit role selection](identity-correction.md). Unlisted metadata, PR descriptions, editor telemetry and source-code style are different evidence surfaces and are not silently treated as attribution.
