import { useEffect, useMemo, useRef, useState } from 'react'
import { ApiError } from '../api/client'
import { audioApi } from '../api/endpoints'

type Phase = 'idle' | 'recording' | 'processing'

// ---------- Web Speech API（Chrome/Edge 内置实时语音识别）类型 ----------
interface SRAlternative {
  transcript: string
}
interface SRResult {
  isFinal: boolean
  0: SRAlternative
}
interface SRResultList {
  length: number
  [index: number]: SRResult
}
interface SREvent {
  resultIndex: number
  results: SRResultList
}
interface SRErrorEvent {
  error: string
}
interface SRecognition {
  lang: string
  continuous: boolean
  interimResults: boolean
  onresult: ((e: SREvent) => void) | null
  onerror: ((e: SRErrorEvent) => void) | null
  onend: (() => void) | null
  start(): void
  stop(): void
  abort(): void
}
type SRCtor = new () => SRecognition

declare global {
  interface Window {
    SpeechRecognition?: SRCtor
    webkitSpeechRecognition?: SRCtor
  }
}

function getSpeechRecognitionCtor(): SRCtor | null {
  if (typeof window === 'undefined') return null
  return window.SpeechRecognition ?? window.webkitSpeechRecognition ?? null
}

// ---------- MediaRecorder 回退链路用到的编码探测 ----------
const MIME_CANDIDATES: { mime: string; ext: string }[] = [
  { mime: 'audio/webm;codecs=opus', ext: 'webm' },
  { mime: 'audio/webm', ext: 'webm' },
  { mime: 'audio/ogg;codecs=opus', ext: 'ogg' },
  { mime: 'audio/mp4', ext: 'm4a' },
]

function pickMime(): { mime: string; ext: string } | null {
  if (typeof MediaRecorder === 'undefined') return null
  for (const c of MIME_CANDIDATES) {
    if (MediaRecorder.isTypeSupported(c.mime)) return c
  }
  return { mime: '', ext: 'webm' }
}

