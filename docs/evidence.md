# Evidence model

ByeClaude can tell you what Git history **declares**. It cannot prove who wrote every line of code.

That distinction matters if the JSON output is later exposed through a website or API.

## Evidence class: declared attribution

Today ByeClaude reads a final Git commit trailer such as:

```text
Co-Authored-By: Claude <noreply@anthropic.com>
```

and matches the parsed name/email against structured rules.

That supports statements such as:

- "17 of 420 reachable commits contain a declared AI co-author matching these rules";
- "4.05% of scanned commits have declared matching attribution";
- "12 repositories out of 80 contain at least one declared matching co-author";
- "the matching trailers were classified by rules X, Y and Z."

It does **not** support the stronger statement:

> 4.05% of the code was written by AI.

A commit can contain one AI-assisted line or an entire generated feature. A trailer does not encode that quantity.

## Fields useful for an API

Single-repository JSON reports include:

- total reachable commits scanned;
- unique commits containing matching attribution;
- matched-commit percentage;
- each matching trailer;
- the commit's normal author;
- the declared co-author name/email;
- the rule IDs that matched that identity;
- per-rule trailer counts;
- scan duration.

Batch reports additionally include:

- repositories scanned / clean / matched / failed;
- percentage of successfully scanned repositories containing matches;
- aggregate commit and matched-commit counts;
- aggregate matched-commit percentage;
- aggregate rule counts;
- per-repository prepare/scan/total timing;
- whole-batch wall time.

This output can be consumed by a later HTTP service without making the CLI itself a hosted service.

## Evidence class: heuristic inference

A future "AI slop" or style/code detector should be modeled separately, for example:

```json
{
  "evidence_type": "heuristic",
  "detector": "code-style-v1",
  "confidence": 0.71,
  "scope": "commit",
  "commit": "..."
}
```

Heuristic inference can be useful for triage, but should never silently upgrade itself into declared authorship.

A useful service can combine both while preserving provenance:

```text
declared_git_attribution  high-confidence factual metadata
tool_metadata             factual when the metadata source is understood
heuristic_code_signal     probabilistic
text/style signal         probabilistic
```

The UI can then say **"AI involvement signals"** rather than claiming a percentage of code authorship that Git metadata cannot establish.
