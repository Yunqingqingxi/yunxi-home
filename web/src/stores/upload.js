import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import api from '../services/api'

// ── Adaptive chunk sizing ──────────────────────────────
function chunkSizeFor(fileSize) {
  if (fileSize < 5 * 1024 * 1024)   return fileSize   // <5MB: no split
  if (fileSize < 50 * 1024 * 1024)  return 5 * 1024 * 1024
  if (fileSize < 500 * 1024 * 1024) return 10 * 1024 * 1024
  return 20 * 1024 * 1024                              // >500MB
}

const CHUNK_THRESHOLD = 10 * 1024 * 1024 // files above this use chunked upload

// ── sessionStorage helpers ─────────────────────────────
function saveState(tasks) {
  try {
    const slim = tasks.map(t => ({
      id: t.id, name: t.name, dir: t.dir,
      uploadId: t._uploadId, fileSize: t._fileSize,
      chunkSize: t._chunkSize, totalChunks: t._totalChunks,
      progress: t.progress, status: t.status,
    }))
    sessionStorage.setItem('upload_state', JSON.stringify(slim))
  } catch (_) {}
}

function loadState() {
  try {
    const raw = sessionStorage.getItem('upload_state')
    return raw ? JSON.parse(raw) : []
  } catch (_) { return [] }
}

function clearState() { sessionStorage.removeItem('upload_state') }

// ── Concurrency control ────────────────────────────────
class ConcurrencyPool {
  constructor(max) { this.max = max; this.running = 0; this.queue = [] }
  acquire() {
    return new Promise(resolve => {
      if (this.running < this.max) { this.running++; resolve() }
      else this.queue.push(resolve)
    })
  }
  release() {
    if (this.queue.length) this.queue.shift()()
    else this.running--
  }
}

