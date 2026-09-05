#!/usr/bin/env bash

# Copyright (c) 2024-2026 chaunsin
# SPDX-License-Identifier: MIT

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TASK_SCRIPT="$SCRIPT_DIR/../qinglong_ncmctl_task.sh"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/ncmctl_task_test.XXXXXX")"
ORIGINAL_PATH="$PATH"
FAKE_BIN="$TEST_ROOT/bin"
COMMAND_LOG="$TEST_ROOT/commands.log"
PASS_COUNT=0

cleanup_test() {
  rm -rf -- "$TEST_ROOT"
}
trap cleanup_test EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_equal() {
  local expected="$1"
  local actual="$2"

  [[ "$actual" == "$expected" ]] || fail "expected <$expected>, got <$actual>"
}

create_fake_ncmctl() {
  mkdir -p "$FAKE_BIN"
  cat > "$FAKE_BIN/ncmctl" <<'EOF'
#!/usr/bin/env bash
set -u
printf '%s\n' "$*" >> "$NCMCTL_TASK_TEST_LOG"
EOF
  chmod 0755 "$FAKE_BIN/ncmctl"
}

test_new_tasks_are_disabled_by_default() {
  : > "$COMMAND_LOG"

  (
    export PATH="$FAKE_BIN:$ORIGINAL_PATH"
    export NCMCTL_TASK_TEST_LOG="$COMMAND_LOG"
    unset NCMCTL_QINGLONG_SIGN
    unset NCMCTL_QINGLONG_SIGN_AUTOMATIC
    unset NCMCTL_QINGLONG_SCROBBLE
    unset NCMCTL_QINGLONG_PARTNER
    unset NCMCTL_QINGLONG_SHARE
    unset NCMCTL_QINGLONG_FANSGROUP
    bash "$TASK_SCRIPT"
  ) > "$TEST_ROOT/default.log"

  assert_equal $'sign --automatic=false\npartner' "$(cat "$COMMAND_LOG")"
}

test_new_tasks_run_when_enabled() {
  : > "$COMMAND_LOG"

  (
    export PATH="$FAKE_BIN:$ORIGINAL_PATH"
    export NCMCTL_TASK_TEST_LOG="$COMMAND_LOG"
    export NCMCTL_QINGLONG_SIGN=false
    export NCMCTL_QINGLONG_SCROBBLE=false
    export NCMCTL_QINGLONG_PARTNER=false
    export NCMCTL_QINGLONG_SHARE=TRUE
    export NCMCTL_QINGLONG_FANSGROUP=True
    bash "$TASK_SCRIPT"
  ) > "$TEST_ROOT/enabled.log"

  assert_equal $'share\nfansgroup' "$(cat "$COMMAND_LOG")"
}

run_test() {
  local name="$1"
  shift

  "$@"
  PASS_COUNT=$((PASS_COUNT + 1))
  echo "ok $PASS_COUNT - $name"
}

create_fake_ncmctl

run_test "new tasks stay disabled by default" test_new_tasks_are_disabled_by_default
run_test "new tasks run when explicitly enabled" test_new_tasks_run_when_enabled

echo "$PASS_COUNT tests passed"
