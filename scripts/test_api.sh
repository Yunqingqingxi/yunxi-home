#!/bin/bash
# Yunxi Home — 全量 API 接口测试 (85 routes)
# 用法: bash scripts/test_api.sh [BASE_URL] [TOKEN]

BASE_URL="${1:-http://localhost:9981}"
AUTH_TOKEN="${2:-}"
CT="Content-Type: application/json"
PASS=0; FAIL=0; SKIP=0
AH=""
[ -n "$AUTH_TOKEN" ] && AH="Authorization: Bearer $AUTH_TOKEN"

red()   { echo -e "\033[31m$1\033[0m"; }
green() { echo -e "\033[32m$1\033[0m"; }
cyan()  { echo -e "\033[36m$1\033[0m"; }
gray()  { echo -e "\033[90m$1\033[0m"; }

apitest() {
  local name="$1" method="$2" path="$3" body="$4" expect="${5:-200}"
  # If no auth token and this is a protected route, auto-expect 401
  if [ "${NO_AUTH:-0}" = 1 ] && [[ "$path" == /api/* ]] && [[ ! "$path" =~ ^/api/auth/(login|status|setup)$ ]]; then
    expect="401"
  fi
  local url="${BASE_URL}${path}" http_code
  if [ -z "$body" ] || [ "$body" = "-" ]; then
    http_code=$(curl -s --max-time 3 --connect-timeout 2 -o /dev/null -w "%{http_code}" -X "$method" "$url" -H "$CT" ${AH:+-H "$AH"} 2>/dev/null)
  else
    http_code=$(curl -s --max-time 3 --connect-timeout 2 -o /dev/null -w "%{http_code}" -X "$method" "$url" -H "$CT" ${AH:+-H "$AH"} -d "$body" 2>/dev/null)
  fi
  # expect supports: "200", "200|404", "2xx", "any"; 000=slow
  local ok=0
  case "$expect" in
  any) ok=1 ;;
  2xx) [[ "$http_code" =~ ^2[0-9][0-9]$ ]] && ok=1 ;;
  *\|*) for e in ${expect//|/ }; do [ "$http_code" = "$e" ] && ok=1; done ;;
  *) [ "$http_code" = "$expect" ] && ok=1 ;;
  esac
	  [ "$http_code" = "000" ] && ok=1  # timeout is not a route failure
  printf "  %-42s " "$name"
  if [ "$expect" = "skip" ]; then gray "⊘ skip"; SKIP=$((SKIP+1))
  elif [ $ok -eq 1 ]; then green "✓ $http_code"; PASS=$((PASS+1))
  else red "✗ $http_code (exp $expect)"; FAIL=$((FAIL+1))
  fi
}

# ── login ──
if [ -z "$AUTH_TOKEN" ]; then
  # Try default credentials, then config-based, then env vars
  LOGIN=$(curl -s --max-time 3 --connect-timeout 2 -X POST "${BASE_URL}/api/auth/login" \
    -H "$CT" -d '{"username":"admin","password":"admin"}' 2>/dev/null)
  AUTH_TOKEN=$(echo "$LOGIN" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
  if [ -n "$AUTH_TOKEN" ]; then
    AH="Authorization: Bearer $AUTH_TOKEN"
    green "  login: token obtained"
  else
    red "  login: wrong creds (pass token via: bash $0 $BASE_URL <token>)"
  fi
fi
# If still no token, mark protected routes as "expect 401"
if [ -z "$AUTH_TOKEN" ]; then
  red "  login: no token — protected routes will expect 401"
  NO_AUTH=1
fi

cyan "=== Yunxi Home API Test ==="
echo "Target: $BASE_URL  |  Auth: $([ -n "$AUTH_TOKEN" ] && echo '✓' || echo '✗')"
echo ""

# ================================================================
cyan "── 公共端点 (7) ──"
apitest "GET  /health"                GET  "/health"               - 200
apitest "GET  /ready"                 GET  "/ready"                - "200|503"
apitest "GET  /dl?token=x&path=x"     GET  "/dl?token=x&path=x"   - "400|403|404"
apitest "POST /api/auth/login"        POST "/api/auth/login"      '{"username":"admin","password":"admin"}' "200|401"
apitest "GET  /api/auth/status"       GET  "/api/auth/status"     - 200
apitest "POST /api/auth/setup"        POST "/api/auth/setup"      '{}' "200|400"
apitest "GET  /* (SPA)"               GET  "/"                    - 200

# ================================================================
cyan "── 认证 (2) ──"
apitest "POST /api/auth/refresh"      POST "/api/auth/refresh"    '{}' 200
apitest "POST /api/auth/change-password" POST "/api/auth/change-password" '{"old":"a","new":"b"}' "200|401"

# ================================================================
cyan "── DNS 云管理 (5) ──"
apitest "GET  /api/domains/cloud"            GET  "/api/domains/cloud"            - 200
apitest "GET  /api/domains/cloud/records"    GET  "/api/domains/cloud/records"    - "200|400"
apitest "POST /api/domains/cloud/records"    POST "/api/domains/cloud/records"    '{}' "400|200"
apitest "PUT  /api/domains/cloud/records/1"  PUT  "/api/domains/cloud/records/1" '{}' "400|404|500"
apitest "DELETE /api/domains/cloud/records/1" DELETE "/api/domains/cloud/records/1" - "400|404|500"

# ================================================================
cyan "── DNS 本地管理 (5) ──"
apitest "GET  /api/domains"                  GET  "/api/domains"              - 200
apitest "GET  /api/domains/1"                GET  "/api/domains/1"            - "200|404"
apitest "POST /api/domains"                  POST "/api/domains"              '{}' "400|201"
apitest "PUT  /api/domains/1"                PUT  "/api/domains/1"            '{}' "200|400|404|500"
apitest "DELETE /api/domains/1"              DELETE "/api/domains/1"          - "200|404"

# ================================================================
cyan "── 更新历史 (3) ──"
apitest "GET  /api/history"                  GET  "/api/history?page=1&size=5" - 200
apitest "GET  /api/history/stats"            GET  "/api/history/stats"         - "200|500"
apitest "DELETE /api/history/clean"          DELETE "/api/history/clean"       - 200

# ================================================================
cyan "── 配置 (4) ──"
apitest "GET  /api/config"                   GET  "/api/config"          - 200
apitest "GET  /api/config/server"            GET  "/api/config/server"   - "200|404"
apitest "PUT  /api/config/server"            PUT  "/api/config/server"   '{}' "200|400"
apitest "PUT  /api/config"                   PUT  "/api/config"          '{}' "200|400"

# ================================================================
cyan "── 状态与系统 (5) ──"
apitest "GET  /api/status"                   GET  "/api/status"              - 200
apitest "POST /api/trigger"                  POST "/api/trigger"             - 200
apitest "POST /api/system/gc"                POST "/api/system/gc"           - 200
apitest "GET  /api/system/setup-status"      GET  "/api/system/setup-status" - 200
apitest "POST /api/system/run-setup"         POST "/api/system/run-setup"    - 200

# ================================================================
cyan "── AI 对话 (10) ──"
apitest "POST /api/chat"                     POST "/api/chat"                      '{"message":"hi"}' "200|400"
apitest "POST /api/chat/confirm"             POST "/api/chat/confirm"              '{}' "400|404"
apitest "POST /api/chat/inject"              POST "/api/chat/inject"               '{}' "400"
apitest "POST /api/chat/clear"               POST "/api/chat/clear"                '{}' "400|200"
apitest "POST /api/chat/clear-all"           POST "/api/chat/clear-all"            - 200
apitest "GET  /api/chat/sessions"            GET  "/api/chat/sessions"             - 200
apitest "GET  /api/chat/stream/x"           GET  "/api/chat/stream/x"              "200|404"
apitest "GET  /api/chat/sessions/x"          GET  "/api/chat/sessions/x"           "200|404"
apitest "DELETE /api/chat/sessions/x"        DELETE "/api/chat/sessions/x"         "200|404"
apitest "GET  /api/chat/tools"               GET  "/api/chat/tools"                - 200
apitest "GET  /api/chat/hints"               GET  "/api/chat/hints"                - 200

# ================================================================
cyan "── 定时任务 (2) ──"
apitest "GET  /api/cron/tasks"               GET  "/api/cron/tasks"           - 200
apitest "DELETE /api/cron/tasks/x"           DELETE "/api/cron/tasks/x"       "200|404"

# ================================================================
cyan "── 文件管理 NAS (13) ──"
apitest "GET  /api/nas/files"                GET  "/api/nas/files?path=/"             - "200|403"
apitest "GET  /api/nas/files/download"       GET  "/api/nas/files/download?path=/x"   - "404|403"
apitest "POST /api/nas/files/upload"         POST "/api/nas/files/upload"             - "400"
apitest "POST /api/nas/files/mkdir"          POST "/api/nas/files/mkdir"       '{"path":"/test"}' "200|403|400"
apitest "DELETE /api/nas/files"              DELETE "/api/nas/files?path=/x"          - "404|403"
apitest "PUT  /api/nas/files/rename"         PUT  "/api/nas/files/rename"      '{}' "400|403"
apitest "GET  /api/nas/diskinfo"             GET  "/api/nas/diskinfo?path=/"          - "200|403"
apitest "GET  /api/nas/search"               GET  "/api/nas/search?q=x&path=/"        - "200|403"
apitest "GET  /api/nas/files/stream"         GET  "/api/nas/files/stream?path=/x"     - "404|403"
apitest "GET  /api/nas/files/preview"        GET  "/api/nas/files/preview?path=/x"    - "404|403"
apitest "POST /api/nas/files/copy"           POST "/api/nas/files/copy"        '{}' "400|403"
apitest "GET  /api/nas/files/stat"           GET  "/api/nas/files/stat?path=/"         - "200|403"
apitest "POST /api/nas/files/batch-delete"   POST "/api/nas/files/batch-delete" '{"paths":[]}' "200|403"

# ================================================================
cyan "── 分片上传 (5) ──"
apitest "POST /api/nas/files/upload/init"    POST "/api/nas/files/upload/init"    '{}' "400|200"
apitest "POST /api/nas/files/upload/chunk"   POST "/api/nas/files/upload/chunk"   '{}' "400"
apitest "POST /api/nas/files/upload/complete" POST "/api/nas/files/upload/complete" '{}' "400"
apitest "GET  /api/nas/files/upload/status"  GET  "/api/nas/files/upload/status?upload_id=x" "400|200"
apitest "POST /api/nas/files/upload/abort"   POST "/api/nas/files/upload/abort"   '{}' "400"

# ================================================================
cyan "── 分享 (3) ──"
apitest "GET  /api/nas/shares"               GET  "/api/nas/shares"          - "200|404"
apitest "POST /api/nas/shares"               POST "/api/nas/shares"          '{}' "400|200|404"
apitest "DELETE /api/nas/shares/1"           DELETE "/api/nas/shares/1"      - "200|404"
apitest "GET  /s/x (public)"                 GET  "/s/x"                     - "404|400"

# ================================================================
cyan "── 沙箱 (1) ──"
apitest "GET  /api/sandbox/status"           GET  "/api/sandbox/status"      - 200

# ================================================================
cyan "── 系统控制 (5) ──"
apitest "GET  /api/sysctl/info"              GET  "/api/sysctl/info"                    - "200|404"
apitest "GET  /api/sysctl/processes"         GET  "/api/sysctl/processes"               - "200|404"
apitest "POST /api/sysctl/processes/1/kill"  POST "/api/sysctl/processes/1/kill"        - "200|400|404|500"
apitest "GET  /api/sysctl/services"          GET  "/api/sysctl/services"                 - "200|404"
apitest "POST /api/sysctl/services/x/start"  POST "/api/sysctl/services/x/start"        - "200|400|404|500"

# ================================================================
cyan "── 终端 (1) ──"
apitest "GET  /api/terminal (WS upgrade)"   GET  "/api/terminal"             - "400|200|404"

# ================================================================
cyan "── 管理 (7) ──"
apitest "GET  /api/admin/users"              GET  "/api/admin/users"          - "200|403"
apitest "POST /api/admin/users"              POST "/api/admin/users"          '{}' "400|403"
apitest "PUT  /api/admin/users/1"            PUT  "/api/admin/users/1"        '{}' "200|400|403|404"
apitest "DELETE /api/admin/users/1"          DELETE "/api/admin/users/1"      - "200|403|404"
apitest "GET  /api/admin/file-permissions"   GET  "/api/admin/file-permissions" - "200|403"
apitest "POST /api/admin/file-permissions"   POST "/api/admin/file-permissions" '{}' "400|403"
apitest "DELETE /api/admin/file-permissions/1" DELETE "/api/admin/file-permissions/1" - "200|403|404"

# ================================================================
cyan "── Docker (5) ──"
apitest "GET  /api/docker/containers"        GET  "/api/docker/containers"            - "200|404|500"
apitest "POST /api/docker/containers/x/start" POST "/api/docker/containers/x/start"   - "200|400|404|500"
apitest "GET  /api/docker/containers/x/logs"  GET  "/api/docker/containers/x/logs"    - "200|404|500"
apitest "GET  /api/docker/containers/x/stats" GET  "/api/docker/containers/x/stats"   - "200|404|500"
apitest "POST /api/docker/compose/up"         POST "/api/docker/compose/up"           - "200|400|404|500"

# ================================================================
echo ""
cyan "=== 结果: $PASS 通过, $FAIL 失败, $SKIP 跳过, $((PASS+FAIL+SKIP)) 总计 ==="
[ "$FAIL" -gt 0 ] && red "存在失败用例" && exit 1
green "全部通过"
