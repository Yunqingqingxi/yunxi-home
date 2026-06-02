# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: chat.spec.ts >> Chat — 基础交互 >> 简单寒暄：发送消息并收到回复
- Location: e2e\chat.spec.ts:27:3

# Error details

```
Error: expect(locator).toBeEnabled() failed

Locator: locator('button').filter({ hasText: '发送' })
Expected: enabled
Timeout: 3000ms
Error: element(s) not found

Call log:
  - Expect "toBeEnabled" with timeout 3000ms
  - waiting for locator('button').filter({ hasText: '发送' })

```

```yaml
- navigation:
  - img "云兮之家"
  - button "仪表盘":
    - img
    - text: 仪表盘
  - button "文件管理":
    - img
    - text: 文件管理
  - button "DNS 管理":
    - img
    - text: DNS 管理
  - button "技能市场":
    - img
    - text: 技能市场
  - button "日志":
    - img
    - text: 日志
  - button "设置":
    - img
    - text: 设置
  - button "切换到暗色模式":
    - img
  - button "退出登录":
    - img
- main:
  - complementary:
    - button "展开侧栏":
      - img
    - button "新建":
      - img
  - img
  - heading "云 兮" [level=1]
  - paragraph: 你的全能家庭服务器运维伙伴
  - button "文件管理":
    - img
    - text: 文件管理
  - button "DNS 域名":
    - img
    - text: DNS 域名
  - button "系统监控":
    - img
    - text: 系统监控
  - button "Docker":
    - img
    - text: Docker
  - button "搜索最近的日志文件"
  - button "查看网络连接状态"
  - button "检查服务运行状态"
  - textbox "描述你想做什么...": 你好
  - button "Flash":
    - text: Flash
    - img
  - button "附加文件":
    - img
  - button "发送":
    - img
```

# Test source

