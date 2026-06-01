#!/bin/bash
TOKEN=$(curl -s -X POST http://localhost:9981/api/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["token"])' 2>/dev/null)
echo "LOGIN: $([ -n "$TOKEN" ] && echo OK || echo FAIL)"
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:9981/api/status | python3 -c 'import sys,json;d=json.load(sys.stdin);print("STATUS: "+str(d["code"])+" Uptime: "+str(d.get("data",{}).get("uptime","?")))'
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:9981/api/domains | python3 -c 'import sys,json;d=json.load(sys.stdin);print("DOMAINS: "+str(d["code"]))'
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:9981/api/history | python3 -c 'import sys,json;d=json.load(sys.stdin);print("HISTORY: "+str(d["code"]))'
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:9981/api/chat/sessions | python3 -c 'import sys,json;d=json.load(sys.stdin);print("SESSIONS: "+str(d["code"]))'
curl -s -o /dev/null -w "SETTINGS_CONFIG: %{http_code}\n" -H "Authorization: Bearer $TOKEN" http://localhost:9981/api/config
curl -s -o /dev/null -w "NAS_FILES: %{http_code}\n" -H "Authorization: Bearer $TOKEN" 'http://localhost:9981/api/nas/files?dir=/'
