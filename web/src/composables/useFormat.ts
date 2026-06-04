import { format, formatDistanceToNow, differenceInMinutes, isSameDay } from 'date-fns'
import { zhCN } from 'date-fns/locale'

// ── Date/Time Formatters ──────────────────────────────────

/** Format ISO string to YYYY-MM-DD HH:mm:ss */
export function formatTime(iso: string): string {
  if (!iso) return '-'
  return format(new Date(iso), 'yyyy-MM-dd HH:mm:ss')
}

/** Format ISO string to M月d日 HH:mm (Chinese locale) */
export function formatDateTime(iso: string): string {
  if (!iso) return '-'
  return format(new Date(iso), 'M月d日 HH:mm', { locale: zhCN })
}

/** Format timestamp to HH:mm */
export function formatHM(ts: number | string): string {
  return format(new Date(ts), 'HH:mm')
}

/** Format timestamp to HH:mm:ss */
export function formatHMS(ts: string): string {
  return format(new Date(ts), 'HH:mm:ss')
}

/** Relative time with Chinese labels: 刚刚 / X分钟前 / X小时前 / date */
export function formatRelative(t: number | string): string {
  const d = new Date(t)
  const now = Date.now()
  const secondsAgo = (now - d.getTime()) / 1000

  if (secondsAgo < 60) return '刚刚'
  if (secondsAgo < 3600) return formatDistanceToNow(d, { addSuffix: true, locale: zhCN })
  if (isSameDay(d, now)) return formatHM(t)

  // Older: show short date
  return format(d, 'M月d日', { locale: zhCN })
}

/** English relative time: "3m ago", "2h ago", or short date */
export function formatRelativeShort(t: number | string): string {
  const d = new Date(t)
  const secondsAgo = (Date.now() - d.getTime()) / 1000

  if (secondsAgo < 3600) return Math.floor(secondsAgo / 60) + 'm ago'
  if (secondsAgo < 86400) return Math.floor(secondsAgo / 3600) + 'h ago'
  return d.toLocaleDateString('zh-CN')
}

/** Minutes between two timestamps */
export function minutesBetween(a: number | string, b: number | string): number {
  return differenceInMinutes(new Date(b), new Date(a))
}

/** Duration formatter: ms → human readable */
export function formatDuration(ms: number): string {
  if (ms < 1000) return ms + 'ms'
  if (ms < 60000) return (ms / 1000).toFixed(1) + 's'
  const minutes = Math.floor(ms / 60000)
  const seconds = Math.round((ms % 60000) / 1000)
  if (seconds === 0) return minutes + 'm'
  return minutes + 'm' + seconds + 's'
}

/** Duration formatter (compact): ms → X.Xs / Xs */
export function formatDurationCompact(ms: number): string {
  if (ms < 1000) return (ms / 1000).toFixed(1) + 's'
  return Math.round(ms / 1000) + 's'
}

// ── Number / Size Formatters ──────────────────────────────

/** Byte size → human readable */
export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  const size = bytes / Math.pow(1024, i)
  return size.toFixed(i === 0 ? 0 : 1) + ' ' + units[i]
}

/** Bytes per second → human readable rate */
export function formatRate(bytesPerSec: number): string {
  return formatBytes(bytesPerSec) + '/s'
}

/** Large number → abbreviated (1.2K, 3.5M) */
export function formatNum(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(n)
}

// ── Cron Formatter ────────────────────────────────────────

export function formatCron(cron: string): string {
  if (!cron) return '未配置'
  const parts = cron.trim().split(/\s+/)
  const minField = parts.length === 6 ? parts[1] : parts[0]
  if (minField && minField.startsWith('*/')) {
    const mins = minField.slice(2)
    if (mins === '1') return '每分钟'
    return '每' + mins + '分钟'
  }
  if (parts.length >= 3 && parts[2] !== '*') return '每天' + parts[2] + ':' + (parts[1] || '').padStart(2, '0')
  return cron
}
