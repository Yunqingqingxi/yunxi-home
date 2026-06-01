# =============================================================================
# Nginx Proxy Manager - 子域名配置指南
# =============================================================================
# 在 NPM Web 界面 (http://your-server:81) 中为每个服务创建代理主机
# =============================================================================

# ── 通用设置 ──────────────────────────────────────────
# SSL: 所有子域名启用 Force SSL, HTTP/2, HSTS
# 证书: 使用 Let's Encrypt, 填写你的邮箱
# 高级配置: 每个服务的 proxy 设置见下方

# ── 子域名映射表 ──────────────────────────────────────

subdomains:
  # 中枢管理面板 (无需 Authelia, 有自身 JWT 认证)
  api:
    domain: "api.{{DOMAIN}}"
    forward: "http://yunxi-home:9981"
    websocket: true    # 支持 /api/terminal WebSocket
    ssl: true
    authelia: false

  # 私有云盘
  cloud:
    domain: "cloud.{{DOMAIN}}"
    forward: "http://nextcloud:80"
    ssl: true
    authelia: true
    custom_config: |
      client_max_body_size 10G;
      proxy_read_timeout 86400s;

  # 照片备份
  photo:
    domain: "photo.{{DOMAIN}}"
    forward: "http://immich-server:3001"
    ssl: true
    authelia: true

  # 影音中心
  media:
    domain: "media.{{DOMAIN}}"
    forward: "http://jellyfin:8096"
    ssl: true
    authelia: true
    custom_config: |
      proxy_buffering off;

  # 离线下载
  dl:
    domain: "dl.{{DOMAIN}}"
    forward: "http://ariang:80"
    ssl: true
    authelia: true

  # VPN 管理 (wg-easy Web UI)
  vpn:
    domain: "vpn.{{DOMAIN}}"
    forward: "http://wireguard:51821"
    ssl: true
    authelia: true

  # AI 对话
  ai:
    domain: "ai.{{DOMAIN}}"
    forward: "http://open-webui:8080"
    ssl: true
    authelia: true
    custom_config: |
      proxy_read_timeout 300s;
      proxy_buffering off;

  # 系统运维 (Portainer)
  portainer:
    domain: "portainer.{{DOMAIN}}"
    forward: "http://portainer:9000"
    ssl: true
    authelia: true
    websocket: true

  # 消息推送 (Apprise)
  notify:
    domain: "notify.{{DOMAIN}}"
    forward: "http://apprise:8000"
    ssl: true
    authelia: true

  # 智能家居
  ha:
    domain: "ha.{{DOMAIN}}"
    forward: "http://homeassistant:8123"
    ssl: true
    authelia: true
    websocket: true
    custom_config: |
      proxy_read_timeout 86400s;

# ── NPM 高级配置 (每个代理主机的 Advanced 选项卡) ─────

_advanced_templates:
  authelia_auth: |
    # 标准的 Authelia 前向认证
    location / {
      proxy_pass $forward_scheme://$server:$port;
      proxy_set_header Host $host;
      proxy_set_header X-Real-IP $remote_addr;
      proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
      proxy_set_header X-Forwarded-Proto $scheme;
      proxy_set_header X-Forwarded-Host $host;
      proxy_set_header X-Forwarded-Uri $request_uri;
    }

  authelia_verify: |
    # Authelia 认证验证端点
    location /authelia {
      internal;
      proxy_pass http://authelia:9091/api/verify;
      proxy_set_header Content-Length "";
      proxy_pass_request_body off;
    }
