#!/usr/bin/env bash
# Fail when a top-level deployable stack is missing from DEPLOY.md.
# A new stack without a deploy-map row is an undeclared deploy path.
set -euo pipefail

status=0
for dir in */; do
  name="${dir%/}"
  case "$name" in
    .*) continue ;;
  esac

  if [ -f "$name/go.mod" ] || [ -f "$name/package.json" ] || [ -f "$name/Dockerfile" ]; then
    if ! grep -Fq -- "$name" DEPLOY.md; then
      echo "deploy map: missing row for stack '$name' in DEPLOY.md" >&2
      status=1
    fi
  fi
done

if [ "$status" -ne 0 ]; then
  echo "Every top-level stack must be listed in DEPLOY.md. Add its deploy target, auto-deploy status, and path filter." >&2
fi

exit "$status"
