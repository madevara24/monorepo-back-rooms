---
name: resolve-conflict
description: Resolve a rebase conflict on a task branch by intent. Use when a resumed task branch conflicts with main.
---
# Resolve conflict

The runner fetches and checks out the preserved branch; reconciling it with `main` is the session's job.

1. `git fetch origin && git rebase origin/main`.
2. For each conflict, read both sides and the commit messages to recover what each change was for.
3. Keep both behaviors when they are independent. When they conflict, keep the behavior the task requires and note the dropped change in the Work Log.
4. Never discard a change whose intent is unclear. Set `status: blocked` and explain.
5. After the rebase: `make test` and `make build`. Continue with `git rebase --continue`.
