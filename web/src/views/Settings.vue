<template>
  <div class="settings-page">
    <div class="settings-header">
      <h3>系统设置</h3><span
        v-if="store.hasDirty"
        class="dirty-bar"
      >⚡ {{ store.dirtyList.length }} 个区段未保存 <button
        class="save-all-btn"
        @click="saveAll"
      >全部保存</button></span>
    </div>

    <div v-if="store.config" class="settings-layout">
      <!-- Left Sidebar -->
      <aside class="settings-sidebar">
        <nav class="sidebar-nav">
          <template v-for="group in navGroups" :key="group.label">
            <div class="nav-group-label">{{ group.label }}</div>
            <div class="nav-group">
              <button
                v-for="item in group.items"
                :key="item.id"
                :class="['nav-item', { active: activeSection === item.id }]"
                @click="scrollToSection(item.id)"
              >
                <span class="nav-icon" v-html="item.icon"></span>
                <span class="nav-label">{{ item.label }}</span>
                <span v-if="item.section && store.isDirty(item.section)" class="nav-dirty-dot" />
              </button>
            </div>
          </template>
        </nav>
      </aside>

      <!-- Right Scrollable Content -->
      <main class="settings-content" ref="contentRef">
        <!-- DNS -->
        <section id="sec-dns" class="setting-section" data-section="dns">
          <div class="setting-card">
            <div class="card-head">
              <span class="card-icon"><svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="8" cy="8" r="6"/><ellipse cx="8" cy="8" rx="3" ry="6"/><path d="M2 8h12M8 2v12"/></svg></span>
              <span class="card-title">DNS 配置</span>
              <span v-if="store.isDirty('dns')" class="dirty-tag">● 未保存</span>
              <button class="card-save-btn" :class="{ saved: savedOk.has('dns') }" :disabled="store.saving.has('dns')" @click="doSaveSection('dns')">
                {{ store.saving.has('dns') ? '...' : savedOk.has('dns') ? '✓ 已保存' : '保存' }}
              </button>
            </div>
            <form class="card-body" @submit.prevent>
              <div class="sub-section">
                <h5>Aliyun<a href="https://ram.console.aliyun.com/manage/ak" target="_blank" class="card-link-tag">AK ↗</a></h5>
                <div class="field"><label>AccessKey ID</label><a-input :model-value="store.localValues.dns?.aliyun?.access_key_id" size="small" @update:model-value="store.setField('dns','aliyun.access_key_id',$event)"/></div>
                <div class="field"><label>AK Secret</label><span class="secret-wrap"><input :value="store.localValues.dns?.aliyun?.access_key_secret" :type="(store.localValues.dns?.aliyun?.has_secret || store.localValues.dns?.aliyun?.access_key_secret) ? 'password' : 'text'" :placeholder="store.localValues.dns?.aliyun?.has_secret && !store.localValues.dns?.aliyun?.access_key_secret ? '已设置' : ''" class="secure-input" autocomplete="off" @input="store.setField('dns','aliyun.access_key_secret',$event.target.value)"/><button v-if="store.localValues.dns?.aliyun?.access_key_secret" class="clear-btn" title="清空" @click="store.setField('dns','aliyun.access_key_secret',''); store.setField('dns','aliyun._cleared',true)">&times;</button></span></div>
              </div>
            </form>
          </div>
        </section>

        <!-- Notify -->
        <section id="sec-notify" class="setting-section" data-section="notify">
          <div class="setting-card">
            <div class="card-head">
              <span class="card-icon"><svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M7 1.5c-2.5 0-4 2-4 4v2L1.5 9v1.5h13V9L13 7.5v-2c0-2-1.5-4-4-4z"/><path d="M5.5 12.5a2 2 0 004 0"/></svg></span>
              <span class="card-title">通知</span>
              <span v-if="store.isDirty('notify')" class="dirty-tag">● 未保存</span>
              <button class="card-save-btn" :disabled="store.saving.has('notify')" @click="doSaveSection('notify')">{{ store.saving.has('notify') ? '...' : '保存' }}</button>
            </div>
            <form class="card-body" @submit.prevent>
              <div class="sub-section"><h5>邮件</h5>
                <div class="field"><label>SMTP 主机</label><a-input :model-value="store.localValues.notify?.email?.host" size="small" @update:model-value="store.setField('notify','email.host',$event)"/></div>
                <div class="field row-2"><span><label>端口</label><a-input-number :model-value="store.localValues.notify?.email?.port" :min="1" :max="65535" size="small" style="width:90px" @update:model-value="store.setField('notify','email.port',$event)"/></span><span><label>发件人</label><a-input :model-value="store.localValues.notify?.email?.user" size="small" @update:model-value="store.setField('notify','email.user',$event)"/></span></div>
                <div class="field"><label>密码</label><span class="secret-wrap"><input :value="store.localValues.notify?.email?.password" :type="(store.localValues.notify?.email?.has_password || store.localValues.notify?.email?.password) ? 'password' : 'text'" :placeholder="store.localValues.notify?.email?.has_password && !store.localValues.notify?.email?.password ? '已设置' : ''" class="secure-input" autocomplete="off" @input="store.setField('notify','email.password',$event.target.value)"/><button v-if="store.localValues.notify?.email?.password" class="clear-btn" @click="store.setField('notify','email.password',''); store.setField('notify','email._cleared',true)">&times;</button></span></div>
                <div class="field"><label>收件人</label><a-input :model-value="store.localValues.notify?.email?.to" size="small" @update:model-value="store.setField('notify','email.to',$event)"/></div>
              </div>
              <div class="sub-section"><h5>Webhook</h5><div class="field"><label>URL</label><a-input :model-value="store.localValues.notify?.webhook?.url" size="small" @update:model-value="store.setField('notify','webhook.url',$event)"/></div></div>
              <div class="sub-section"><h5>钉钉</h5><div class="field"><label>Webhook URL</label><a-input :model-value="store.localValues.notify?.dingtalk?.webhook_url" size="small" @update:model-value="store.setField('notify','dingtalk.webhook_url',$event)"/></div></div>
            </form>
          </div>
        </section>

        <!-- QQ Bot -->
        <section id="sec-qqbot" class="setting-section" data-section="qqbot">
          <div class="setting-card">
            <div class="card-head">
              <span class="card-icon"><svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M14 11c0 1.1-.9 2-2 2h-2l-3 3.5V13H5c-1.1 0-2-.9-2-2V5c0-1.1.9-2 2-2h8c1.1 0 2 .9 2 2v6z"/></svg></span>
              <span class="card-title">QQ Bot</span>
              <span class="card-link-tag">{{ botStatusText }}</span>
              <span v-if="store.isDirty('qqbot')" class="dirty-tag">● 未保存</span>
              <button class="card-save-btn" :disabled="store.saving.has('qqbot')" @click="doSaveSection('qqbot')">{{ store.saving.has('qqbot') ? '...' : '保存' }}</button>
            </div>
            <div class="card-body">
              <div class="field row-2">
                <span><label>App ID</label><a-input :model-value="store.localValues.qqbot?.app_id" size="small" style="width:160px" @update:model-value="store.setField('qqbot','app_id',$event)"/></span>
                <span><label>App Secret</label><span class="secret-wrap"><input :value="store.localValues.qqbot?.app_secret" :type="(store.localValues.qqbot?.has_secret || store.localValues.qqbot?.app_secret) ? 'password' : 'text'" :placeholder="store.localValues.qqbot?.has_secret && !store.localValues.qqbot?.app_secret ? '已设置' : ''" class="secure-input" style="width:160px" autocomplete="off" @input="store.setField('qqbot','app_secret',$event.target.value)"/><button v-if="store.localValues.qqbot?.app_secret" class="clear-btn" @click="store.setField('qqbot','app_secret',''); store.setField('qqbot','_cleared',true)">&times;</button></span></span>
                <span><label>群 ID</label><a-input :model-value="store.localValues.qqbot?.group_id" size="small" style="width:120px" @update:model-value="store.setField('qqbot','group_id',$event)"/></span>
              </div>
            </div>
          </div>
        </section>

        <!-- AI Providers -->
        <section v-for="p in aiProviderDefs" :key="'ai-'+p.key" :id="'sec-ai-'+p.key" class="setting-section" data-section="ai">
          <div class="setting-card">
            <div class="card-head">
              <span class="card-icon"><svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M8 2.5c-1.5 0-2.5 1-2.5 2.5v1.5L2 8.5V15h12V8.5l-3.5-2V5c0-1.5-1-2.5-2.5-2.5z"/><circle cx="8" cy="10" r="1.2"/></svg></span>
              <span class="card-title">{{ p.label }}</span>
              <a :href="p.apiKeyUrl" target="_blank" class="card-link-tag">Keys ↗</a>
              <span v-if="store.testResults[p.key]" :class="store.testResults[p.key].ok ? 'ai-status-ok' : 'ai-status-fail'">{{ store.testResults[p.key].ok ? '✓ ' : '✗ ' }}{{ store.testResults[p.key].ok ? '已连接' : store.testResults[p.key].error }}</span>
              <button class="card-save-btn secondary" :disabled="aiTestLoading === p.key" @click="doTest(p.key)">{{ aiTestLoading === p.key ? '...' : '测试' }}</button>
              <button class="card-save-btn" :disabled="aiSaveLoading === p.key" @click="doSaveProvider(p.key)">{{ aiSaveLoading === p.key ? '...' : '保存' }}</button>
            </div>
            <div class="card-body">
              <div class="field"><label>API Key</label><span class="secret-wrap"><input :value="store.localValues.ai?.[p.key]?.api_key" :type="(store.localValues.ai?.[p.key]?.has_key || store.localValues.ai?.[p.key]?.api_key) ? 'password' : 'text'" :placeholder="store.localValues.ai?.[p.key]?.has_key && !store.localValues.ai?.[p.key]?.api_key ? '已设置' : ''" class="secure-input" autocomplete="off" @input="store.setField('ai',p.key+'.api_key',$event.target.value)"/><button v-if="store.localValues.ai?.[p.key]?.api_key" class="clear-btn" @click="store.setField('ai',p.key+'.api_key',''); store.setField('ai',p.key+'._cleared',true)">&times;</button></span></div>
            </div>
          </div>
        </section>

        <!-- AI Default Settings -->
        <section id="sec-ai-default" class="setting-section" data-section="ai">
          <div class="setting-card">
            <div class="card-head">
              <span class="card-icon"><svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M6 2.5a1.5 1.5 0 0 0 0 3h4a1.5 1.5 0 0 0 0-3H6z"/><path d="M3.5 8a1.5 1.5 0 0 1 1.5-1.5h6a1.5 1.5 0 0 1 0 3H5A1.5 1.5 0 0 1 3.5 8z"/><path d="M2 13.5a1.5 1.5 0 0 1 1.5-1.5h9a1.5 1.5 0 0 1 0 3h-9A1.5 1.5 0 0 1 2 13.5z"/></svg></span>
              <span class="card-title">默认设置</span>
              <button class="card-save-btn" :disabled="store.saving.has('ai')" @click="doSaveSection('ai')">{{ store.saving.has('ai') ? '...' : '保存' }}</button>
            </div>
            <div class="card-body">
              <div class="field row-2">
                <span><label>默认模型</label><a-select :model-value="store.localValues.ai?.default_model" size="small" style="width:180px" @update:model-value="store.setField('ai','default_model',$event)"><a-option v-for="m in allModelDefs" :key="m.value" :value="m.value">{{ m.label }}</a-option></a-select></span>
                <span><label>推理深度</label><a-select :model-value="store.localValues.ai?.default_reasoning" size="small" style="width:100px" @update:model-value="store.setField('ai','default_reasoning',$event)"><a-option value="low">低</a-option><a-option value="medium">中</a-option><a-option value="high">高</a-option></a-select></span>
              </div>
              <div class="sub-section"><h5>偏好</h5>
                <div class="field"><label>流式展开思考</label><a-switch :model-value="store.localValues.ai?.expand_thinking_on_stream" @update:model-value="store.setField('ai','expand_thinking_on_stream',$event)"/></div>
                <div class="field-desc">开启后，AI 在流式输出时会自动展开显示完整的思考内容，结束后自动收起。</div>
              </div>
            </div>
          </div>
        </section>

        <!-- Storage -->
        <section id="sec-database" class="setting-section" data-section="database">
          <div class="setting-card">
            <div class="card-head">
              <span class="card-icon"><svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><ellipse cx="8" cy="8" rx="3" ry="7"/><path d="M1 8c0-3.9 3.1-7 7-7s7 3.1 7 7-3.1 7-7 7-7-3.1-7-7z"/><path d="M1 8h14"/></svg></span>
              <span class="card-title">数据存储</span>
              <span class="card-link-tag">{{ (store.localValues.database?.driver) || 'sqlite' }}</span>
              <span v-if="store.isDirty('database')" class="dirty-tag">● 未保存</span>
              <button class="card-save-btn" :disabled="store.saving.has('database')" @click="doSaveSection('database')">{{ store.saving.has('database') ? '...' : '保存' }}</button>
            </div>
            <div class="card-body">
              <div class="field"><label>存储驱动</label><a-select :model-value="store.localValues.database?.driver" size="small" style="width:130px" @update:model-value="store.setField('database','driver',$event); onDriverChange($event)"><a-option value="sqlite">SQLite</a-option><a-option value="mysql">MySQL</a-option><a-option value="file">File JSON</a-option></a-select></div>
              <div v-if="(store.localValues.database?.driver || 'sqlite') === 'sqlite' || (store.localValues.database?.driver || 'sqlite') === 'file'" class="sub-section">
                <div class="field"><label>{{ (store.localValues.database?.driver || 'sqlite') === 'sqlite' ? '数据库路径' : '数据目录' }}</label><a-input :model-value="store.localValues.database?.path" size="small" @update:model-value="store.setField('database','path',$event)"/></div>
              </div>
              <div v-if="(store.localValues.database?.driver || 'sqlite') === 'mysql'" class="sub-section">
                <h5>MySQL 连接</h5>
                <div class="field row-2"><span><label>主机</label><a-input :model-value="store.localValues.database?.mysql?.host" size="small" style="width:140px" @update:model-value="store.setField('database','mysql.host',$event)"/></span><span><label>端口</label><a-input-number :model-value="store.localValues.database?.mysql?.port" :min="1" :max="65535" size="small" style="width:80px" @update:model-value="store.setField('database','mysql.port',$event)"/></span></div>
                <div class="field row-2"><span><label>用户名</label><a-input :model-value="store.localValues.database?.mysql?.user" size="small" style="width:140px" @update:model-value="store.setField('database','mysql.user',$event)"/></span><span><label>密码</label><span class="secret-wrap"><input :value="store.localValues.database?.mysql?.password" :type="(store.localValues.database?.mysql?.has_password || store.localValues.database?.mysql?.password) ? 'password' : 'text'" :placeholder="store.localValues.database?.mysql?.has_password && !store.localValues.database?.mysql?.password ? '已设置' : ''" class="secure-input" style="width:140px" autocomplete="off" @input="store.setField('database','mysql.password',$event.target.value)"/><button v-if="store.localValues.database?.mysql?.password" class="clear-btn" @click="store.setField('database','mysql.password',''); store.setField('database','mysql._cleared',true)">&times;</button></span></span></div>
                <div class="field"><label>数据库名</label><a-input :model-value="store.localValues.database?.mysql?.dbname" size="small" style="width:140px" @update:model-value="store.setField('database','mysql.dbname',$event)"/></div>
              </div>
            </div>
          </div>
        </section>

        <!-- Log -->
        <section id="sec-log" class="setting-section" data-section="log">
          <div class="setting-card">
            <div class="card-head">
              <span class="card-icon"><svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M3 2h10a1 1 0 011 1v10a1 1 0 01-1 1H3a1 1 0 01-1-1V3a1 1 0 011-1z"/><line x1="3" y1="6" x2="13" y2="6"/><line x1="5" y1="4" x2="5" y2="6"/><line x1="7" y1="9" x2="11" y2="9"/><line x1="7" y1="11" x2="9" y2="11"/></svg></span>
              <span class="card-title">日志</span>
              <span v-if="store.isDirty('log')" class="dirty-tag">● 未保存</span>
              <button class="card-save-btn" :disabled="store.saving.has('log')" @click="doSaveSection('log')">{{ store.saving.has('log') ? '...' : '保存' }}</button>
            </div>
            <div class="card-body">
              <div class="field row-2">
                <span><label>级别</label><a-select :model-value="store.localValues.log?.level" size="small" style="width:110px" @update:model-value="store.setField('log','level',$event)"><a-option value="debug">DEBUG</a-option><a-option value="info">INFO</a-option><a-option value="warn">WARN</a-option><a-option value="error">ERROR</a-option></a-select></span>
                <span><label>格式</label><a-select :model-value="store.localValues.log?.format" size="small" style="width:100px" @update:model-value="store.setField('log','format',$event)"><a-option value="text">文本</a-option><a-option value="json">JSON</a-option></a-select></span>
                <span><label>保留天数</label><a-input-number :model-value="store.localValues.log?.max_days" :min="1" :max="365" size="small" style="width:80px" @update:model-value="store.setField('log','max_days',$event)"/></span>
              </div>
            </div>
          </div>
        </section>

        <!-- Prompts -->
        <section id="sec-prompts" class="setting-section">
          <PromptsCard />
        </section>

        <!-- Users -->
        <section id="sec-users" class="setting-section">
          <div class="setting-card">
            <div class="card-head">
              <span class="card-icon"><svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="1.5" y="1.5" width="13" height="13" rx="2.5"/><circle cx="8" cy="5.5" r="2"/><path d="M3.5 12c0-2 2-4 4.5-4s4.5 2 4.5 4"/></svg></span><span class="card-title">用户管理</span>
            </div>
            <div class="card-body">
              <div class="perm-create">
                <a-input v-model="newUser.username" size="small" placeholder="用户名" style="width:110px"/>
                <a-input-password v-model="newUser.password" size="small" placeholder="密码" style="width:110px"/>
                <a-select v-model="newUser.role" size="small" style="width:75px"><a-option value="user">user</a-option><a-option value="admin">admin</a-option></a-select>
                <a-input-number v-model="newUser.storage_quota" size="small" :min="0" placeholder="配额(GB)" style="width:80px"><template #suffix>GB</template></a-input-number>
                <a-button size="small" type="primary" :loading="creatingUser" @click="createUser">创建</a-button>
              </div>
              <div v-if="users.length" class="perm-table">
                <div class="perm-hd"><span>用户名</span><span>角色</span><span>配额</span><span>已用</span><span>时间</span><span></span></div>
                <div v-for="u in users" :key="u.id" class="perm-row">
                  <span class="perm-name">{{ u.username }}</span>
                  <span :class="['perm-role', u.role]">{{ u.role === 'admin' ? '管理员' : '用户' }}</span>
                  <span class="perm-quota">{{ u.storage_quota ? u.storage_quota+'GB' : '无限制' }}</span>
                  <span class="perm-used">{{ formatBytes(u.storage_used || 0) }}</span>
                  <span class="perm-time">{{ u.created_at?.slice(0,10) }}</span>
                  <span><a-button size="mini" @click="editUser(u)">编辑</a-button><a-button size="mini" status="danger" @click="deleteUser(u.id)">删除</a-button></span>
                </div>
              </div>
            </div>
          </div>
        </section>
      </main>
    </div>

    <Transition name="modal">
      <div v-if="showUserEdit" class="modal-overlay" @click.self="showUserEdit = false">
        <div class="modal-card">
          <div class="modal-head"><span>编辑用户</span><button class="modal-close" @click="showUserEdit = false">✕</button></div>
          <div class="modal-body">
            <div class="field"><label>角色</label><a-select v-model="userEditForm.role" size="small" style="width:120px"><a-option value="user">user</a-option><a-option value="admin">admin</a-option></a-select></div>
            <div class="field"><label>配额</label><a-input-number v-model="userEditForm.storage_quota" size="small" :min="0" style="width:100px"/></div>
            <div class="field"><label>新密码</label><a-input-password v-model="userEditForm.password" size="small" placeholder="留空不修改" style="width:160px"/></div>
          </div>
          <div class="modal-actions"><button class="btn-cancel" @click="showUserEdit = false">取消</button><button class="btn-save" :disabled="savingUser" @click="doEditUser">{{ savingUser ? '...' : '保存' }}</button></div>
        </div>
      </div>
    </Transition>

    <ConfirmDialog :visible="confirmDialog.visible" :title="confirmDialog.title" :message="confirmDialog.message" :confirm-text="confirmDialog.confirmText" :variant="confirmDialog.variant" icon="warn" @confirm="confirmDialog.visible = false; confirmDialog.resolve(true)" @cancel="confirmDialog.visible = false; confirmDialog.resolve(false)"/>
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref, reactive, computed, onMounted, nextTick } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import api from '../services/api'
import { formatBytes } from '../composables/useFormat'
import { useToast } from '../composables/useToast.js'
import ConfirmDialog from '../components/ui/ConfirmDialog.vue'
import PromptsCard from '../components/settings/PromptsCard.vue'
import { useSettingsStore } from '../stores/settings'

