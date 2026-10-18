---
name: create-pr
description: Push the current task branch and open or update its pull request. Use when the task's work is committed and ready for review.
---
# Create PR

Done means committed, pushed, a PR open, and the PR URL in the task-note copy's `pr_url`.

1. Check for an existing PR:
   ```bash
   gh pr view --json url,state,isDraft
   ```
2. Commit anything still uncommitted on the task branch, then push:
   ```bash
   git push -u origin "$(git branch --show-current)"
   ```
3. If no PR exists, create one with a title from the task and a body listing what changed and how it was verified:
   ```bash
   gh pr create --title "<task title>" --body "<what changed; how verified>"
   ```
   If one exists, push updates; run `gh pr ready` when the work is complete.
4. Verify and record:
   ```bash
   gh pr view --json url,state
   ```
   Set `pr_url` and `status: done` in the task-note copy, and append a Work Log line. Do not commit the note copy.

A failed push or PR creation is a blocker: set `status: blocked` and put the exact error in the Work Log.
