<template>
  <div class="market-page">
    <h3>技能与工具市场</h3>

    <div class="tab-bar">
      <button
        :class="['tab', { active: tab === 'skills' }]"
        @click="tab = 'skills'"
      >
        ⭐ 技能 Skills
      </button>
      <button
        :class="['tab', { active: tab === 'mcp' }]"
        @click="tab = 'mcp'"
      >
        🔌 MCP 工具
      </button>
      <button
        :class="['tab', { active: tab === 'installed' }]"
        @click="tab = 'installed'; loadInstalled()"
      >
        📦 已安装
      </button>
    </div>

    <!-- ── Skills Market ── -->
    <div
      v-if="tab === 'skills'"
      class="tab-content"
    >
      <div class="action-bar">
        <div
          class="search-wrap"
          style="flex:1"
        >
          <svg
            width="13"
            height="13"
            viewBox="0 0 13 13"
            fill="none"
            stroke="currentColor"
            stroke-width="1.6"
          ><circle
            cx="5.5"
            cy="5.5"
            r="3.5"
          /><path d="M8.5 8.5L12 12" /></svg>
          <input
            v-model="skillSearch"
            placeholder="搜索技能，如 backup, monitor, docker..."
            class="search-inp"
            style="flex:1"
            @keyup.enter="searchSkills"
          />
        </div>
        <button
          class="btn-accent"
          :disabled="searching"
          @click="searchSkills"
        >
          {{ searching ? '搜索中...' : '搜索' }}
        </button>
      </div>

      <div
        v-if="skillResults.length"
        class="card-grid"
      >
        <div
          v-for="s in skillResults"
          :key="s.name"
          class="card-item"
          :class="{ installed: s._installed }"
        >
          <div class="card-head">
            <span class="card-name">⭐ {{ s.name }}</span>
            <span class="card-cat">{{ s.category }}</span>
          </div>
          <div class="card-desc">
            {{ s.description || '无描述' }}
          </div>
          <div class="card-meta">
            <span v-if="s.stars">★ {{ s.stars }}</span>
            <span v-if="s.author">@{{ s.author }}</span>
          </div>
          <button
            v-if="s._installed"
            class="card-action done"
          >
            ✓ 已安装
          </button>
          <button
            v-else
            class="card-action accent"
            :disabled="installing === s.name"
            @click="installSkill(s)"
          >
            {{ installing === s.name ? '安装中...' : '下载安装' }}
          </button>
        </div>
      </div>
      <div
        v-else-if="skillSearched && !searching"
        class="empty-state"
      >
        <p>未找到匹配的技能</p>
        <span class="hint">尝试其他关键词</span>
      </div>
      <div
        v-else-if="!searching"
        class="hint-banner"
      >
        <p>
          🔍 输入关键词搜索在线技能市场<br />
          <span style="font-size:11px;color:var(--text-muted)">来源：GitHub 开源社区 + 内置推荐</span>
        </p>
      </div>
    </div>

    <!-- ── MCP Market ── -->
    <div
      v-if="tab === 'mcp'"
      class="tab-content"
    >
      <div
        v-if="installError"
        class="install-banner error"
      >
        {{ installError }} <button @click="installError=''">
          ✕
        </button>
      </div>
      <div
        v-if="installSuccess"
        class="install-banner success"
      >
        ✅ {{ installSuccess }} 安装成功！已重载 MCP 工具。 <button @click="installSuccess=''">
          ✕
        </button>
      </div>
      <div class="section">
        <h4>🔥 热门推荐</h4>
        <div class="card-grid">
          <div
            v-for="m in popularMCP"
            :key="m.package"
            class="card-item"
            :class="{ installed: m._installed }"
          >
            <div class="card-head">
              <span class="card-name">{{ m.name }}</span>
              <span class="card-cat">{{ m.category }}</span>
            </div>
            <div class="card-desc">
              {{ m.description }}
            </div>
            <div class="card-meta">
              {{ m.package }}
            </div>
            <button
              v-if="m._installed"
              class="card-action done"
            >
              ✓ 已安装
            </button>
            <button
              v-else
              class="card-action accent"
              :disabled="installing === m.package"
              @click="installMCP(m.package)"
            >
              {{ installing === m.package ? '安装中...' : '一键安装' }}
            </button>
          </div>
        </div>
      </div>

      <div class="section">
        <h4>🔍 搜索更多</h4>
        <div class="action-bar">
          <div
            class="search-wrap"
            style="flex:1"
          >
            <input
              v-model="mcpSearch"
              placeholder="如 filesystem, github, puppeteer..."
              class="search-inp"
              style="flex:1"
              @keyup.enter="searchMCP"
            />
          </div>
          <button
            class="btn-accent"
            :disabled="searching"
            @click="searchMCP"
          >
            搜索
          </button>
        </div>
        <div
          v-if="mcpResults.length"
          class="card-grid"
          style="margin-top:10px"
        >
          <div
            v-for="r in mcpResults"
            :key="r.name"
            class="card-item"
            :class="{ installed: r._installed }"
          >
            <div class="card-head">
              <span class="card-name">{{ r.name }}</span>
              <span
                v-if="r.score"
                class="card-cat"
              >{{ r.score }}% 匹配</span>
            </div>
            <div class="card-desc">
              {{ r.desc }}
            </div>
            <button
              v-if="r._installed"
              class="card-action done"
            >
              ✓ 已安装
            </button>
            <button
              v-else
              class="card-action accent"
              :disabled="installing === r.name"
              @click="installMCP(r.name)"
            >
              安装
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- ── Installed ── -->
    <div
      v-if="tab === 'installed'"
      class="tab-content"
    >
      <div
        v-if="installedSkills.length"
        class="section"
      >
        <h4>⭐ 已安装技能 ({{ installedSkills.length }})</h4>
        <div class="card-grid">
          <div
            v-for="s in installedSkills"
            :key="s.name"
            class="card-item"
          >
            <div class="card-head">
              <span class="card-name">/{{ s.name }}</span>
            </div>
            <div class="card-desc">
              {{ s.description }}
            </div>
            <button
              class="card-action"
              @click="runSkill(s.name)"
            >
              执行
            </button>
          </div>
        </div>
      </div>
      <div
        v-if="installedMCPServers.length"
        class="section"
      >
        <h4>🔌 已安装 MCP ({{ installedMCPServers.length }})</h4>
        <div class="card-grid">
          <div
            v-for="s in installedMCPServers"
            :key="s.name"
            class="card-item"
          >
            <div class="card-head">
              <span class="card-name">{{ s.name }}</span>
            </div>
            <div class="card-desc">
              {{ s.command }} {{ (s.args||[]).join(' ') }}
            </div>
          </div>
        </div>
      </div>
      <div
        v-if="!installedSkills.length && !installedMCPServers.length"
        class="empty-state"
      >
        <p>暂无已安装项</p>
      </div>
    </div>

    <!-- MCP Config Modal -->
    <div
      v-if="showMCPConfig"
      class="modal-overlay"
      @click.self="showMCPConfig = false"
    >
      <div class="modal-card">
        <h4>🔧 配置 {{ mcpConfigPkg }}</h4>
        <p style="font-size:12px;color:var(--text-muted)">
          此 MCP 服务器需要以下参数才能运行：
        </p>
        <div
          v-for="p in mcpConfigParams"
          :key="p.name"
          class="config-field"
        >
          <label>{{ p.label }} <span
            v-if="p.required"
            style="color:var(--color-danger)"
          >*</span></label>
          <input
            v-model="mcpConfigValues[p.name]"
            :placeholder="p.description"
            class="modal-input"
            :value="mcpConfigValues[p.name] || p.default || ''"
          />
          <span class="config-hint">{{ p.description }}</span>
        </div>
        <div class="modal-actions">
          <button
            class="btn-cancel"
            @click="showMCPConfig = false; installing = null"
          >
            取消
          </button>
          <button
            class="btn-ok"
            @click="submitMCPConfig"
          >
            安装
          </button>
        </div>
      </div>
    </div>

    <!-- MCP Install Progress -->
    <MCPInstallProgress
      :tasks="installTasks"
      @clear="installTasks = installTasks.filter(t => t.status === 'running')"
    />

    <!-- Result Modal -->
    <div
      v-if="resultModal"
      class="modal-overlay"
      @click.self="resultModal = ''"
    >
      <div class="modal-card">
        <h4>{{ resultTitle }}</h4>
        <pre class="install-result">{{ resultModal }}</pre>
        <div class="modal-actions">
          <button
            class="btn-ok"
            @click="resultModal = ''"
          >
            确定
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import MCPInstallProgress from '../components/MCPInstallProgress.vue'

