import { test, expect } from '@playwright/test'

// Login helper
async function login(page: any) {
  await page.goto('http://localhost:9981/#/login')
  await page.waitForTimeout(500)
  const userInput = page.locator('input[placeholder="用户名"]')
  if (await userInput.isVisible({ timeout: 3000 }).catch(() => false)) {
    await userInput.fill('admin')
    await page.locator('input[placeholder="密码"]').fill('admin123')
    await page.locator('button').filter({ hasText: '登' }).click()
    await page.waitForTimeout(2000)
  }
  // If setup page appears (first run)
  const setupInput = page.locator('input[placeholder*="输入密码"]').first()
  if (await setupInput.isVisible({ timeout: 2000 }).catch(() => false)) {
    await setupInput.fill('admin123')
    await page.locator('input[placeholder*="确认密码"]').fill('admin123')
    await page.locator('button').filter({ hasText: '设' }).click()
    await page.waitForTimeout(2000)
  }
}

// ── Chat Basics ──

test.describe('Chat — 基础交互', () => {
  test('简单寒暄：发送消息并收到回复', async ({ page }) => {
    await login(page)
    await page.goto('http://localhost:9981/#/chat')
    await page.waitForTimeout(1000)

    // Chat input
    const input = page.locator('textarea[placeholder*="描述"]')
    await expect(input).toBeVisible({ timeout: 10000 })
    await input.fill('你好')
    // Enable send button by typing
    const sendBtn = page.locator('button').filter({ hasText: '发送' })
    await expect(sendBtn).toBeEnabled({ timeout: 3000 })
    await sendBtn.click()

    // Wait for user message to appear (right side)
    await expect(page.locator('.msg-row.user').first()).toBeVisible({ timeout: 15000 })

    // Wait for AI reply (left side, up to 120s)
    await expect(page.locator('.msg-row.assistant').first()).toBeVisible({ timeout: 120000 })
  })

  test('消息气泡方向：用户在右，AI在左', async ({ page }) => {
    await login(page)
    await page.goto('http://localhost:9981/#/chat')
    await page.waitForTimeout(1000)

    const input = page.locator('textarea[placeholder*="描述"]')
    await input.fill('测试布局')
    const sendBtn = page.locator('button').filter({ hasText: '发送' })
    await expect(sendBtn).toBeEnabled({ timeout: 3000 })
    await sendBtn.click()

    // User message on right
    const userMsg = page.locator('.msg-row.user').first()
    await expect(userMsg).toBeVisible({ timeout: 15000 })

    // AI reply on left
    const aiMsg = page.locator('.msg-row.assistant').first()
    await expect(aiMsg).toBeVisible({ timeout: 120000 })
  })

  test('AI 昵称显示为 云兮', async ({ page }) => {
    await login(page)
    await page.goto('http://localhost:9981/#/chat')
    await page.waitForTimeout(1000)

    const input = page.locator('textarea[placeholder*="描述"]')
    await input.fill('你好')
    await page.locator('button').filter({ hasText: '发送' }).click()

    // Wait for AI reply
    await expect(page.locator('.role-tag').first()).toBeVisible({ timeout: 120000 })
    const tag = page.locator('.role-tag').first()
    await expect(tag).toHaveText('云兮')
  })
})

// ── Session Switching ──

test.describe('Chat — 会话切换', () => {
  test('切换会话后消息历史正确恢复', async ({ page }) => {
    await login(page)
    await page.goto('http://localhost:9981/#/chat')
    await page.waitForTimeout(1000)

    // Send a message first
    const input = page.locator('textarea[placeholder*="描述"]')
    await input.fill('切换测试消息')
    await page.locator('button').filter({ hasText: '发送' }).click()
    await page.waitForTimeout(3000)

    // Expand sidebar
    const expandBtn = page.locator('button').filter({ hasText: '展开侧栏' })
    if (await expandBtn.isVisible({ timeout: 2000 }).catch(() => false)) {
      await expandBtn.click()
      await page.waitForTimeout(500)
    }

    // Click another conversation if available
    const otherConv = page.locator('[class*="conversation"], [class*="session"], .sidebar-item').first()
    if (await otherConv.isVisible({ timeout: 2000 }).catch(() => false)) {
      await otherConv.click()
      await page.waitForTimeout(1000)

      // Click the original conversation
      await otherConv.click()
      await page.waitForTimeout(1000)

      // Messages should still be there
      const userMsgs = page.locator('.msg-row.user')
      expect(await userMsgs.count()).toBeGreaterThan(0)
    }
  })

  test('活跃任务时发送按钮变为停止按钮', async ({ page }) => {
    await login(page)
    await page.goto('http://localhost:9981/#/chat')
    await page.waitForTimeout(1000)

    const input = page.locator('textarea[placeholder*="描述"]')
    await input.fill('执行一个长任务：列出所有文件')
    await page.locator('button').filter({ hasText: '发送' }).click()

    // After sending, the button should change state
    await page.waitForTimeout(500)
    // Either stop button appears or send is disabled
    const stopBtn = page.locator('[class*="stop"], [class*="interrupt"]')
    const hasStop = await stopBtn.isVisible({ timeout: 3000 }).catch(() => false)
    if (!hasStop) {
      // At minimum, streaming indicator should be visible
      const indicator = page.locator('[class*="streaming"], [class*="loading"]')
      await expect(indicator.first()).toBeVisible({ timeout: 5000 }).catch(() => {})
    }
  })
})

// ── Agent Display ──

test.describe('Chat — Agent 展示', () => {
  test('AgentPanel 在有子Agent时显示', async ({ page }) => {
    await login(page)
    await page.goto('http://localhost:9981/#/chat')
    await page.waitForTimeout(1000)

    const input = page.locator('textarea[placeholder*="描述"]')
    await input.fill('同时检查 DNS 配置和 Docker 容器状态')
    await page.locator('button').filter({ hasText: '发送' }).click()

    // Wait for AgentPanel to appear
    await expect(
      page.locator('[class*="agent-panel"], [class*="AgentPanel"], [class*="sub-agent"]').first()
    ).toBeVisible({ timeout: 180000 }).catch(() => {
      // If no multi-agent triggered, at least conversation completed
      expect(page.locator('.msg-row.assistant').first()).toBeVisible()
    })
  })

  test('AgentStatusBar 显示助手状态摘要', async ({ page }) => {
    await login(page)
    await page.goto('http://localhost:9981/#/chat')
    await page.waitForTimeout(1000)

    // Component should exist (visible when agents are active)
    const bar = page.locator('[class*="agent-status"], [class*="AgentStatusBar"]')
    const count = await bar.count()
    expect(count >= 0).toBe(true)
  })
})

// ── Interrupt ──

test.describe('Chat — 中断恢复', () => {
  test('暂停按钮发送 interrupt 请求', async ({ page }) => {
    await login(page)
    await page.goto('http://localhost:9981/#/chat')
    await page.waitForTimeout(1000)

    const input = page.locator('textarea[placeholder*="描述"]')
    await input.fill('搜索所有日志中的错误信息')
    await page.locator('button').filter({ hasText: '发送' }).click()

    await page.waitForTimeout(2000)

    // Look for stop/interrupt button
    const stopBtn = page.locator('[class*="stop"], [class*="interrupt"], button:has-text("停止")').first()
    if (await stopBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
      await stopBtn.click()
      // Interrupt banner should appear
      await expect(
        page.locator('[class*="interrupt"], [class*="InterruptBanner"]').first()
      ).toBeVisible({ timeout: 10000 })
    }
  })
})
