#!/bin/sh
# Run any command with Bitwarden Secrets Manager values injected as env.
# Usage: BWS_PROJECT_ID=<id> sh scripts/bws-run.sh go run ./cmd/server
# Requires: bws CLI + BWS_ACCESS_TOKEN exported (or `bws config` profile).
# Secret names in the project must equal env names (SECTORS_API_KEY, ...).
# Without BWS_PROJECT_ID this just execs the command (plain .env fallback).
set -e
if [ -z "$BWS_PROJECT_ID" ]; then
  exec "$@"
fi
if ! command -v bws >/dev/null 2>&1; then
  echo "bws-run: bws CLI not found; running without injected secrets" >&2
  exec "$@"
fi
exec bws run --project-id "$BWS_PROJECT_ID" -- "$@"