const tab = ref('skills')
const installTasks = ref([])
const token = () => localStorage.getItem('token') || ''
const authHeaders = () => ({ 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + token() })

// ── Skills ──
const skillSearch = ref('')
const skillResults = ref([])
const skillSearched = ref(false)
const searching = ref(false)

async function searchSkills() {
  const q = skillSearch.value.trim() || 'backup monitor docker git'
  searching.value = true; skillSearched.value = true
  try {
    const res = await fetch('/api/market/search-skills', { method: 'POST', headers: authHeaders(), body: JSON.stringify({ query: q }) })
    const data = await res.json()
    skillResults.value = (data.data?.items || []).map(s => ({ ...s, _installed: isSkillInstalled(s.name) }))
  } catch (e) { skillResults.value = [] }
  searching.value = false
}

// ── MCP ──
const mcpSearch = ref('')
const mcpResults = ref([])
const popularMCP = ref([])

async function loadPopularMCP() {
  try {
    const res = await fetch('/api/market/popular-mcp', { headers: authHeaders() })
    const data = await res.json()
    popularMCP.value = (data.data?.items || []).map(m => ({ ...m, _installed: isMCPInstalled(m.name) }))
  } catch (e) {}
}

async function searchMCP() {
  const q = mcpSearch.value.trim()
  if (!q) return
  searching.value = true
  try {
    const res = await fetch('/api/chat/command', { method: 'POST', headers: authHeaders(), body: JSON.stringify({ command: '/get-mcp ' + q, session_id: '' }) })
    const data = await res.json()
    if (data.code === 200) {
      const result = data.data?.result || ''
      if (result.includes('✅') && result.includes('已安装')) {
        resultModal.value = result; resultTitle.value = '安装结果'
        loadPopularMCP(); loadInstalled()
      } else {
        mcpResults.value = parseMCPSearchResults(result)
      }
    }
  } catch (e) { mcpResults.value = [] }
  searching.value = false
}

