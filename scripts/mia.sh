#!/usr/bin/env bash
# Dev helper: sync the local rbstat tree to the Linux test host `mia`,
# build it there, and run it with any passed args against real /proc.
#
#   scripts/mia.sh                 # build + run with no args (default dstat view)
#   scripts/mia.sh -cdn 1 5        # forward args to rbstat
#   BUILD_ONLY=1 scripts/mia.sh    # just compile on mia, don't run
set -euo pipefail

HOST="${MIA_HOST:-mia}"
REMOTE_DIR="${MIA_DIR:-~/rbstat}"
LOCAL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Fast local compile-check (cross to linux) before paying the network round-trip.
( cd "$LOCAL_DIR" && GOOS=linux GOARCH=amd64 go build -o /tmp/rbstat-xcheck . )

rsync -az --delete --exclude '.git' --exclude '/rbstat' "$LOCAL_DIR"/ "$HOST:$REMOTE_DIR"/

if [ -n "${BUILD_ONLY:-}" ]; then
  RUN_CMD="echo 'build OK'"
else
  RUN_CMD="./rbstat $*"
fi

# shellcheck disable=SC2029
ssh "$HOST" "export PATH=\$PATH:/usr/local/go/bin; cd $REMOTE_DIR && go build -o rbstat . && $RUN_CMD"