function formatDuration(ms: number): string {
  const total = Math.max(0, Math.round(ms / 1000))
  const m = Math.floor(total / 60)
  const s = total % 60
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

// 单次录音最长时长（到点自动收尾，避免识别器异常时无限占用麦克风）
const MAX_RECORD_MS = 180_000
// 点击停止后等待识别器收尾的最长时间，超时强制 abort 并提交已有文本
const STOP_WATCHDOG_MS = 1500
// 识别器自行断开（静默/网络抖动）后的自动续听延迟与无语音重试上限
const RESTART_DELAY_MS = 250
const MAX_AUTO_RESTARTS = 8

// VoiceRecorder 录制答题语音。
// 优先使用浏览器原生 Web Speech API 实时识别（中文，停止即回填，无需后端 ASR key）；
// 浏览器不支持时回退 MediaRecorder 上传 → 后端转写链路。
// 转写文本进入回答输入框，候选人可在提交前检查/修改（语音不是最终事实）。
export function VoiceRecorder({
  questionId,
  disabled,
  onTranscript,
}: {
  questionId: string
  disabled?: boolean
  onTranscript: (text: string) => void
}) {
  const [phase, setPhase] = useState<Phase>('idle')
  const [elapsedMs, setElapsedMs] = useState(0)
  const [error, setError] = useState<string | null>(null)
  const [committed, setCommitted] = useState('') // 已确定的识别文本（稳定，不回退）
  const [interim, setInterim] = useState('') // 正在识别中的临时文本（可能变化）

  const recorderRef = useRef<MediaRecorder | null>(null)
  const streamRef = useRef<MediaStream | null>(null)
  const chunksRef = useRef<Blob[]>([])
  const mimeRef = useRef<{ mime: string; ext: string } | null>(null)
  const startedAtRef = useRef(0)
  const tickRef = useRef<number | null>(null)

  // 浏览器原生识别相关
  const recognitionRef = useRef<SRecognition | null>(null)
  const finalTextRef = useRef('')
  const interimRef = useRef('')
  const userStoppedRef = useRef(false)
  const finishedRef = useRef(false) // 保证一次录音只收尾一次
  const unmountedRef = useRef(false)
  const maxTimerRef = useRef<number | null>(null)
  const watchdogRef = useRef<number | null>(null)
  const restartTimerRef = useRef<number | null>(null)
  const autoRestartsRef = useRef(0)
  const startBrowserASRRef = useRef<() => void>(() => undefined)
  const onTranscriptRef = useRef(onTranscript)
  onTranscriptRef.current = onTranscript

  const SRTor = useMemo(() => getSpeechRecognitionCtor(), [])
  const useBrowserASR = SRTor !== null
  const mediaRecorderSupported =
    typeof navigator !== 'undefined' && !!navigator.mediaDevices && pickMime() !== null
  const supported = useBrowserASR || mediaRecorderSupported

  const stopTick = () => {
    if (tickRef.current) {
      window.clearInterval(tickRef.current)
      tickRef.current = null
    }
  }

  const cleanupStream = () => {
    streamRef.current?.getTracks().forEach((t) => t.stop())
    streamRef.current = null
    stopTick()
  }

  // 卸载/切题时释放麦克风与识别器
  useEffect(() => {
    unmountedRef.current = false
    return () => {
      unmountedRef.current = true
      if (maxTimerRef.current) window.clearTimeout(maxTimerRef.current)
      if (watchdogRef.current) window.clearTimeout(watchdogRef.current)
      if (restartTimerRef.current) window.clearTimeout(restartTimerRef.current)
      if (recognitionRef.current) {
        recognitionRef.current.onresult = null
        recognitionRef.current.onerror = null
        recognitionRef.current.onend = null
        try {
          recognitionRef.current.abort()
        } catch {
          /* noop */
        }
        recognitionRef.current = null
      }
      if (recorderRef.current && recorderRef.current.state !== 'inactive') {
        recorderRef.current.onstop = null
        try {
          recorderRef.current.stop()
        } catch {
          /* noop */
        }
      }
      cleanupStream()
    }
  }, [questionId])

  // ---------- 路径 A：浏览器原生 Web Speech API ----------
  // 收尾：停止计时/定时器，提交 final+interim 合并文本；幂等
  const finishBrowserASR = (reason: 'manual' | 'auto' | 'error', errorMsg?: string) => {
    if (finishedRef.current) return
    finishedRef.current = true
    stopTick()
    if (maxTimerRef.current) {
      window.clearTimeout(maxTimerRef.current)
      maxTimerRef.current = null
    }
    if (watchdogRef.current) {
      window.clearTimeout(watchdogRef.current)
      watchdogRef.current = null
    }
    if (restartTimerRef.current) {
      window.clearTimeout(restartTimerRef.current)
      restartTimerRef.current = null
    }
    const recognition = recognitionRef.current
    if (recognition) {
      recognition.onresult = null
      recognition.onerror = null
      recognition.onend = null
      recognitionRef.current = null
    }
    const text = (finalTextRef.current + interimRef.current).trim()
    finalTextRef.current = ''
    interimRef.current = ''
    setCommitted('')
    setInterim('')
    setPhase('idle')
    setElapsedMs(0)
    if (unmountedRef.current) return
    if (text) {
      onTranscriptRef.current(text)
    } else if (reason !== 'auto') {
      setError(errorMsg ?? '转写结果为空，请改用文字作答或重试')
    }
  }

  // 识别器会因静默/网络抖动自行结束会话；用户未主动停止时自动续听，保留已识别文本
  const scheduleRestart = () => {
    if (finishedRef.current || unmountedRef.current || userStoppedRef.current) return
    if (autoRestartsRef.current >= MAX_AUTO_RESTARTS) {
      if (finalTextRef.current.trim()) {
        finishBrowserASR('auto')
      } else {
        finishBrowserASR('error', '语音识别服务多次连接失败，请检查网络/代理后重试，或改用文字作答')
      }
      return
    }
    autoRestartsRef.current += 1
    restartTimerRef.current = window.setTimeout(() => {
      restartTimerRef.current = null
      if (!finishedRef.current && !unmountedRef.current && !userStoppedRef.current) {
        startBrowserASRRef.current()
      }
    }, RESTART_DELAY_MS)
  }

  // beginRecognition 启动一轮识别会话（一次录音中可能包含多轮，自动续听）
  const beginRecognition = () => {
    if (!SRTor || finishedRef.current || unmountedRef.current) return
    const recognition = new SRTor()
    recognition.lang = 'zh-CN'
    recognition.continuous = true
    recognition.interimResults = true

    recognition.onresult = (e: SREvent) => {
      // 关键去重：只处理 resultIndex 之后的新结果。
      // Chrome 的事件 results 包含本会话全部历史结果，从头遍历并累加 final
      // 会把同一句话重复追加 N 遍。
      autoRestartsRef.current = 0 // 听到声音说明服务正常，重置续听计数
      let interimText = ''
      for (let i = e.resultIndex; i < e.results.length; i++) {
        const result = e.results[i]
        if (result.isFinal) {
          finalTextRef.current += result[0].transcript
        } else {
          interimText += result[0].transcript
        }
      }
      interimRef.current = interimText
      setCommitted(finalTextRef.current)
      setInterim(interimText)
    }

    recognition.onerror = (e: SRErrorEvent) => {
      const name = e.error
      const hasText = !!(finalTextRef.current + interimRef.current).trim()
      // 已有文本：权限类以外的瞬时错误不打断，交给 onend 自动续听或用户手动停止
      if (name === 'not-allowed' || name === 'service-not-allowed') {
        finishBrowserASR('error', '麦克风权限被拒绝，请在浏览器地址栏允许麦克风后重试')
        return
      }
      if (name === 'audio-capture') {
        finishBrowserASR('error', '未检测到麦克风设备，请检查设备后重试')
        return
      }
      if (name === 'network' && !hasText && autoRestartsRef.current >= MAX_AUTO_RESTARTS - 1) {
        finishBrowserASR('error', '语音识别服务不可达（需联网，可开启系统代理后重试），请改用文字作答')
        return
      }
      // no-speech / network（前几次）/ aborted 等：静默等待 onend 续听，不清空预览
    }

    recognition.onend = () => {
      // 新一轮会话开始后本回调失效
      if (recognitionRef.current !== recognition) return
      recognitionRef.current = null
      if (finishedRef.current || unmountedRef.current) return
      if (userStoppedRef.current) {
        finishBrowserASR('manual')
      } else {
        // 静默或网络抖动导致的结束：保留已识别文本，自动续听
        scheduleRestart()
      }
    }

    try {
      recognition.start()
      recognitionRef.current = recognition
    } catch {
      // start 过快（旧会话未完全释放）时短暂重试，其余情况交给 onend/用户重试
      restartTimerRef.current = window.setTimeout(() => {
        if (!finishedRef.current && !userStoppedRef.current && !unmountedRef.current) {
          beginRecognition()
        }
      }, RESTART_DELAY_MS)
    }
  }

  const startBrowserASR = () => {
    if (!SRTor) return
    setError(null)
    setCommitted('')
    setInterim('')
    finalTextRef.current = ''
    interimRef.current = ''
    userStoppedRef.current = false
    finishedRef.current = false
    autoRestartsRef.current = 0

    startedAtRef.current = Date.now()
    setElapsedMs(0)
    stopTick()
    tickRef.current = window.setInterval(() => {
      setElapsedMs(Date.now() - startedAtRef.current)
    }, 500)
    // 兜底：到达最长时长自动收尾
    maxTimerRef.current = window.setTimeout(() => {
      stopBrowserASR()
    }, MAX_RECORD_MS)
    setPhase('recording')
    beginRecognition()
  }
  startBrowserASRRef.current = startBrowserASR

  const stopBrowserASR = () => {
    if (finishedRef.current) return
    userStoppedRef.current = true
    if (restartTimerRef.current) {
      window.clearTimeout(restartTimerRef.current)
      restartTimerRef.current = null
    }
    const recognition = recognitionRef.current
    try {
      recognition?.stop()
    } catch {
      /* noop */
    }
    // 识别服务异常时 stop() 可能不触发 onend（表现为“点停止没反应”），
    // 1.5s 后强制 abort 并立即提交已有的 final/interim 文本
    watchdogRef.current = window.setTimeout(() => {
      try {
        recognition?.abort()
      } catch {
        /* noop */
      }
      finishBrowserASR('manual')
    }, STOP_WATCHDOG_MS)
  }

  // ---------- 路径 B：MediaRecorder 上传 → 后端转写（回退） ----------
  const startUpload = async () => {
    setError(null)
    const mime = pickMime()
    if (!mime) {
      setError('当前浏览器不支持录音（需要 Chrome/Edge/Firefox/Safari 较新版本）')
      return
    }
    mimeRef.current = mime
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
      streamRef.current = stream
      const recorder = mime.mime ? new MediaRecorder(stream, { mimeType: mime.mime }) : new MediaRecorder(stream)
      recorderRef.current = recorder
      chunksRef.current = []

      recorder.ondataavailable = (e) => {
        if (e.data.size > 0) chunksRef.current.push(e.data)
      }
      recorder.onstop = async () => {
        const durationMs = Date.now() - startedAtRef.current
        cleanupStream()
        setPhase('processing')
        try {
          const blob = new Blob(chunksRef.current, { type: mime.mime || 'audio/webm' })
          const file = new File([blob], `answer-${Date.now()}.${mime.ext}`, {
            type: blob.type || 'audio/webm',
          })
          await audioApi.upload(questionId, file, durationMs)
          const result = await audioApi.transcribe(questionId)
          if (result.transcript?.trim()) {
            onTranscript(result.transcript.trim())
          } else {
            setError('转写结果为空，请改用文字作答或重试')
          }
        } catch (err) {
          setError(
            err instanceof ApiError
              ? `语音处理失败：${err.message}`
              : '语音处理失败，请检查网络后重试',
          )
        } finally {
          setPhase('idle')
          setElapsedMs(0)
        }
      }

      startedAtRef.current = Date.now()
      setElapsedMs(0)
      tickRef.current = window.setInterval(() => {
        setElapsedMs(Date.now() - startedAtRef.current)
      }, 500)
      recorder.start(250)
      setPhase('recording')
    } catch (err) {
      cleanupStream()
      const name = err instanceof DOMException ? err.name : ''
      if (name === 'NotAllowedError' || name === 'SecurityError') {
        setError('麦克风权限被拒绝，请在浏览器地址栏允许麦克风后重试')
      } else if (name === 'NotFoundError') {
        setError('未检测到麦克风设备')
      } else {
        setError('无法启动录音，请检查设备与浏览器权限')
      }
    }
  }

  const stopUpload = () => {
    if (recorderRef.current && recorderRef.current.state !== 'inactive') {
      recorderRef.current.stop()
    }
  }

  const start = useBrowserASR ? startBrowserASR : startUpload
  const stop = useBrowserASR ? stopBrowserASR : stopUpload

  if (!supported) return null

  const busy = !!disabled
  const recording = phase === 'recording'
  const processing = phase === 'processing'

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
        {recording ? (
          <button
            type="button"
            className="btn"
            onClick={stop}
            style={{ padding: '7px 14px', fontSize: 13, display: 'flex', alignItems: 'center', gap: 8 }}
          >
            <span
              style={{
                width: 9,
                height: 9,
                borderRadius: 2,
                background: 'var(--accent)',
                display: 'inline-block',
              }}
            />
            停止并转写 {formatDuration(elapsedMs)}
          </button>
        ) : (
          <button
            type="button"
            className="btn"
            onClick={start}
            disabled={busy || processing}
            style={{ padding: '7px 14px', fontSize: 13, display: 'flex', alignItems: 'center', gap: 8 }}
            title="用麦克风回答，转写后可在文本框中修改"
          >
            {processing ? (
              <>
                <span className="spinner" /> 转写中…
              </>
            ) : (
              <>
                <span style={{ color: 'var(--accent)', fontSize: 14 }}>●</span> 语音作答
                {useBrowserASR && (
                  <span className="dim" style={{ fontSize: 11 }}>
                    实时识别
                  </span>
                )}
              </>
            )}
          </button>
        )}
        {recording && (
          <span className="dim" style={{ fontSize: 12 }}>
            正在录音，回答完点击「停止并转写」
          </span>
        )}
        {error && (
          <span style={{ fontSize: 12, color: 'var(--accent)' }} title={error}>
            {error}
          </span>
        )}
      </div>
      {recording && (committed || interim) && (
        <div
          style={{
            fontSize: 13,
            lineHeight: 1.7,
            padding: '8px 12px',
            background: 'var(--surface-2, #f7f7f9)',
            borderRadius: 8,
            border: '1px dashed var(--line)',
          }}
        >
          {committed}
          {interim && <span className="dim">{interim}</span>}
        </div>
      )}
    </div>
  )
}
