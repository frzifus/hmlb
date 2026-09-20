kubectl exec -n openebs deploy/openebs-mayastor-api-rest -c api-rest -- \
    wget -qO- "http://localhost:8081/v0/volumes?max_entries=100" | \
    python3 -c "
  import json,sys
  for v in json.load(sys.stdin)['entries']:
    t = v.get('state',{}).get('target',{})
    if t.get('rebuilds',0) > 0:
      uuid = v['spec']['uuid'][:12]
      gb = v['spec']['size']/(1024**3)
      for c in t.get('children',[]):
        p = c.get('rebuildProgress')
        if p is not None:
          print(f'{uuid}  {gb:7.1f}G  rebuild={p}%')
  "