function parseMCPSearchResults(text) {
  const results = []; let cur = null
  for (const line of text.split('\n')) {
    const m = line.match(/^\s*\d+\.\s+\*\*(.+?)\*\*\s+`v(.+?)`\s*(?:\(匹配度:\s*(\d+)%\))?/)
    if (m) { if (cur) results.push(cur); cur = { name: m[1].trim(), desc: '', score: parseInt(m[3])||0, _installed: isMCPInstalled(m[1].split('/').pop()?.replace(/^server-|^mcp-/, '') || m[1]) } }
    else if (cur && line.trim() && !line.startsWith('──') && !line.startsWith('安装:') && !line.startsWith('例如:')) { cur.desc = (cur.desc + ' ' + line.trim()).trim() }
  }
  if (cur) results.push(cur)
  return results
}

// ── Install ──
const installing = ref(null)
const installError = ref('')
const installSuccess = ref('')
const resultModal = ref('')
const resultTitle = ref('安装结果')

async function installSkill(s) {
  installing.value = s.name
  try {
    const res = await fetch('/api/market/install-skill', { method: 'POST', headers: authHeaders(), body: JSON.stringify({ download_url: s.download_url }) })
    const data = await res.json()
    resultTitle.value = '技能安装'
    resultModal.value = data.data?.result || data.message || '安装完成'
    if (data.code === 200) { s._installed = true; loadInstalled() }
  } catch (e) { resultModal.value = '安装失败: ' + e.message }
  installing.value = null
}

// ── 安装（后端异步 + 前端轮询）──
let _pollTimer = null
const router = useRouter()

async function installMCP(pkg) {
  const token = localStorage.getItem('token')
  installing.value = pkg
  installError.value = ''

  try {
    const res = await fetch('/api/market/install-mcp', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + token },
      body: JSON.stringify({ package: pkg })
    })
    const data = await res.json()

    if (!res.ok || data.code !== 200) {
      installError.value = data.message || '安装请求失败'
      installing.value = null
      return
    }

    if (data.data?.status === 'need_params') {
      mcpConfigParams.value = data.data.need_params || []
      mcpConfigPkg.value = pkg
      showMCPConfig.value = true
      installing.value = null
      return
    }

    // 安装成功 — 刷新列表
    installSuccess.value = data.data?.server_name || pkg
    setTimeout(() => { installSuccess.value = '' }, 4000)
    loadPopularMCP(); loadInstalled()
  } catch (e) {
    installError.value = e.message || '网络错误'
  }
  installing.value = null
}

async function loadInstallTasks() {
  try {
    const t = localStorage.getItem('token')
    if (!t) return
    const res = await fetch('/api/market/install-tasks', { headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + t } })
    const data = await res.json()
    if (data.code === 200 && data.data?.tasks?.length) {
      installTasks.value = data.data.tasks.map(tk => ({
        id: tk.id, package: tk.package, status: tk.status,
        progress: tk.progress, steps: tk.steps || [], error: tk.error || ''
      }))
      const allDone = installTasks.value.every(tk => tk.status !== 'running')
      if (allDone && _pollTimer) { clearInterval(_pollTimer); _pollTimer = null }
      loadPopularMCP(); loadInstalled()
    }
  } catch (e) {}
}

