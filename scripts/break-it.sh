#!/bin/sh
# Send a few good requests and one that fails deep in the chain (payments),
# then print the failing request's ID: find it in Grafana → Explore → Loki.
#
#   ./scripts/break-it.sh            # uses orders' default FANOUT_MODE
#   MODE=pool ./scripts/break-it.sh  # fail in a specific fan-out mode
set -eu
GATEWAY=${GATEWAY:-http://localhost:8080}
GOOD=${GOOD:-5}
MODE=${MODE:-}
q=${MODE:+"&mode=$MODE"}

id_of() { tr -d '\r' | awk 'tolower($1)=="x-request-id:" {print $2}'; }

i=1
while [ "$i" -le "$GOOD" ]; do
  # fail=0 still lets payments fail at its random FAIL_RATE; that's fine.
  id=$(curl -s -D - -o /dev/null "$GATEWAY/checkout?fail=0$q" | id_of)
  echo "sent      $id"
  i=$((i + 1))
done

id=$(curl -s -D - -o /dev/null "$GATEWAY/checkout?fail=1$q" | id_of)
echo "FAILING   $id"
echo
echo "Grafana:  http://localhost:3000/explore  (Loki: {service=~\".+\"} |= \"$id\")"
echo "Tempo:    { span.\"http.request.header.x-request-id\" = \"$id\" || span.\"http.response.header.x-request-id\" = \"$id\" }"
