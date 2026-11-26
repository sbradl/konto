#!/usr/bin/env bash
# Runs `gremlins unleash "$@"`. gremlins scores a run with no mutant at all as 0% efficacy and fails it; a
# scope with nothing to mutate (a diff touching no mutable line, a package of plain wiring) has nothing to gate,
# so that run passes. Any run with a killed, surviving or timed-out mutant keeps gremlins' own verdict.
set -uo pipefail
out=$(gremlins unleash "$@" 2>&1)
status=$?
printf '%s\n' "$out"
if [ "$status" -ne 0 ] && grep -q 'Killed: 0, Lived: 0,' <<<"$out" && grep -q 'Timed out: 0,' <<<"$out"; then
  echo "unleash: no mutant in scope, nothing to gate"
  exit 0
fi
exit "$status"