const toast = useToast()
const store = useSettingsStore()
const users = ref([])
const creatingUser = ref(false)
const savingUser = ref(false)
const newUser = reactive({ username: '', password: '', role: 'user', storage_quota: 0 })
const showUserEdit = ref(false)
const userEditForm = reactive({ id: 0, role: 'user', storage_quota: 0, password: '' })
const aiTestLoading = ref('')
const aiSaveLoading = ref('')
const savedOk = reactive(new Set())

async function doSaveSection(section) {
  const r = await store.saveSection(section)
  if (r.ok) { savedOk.add(section); setTimeout(() => savedOk.delete(section), 2000) }
  return r
}

function onDriverChange(driver) {
  if (driver === 'sqlite' && !store.localValues.database?.path) store.setField('database', 'path', './data/yunxi-home.db')
  else if (driver === 'file' && !store.localValues.database?.path) store.setField('database', 'path', './data')
}

const aiProviderDefs = [
  { key: 'deepseek', label: 'DeepSeek', apiKeyUrl: 'https://platform.deepseek.com/api_keys' },
  { key: 'qwen', label: 'Qwen 通义千问', apiKeyUrl: 'https://bailian.console.aliyun.com/#/api-key' },
]
const allModelDefs = [
  { value: 'deepseek-v4-flash', label: 'deepseek-v4-flash' }, { value: 'deepseek-v4-pro', label: 'deepseek-v4-pro' },
  { value: 'qwen-plus', label: 'qwen-plus' }, { value: 'qwen-max', label: 'qwen-max' },
]

