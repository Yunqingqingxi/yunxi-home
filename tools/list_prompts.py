import json, urllib.request

TOKEN = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjo0LCJ1c2VybmFtZSI6ImFkbWluIiwicm9sZSI6ImFkbWluIiwiaXNzIjoieXVueGktaG9tZSIsImV4cCI6MTc4MDU1MTgyOCwiaWF0IjoxNzgwNDY1NDI4fQ.2vl50pMQyrEOWxIkiaEoeBmGCfFm5mZxRFDtzpfVcuM"
BASE = "http://localhost:9981"

def api(method, path, data=None):
    url = f"{BASE}{path}"
    headers = {"Authorization": f"Bearer {TOKEN}", "Content-Type": "application/json"}
    body = json.dumps(data).encode() if data else None
    req = urllib.request.Request(url, data=body, headers=headers, method=method)
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read())

result = api("GET", "/api/config/prompts")
sections = result["data"]["sections"]
print(f"Total sections: {len(sections)}")
for k, v in sorted(sections.items()):
    has = "action_url" in v.get("content", "")
    print(f"  {k}: {v['name']} [{v['category']}] len={len(v['content'])} action_url={has}")
