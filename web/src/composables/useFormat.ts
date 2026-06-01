export function formatTime(iso: string): string {
  if (!iso) return '-'
  const d = new Date(iso)
  const pad = (n: number): string => String(n).padStart(2, '0')
  return d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate()) + ' ' +
         pad(d.getHours()) + ':' + pad(d.getMinutes()) + ':' + pad(d.getSeconds())
}

export function formatCron(cron: string): string {
  if (!cron) return '未配置'
  const parts = cron.trim().split(/\s+/)
  const minField = parts.length === 6 ? parts[1] : parts[0]
  if (minField && minField.startsWith('*/')) {
    const mins = minField.slice(2)
    if (mins === '1') return '每分钟'
    return '每' + mins + '分钟'
  }
  if (parts.length >= 3 && parts[2] !== '*')
    return '每天' + parts[2] + ':' + (parts[1] || '').padStart(2, '0')
  return cron
}