export const useUploadStore = defineStore('upload', () => {
  const tasks = ref([])
  const initialized = ref(false)

  const hasActive = computed(() => tasks.value.some(t => t.status === 'uploading'))
  const totalProgress = computed(() => {
    if (!tasks.value.length) return 0
    const sum = tasks.value.reduce((s, t) => s + t.progress, 0)
    return Math.round(sum / tasks.value.length)
  })

  let _completeListener = null
  function onAllComplete(fn) { _completeListener = fn }
  function _checkAllDone() {
    if (!hasActive.value && _completeListener) _completeListener()
  }

  function _addTask(name, fileSize, dir, abortCtrl) {
    const id = Date.now() + '_' + Math.random().toString(36).slice(2, 6)
    const task = {
      id, name, dir, fileSize,
      progress: 0, status: 'uploading',
      _abort: abortCtrl,
      _uploadId: null, _chunkSize: 0, _totalChunks: 0,
    }
    tasks.value.push(task)
    saveState(tasks.value)
    return id
  }

  function _updateProgress(id, pct) {
    const t = tasks.value.find(t => t.id === id)
    if (t) { t.progress = Math.min(pct, 100); saveState(tasks.value) }
  }

  function _markDone(id) {
    const t = tasks.value.find(t => t.id === id)
    if (t) { t.progress = 100; t.status = 'done' }
    saveState(tasks.value)
    _checkAllDone()
  }

  function _markError(id) {
    const t = tasks.value.find(t => t.id === id)
    if (t) t.status = 'error'
    saveState(tasks.value)
    _checkAllDone()
  }

  function cancelTask(id) {
    const t = tasks.value.find(t => t.id === id)
    if (t && t._abort) { t._abort.abort(); t.status = 'error'; t.progress = 0 }
    saveState(tasks.value)
    _checkAllDone()
  }

  function removeTask(id) {
    tasks.value = tasks.value.filter(t => t.id !== id)
    saveState(tasks.value)
    _checkAllDone()
  }

  function clearDone() {
    tasks.value = tasks.value.filter(t => t.status === 'uploading')
    if (!tasks.value.length) clearState()
    else saveState(tasks.value)
  }

  // ── Single file upload ──────────────────────────────
  function startUpload(file, dir) {
    const abortCtrl = new AbortController()
    if (file.size > CHUNK_THRESHOLD) {
      return startChunkedUpload(file, dir, abortCtrl)
    }

    const taskId = _addTask(file.name, file.size, dir, abortCtrl)
    const t = tasks.value.find(t => t.id === taskId)
    if (t) { t._isSmall = true; saveState(tasks.value) }
    const form = new FormData()
    form.append('file', file)
    form.append('dir', dir)
    api.post('/api/nas/files/upload', form, { timeout: 0, signal: abortCtrl.signal,
      onUploadProgress: (evt) => {
        if (evt.total > 0) _updateProgress(taskId, Math.round((evt.loaded / evt.total) * 100))
      }
    }).then(() => _markDone(taskId)).catch(() => _markError(taskId))
    return taskId
  }

  // ── Chunked upload ───────────────────────────────────
  async function startChunkedUpload(file, dir, abortCtrl) {
    const chunkSize = chunkSizeFor(file.size)
    const totalChunks = Math.ceil(file.size / chunkSize)
    const taskId = _addTask(file.name, file.size, dir, abortCtrl)
    const t = tasks.value.find(t => t.id === taskId)
    if (t) { t._chunkSize = chunkSize; t._totalChunks = totalChunks }
    saveState(tasks.value)

    try {
      const initRes = await api.post('/api/nas/files/upload/init', {
        filename: file.name, dir, total_size: file.size, chunk_size: chunkSize,
      })
      const uploadId = initRes.data?.data?.upload_id
      if (!uploadId) throw new Error('Init failed')
      if (t) t._uploadId = uploadId
      saveState(tasks.value)

      let completedChunks = 0
      const total = totalChunks

      async function uploadChunk(chunkIndex, retries = 3) {
        const start = chunkIndex * chunkSize
        const end = Math.min(start + chunkSize, file.size)
        const blob = file.slice(start, end)
        const form = new FormData()
        form.append('file', blob, file.name + '.part' + chunkIndex)
        form.append('upload_id', uploadId)
        form.append('chunk_index', String(chunkIndex))

        for (let attempt = 0; attempt <= retries; attempt++) {
          try {
            const startTime = Date.now()
            await api.post('/api/nas/files/upload/chunk', form, {
              timeout: 120000,
              signal: abortCtrl.signal,
            })
            completedChunks++
            _updateProgress(taskId, Math.round((completedChunks / total) * 100))
            // Adaptive concurrency hint (stored on pool for adjustment)
            const dur = Date.now() - startTime
            _adjustConcurrency(dur)
            return
          } catch (e) {
            if (e.name === 'AbortError' || e.code === 'ERR_CANCELED') throw e
            if (attempt === retries) throw e
            await new Promise(r => setTimeout(r, 1000 * (attempt + 1)))
          }
        }
      }

      // Concurrency pool with adaptive max
      const pool = new ConcurrencyPool(_concurrency)
      const promises = []
      for (let i = 0; i < totalChunks; i++) {
        promises.push((async (idx) => {
          await pool.acquire()
          try { await uploadChunk(idx) } finally { pool.release() }
        })(i))
      }
      await Promise.all(promises)

      await api.post('/api/nas/files/upload/complete', { upload_id: uploadId })
      _markDone(taskId)
    } catch (e) {
      if (e.name === 'AbortError' || e.code === 'ERR_CANCELED') {
        _markError(taskId)
        return taskId
      }
      console.error('Chunked upload failed:', e)
      _markError(taskId)
    }
    return taskId
  }

  // ── Adaptive concurrency ─────────────────────────────
  let _concurrency = 4
  function _adjustConcurrency(chunkDurationMs) {
    if (chunkDurationMs < 500)       _concurrency = Math.min(8, _concurrency + 1)
    else if (chunkDurationMs > 3000) _concurrency = Math.max(2, _concurrency - 1)
  }

  // ── Batch upload ─────────────────────────────────────
  function startBatchUpload(items, onProgress) {
    if (!items || !items.length) return
    const abortCtrl = new AbortController()
    const total = items.length
    let done = 0
    const label = items.length === 1 ? items[0].file.name : `${items.length} 个文件`
    const taskId = _addTask(label, 0, items[0]?.dir || '/', abortCtrl)

    const results = { success: [], failed: [] }

    async function worker() {
      while (items.length) {
        const item = items.shift()
        if (!item) break
        try {
          if (item.file.size > CHUNK_THRESHOLD) {
            await uploadSingleChunked(item.file, item.dir, abortCtrl)
          } else {
            await uploadSingle(item.file, item.dir, abortCtrl)
          }
          results.success.push(item.file.name)
        } catch (e) {
          results.failed.push({ name: item.file.name, error: e.message || 'Upload failed' })
        }
        done++
        _updateProgress(taskId, Math.round((done / total) * 100))
        if (onProgress) onProgress(done, total, item.file.name)
      }
    }

    const MAX_SMALL_CONCURRENT = 6
    const concurrency = Math.min(MAX_SMALL_CONCURRENT, total)
    Promise.all(Array.from({ length: concurrency }, () => worker()))
      .then(() => {
        const t = tasks.value.find(t => t.id === taskId)
        if (t) {
          if (results.failed.length) {
            t.status = results.success.length ? 'partial' : 'error'
            t._results = results
          }
          if (t.progress >= 100) _markDone(taskId)
        }
        if (results.failed.length) console.warn('Upload failures:', results.failed)
        if (onProgress) onProgress(results.success.length, total, '', results.failed)
        return results
      })
      .catch(() => _markError(taskId))

    return taskId
  }

  // ── Small file in batch ──────────────────────────────
  async function uploadSingle(file, dir, abortCtrl) {
    const form = new FormData()
    form.append('file', file)
    form.append('dir', dir)
    await api.post('/api/nas/files/upload', form, { timeout: 0, signal: abortCtrl?.signal })
  }

  // ── Large file in batch (inline chunked) ─────────────
  async function uploadSingleChunked(file, dir, abortCtrl) {
    const chunkSize = chunkSizeFor(file.size)
    const totalChunks = Math.ceil(file.size / chunkSize)

    const initRes = await api.post('/api/nas/files/upload/init', {
      filename: file.name, dir, total_size: file.size, chunk_size: chunkSize,
    })
    const uploadId = initRes.data?.data?.upload_id
    if (!uploadId) throw new Error('Init failed')

    const pool = new ConcurrencyPool(_concurrency)
    const promises = []
    for (let i = 0; i < totalChunks; i++) {
      promises.push((async (idx) => {
        await pool.acquire()
        try {
          const start = idx * chunkSize
          const end = Math.min(start + chunkSize, file.size)
          const blob = file.slice(start, end)
          const form = new FormData()
          form.append('file', blob, file.name + '.part' + idx)
          form.append('upload_id', uploadId)
          form.append('chunk_index', String(idx))
          for (let attempt = 0; attempt <= 3; attempt++) {
            try {
              await api.post('/api/nas/files/upload/chunk', form, { timeout: 120000, signal: abortCtrl?.signal })
              return
            } catch (e) {
              if (e.name === 'AbortError' || e.code === 'ERR_CANCELED') throw e
              if (attempt === 3) throw e
              await new Promise(r => setTimeout(r, 1000 * (attempt + 1)))
            }
          }
        } finally { pool.release() }
      })(i))
    }
    await Promise.all(promises)
    await api.post('/api/nas/files/upload/complete', { upload_id: uploadId })
  }

  // ── Ensure directories before folder upload ──────────
  async function ensureDirectories(items) {
    const dirs = new Set(items.map(i => i.dir))
    const sorted = [...dirs].sort((a, b) => a.split('/').length - b.split('/').length)
    for (const dir of sorted) {
      if (dir === '/' || dir === '') continue
      try { await api.post('/api/nas/files/mkdir', { path: dir }) }
      catch (e) { console.warn('Mkdir failed for', dir, e) }
    }
  }

  async function startFolderUpload(items, onProgress) {
    await ensureDirectories(items)
    return startBatchUpload(items, onProgress)
  }

  // ── Resume on page load ──────────────────────────────
  async function resumeFromStorage() {
    if (initialized.value) return
    initialized.value = true

    const saved = loadState()
    if (!saved.length) return

    for (const savedTask of saved) {
      if (savedTask._isSmall) { savedTask.status = 'error'; continue }
      if (savedTask.status !== 'uploading' || !savedTask._uploadId) continue
      // Check server-side status
      try {
        const res = await api.get('/api/nas/files/upload/status', {
          params: { upload_id: savedTask._uploadId }
        })
        const meta = res.data?.data
        if (!meta) continue

        const doneChunks = meta.chunks_done?.filter(Boolean).length || 0
        const totalChunks = meta.total_chunks || savedTask.totalChunks
        if (doneChunks >= totalChunks) {
          // All chunks done - complete
          await api.post('/api/nas/files/upload/complete', { upload_id: savedTask._uploadId })
          savedTask.status = 'done'
          savedTask.progress = 100
        } else {
          // Update progress
          savedTask.progress = Math.round((doneChunks / totalChunks) * 100)
        }
      } catch (_) {
        // Upload session expired or server restarted
        savedTask.status = 'error'
      }
    }

    // Load non-uploading tasks for display
    const displayTasks = saved.filter(t => t.status !== 'uploading')
    if (displayTasks.length) {
      const now = Date.now()
      tasks.value = displayTasks.map(t => ({
        ...t, _abort: new AbortController(),
        id: t.id || (now + '_' + Math.random().toString(36).slice(2, 6)),
      }))
    }
    if (!tasks.value.length) clearState()
  }

  return {
    tasks, hasActive, totalProgress,
    startUpload, startBatchUpload, startFolderUpload,
    cancelTask, removeTask, clearDone,
    onAllComplete, resumeFromStorage,
    ensureDirectories,
  }
})
