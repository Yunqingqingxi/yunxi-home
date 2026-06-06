export interface AliyunDNSConfig {
  access_key_id?: string
  access_key_secret?: string
  has_secret?: boolean
  _cleared?: boolean
}

export interface EmailConfig {
  host?: string
  port?: number
  user?: string
  password?: string
  has_password?: boolean
  to?: string
}

export interface WebhookConfig {
  url?: string
}

export interface DingtalkConfig {
  webhook_url?: string
}

export interface NotifyConfig {
  email?: EmailConfig
  webhook?: WebhookConfig
  dingtalk?: DingtalkConfig
}

export interface AIProviderConfig {
  api_key?: string
  has_key?: boolean
  model?: string
  _cleared?: boolean
}

export interface AIConfig {
  default_model?: string
  default_reasoning?: 'low' | 'medium' | 'high'
  expand_thinking_on_stream?: boolean
  [provider: string]: AIProviderConfig | string | boolean | undefined
}

export interface DetectConfig {
  [key: string]: any
}

export interface NASConfig {
  [key: string]: any
}

export interface DatabaseMySQLConfig {
  host?: string
  port?: number
  user?: string
  password?: string
  name?: string
  has_password?: boolean
  _cleared?: boolean
}

export interface DatabaseConfig {
  driver?: string
  host?: string
  port?: number
  user?: string
  password?: string
  name?: string
  mysql?: DatabaseMySQLConfig
}

export interface ServerConfig {
  [key: string]: any
}

export interface LogConfig {
  level?: string
  file?: string
  max_size?: number
  max_backups?: number
  max_age?: number
}

export interface QQBotConfig {
  app_id?: string
  app_secret?: string
  has_secret?: boolean
  group_id?: string
  _cleared?: boolean
}

export interface AppConfig {
  alidns?: AliyunDNSConfig
  notify?: NotifyConfig
  ai?: AIConfig
  detect?: DetectConfig
  nas?: NASConfig
  database?: DatabaseConfig
  server?: ServerConfig
  log?: LogConfig
  qqbot?: QQBotConfig
  [section: string]: any
}
