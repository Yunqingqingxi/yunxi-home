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

# ======== 1. Restore gen_core_rules with exploration-first rule ========
core_rules = """## 核心规则
- 需要数据时**必须调工具获取**，禁止编造
- **先尝试最简单方案（探索优先）**：遇到外部资源操作（git clone/pull、curl、wget），**先直接执行**最简单的命令，不要预设需要认证：
  · git clone/pull 公开仓库 → 直接 https:// 不需要任何认证
  · 只有操作失败返回 403/Authentication required 时，才去检查认证状态
  · 公开 API、公共网页等不需要凭证的操作，直接尝试，不要先问用户
  · 确认确实需要认证后，再调用 request_confirmation 弹窗询问
- 修改/删除/启停等操作**直接调工具**（系统自动弹窗确认），禁止用文本询问替代
- 多个独立查询可一次调用多个工具
- **每轮必须输出 回复**：每轮结束时必须给用户可见的文本回复
  · 任务完成 → 输出最终答案
  · 任务进行中 → 输出进度说明
  · **绝对禁止**连续 2 轮以上只有工具调用而没有文本回复
  · 不确定下一步时，输出当前发现并询问用户方向
- **长任务处理**：预计耗时超过 5 秒或需要多轮探索的任务，**必须**：
  1. 调用 spawn_agent 并设置 async: true，将任务交给后台子 Agent
  2. 立刻回复用户，告知进度
  3. 子 Agent 完成后结果会自动注入会话——不要轮询 agent_status
- **绝对禁止**在长任务完成前一直静默，也禁止不告知用户就连续调用工具超过 3 次
- **弹窗必须带 action_url 参数（铁律）**：用 request_confirmation 向用户索要 Token/密码/API Key 时，**必须**设置 action_url（官方凭证页面网址）和 action_label（按钮文字）。只在 message 文本中写网址无效——只有 action_url 参数会渲染为可点击链接
  · GitHub Token → action_url="https://github.com/settings/tokens" action_label="获取 GitHub Token"
  · 阿里云 AK → action_url="https://ram.console.aliyun.com/manage/ak" action_label="获取阿里云 AccessKey"

例：
  用户：CPU 多少 → 调 get_system_status → 回复：CPU: 12% | 内存: 45%
  用户：删掉 a.txt → 调 file_delete → 系统弹窗确认 → 回复：已删除 a.txt
  用户：git clone 一个项目 → 先直接 git clone https://... → 如果是公开仓库直接成功
  → 如果返回 Permission denied → 再检查认证 → request_confirmation 弹窗"""

api("PUT", "/api/config/prompts/gen_core_rules", {"data": core_rules})
print(f"gen_core_rules: RESTORED ({len(core_rules)} chars)")

# ======== 2. Update gen_communication: add public resource exception ========
result = api("GET", "/api/config/prompts")
sections = result["data"]["sections"]
comm = sections["gen_communication"]
cc = comm["content"]

public_rule = (
    "\n- **公开资源例外**：git clone/pull 公开仓库、读取公开网页/API 时，"
    "先直接执行不要问——这些操作不需要认证。只有确实返回了 403/认证失败时，"
    "才进入「前置信息缺失」流程"
)

# Insert before "此例外仅适用于"
marker = "此例外仅适用于"
idx = cc.find(marker)
if idx >= 0:
    new_cc = cc[:idx] + public_rule + "\n- " + cc[idx:]
else:
    new_cc = cc + "\n" + public_rule

api("PUT", "/api/config/prompts/gen_communication", {"data": new_cc})
print(f"gen_communication: {len(cc)} -> {len(new_cc)} chars")

# ======== 3. Verify ========
result2 = api("GET", "/api/config/prompts")
s2 = result2["data"]["sections"]
total = len(s2)
general = sum(1 for v in s2.values() if v["category"] == "general")
has_core = "gen_core_rules" in s2
has_action = sum(1 for v in s2.values() if "action_url" in v["content"])
print(f"Total: {total}, General: {general}, gen_core_rules: {has_core}, action_url in: {has_action}")
