#!/usr/bin/env python3
"""Publish measured CI evidence using the vendored IamAngusU/Badges design."""
import html
import json
import os
from pathlib import Path
import re
import sys
import urllib.request

DESIGN_COMMIT = "775ec50cbc778fc4e4181ba14b98f9fc0beb4e5c"
REPOSITORY = "angusu-de/ByeClaude"
EXPECTED = (
    "Go ubuntu-latest", "Go windows-latest", "Go macos-latest",
    "Race, coverage and fixtures", "Dependency and static security gates",
    "Shell installer", "PowerShell installer", "Reproducible release set and SBOM",
    "Demo container health", "Composite action self-test",
)
TEMPLATE = Path(__file__).resolve().parents[1] / "docs/assets/readme/public-proof-template.svg"


def github_json(path):
    request = urllib.request.Request("https://api.github.com/repos/" + REPOSITORY + "/" + path,
        headers={"Accept": "application/vnd.github+json", "User-Agent": "ByeClaude-CI-proof",
                 "Authorization": "Bearer " + os.environ["GITHUB_TOKEN"],
                 "X-GitHub-Api-Version": "2022-11-28"})
    with urllib.request.urlopen(request, timeout=30) as response:
        return json.load(response)


def load_jobs(run):
    jobs = []
    for page in range(1, 21):
        data = github_json(f"actions/runs/{run['id']}/attempts/{run['run_attempt']}/jobs?per_page=100&page={page}")
        batch = data["jobs"]
        if not isinstance(batch, list) or any(not isinstance(j, dict) for j in batch):
            raise ValueError("Malformed jobs response")
        jobs.extend(batch)
        if len(batch) < 100:
            return jobs
    raise ValueError("CI job pagination exceeded 2000 jobs")


def ordered_jobs(jobs):
    by_name = {}
    for job in jobs:
        if job["name"] in by_name:
            raise ValueError("Duplicate CI job name")
        by_name[job["name"]] = job
    return [by_name.pop(name, {"name": name, "status": "missing", "conclusion": None, "steps": []})
            for name in EXPECTED] + [by_name[name] for name in sorted(by_name)]


def status(job):
    conclusion = job.get("conclusion")
    if job.get("status") == "completed" and conclusion == "success":
        return "success"
    if conclusion in ("failure", "timed_out", "startup_failure", "action_required"):
        return "failure"
    if conclusion in ("cancelled", "stale", "skipped", "neutral"):
        return "warning"
    return "neutral"


def render(run, jobs):
    jobs = ordered_jobs(jobs)
    passed = sum(status(j) == "success" for j in jobs)
    metric = f"{passed}/{len(jobs)}"
    claim = "Separate account, same maintainer; CI evidence, not a third-party audit."
    title = f"ByeClaude CI: {metric} jobs passed; {run['head_sha'][:12]}; run {run['id']}, attempt {run['run_attempt']}. {claim}"
    svg = TEMPLATE.read_text(encoding="utf-8")
    for anchor in ('data-badge-system="IamAngusU/Badges"', 'id="proof-metric"', 'id="proof-segments"'):
        if anchor not in svg:
            raise ValueError("Missing badge-system anchor: " + anchor)
    escaped = html.escape(title, quote=True)
    svg = re.sub(r'aria-label="[^"]*"', lambda _: f'aria-label="{escaped}"', svg, count=1)
    svg = re.sub(r'<title>.*?</title>', lambda _: f'<title>{escaped}</title>', svg, count=1, flags=re.S)
    svg = re.sub(r'(<text id="proof-metric"[^>]*>).*?(</text>)', lambda m: m[1] + metric + m[2], svg, count=1, flags=re.S)
    gap = 8 if len(jobs) <= 20 else 3
    width = (355 - gap * (len(jobs) - 1)) / len(jobs)
    if width < 1:
        raise ValueError("Too many jobs for the badge")
    segments = []
    for i, job in enumerate(jobs):
        x = 101 + i * (width + gap)
        label = html.escape(job['name'] + ': ' + (job.get('conclusion') or job['status']))
        segments.append(f'<line class="proof-{status(job)}" x1="{x:.2f}" y1="63" x2="{x+width:.2f}" y2="63"><title>{label}</title></line>')
    svg = re.sub(r'(<g id="proof-segments">).*?(</g>)', lambda m: m[1] + '\n' + '\n'.join(segments) + '\n' + m[2], svg, count=1, flags=re.S)
    metadata = {"claim": claim, "badge_design_source": "IamAngusU/Badges",
        "badge_design_source_commit": DESIGN_COMMIT, "repository": REPOSITORY,
        "head_sha": run['head_sha'], "run_id": run['id'], "run_attempt": run['run_attempt'],
        "run_url": run['html_url'], "run_conclusion": run.get('conclusion'),
        "jobs_success": passed, "jobs_total": len(jobs),
        "jobs": [{k: j.get(k) for k in ('name', 'status', 'conclusion', 'html_url', 'steps')} for j in jobs]}
    lines = ["# ByeClaude public CI evidence", "", claim, "",
        f"Commit: `{run['head_sha']}` · [Run {run['id']}, attempt {run['run_attempt']}]({run['html_url']})",
        "", f"**{metric} jobs passed.** Overall workflow: `{run.get('conclusion') or run['status']}`.", "",
        "This snapshot describes the linked commit. Check the commit before relying on it for a newer checkout.",
        "", "[Machine-readable evidence](public-proof.json)", ""]
    for job in jobs:
        lines += [f"## {html.escape(job['name'])}", "", f"Result: `{job.get('conclusion') or job['status']}`", ""]
        for step in job.get('steps', []):
            lines.append(f"- {html.escape(step['name'])}: `{step.get('conclusion') or step['status']}`")
        lines.append("")
    return svg, metadata, '\n'.join(lines)


def main():
    if len(sys.argv) != 3 or not sys.argv[1].isdigit():
        raise SystemExit("usage: render-public-proof-badge.py RUN_ID OUTPUT_DIRECTORY")
    run = github_json("actions/runs/" + sys.argv[1])
    if (run['name'] != 'CI' or run['event'] != 'push' or run['head_branch'] != 'main'
            or run['head_repository']['full_name'] != REPOSITORY):
        raise SystemExit("Refusing proof from a different workflow, branch or repository")
    if github_json('git/ref/heads/main')['object']['sha'] != run['head_sha']:
        print("Skipping an older commit; mirror main has advanced.")
        return
    svg, metadata, readme = render(run, load_jobs(run))
    destination = Path(sys.argv[2])
    destination.mkdir(parents=True, exist_ok=True)
    (destination / 'public-proof.svg').write_text(svg, encoding='utf-8')
    (destination / 'public-proof.json').write_text(json.dumps(metadata, indent=2) + '\n', encoding='utf-8')
    (destination / 'README.md').write_text(readme, encoding='utf-8')


if __name__ == '__main__':
    main()