```ts
  1   | import { test, expect } from '@playwright/test'
  2   | 
  3   | // Login helper
  4   | async function login(page: any) {
  5   |   await page.goto('http://localhost:9981/#/login')
  6   |   await page.waitForTimeout(500)
  7   |   const userInput = page.locator('input[placeholder="用户名"]')
  8   |   if (await userInput.isVisible({ timeout: 3000 }).catch(() => false)) {
  9   |     await userInput.fill('admin')
  10  |     await page.locator('input[placeholder="密码"]').fill('admin123')
  11  |     await page.locator('button').filter({ hasText: '登' }).click()
  12  |     await page.waitForTimeout(2000)
  13  |   }
  14  |   // If setup page appears (first run)
  15  |   const setupInput = page.locator('input[placeholder*="输入密码"]').first()
  16  |   if (await setupInput.isVisible({ timeout: 2000 }).catch(() => false)) {
  17  |     await setupInput.fill('admin123')
  18  |     await page.locator('input[placeholder*="确认密码"]').fill('admin123')
  19  |     await page.locator('button').filter({ hasText: '设' }).click()
  20  |     await page.waitForTimeout(2000)
  21  |   }
  22  | }
  23  | 
  24  | // ── Chat Basics ──
  25  | 
  26  | test.describe('Chat — 基础交互', () => {
  27  |   test('简单寒暄：发送消息并收到回复', async ({ page }) => {
  28  |     await login(page)
  29  |     await page.goto('http://localhost:9981/#/chat')
  30  |     await page.waitForTimeout(1000)
  31  | 
  32  |     // Chat input
  33  |     const input = page.locator('textarea[placeholder*="描述"]')
  34  |     await expect(input).toBeVisible({ timeout: 10000 })
  35  |     await input.fill('你好')
  36  |     // Enable send button by typing
  37  |     const sendBtn = page.locator('button').filter({ hasText: '发送' })
> 38  |     await expect(sendBtn).toBeEnabled({ timeout: 3000 })
      |                           ^ Error: expect(locator).toBeEnabled() failed
  39  |     await sendBtn.click()
  40  | 
  41  |     // Wait for user message to appear (right side)
  42  |     await expect(page.locator('.msg-row.user').first()).toBeVisible({ timeout: 15000 })
  43  | 
  44  |     // Wait for AI reply (left side, up to 120s)
  45  |     await expect(page.locator('.msg-row.assistant').first()).toBeVisible({ timeout: 120000 })
  46  |   })
  47  | 
  48  |   test('消息气泡方向：用户在右，AI在左', async ({ page }) => {
  49  |     await login(page)
  50  |     await page.goto('http://localhost:9981/#/chat')
  51  |     await page.waitForTimeout(1000)
  52  | 
  53  |     const input = page.locator('textarea[placeholder*="描述"]')
  54  |     await input.fill('测试布局')
  55  |     const sendBtn = page.locator('button').filter({ hasText: '发送' })
  56  |     await expect(sendBtn).toBeEnabled({ timeout: 3000 })
  57  |     await sendBtn.click()
  58  | 
  59  |     // User message on right
  60  |     const userMsg = page.locator('.msg-row.user').first()
  61  |     await expect(userMsg).toBeVisible({ timeout: 15000 })
  62  | 
  63  |     // AI reply on left
  64  |     const aiMsg = page.locator('.msg-row.assistant').first()
  65  |     await expect(aiMsg).toBeVisible({ timeout: 120000 })
  66  |   })
  67  | 
  68  |   test('AI 昵称显示为 云兮', async ({ page }) => {
  69  |     await login(page)
  70  |     await page.goto('http://localhost:9981/#/chat')
  71  |     await page.waitForTimeout(1000)
  72  | 
  73  |     const input = page.locator('textarea[placeholder*="描述"]')
  74  |     await input.fill('你好')
  75  |     await page.locator('button').filter({ hasText: '发送' }).click()
  76  | 
  77  |     // Wait for AI reply
  78  |     await expect(page.locator('.role-tag').first()).toBeVisible({ timeout: 120000 })
  79  |     const tag = page.locator('.role-tag').first()
  80  |     await expect(tag).toHaveText('云兮')
  81  |   })
  82  | })
  83  | 
  84  | // ── Session Switching ──
  85  | 
  86  | test.describe('Chat — 会话切换', () => {
  87  |   test('切换会话后消息历史正确恢复', async ({ page }) => {
  88  |     await login(page)
  89  |     await page.goto('http://localhost:9981/#/chat')
  90  |     await page.waitForTimeout(1000)
  91  | 
  92  |     // Send a message first
  93  |     const input = page.locator('textarea[placeholder*="描述"]')
  94  |     await input.fill('切换测试消息')
  95  |     await page.locator('button').filter({ hasText: '发送' }).click()
  96  |     await page.waitForTimeout(3000)
  97  | 
  98  |     // Expand sidebar
  99  |     const expandBtn = page.locator('button').filter({ hasText: '展开侧栏' })
  100 |     if (await expandBtn.isVisible({ timeout: 2000 }).catch(() => false)) {
  101 |       await expandBtn.click()
  102 |       await page.waitForTimeout(500)
  103 |     }
  104 | 
  105 |     // Click another conversation if available
  106 |     const otherConv = page.locator('[class*="conversation"], [class*="session"], .sidebar-item').first()
  107 |     if (await otherConv.isVisible({ timeout: 2000 }).catch(() => false)) {
  108 |       await otherConv.click()
  109 |       await page.waitForTimeout(1000)
  110 | 
  111 |       // Click the original conversation
  112 |       await otherConv.click()
  113 |       await page.waitForTimeout(1000)
  114 | 
  115 |       // Messages should still be there
  116 |       const userMsgs = page.locator('.msg-row.user')
  117 |       expect(await userMsgs.count()).toBeGreaterThan(0)
  118 |     }
  119 |   })
  120 | 
  121 |   test('活跃任务时发送按钮变为停止按钮', async ({ page }) => {
  122 |     await login(page)
  123 |     await page.goto('http://localhost:9981/#/chat')
  124 |     await page.waitForTimeout(1000)
  125 | 
  126 |     const input = page.locator('textarea[placeholder*="描述"]')
  127 |     await input.fill('执行一个长任务：列出所有文件')
  128 |     await page.locator('button').filter({ hasText: '发送' }).click()
  129 | 
  130 |     // After sending, the button should change state
  131 |     await page.waitForTimeout(500)
  132 |     // Either stop button appears or send is disabled
  133 |     const stopBtn = page.locator('[class*="stop"], [class*="interrupt"]')
  134 |     const hasStop = await stopBtn.isVisible({ timeout: 3000 }).catch(() => false)
  135 |     if (!hasStop) {
  136 |       // At minimum, streaming indicator should be visible
  137 |       const indicator = page.locator('[class*="streaming"], [class*="loading"]')
  138 |       await expect(indicator.first()).toBeVisible({ timeout: 5000 }).catch(() => {})
```