const botStatusText = computed(() => {
  const q = store.qqbotConfig
  if (q.online) return '✔ ' + (q.username || '在线')
  if (q.enabled) return '⚫ 已启用'
  if (q.app_id && q.app_secret) return '⚫ 已配置·未启用'
  return '⚫ 未配置'
})

const confirmDialog = reactive({ visible: false, title: '', message: '', confirmText: '确定', variant: 'danger', resolve: (_) => {} })
function showConfirm(title, msg, opts = {}) { return new Promise(r => { Object.assign(confirmDialog, { visible: true, title, message: msg, confirmText: opts.confirmText || '确定', variant: opts.variant || 'danger', resolve: r }) }) }

async function doTest(key) { aiTestLoading.value = key; await store.testProvider(key); aiTestLoading.value = '' }
async function doSaveProvider(key) { aiSaveLoading.value = key; await store.saveSection('ai'); aiSaveLoading.value = '' }
async function saveAll() { const r = await store.saveAll(); toast.success('已保存 ' + r.filter(x => x.ok).length + ' 个区段') }

function editUser(u) { Object.assign(userEditForm, { id: u.id, role: u.role, storage_quota: u.storage_quota || 0, password: '' }); showUserEdit.value = true }
async function doEditUser() {
  savingUser.value = true
  try { const b = { role: userEditForm.role, storage_quota: userEditForm.storage_quota }; if (userEditForm.password) b.password = userEditForm.password; await api.put('/api/admin/users/'+userEditForm.id, b); showUserEdit.value = false; loadUsers(); toast.success('已更新') }
  catch (e) { toast.error('编辑失败') } finally { savingUser.value = false }
}

