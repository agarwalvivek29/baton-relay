#!/usr/bin/env python3
"""Measure what eBPF did with a batch of requests.

usage: count-traces.py PREFIX N   (request IDs PREFIX-1 .. PREFIX-N)

For each request ID, search Tempo by the X-Request-ID span attribute (the
same TraceQL the Grafana log link runs), then report:
  - how many separate traces eBPF produced for that one request
  - whether all 7 HTTP hops carry that request's ID
Standard library only. Used to fill in docs/goroutines.md.
"""
import collections, json, os, sys, time, urllib.parse, urllib.request

TEMPO = os.environ.get("TEMPO_URL", "http://localhost:3200")
prefix = sys.argv[1]; n = int(sys.argv[2])
EXPECTED={('gateway','SERVER'),('gateway','CLIENT'),('orders','SERVER'),('orders','CLIENT','/charge'),('orders','CLIENT','/reserve'),('payments','SERVER'),('inventory','SERVER')}
now=int(time.time())
get=lambda u: json.load(urllib.request.urlopen(u))
full=0; ntr=collections.Counter(); missing=collections.Counter()
for i in range(1,n+1):
    rid=f"{prefix}-{i}"
    q=urllib.parse.urlencode({'q':'{ span."http.request.header.x-request-id" = "%s" || span."http.response.header.x-request-id" = "%s" }'%(rid,rid),'start':now-3600,'end':now,'spss':100})
    ts=get(TEMPO+'/api/search?'+q).get('traces',[])
    ntr[len(ts)]+=1
    hops=set()
    for t in ts:
        d=get(TEMPO+'/api/v2/traces/'+t['traceID'])
        for rs in d['trace']['resourceSpans']:
            svc=[a['value']['stringValue'] for a in rs['resource']['attributes'] if a['key']=='service.name'][0]
            for ss in rs['scopeSpans']:
                for s in ss['spans']:
                    k=s.get('kind','')[10:]
                    vals=[v['stringValue'] for a in s.get('attributes',[]) if a['key'] in ('http.request.header.x-request-id','http.response.header.x-request-id') for v in a['value']['arrayValue']['values']]
                    if rid in vals:
                        hops.add((svc,k) if not (svc=='orders' and k=='CLIENT') else (svc,k,s['name'].split()[-1]))
    if EXPECTED<=hops: full+=1
    for h in EXPECTED-hops: missing[h]+=1
print(f"{prefix}: {full}/{n} requests have the ID on all 7 hops; traces per request: {dict(sorted(ntr.items()))}")
for h,c in missing.most_common(): print("   missing",h,c)
