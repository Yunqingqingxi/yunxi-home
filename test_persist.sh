#!/bin/bash
TOKEN=$(curl -s -X POST http://localhost:9981/api/auth/login -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["token"])')

echo "=== Before ==="
curl -s http://localhost:9981/api/chat/sessions -H "Authorization: Bearer $TOKEN"

curl -s -X POST http://localhost:9981/api/chat -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"message":"test persist","session_id":"persist-test-1"}' --max-time 20 > /dev/null 2>&1 &
sleep 8

echo ""
echo "=== After ==="
curl -s http://localhost:9981/api/chat/sessions -H "Authorization: Bearer $TOKEN"

echo ""
echo "=== DB check ==="
sudo sqlite3 /opt/yunxi-home/data/yunxi-home.db "SELECT id, title FROM chat_sessions;"
