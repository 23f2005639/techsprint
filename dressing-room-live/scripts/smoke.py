#!/usr/bin/env python3
"""Read-only HTTP smoke check against the already running local demo."""
import csv,io,json,os,urllib.request
base='http://127.0.0.1:8080'
key=os.environ.get('STAFF_KEY','local-demo-staff')
def get(path,staff=False):
 return urllib.request.urlopen(urllib.request.Request(base+path,headers={'X-Staff-Key':key} if staff else {}),timeout=10)
for path in ('/','/staff','/benchmarks'):
 with get(path) as r:assert b'<div id="root"></div>' in r.read(),path
with get('/api/health') as r:assert json.load(r)['ok']
with get('/api/catalog') as r:cat=json.load(r)
assert len(cat['products'])==8
for p in cat['products']:
 with get('/assets/'+p['photo']) as r:assert r.read(2)==b'\xff\xd8',p['photo']
with get('/api/social/site') as r:assert b'Synthetic social site' in r.read()
with get('/api/benchmarks/runs',True) as r:runs=json.load(r)
assert runs,'run scripts/benchmarks.sh first'
id=runs[0]['id']
with get('/api/benchmarks/runs/'+id,True) as r:snap=json.load(r)
assert snap['run']['status']=='finished'
for _ in range(2):
 with get('/api/benchmarks/runs/'+id+'/events',True) as r:
  assert r.headers['Content-Type']=='text/event-stream'
  assert r.readline().strip()==b'event: snapshot'
  event=json.loads(r.readline().decode().removeprefix('data: '));assert event['run']['id']==id
with get('/api/benchmarks/runs/'+id+'/export.csv',True) as r:rows=list(csv.reader(io.StringIO(r.read().decode())))
assert len(rows)==snap['metrics']['completed']+1
with get('/api/invariants',True) as r:inv=json.load(r)
assert all(inv[k] for k in ('no_duplicate_allocation','inventory_conserved','protected_floor'))
print('PASS: storefront/staff/benchmark shells, 8 local photographs, health/catalog, synthetic social site, persisted run, SSE connection/reconnection, CSV row count and protected invariants')
