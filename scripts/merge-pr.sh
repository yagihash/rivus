#!/bin/bash
# Merge a pull request of this repository with a merge commit, after checking that it is one the developer may merge.
#   make merge PR=<number>
# The checks are the rules the user set (2026-11-23): the PR is open, not a draft, into main, CLEAN (every required check passed),
# and none of the exceptions the user keeps for themself holds: a release PR (`tagpr-from-*`), a PR whose body says the user merges,
# a PR that changes .claude/settings*, .claude/prompts/, .github/ghmint/, a ruleset file, or this script
# (the guard itself), or a PR with a `Zoo-Loosened:` line. Anything else is refused with the reason, and nothing is merged.
set -euo pipefail
n="${1:?usage: scripts/merge-pr.sh <pull request number>}"
case "$n" in ''|*[!0-9]*) echo "merge: the argument must be a pull request number" >&2; exit 2;; esac
repo=$(gh repo view --json nameWithOwner -q .nameWithOwner)
json=$(gh pr view "$n" --repo "$repo" --json state,isDraft,baseRefName,headRefName,mergeStateStatus,body,files,commits)
bad=$(printf '%s' "$json" | python3 -c '
import sys,json,re
d=json.load(sys.stdin)
bad=[]
if d["state"]!="OPEN": bad.append("not open: "+d["state"])
if d["isDraft"]: bad.append("draft")
if d["baseRefName"]!="main": bad.append("base is not main")
if d["mergeStateStatus"]!="CLEAN": bad.append("not CLEAN: "+d["mergeStateStatus"])
if d["headRefName"].startswith("tagpr-from-"): bad.append("release PR (the user merges it)")
if "ユーザーがマージする" in (d.get("body") or ""): bad.append("the PR says the user merges")
for f in d.get("files",[]):
    p=f["path"]
    if re.match(r"\.claude/(settings|prompts/)",p) or p.startswith(".github/ghmint/") or "ruleset" in p.lower() or p in ("scripts/merge-pr.sh",):
        bad.append("exception file: "+p)
for c in d.get("commits",[]):
    if "Zoo-Loosened:" in (c.get("messageBody") or ""): bad.append("Zoo-Loosened line")
print("; ".join(bad))
')
if [ -n "$bad" ]; then echo "merge: PR #$n is not merged by the developer: $bad" >&2; exit 1; fi
echo "merge: PR #$n of $repo: open, not draft, CLEAN, into main, no exception: merging with a merge commit"
gh pr merge "$n" --repo "$repo" --merge