// ── MCP Config Form ──
const showMCPConfig = ref(false)
const mcpConfigPkg = ref('')
const mcpConfigParams = ref([])
const mcpConfigValues = ref({})

async function submitMCPConfig() {
  const pkg = mcpConfigPkg.value
  const env = { ...mcpConfigValues.value }
  for (const p of mcpConfigParams.value) {
    if (!env[p.name] && p.default) env[p.name] = p.default
  }
  showMCPConfig.value = false
  mcpConfigValues.value = {}

  const token = localStorage.getItem('token')
  try {
    const res = await fetch('/api/market/install-mcp', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + token },
      body: JSON.stringify({ package: pkg, env })
    })
    const data = await res.json()
    const sid = data.data?.session_id
    if (sid) { router.push('/chat/' + sid) }
  } catch (e) {}
  loadPopularMCP(); loadInstalled()
}

// ── Installed ──
const installedSkills = ref([])
const installedMCPServers = ref([])

const installedSkillNames = ref(new Set())
const installedMCPNames = ref(new Set())
function isSkillInstalled(n) { return installedSkillNames.value.has(n) }
function isMCPInstalled(n) { return installedMCPNames.value.has(n) }

async function loadInstalled() {
  try {
    // Skills
    const sRes = await fetch('/api/market/installed', { headers: authHeaders() })
    const sData = await sRes.json()
    if (sData.code === 200) {
      installedSkills.value = sData.data?.skills || []
      installedSkillNames.value = new Set(installedSkills.value.map(s => s.name))
    }
    // MCP via tools list
    const tRes = await fetch('/api/chat/tools', { headers: authHeaders() })
    const tools = await tRes.json()
    const mcpNames = new Set(); const servers = []
    for (const t of (tools || [])) {
      if (t.name?.startsWith('mcp_')) {
        const parts = t.name.split('_')
        if (parts.length >= 2 && !mcpNames.has(parts[1])) {
          mcpNames.add(parts[1])
          servers.push({ name: parts[1], command: 'npx', args: ['-y', (t.description || '').replace(/\[MCP:[^\]]+\]\s*/, '')] })
        }
      }
    }
    installedMCPServers.value = servers
    installedMCPNames.value = mcpNames
    // Refresh installed status on visible items
    skillResults.value.forEach(s => { s._installed = isSkillInstalled(s.name) })
    popularMCP.value.forEach(m => { m._installed = isMCPInstalled(m.name) })
    mcpResults.value.forEach(r => { r._installed = isMCPInstalled(r.name) })
  } catch (e) {}
}

async function runSkill(name) {
  try {
    const res = await fetch('/api/chat/command', { method: 'POST', headers: authHeaders(), body: JSON.stringify({ command: '/' + name, session_id: '' }) })
    const data = await res.json()
    resultTitle.value = '执行: /' + name
    resultModal.value = data.data?.result || '执行完成'
  } catch (e) { resultModal.value = '错误: ' + e.message }
}

onMounted(() => {
  searchSkills()
  loadPopularMCP()
  loadInstalled()
  loadInstallTasks() // 恢复刷新前的安装任务
})
</script>

<style scoped>
.market-page { display: flex; flex-direction: column; gap: 14px; }
.market-page h3 { margin: 0; font-size: var(--text-xl); font-weight: var(--weight-bold); color: var(--text-primary); }

