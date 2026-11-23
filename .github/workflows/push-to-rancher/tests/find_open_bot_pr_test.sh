#!/usr/bin/env bash
# Tests of find_open_bot_pr (common.sh) with a stub of gh that prints the JSON of "pr list": no network.
#   .github/workflows/push-to-rancher/tests/find_open_bot_pr_test.sh
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$HERE/../common.sh"
failures=0

STUB_DIR=$(mktemp -d)
cat >"$STUB_DIR/gh" <<'STUB'
#!/usr/bin/env bash
# prints the JSON that the test put in $STUB_JSON; "$STUB_FAIL" makes it fail like a gh without access
[ -n "${STUB_FAIL:-}" ] && { echo "HTTP 403" >&2; exit 1; }
echo "$STUB_JSON" > /dev/null
printf '%s\n' "$STUB_JSON"
STUB
chmod +x "$STUB_DIR/gh"
export PATH="$STUB_DIR:$PATH"

check() { # check <name> <expected> <actual>
  if [ "$2" = "$3" ]; then
    echo "ok   - $1"
  else
    echo "FAIL - $1"
    echo "       expected: '$2'"
    echo "       actual:   '$3'"
    failures=$((failures + 1))
  fi
}

BOT='"author":{"login":"app/rancher-pr-actions-write"}'
HUMAN='"author":{"login":"nicholasSUSE"}'

# the newest of several bot PRs of the base branch
export STUB_JSON='[
 {"number":58827,"headRefName":"bot/rancher-assets-v2.16-20261116T1115Z-dev-v2.16-1","createdAt":"2026-11-16T11:21:00Z",'"$BOT"'},
 {"number":58986,"headRefName":"bot/rancher-assets-v2.16-20261123T1119Z-dev-v2.16-3","createdAt":"2026-11-23T11:27:00Z",'"$BOT"'},
 {"number":58868,"headRefName":"bot/rancher-assets-v2.16-20261118T1113Z-dev-v2.16-2","createdAt":"2026-11-18T11:19:00Z",'"$BOT"'}]'
unset APP_USER
check "the newest bot PR is the one found" "58986 bot/rancher-assets-v2.16-20261123T1119Z-dev-v2.16-3 2026-11-23T11:27:00Z" "$(find_open_bot_pr release/v2.16)"

# a PR that is not the bot's, whatever its date
export STUB_JSON='[
 {"number":58880,"headRefName":"test/charts-staging-main","createdAt":"2026-11-23T12:00:00Z",'"$HUMAN"'},
 {"number":58985,"headRefName":"bot/rancher-assets-v2.16-20261123T1119Z-dev-main-3","createdAt":"2026-11-23T11:27:00Z",'"$BOT"'}]'
check "a branch that is not the bot's is ignored, even if newer" "58985 bot/rancher-assets-v2.16-20261123T1119Z-dev-main-3 2026-11-23T11:27:00Z" "$(find_open_bot_pr main)"

# the author, when the app is known
export STUB_JSON='[
 {"number":1,"headRefName":"bot/rancher-assets-v2.16-x-main-1","createdAt":"2026-11-23T12:00:00Z",'"$HUMAN"'},
 {"number":2,"headRefName":"bot/rancher-assets-v2.16-y-main-2","createdAt":"2026-11-22T12:00:00Z",'"$BOT"'}]'
check "without the app, the branch name is enough" "1 bot/rancher-assets-v2.16-x-main-1 2026-11-23T12:00:00Z" "$(unset APP_USER; find_open_bot_pr main)"
check "with the app, a PR of another author is ignored" "2 bot/rancher-assets-v2.16-y-main-2 2026-11-22T12:00:00Z" "$(APP_USER='rancher-pr-actions-write[bot]' find_open_bot_pr main)"

# nothing to find
export STUB_JSON='[]'
check "no open PR: nothing is printed" "" "$(find_open_bot_pr main)"
export STUB_JSON='[{"number":9,"headRefName":"feature/other","createdAt":"2026-11-23T12:00:00Z",'"$HUMAN"'}]'
check "only PRs that are not the bot's: nothing is printed" "" "$(find_open_bot_pr main)"
# create-prs.sh runs with set -e: finding nothing must not be a failure, and must print nothing on stderr either
export STUB_JSON='[]'
find_open_bot_pr main >/dev/null 2>"$STUB_DIR/err"; status=$?
check "no open PR: the helper does not fail" 0 "$status"
check "no open PR: nothing on stderr" "" "$(cat "$STUB_DIR/err")"
export STUB_JSON='[{"number":9,"headRefName":"feature/other","createdAt":"2026-11-23T12:00:00Z",'"$HUMAN"'}]'
find_open_bot_pr main >/dev/null 2>"$STUB_DIR/err"; status=$?
check "only other PRs: the helper does not fail" 0 "$status"

# gh cannot list: not an error, the caller opens a new PR
export STUB_FAIL=1
check "gh fails: nothing is printed" "" "$(find_open_bot_pr main 2>/dev/null)"
find_open_bot_pr main >/dev/null 2>&1
check "gh fails: the helper does not fail" 0 "$?"
unset STUB_FAIL

rm -rf "$STUB_DIR"
if [ "$failures" -ne 0 ]; then
  echo "$failures check(s) failed"
  exit 1
fi
echo "all checks passed"
