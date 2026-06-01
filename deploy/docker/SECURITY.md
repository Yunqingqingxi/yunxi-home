# =============================================================================
# Yunxi Home v4.0 - 安全加固部署清单
# =============================================================================

## 1. 操作系统层
### 1.1 SSH 硬化
```bash
# /etc/ssh/sshd_config
Port 2222                    # 非标准端口
PermitRootLogin no           # 禁止 root 登录
PasswordAuthentication no    # 仅密钥登录
PubkeyAuthentication yes
AllowUsers admin             # 仅允许特定用户
MaxAuthTries 3
ClientAliveInterval 300
```

### 1.2 防火墙 (UFW)
```bash
ufw default deny incoming
ufw default allow outgoing
ufw allow 443/tcp            # HTTPS
ufw allow 51820/udp          # WireGuard
ufw allow from 192.168.0.0/16 to any port 2222  # SSH 仅内网
ufw enable
```

### 1.3 自动更新
```bash
apt install unattended-upgrades
dpkg-reconfigure --priority=low unattended-upgrades
```

## 2. Docker 层
### 2.1 Docker Daemon
```json
// /etc/docker/daemon.json
{
  "icc": false,
  "log-driver": "json-file",
  "log-opts": { "max-size": "10m", "max-file": "3" },
  "userns-remap": "default"
}
```

### 2.2 容器安全
```yaml
# 每个服务在 docker-compose.yml 中应添加:
security_opt:
  - no-new-privileges:true
read_only: true  # 除需要写入的卷外
```

## 3. 网络层
### 3.1 公网暴露
- 仅暴露 443 (HTTPS) 和 51820/udp (WireGuard)
- 所有 HTTP 流量强制重定向到 HTTPS
- SSH 仅通过 WireGuard VPN 内网访问

### 3.2 Nginx 速率限制
```nginx
# 添加到 NPM 的 Advanced 配置
limit_req_zone $binary_remote_addr zone=api:10m rate=10r/s;
limit_req zone=api burst=20 nodelay;

# 对登录端点特殊限制
location /api/auth/login {
    limit_req zone=api burst=5 nodelay;
    proxy_pass http://yunxi-home:9981;
}
```

## 4. 认证层
### 4.1 Authelia 双因素认证
- 管理面板 (Portainer, NPM): 强制 2FA
- 用户面板: 可选 TOTP
- API 访问: JWT token + IP 白名单

### 4.2 密码策略
```bash
# 随机生成所有密钥
openssl rand -base64 32  # JWT secrets
openssl rand -base64 24  # 数据库密码
```

## 5. 数据层
### 5.1 Docker Secrets (生产环境)
```bash
echo "my-secret-password" | docker secret create db_password -
```

### 5.2 加密存储
- 数据库密码通过环境变量或 Docker Secret 注入
- 备份文件使用 GPG 加密
- TLS 证书自动续签 (Let's Encrypt)

## 6. AI 安全层
### 6.1 工具权限分级
```
Level 0 (只读): system_status, docker_ps, nas_list_files, jellyfin_search
Level 1 (查询): docker_logs, ha_get_state, dns_list_domains  
Level 2 (操作,需确认): container_restart, ha_control, aria2_add_uri
Level 3 (破坏,需双确认): db_backup, container_stop, nas_delete
```

### 6.2 敏感操作二次确认
```python
DANGEROUS_TOOLS = {"nas_delete", "container_restart", "container_stop", "db_backup", "ssh_exec_command"}
# 调用这些工具前要求用户明确确认
```

## 7. 监控与告警
### 7.1 日志监控
- 所有容器日志通过 json-file driver 收集
- 关键事件通过 Apprise 推送通知
- 定期检查磁盘使用率 (>85% 告警)

### 7.2 入侵检测
```bash
# 安装 fail2ban
apt install fail2ban
# 配置监控 Nginx 认证失败
```

## 8. 备份策略 (3-2-1)
- 3 份副本: 生产 + 本地备份 + 异地备份
- 2 种介质: SSD + HDD
- 1 份异地: 外置硬盘或远程 rsync
