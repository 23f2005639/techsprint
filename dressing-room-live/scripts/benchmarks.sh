#!/usr/bin/env bash
source "$(dirname "$0")/env.sh"
cd "$ROOT"
mkdir -p results
for scenario in race five floor duplicate; do
 bin/load -scenario "$scenario" -requests 100 -concurrency 100 > "results/$scenario.json"
done
python3 - <<'PY'
import json,pathlib
summary=[]
for scenario in ('race','five','floor','duplicate'):
 r=json.loads(pathlib.Path('results',scenario+'.json').read_text())
 m=r['metrics'];summary.append({'scenario':scenario,'run':r['run']['id'],'requests':m['completed'],'unique_holds':m['unique_holds'],'successful_http':m['successful'],'conflicts':m['conflicts'],'failures':m['failures'],'rate_limits':m['rate_limits'],'p50_ms':m['p50'],'p95_ms':m['p95'],'p99_ms':m['p99'],'rps':m['rps'],'invariants':{k:r['verified_inventory'][k] for k in ('no_duplicate_allocation','protected_floor','inventory_conserved')}})
pathlib.Path('results/summary.json').write_text(json.dumps(summary,indent=2)+'\n')
for r in summary:print(f"{r['scenario']:10} {r['unique_holds']} unique holds, {r['successful_http']} successful HTTP, {r['conflicts']} conflicts, {r['failures']} failures; p95 {r['p95_ms']:.2f}ms, {r['rps']:.1f} RPS")
PY
