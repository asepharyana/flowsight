#!/bin/sh
# FlowSight offline demo: briefing -> radar alert -> report -> interrogation.
set -e
BASE="${BASE:-http://localhost:8080}"
echo "== health ==";        curl -s "$BASE/api/health" | head -c 300; echo
echo "== briefing ==";      curl -s "$BASE/api/briefing/today" | head -c 300; echo
echo "== radar ==";         curl -s -X POST "$BASE/api/screen" -d '{"limit":3}' | head -c 300; echo
echo "== report ==";        curl -s -X POST "$BASE/api/report/BBCA?profile=moderate" | head -c 300; echo
echo "== interrogation =="; curl -s -X POST "$BASE/api/report/BBCA/ask" -d '{"question":"kenapa conviction segitu?"}' | head -c 300; echo
echo DEMO_OK
