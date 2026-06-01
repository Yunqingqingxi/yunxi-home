<template>
  <div v-if="isLoginPage" class="login-layout">
    <router-view />
  </div>

  <div v-else class="app-shell">
    <header class="app-header">
      <button class="menu-trigger" @click="siderOpen = !siderOpen" aria-label="menu">
        <svg width="20" height="20" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
          <path d="M3 5h14M3 10h14M3 15h14"/>
        </svg>
      </button>
      <img src="/logo.svg" alt="Yunxi Home" class="logo" />
      <div class="header-spacer"></div>
      <div class="header-actions">
        <button class="theme-toggle" :title="theme.theme === 'dark' ? 'light' : 'dark'" @click="theme.toggle()">
          <svg v-if="theme.theme === 'dark'" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round">
            <circle cx="8" cy="8" r="4"/><path d="M8 2v1M8 13v1M2 8h1M13 8h1M3.8 3.8l.7.7M11.5 11.5l.7.7M3.8 12.2l.7-.7M11.5 4.5l.7-.7"/>
          </svg>
          <svg v-else width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round">
            <path d="M8 2a6 6 0 1 0 6 5.4A4.5 4.5 0 0 1 8 2Z"/>
          </svg>
        </button>
      </div>
    </header>

    <div class="app-body">
      <div v-if="siderOpen" class="sider-overlay" @click="siderOpen = false"></div>

      <aside :class="['app-sider', { open: siderOpen }]">
        <nav class="sider-nav">
          <a v-for="item in navItems" :key="item.path"
            :class="['nav-item', { active: currentRoute === item.path }]"
            @click="navigate(item.path)">
            <span class="nav-icon" v-html="item.icon"></span>
            <span class="nav-label">{{ item.label }}</span>
          </a>
        </nav>

        <div class="session-tree">
          <button class="session-tree-trigger" @click="toggleChatTree">
            <svg :class="['tree-chevron', { open: chatExpanded }]" width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 3l3 3-3 3"/></svg>
            <span class="tree-label">AI 对话</span>
            <span v-if="chatSessions.length" class="tree-count">{{ chatSessions.length }}</span>
          </button>
          <div v-if="chatExpanded" class="session-subtree">
            <div v-if="chatSessions.length === 0" class="session-empty">no chat yet</div>
            <div v-for="s in chatSessions" :key="s.id"
              :class="['session-sub-item', { active: activeSessionId === s.id }]"
              @click="navigateChat(s.id)">
              <span class="sub-title">{{ s.title || 'new chat' }}</span>
              <span class="sub-time">{{ sessionTime(s.updated_at) }}</span>
            </div>
          </div>
        </div>

        <div class="sider-bottom">
          <a :class="['nav-item', { active: currentRoute === '/settings' }]" @click="navigate('/settings')">
            <span class="nav-icon">
              <svg width="18" height="18" viewBox="0 0 18 18" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round">
                <circle cx="9" cy="9" r="2.5"/><path d="M9 1.5v2M9 14.5v2M1.5 9h2M14.5 9h2M3.5 3.5l1.5 1.5M13 13l1.5 1.5M3.5 14.5l1.5-1.5M13 5l1.5-1.5"/>
              </svg>
            </span>
            <span class="nav-label">settings</span>
          </a>

          <a class="nav-item nav-logout" @click="doLogout">
            <span class="nav-icon">
              <svg width="18" height="18" viewBox="0 0 18 18" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round">
                <path d="M7 3H4a1 1 0 00-1 1v10a1 1 0 001 1h3M11 13l4-4-4-4M15 9H7"/>
              </svg>
            </span>
            <span class="nav-label">logout</span>
          </a>

          <span class="sider-version">v3.1</span>
        </div>
      </aside>

      <main class="app-main"><router-view /></main>
    </div>
  </div>
</template>