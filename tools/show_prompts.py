import json, urllib.request

TOKEN = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjo0LCJ1c2VybmFtZSI6ImFkbWluIiwicm9sZSI6ImFkbWluIiwiaXNzIjoieXVueGktaG9tZSIsImV4cCI6MTc4MDU1MTgyOCwiaWF0IjoxNzgwNDY1NDI4fQ.2vl50pMQyrEOWxIkiaEoeBmGCfFm5mZxRFDtzpfVcuM"

req = urllib.request.Request("http://localhost:9981/api/config/prompts",
    headers={"Authorization": "Bearer " + TOKEN})
with urllib.request.urlopen(req) as resp:
    data = json.loads(resp.read())

sections = data["data"]["sections"]
for key in ["gen_core_rules", "gen_communication", "gen_task_boundary", "gen_tool_strategy"]:
    s = sections.get(key, {})
    print(f"\n===== {key} ({s.get('name','')}) [{len(s.get('content',''))} chars] =====")
    print(s.get("content", "NOT FOUND"))
