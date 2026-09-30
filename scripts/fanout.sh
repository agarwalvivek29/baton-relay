#!/bin/sh
# Send requests through each fan-out mode of `orders` and print the request ID
# of each. Paste an ID into Grafana, or run count-traces.py to measure.
#
#   ./scripts/fanout.sh              # one request per mode: sync, go, pool
#   N=20 RUN=r1 ./scripts/fanout.sh  # 20 per mode, IDs <mode>-r1-1..20
#   ./scripts/fanout.sh pool         # only some modes
set -eu
GATEWAY=${GATEWAY:-http://localhost:8080}
N=${N:-1}
RUN=${RUN:-$(date +%H%M%S)}
[ $# -eq 0 ] && set -- sync go pool

for mode in "$@"; do
  i=1
  while [ "$i" -le "$N" ]; do
    id="$mode-$RUN-$i"
    code=$(curl -s -o /dev/null -w '%{http_code}' -H "X-Request-ID: $id" "$GATEWAY/checkout?mode=$mode")
    printf '%-5s %s  HTTP %s\n' "$mode" "$id" "$code"
    i=$((i + 1))
    sleep 0.3
  done
done
