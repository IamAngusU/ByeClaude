# Multiple attribution rules

ByeClaude defaults to one built-in rule: Claude + the `anthropic.com` email domain.

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
      "exact_emails": ["bot@example.com"]
    }
  ]
}
```

Within one rule:

- multiple `name_contains` values are OR conditions;
- `email_domains` and `exact_emails` are alternative allowed email conditions;
- if both a name constraint and an email constraint are present, both sides must match;
- email-domain matching requires a real `@domain` boundary;
- rule IDs must be unique;
- an empty rule is rejected.

No arbitrary regular expressions are executed.

## Example provider set

See [`examples/rules/multi-ai.example.json`](../examples/rules/multi-ai.example.json).

That example contains currently observed attribution shapes for Claude, GitHub Copilot and Cursor. Treat it as an editable starting point, not a permanent registry: providers can change commit attribution formats, and some products expose settings that disable attribution entirely.

The default ByeClaude behavior remains the narrower built-in Claude rule.

## Scope

These rules currently classify and remove **`Co-Authored-By`** trailers. Other metadata such as `Made-with:`, `Assisted-by:`, PR footers, editor telemetry or source-code style is a different evidence surface and is not silently treated as a co-author.