.install-banner { padding: 10px 14px; border-radius: 8px; font-size: 13px; display: flex; justify-content: space-between; align-items: center; }
.install-banner.error { background: rgba(239,68,68,0.1); border: 1px solid rgba(239,68,68,0.3); color: var(--color-danger); }
.install-banner.success { background: rgba(34,197,94,0.1); border: 1px solid rgba(34,197,94,0.3); color: #16a34a; }
.install-banner button { background: none; border: none; color: inherit; cursor: pointer; font-size: 14px; padding: 0 4px; }

.tab-bar { display: flex; gap: 4px; }
.tab { display: flex; align-items: center; gap: 6px; padding: 8px 16px; border: 1px solid var(--border-default); border-radius: 10px; background: transparent; color: var(--text-secondary); cursor: pointer; font-size: var(--text-sm); font-family: inherit; transition: all 0.12s; }
.tab:hover { background: var(--surface-hover); color: var(--text-primary); }
.tab.active { background: var(--brand-50); color: var(--brand-600); border-color: var(--brand-300); font-weight: var(--weight-semibold); }
[data-theme="dark"] .tab.active { background: rgba(6,182,212,0.12); color: #22d3ee; border-color: rgba(34,211,238,0.25); }

.tab-content { display: flex; flex-direction: column; gap: 16px; }
.section { display: flex; flex-direction: column; gap: 8px; }
.section h4 { margin: 0; font-size: var(--text-md); font-weight: var(--weight-semibold); color: var(--text-primary); }

.action-bar { display: flex; gap: 8px; align-items: center; }
.search-wrap { display: flex; align-items: center; gap: 6px; padding: 6px 10px; border: 1px solid var(--border-default); border-radius: 8px; background: rgba(255,255,255,0.3); }
.search-inp { border: none; background: transparent; outline: none; font-size: 13px; color: var(--text-primary); width: 200px; font-family: inherit; }
.search-inp::placeholder { color: var(--text-muted); }

.btn-accent { display: flex; align-items: center; gap: 4px; padding: 7px 14px; border-radius: 8px; border: 1px solid var(--brand-300); background: var(--brand-50); color: var(--brand-600); cursor: pointer; font-size: 12.5px; font-family: inherit; font-weight: 500; white-space: nowrap; transition: all 0.12s; }
.btn-accent:hover { background: var(--brand-100); }
.btn-accent:disabled { opacity: 0.5; cursor: default; }
[data-theme="dark"] .btn-accent { background: rgba(6,182,212,0.12); color: #22d3ee; border-color: rgba(34,211,238,0.25); }

.card-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 8px; }
.card-item { display: flex; flex-direction: column; gap: 6px; padding: 12px 14px; border-radius: 10px; background: var(--glass-bg-card); border: 1px solid var(--glass-border); transition: all 0.12s; position: relative; }
.card-item:hover { border-color: var(--brand-200); }
.card-item.installed { opacity: 0.65; }
.card-head { display: flex; align-items: center; gap: 8px; }
.card-name { font-size: 13px; font-weight: 600; color: var(--text-primary); flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.card-cat { font-size: 10px; padding: 1px 6px; border-radius: 4px; background: var(--brand-50); color: var(--brand-600); font-weight: 500; flex-shrink: 0; }
.card-desc { font-size: 11px; color: var(--text-muted); line-height: 1.5; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
.card-meta { font-size: 10px; color: var(--text-muted); display: flex; gap: 10px; }
.card-action { align-self: flex-end; padding: 4px 12px; border-radius: 6px; border: 1px solid var(--border-default); background: transparent; color: var(--text-secondary); cursor: pointer; font-size: 11px; font-family: inherit; transition: all 0.12s; margin-top: auto; }
.card-action:hover { background: var(--surface-hover); }
.card-action.accent { border-color: var(--brand-300); color: var(--brand-600); }
.card-action.accent:hover { background: var(--brand-50); }
.card-action.done { border-color: var(--color-success); color: var(--color-success); cursor: default; }
.card-action:disabled { opacity: 0.5; }

.hint-banner { text-align: center; padding: 30px; color: var(--text-muted); background: var(--glass-bg-card); border-radius: 12px; border: 1px dashed var(--border-default); }
.hint-banner p { margin: 0; font-size: 13px; }

.empty-state { text-align: center; padding: 40px; color: var(--text-muted); }
.hint { font-size: 11px; color: var(--text-muted); }

.modal-overlay { position: fixed; inset: 0; z-index: 500; background: rgba(15,23,42,0.3); backdrop-filter: blur(6px); display: flex; align-items: center; justify-content: center; }
.modal-card { background: var(--glass-bg-elevated); border: 1px solid var(--glass-border-strong); border-radius: var(--radius-lg); padding: 20px; min-width: 320px; max-width: 540px; display: flex; flex-direction: column; gap: 12px; box-shadow: var(--glass-shadow-elevated); }
.modal-card h4 { margin: 0; font-size: 15px; font-weight: 600; }
.modal-actions { display: flex; justify-content: flex-end; }
.btn-ok { padding: 7px 16px; border-radius: 8px; border: none; background: var(--gradient-brand-btn); color: #fff; cursor: pointer; font-size: 12.5px; font-family: inherit; font-weight: 500; }
.install-result { margin: 0; padding: 10px; background: var(--code-bg); border-radius: 6px; font-size: 12px; font-family: var(--font-mono); white-space: pre-wrap; max-height: 300px; overflow: auto; color: var(--text-primary); }

@media (max-width: 767px) {
  .card-grid { grid-template-columns: 1fr; }
  .action-bar { flex-direction: column; align-items: stretch; }
}
</style>
