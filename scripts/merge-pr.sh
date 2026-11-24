#!/bin/bash
# Merge a pull request of this repository with a merge commit, after checking that it is one the developer may merge.
#   make merge PR=<number>
# The checks are the rules the user set (2026-11-23): the PR is open, not a draft, into main, CLEAN (every required check passed),
# and none of the exceptions the user keeps for themself holds: a release PR (`tagpr-from-*`), a PR whose body says the user merges,
# a PR that changes .claude/settings*, .claude/prompts/, .github/ghmint/, a ruleset file, or this script
# (the guard itself). Anything else is refused with the reason, and nothing is merged.
# A `Zoo-Loosened:` line (2026-11-24, the user chose "B"): a PR whose commits (origin/main..the head) have such lines is let through
# only if EVERY line has the form `Zoo-Loosened: <reason>。ユーザーの承認: YYYY-MM-DD` with a real date that is not in the future, and
# scripts/zoo-loosened-check.sh passes for the range (as well as CI). One line that says "まだ" or has another form refuses it.
# The approval stays the user's: the date is written by the developer only when the advisor relayed the user's approval;
# never write a date that was not relayed.
# Also: every check of the PR must be finished and not failed (`gh pr checks --json name,bucket`: each bucket is pass or skipping;
# pending, fail, cancel stop it), because CLEAN alone let a PR through while `ci-ok` was still pending (#435). The check `ci-ok` must be
# there and pass: with no `ci-ok` (or no check at all, or gh cannot be asked) nothing shows that CI ran, so that refuses too.
# Also: the latest finished `ci.yml` run on main (event push) must not have failed (PRs run -short, so the heavy tests run only on main).
# A run still in progress is skipped (the latest finished one counts). If gh cannot be asked, that refuses too.
# A PR that fixes main's CI is let through with RIVUS_MERGE_FIXES_MAIN=1; the output then says so, with the run URL.
set -euo pipefail
n="${1:?usage: scripts/merge-pr.sh <pull request number>}"
case "$n" in ''|*[!0-9]*) echo "merge: the argument must be a pull request number" >&2; exit 2;; esac
repo=$(gh repo view --json nameWithOwner -q .nameWithOwner)
json=$(gh pr view "$n" --repo "$repo" --json state,isDraft,baseRefName,headRefName,headRefOid,mergeStateStatus,body,files,commits)
bad=$(printf '%s' "$json" | python3 -c '
import sys,json,re,datetime
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
ok=re.compile(r"^Zoo-Loosened: .+ユーザーの承認: (\d{4})-(\d{2})-(\d{2})\s*$")
for c in d.get("commits",[]):
    for l in ((c.get("messageHeadline") or "")+"\n"+(c.get("messageBody") or "")).splitlines():
        if "Zoo-Loosened:" not in l: continue
        m=ok.match(l)
        try:
            day=datetime.date(int(m.group(1)),int(m.group(2)),int(m.group(3)))
            if day>datetime.date.today(): raise ValueError
        except Exception:
            bad.append("Zoo-Loosened line without a valid approval date: "+l.strip()[:80])
print("; ".join(bad))
')
if [ -n "$bad" ]; then echo "merge: PR #$n is not merged by the developer: $bad" >&2; exit 1; fi
zoo=$(printf '%s' "$json" | python3 -c '
import sys,json
d=json.load(sys.stdin)
print(sum(("Zoo-Loosened:" in (c.get("messageHeadline") or "")+"\n"+(c.get("messageBody") or "")) for c in d.get("commits",[])))
')
if [ "$zoo" != 0 ]; then
  headref=$(printf '%s' "$json" | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d["headRefName"]); print(d["headRefOid"])')
  zref=$(printf '%s' "$headref" | sed -n 1p); zoid=$(printf '%s' "$headref" | sed -n 2p)
  if ! git fetch -q origin main "$zref" 2>/dev/null || ! zout=$(bash "${BASH_SOURCE[0]%/*}/zoo-loosened-check.sh" origin/main "$zoid" 2>&1); then
    echo "merge: PR #$n is not merged by the developer: scripts/zoo-loosened-check.sh does not pass for the range: ${zout:-cannot fetch}" >&2; exit 1
  fi
fi
checks=$(gh pr checks "$n" --repo "$repo" --json name,bucket 2>/dev/null) || true  # exits non-zero for pending / failed checks; the JSON on stdout is what counts
badchecks=$(printf '%s' "$checks" | python3 -c '
import sys,json
try:
    cs=json.load(sys.stdin)
    assert isinstance(cs,list)
except Exception:
    print("cannot read the checks of the PR"); sys.exit()
notdone=["%s (%s)"%(c.get("name"),c.get("bucket")) for c in cs if c.get("bucket") not in ("pass","skipping")]
if notdone: print("checks not finished or failed: "+", ".join(notdone)); sys.exit()
if not any(c.get("name")=="ci-ok" and c.get("bucket")=="pass" for c in cs): print("no passing ci-ok check")
' 2>/dev/null) || badchecks="cannot read the checks of the PR"
if [ -n "$badchecks" ]; then echo "merge: PR #$n is not merged by the developer: $badchecks. Nothing is merged." >&2; exit 1; fi
fixes="${RIVUS_MERGE_FIXES_MAIN:-}"
runs=$(gh run list --repo "$repo" --workflow ci.yml --branch main --event push --limit 20 --json status,conclusion,headSha,url 2>/dev/null) || runs=""
ci=$(printf '%s' "$runs" | python3 -c '
import sys,json
try:
    runs=json.load(sys.stdin)
    assert isinstance(runs,list)
except Exception:
    print("ERROR\tcannot read the ci.yml runs on main"); sys.exit()
for r in runs:  # newest first
    c=r.get("conclusion") or ""
    if r.get("status")!="completed" or not c or c=="cancelled": continue
    if c in ("success","neutral","skipped"): print("OK\t"+r["headSha"]+"\t"+r["url"])
    else: print("FAIL\t"+r["headSha"]+"\t"+r["url"]+"\t"+c)
    sys.exit()
print("ERROR\tno finished ci.yml run on main")
' 2>/dev/null) || ci="ERROR	cannot read the ci.yml runs on main"
[ -n "$ci" ] || ci="ERROR	cannot read the ci.yml runs on main"
ci_state="${ci%%	*}"
ci_rest="${ci#*	}"
case "$ci_state" in
OK) ;;
FAIL)
  url=$(printf '%s' "$ci_rest" | cut -f2); sha=$(printf '%s' "$ci_rest" | cut -f1)
  if [ "$fixes" = 1 ]; then
    echo "merge: main の CI が失敗しているが、RIVUS_MERGE_FIXES_MAIN=1 で通す: $url (commit $sha)"
  else
    echo "merge: main's latest finished ci.yml run failed: $url (commit $sha). Fix main first, or for a PR that fixes it set RIVUS_MERGE_FIXES_MAIN=1. Nothing is merged." >&2; exit 1
  fi;;
*)
  if [ "$fixes" = 1 ]; then
    echo "merge: main の CI を確認できない($ci_rest)が、RIVUS_MERGE_FIXES_MAIN=1 で通す"
  else
    echo "merge: cannot check main's CI ($ci_rest). Nothing is merged." >&2; exit 1
  fi;;
esac
echo "merge: PR #$n of $repo: open, not draft, CLEAN, all checks finished, into main, no exception: merging with a merge commit"
gh pr merge "$n" --repo "$repo" --merge