async function loadUsers() { try { const r = await api.get('/api/admin/users'); users.value = r.data.data || [] } catch (_) {} }
async function createUser() { if (!newUser.username || !newUser.password) { toast.error('请填写用户名和密码'); return }; creatingUser.value = true; try { await api.post('/api/admin/users', { ...newUser }); Object.assign(newUser, { username: '', password: '', role: 'user', storage_quota: 0 }); loadUsers(); toast.success('已创建') } catch (e) { toast.error('创建失败') } finally { creatingUser.value = false } }
async function deleteUser(id) { if (!await showConfirm('删除', '确定删除此用户？')) return; try { await api.delete('/api/admin/users/'+id); loadUsers(); toast.success('已删除') } catch (_) {} }

// ── Sidebar navigation ──
const contentRef = ref<HTMLElement>()
const activeSection = ref('sec-dns')

const navGroups = [
  { label: '服务', items: [
    { id: 'sec-dns', section: 'dns', icon: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="8" cy="8" r="6"/><ellipse cx="8" cy="8" rx="3" ry="6"/><path d="M2 8h12M8 2v12"/></svg>', label: 'DNS 配置' },
    { id: 'sec-notify', section: 'notify', icon: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M7 1.5c-2.5 0-4 2-4 4v2L1.5 9v1.5h13V9L13 7.5v-2c0-2-1.5-4-4-4z"/><path d="M5.5 12.5a2 2 0 004 0"/></svg>', label: '通知' },
    { id: 'sec-qqbot', section: 'qqbot', icon: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M14 11c0 1.1-.9 2-2 2h-2l-3 3.5V13H5c-1.1 0-2-.9-2-2V5c0-1.1.9-2 2-2h8c1.1 0 2 .9 2 2v6z"/></svg>', label: 'QQ Bot' },
  ]},
  { label: 'AI', items: [
    { id: 'sec-ai-deepseek', section: 'ai', icon: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M8 2.5c-1.5 0-2.5 1-2.5 2.5v1.5L2 8.5V15h12V8.5l-3.5-2V5c0-1.5-1-2.5-2.5-2.5z"/><circle cx="8" cy="10" r="1.2"/></svg>', label: 'DeepSeek' },
    { id: 'sec-ai-qwen', section: 'ai', icon: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M8 2.5c-1.5 0-2.5 1-2.5 2.5v1.5L2 8.5V15h12V8.5l-3.5-2V5c0-1.5-1-2.5-2.5-2.5z"/><circle cx="8" cy="10" r="1.2"/></svg>', label: '通义千问' },
    { id: 'sec-ai-default', section: 'ai', icon: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M6 2.5a1.5 1.5 0 0 0 0 3h4a1.5 1.5 0 0 0 0-3H6z"/><path d="M3.5 8a1.5 1.5 0 0 1 1.5-1.5h6a1.5 1.5 0 0 1 0 3H5A1.5 1.5 0 0 1 3.5 8z"/><path d="M2 13.5a1.5 1.5 0 0 1 1.5-1.5h9a1.5 1.5 0 0 1 0 3h-9A1.5 1.5 0 0 1 2 13.5z"/></svg>', label: '默认 & 偏好' },
  ]},
  { label: '系统', items: [
    { id: 'sec-database', section: 'database', icon: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><ellipse cx="8" cy="8" rx="3" ry="7"/><path d="M1 8c0-3.9 3.1-7 7-7s7 3.1 7 7-3.1 7-7 7-7-3.1-7-7z"/><path d="M1 8h14"/></svg>', label: '数据存储' },
    { id: 'sec-log', section: 'log', icon: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M3 2h10a1 1 0 011 1v10a1 1 0 01-1 1H3a1 1 0 01-1-1V3a1 1 0 011-1z"/><line x1="3" y1="6" x2="13" y2="6"/></svg>', label: '日志' },
    { id: 'sec-prompts', section: null, icon: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 3h12v10H2z"/><line x1="5" y1="6" x2="11" y2="6"/><line x1="5" y1="9" x2="9" y2="9"/></svg>', label: 'AI 提示词' },
    { id: 'sec-users', section: null, icon: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="1.5" y="1.5" width="13" height="13" rx="2.5"/><circle cx="8" cy="5.5" r="2"/><path d="M3.5 12c0-2 2-4 4.5-4s4.5 2 4.5 4"/></svg>', label: '用户管理' },
  ]},
]

function scrollToSection(id: string) {
  const el = document.getElementById(id)
  if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

let _sectionObserver: IntersectionObserver | null = null

function initSectionObserver() {
  _sectionObserver = new IntersectionObserver((entries) => {
    for (const entry of entries) {
      if (entry.isIntersecting) { activeSection.value = entry.target.id; break }
    }
  }, { rootMargin: '-80px 0px -60% 0px' })
  nextTick(() => {
    contentRef.value?.querySelectorAll('.setting-section').forEach(el => {
      _sectionObserver?.observe(el)
    })
  })
}

onMounted(async () => { await store.load(); loadUsers(); initSectionObserver() })
onBeforeRouteLeave((_t, _f, next) => {
  if (store.hasDirty) showConfirm('未保存', '有未保存的配置，确定离开？', { variant: 'warning', confirmText: '离开' }).then(ok => ok ? next() : next(false))
  else next()
})
</script>

<style scoped>
.settings-page { display: flex; flex-direction: column; gap: 14px; height: 100%; overflow: hidden; }
.settings-header { display: flex; justify-content: space-between; align-items: center; flex-shrink: 0; }
.settings-header h3 { margin: 0; font-size: 18px; font-weight: 700; color: var(--text-primary); }
.dirty-bar { display: flex; align-items: center; gap: 8px; font-size: 12px; color: #d97706; font-weight: 500; }
.save-all-btn { padding: 2px 12px; border-radius: 6px; border: 1px solid #d97706; background: transparent; color: #d97706; cursor: pointer; font-size: 11px; }

.settings-layout { display: flex; gap: 24px; align-items: flex-start; flex: 1; min-height: 0; overflow: hidden; }

/* Sidebar */
.settings-sidebar { position: sticky; top: 0; width: 200px; min-width: 200px; flex-shrink: 0; padding-top: 4px; max-height: 100%; overflow-y: auto; }
.nav-group { margin-bottom: 8px; }
.nav-group-label { font-size: 10px; font-weight: 600; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.5px; padding: 6px 12px 2px; }
.sidebar-nav { display: flex; flex-direction: column; gap: 2px; }
.nav-item { display: flex; align-items: center; gap: 10px; width: 100%; padding: 7px 12px; border: none; border-radius: 8px; background: transparent; color: var(--text-secondary); font-size: 12.5px; font-family: inherit; cursor: pointer; text-align: left; transition: all 0.12s; }
.nav-item:hover { background: var(--surface-hover); color: var(--text-primary); }
.nav-item.active { background: var(--brand-50); color: var(--brand-600); font-weight: 600; }
[data-theme="dark"] .nav-item.active { background: rgba(6,182,212,0.12); color: #22d3ee; }
.nav-icon { display: flex; width: 16px; height: 16px; flex-shrink: 0; color: inherit; }
.nav-label { flex: 1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.nav-dirty-dot { width: 6px; height: 6px; border-radius: 50%; background: #d97706; flex-shrink: 0; }

/* Content */
.settings-content { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 14px; overflow-y: auto; }
.setting-section { scroll-margin-top: 80px; }

/* Cards (unchanged from original, adapted for full-width) */
.setting-card { background: var(--glass-bg-card); border: 1px solid var(--glass-border); border-radius: var(--radius-lg); overflow: hidden; }
.card-head { display: flex; align-items: center; gap: 10px; padding: 12px 16px; border-bottom: 1px solid var(--border-subtle); }
.card-icon { color: var(--brand-500); display: flex; }
.card-title { font-size: 13px; font-weight: 600; color: var(--text-primary); flex: 1; }
.card-link-tag { padding: 1px 8px; border-radius: 100px; border: 1px solid var(--border-default); color: var(--text-muted); font-size: 10px; text-decoration: none; }
.dirty-tag { font-size: 10px; color: #d97706; font-weight: 600; }
.card-save-btn { padding: 3px 14px; border-radius: 6px; border: 1px solid var(--brand-500); background: transparent; color: var(--brand-500); font-size: 11px; cursor: pointer; font-weight: 500; }
.card-save-btn:hover { background: var(--brand-500); color: #fff; }
.card-save-btn:disabled { opacity: 0.4; }
.card-save-btn.secondary { border-color: var(--border-default); color: var(--text-secondary); }

.card-body { padding: 14px 16px; display: flex; flex-direction: column; gap: 10px; }
.field { display: flex; align-items: center; gap: 10px; }
.field label { font-size: 11px; color: var(--text-muted); width: 80px; flex-shrink: 0; text-align: right; }
.field :deep(.arco-input-wrapper) { flex: 1; max-width: 260px; }
.field-desc { font-size: 11px; color: var(--text-muted); line-height: 1.5; padding-left: 90px; }
.row-2 { gap: 20px; }
.row-2 span { display: flex; align-items: center; gap: 6px; }
.row-2 label { width: auto; }
.secure-input { flex: 1; max-width: 260px; padding: 5px 10px; border: 1px solid var(--border-default); border-radius: 4px; background: var(--surface-input); color: var(--text-primary); font-size: 12px; font-family: var(--font-mono); outline: none; }
.secure-input:focus { border-color: var(--border-focus); }
.secret-wrap { position: relative; display: flex; align-items: center; flex: 1; max-width: 260px; }
.secret-wrap .secure-input { flex: 1; }
.clear-btn { position: absolute; right: 4px; width: 16px; height: 16px; border-radius: 50%; border: none; background: var(--text-muted); color: #fff; font-size: 8px; cursor: pointer; display: flex; align-items: center; justify-content: center; opacity: 0; transition: opacity 0.15s; }
.secret-wrap:hover .clear-btn { opacity: 0.6; }
.clear-btn:hover { opacity: 1 !important; background: var(--color-danger); }

.sub-section { display: flex; flex-direction: column; gap: 8px; padding: 8px 0; border-top: 1px solid var(--border-subtle); }
.sub-section:first-child { border-top: none; padding-top: 0; }
.sub-section h5 { margin: 0; font-size: 11px; font-weight: 600; color: var(--brand-600); text-transform: uppercase; }

.perm-create { display: flex; gap: 6px; flex-wrap: wrap; align-items: center; }
.perm-table { display: flex; flex-direction: column; margin-top: 4px; }
.perm-hd, .perm-row { display: grid; grid-template-columns: 1fr 55px 55px 55px 75px 80px; gap: 5px; align-items: center; padding: 6px 0; font-size: 11px; border-bottom: 1px solid var(--border-subtle); }
.perm-hd { color: var(--text-muted); font-weight: 600; }
.perm-name { font-weight: 500; color: var(--text-primary); }
.perm-role { font-size: 9px; padding: 1px 5px; border-radius: 3px; }
.perm-role.admin { background: #fef3c7; color: #a16207; }
.perm-role.user { background: var(--brand-50); color: var(--brand-600); }

.ai-status-ok { font-size: 10px; color: #22c55e; }
.ai-status-fail { font-size: 10px; color: #ef4444; }

.modal-overlay { position: fixed; inset: 0; z-index: 500; background: rgba(0,0,0,0.3); display: flex; align-items: center; justify-content: center; }
.modal-card { background: var(--surface-raised); border: 1px solid var(--border-default); border-radius: 14px; padding: 20px; max-width: 380px; width: 90vw; display: flex; flex-direction: column; gap: 14px; }
.modal-head { display: flex; align-items: center; justify-content: space-between; font-size: 15px; font-weight: 600; }
.modal-close { width: 28px; height: 28px; border-radius: 6px; border: none; background: transparent; cursor: pointer; }
.modal-body { display: flex; flex-direction: column; gap: 10px; }
.modal-actions { display: flex; gap: 8px; justify-content: flex-end; }
.btn-cancel { padding: 6px 16px; border-radius: 8px; border: 1px solid var(--border-default); background: transparent; cursor: pointer; font-size: 13px; }
.btn-save { padding: 6px 16px; border-radius: 8px; border: none; background: var(--brand-500); color: #fff; cursor: pointer; font-size: 13px; font-weight: 600; }
.modal-enter-active { transition: all 0.15s; }
.modal-leave-active { transition: all 0.1s; }
.modal-enter-from, .modal-leave-to { opacity: 0; }

[data-theme="dark"] .sub-section h5 { color: #22d3ee; }
[data-theme="dark"] .perm-role.admin { background: rgba(251,191,36,0.15); color: #fbbf24; }

@media (max-width: 767px) {
  .settings-layout { flex-direction: column; }
  .settings-sidebar { position: static; width: 100%; min-width: unset; max-height: unset; overflow-y: visible; }
  .sidebar-nav { flex-direction: row; overflow-x: auto; gap: 4px; padding-bottom: 8px; }
  .nav-item { white-space: nowrap; font-size: 11px; padding: 6px 10px; }
  .field :deep(.arco-input-wrapper), .secure-input { max-width: 100%; }
  .field-desc { padding-left: 0; }
}
</style>
