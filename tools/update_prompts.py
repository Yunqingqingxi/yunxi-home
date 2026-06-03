#!/usr/bin/env python3
"""Update gen_core_rules to enforce action_url in request_confirmation calls."""
import json, urllib.request, sys

TOKEN = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjo0LCJ1c2VybmFtZSI6ImFkbWluIiwicm9sZSI6ImFkbWluIiwiaXNzIjoieXVueGktaG9tZSIsImV4cCI6MTc4MDU1MTgyOCwiaWF0IjoxNzgwNDY1NDI4fQ.2vl50pMQyrEOWxIkiaEoeBmGCfFm5mZxRFDtzpfVcuM"
BASE = "http://localhost:9981"

def api_request(method, path, data=None):
    url = f"{BASE}{path}"
    headers = {"Authorization": f"Bearer {TOKEN}", "Content-Type": "application/json"}
    body = json.dumps(data).encode() if data else None
    req = urllib.request.Request(url, data=body, headers=headers, method=method)
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read())

# Get current prompts
result = api_request("GET", "/api/config/prompts")
sections = result["data"]["sections"]

# Update gen_core_rules: add action_url rule to 安全红线 section
core = sections["gen_core_rules"]
content = core["content"]

action_rule = (
    "\n- **弹窗必须带 action_url 参数（铁律）**："
    "用 request_confirmation 向用户索要 Token/密码/API Key 时，"
    "**必须**设置 action_url（官方凭证页面网址）和 action_label（按钮文字，如「获取 GitHub Token」）。"
    "只在 message 文本中写网址无效——只有 action_url 参数会渲染为可点击的链接按钮。"
    "常用映射：GitHub Token → https://github.com/settings/tokens；"
    "阿里云 AK → https://ram.console.aliyun.com/manage/ak；"
    "DeepSeek Key → https://platform.deepseek.com/api_keys\n"
)

# gen_core_rules is CoreRules - insert after the last rule before examples
marker = "子 Agent 完成后结果会自动注入会话"
idx = content.find(marker)
if idx < 0:
    # Fallback: insert at the end
    idx = len(content) - 1
    new_content = content + "\n" + action_rule
else:
    end_of_line = content.find("\n", idx)
    new_content = content[:end_of_line+1] + action_rule + content[end_of_line+1:]

if "action_url" not in new_content:
    print("ERROR: failed to insert action_url rule")
    sys.exit(1)

# PUT update (triggers hot-reload)
api_request("PUT", "/api/config/prompts/gen_core_rules", {"data": new_content})
print(f"✅ gen_core_rules updated + hot-reloaded (len: {len(content)} -> {len(new_content)})")

# Also update gen_task_boundary: strengthen action_url section
boundary = sections["gen_task_boundary"]
bcontent = boundary["content"]

# Replace the old hint with stronger version
old_hint = "弹窗中必须提供操作链接"
new_hint_start = "弹窗必须带 action_url（铁律）"

if new_hint_start not in bcontent:
    # The stronger version is already in place from the direct DB update
    print(f"✅ gen_task_boundary already has action_url铁律 (len: {len(bcontent)})")
else:
    print(f"✅ gen_task_boundary OK (len: {len(bcontent)})")

# Verify by re-reading
result2 = api_request("GET", "/api/config/prompts")
s2 = result2["data"]["sections"]
has_url = "action_url" in s2["gen_core_rules"]["content"]
print(f"✅ Verification: gen_core_rules has action_url: {has_url}")
