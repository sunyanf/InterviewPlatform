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
  const [interim, setInterim] = useState('')

  const recorderRef = useRef<MediaRecorder | null>(null)
  const streamRef = useRef<MediaStream | null>(null)
  const chunksRef = useRef<Blob[]>([])
  const mimeRef = useRef<{ mime: string; ext: string } | null>(null)
  const startedAtRef = useRef(0)
  const tickRef = useRef<number | null>(null)

  // 浏览器原生识别相关
  const recognitionRef = useRef<SRecognition | null>(null)
  const finalTextRef = useRef('')
  const userStoppedRef = useRef(false)
  const unmountedRef = useRef(false)

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
  const startBrowserASR = () => {
    if (!SRTor) return
    setError(null)
    setInterim('')
    finalTextRef.current = ''
    userStoppedRef.current = false

    const recognition = new SRTor()
    recognition.lang = 'zh-CN'
    recognition.continuous = true
    recognition.interimResults = true

    recognition.onresult = (e: SREvent) => {
      let interimText = ''
      for (let i = 0; i < e.results.length; i++) {
        const result = e.results[i]
        if (result.isFinal) {
          finalTextRef.current += result[0].transcript
        } else {
          interimText += result[0].transcript
        }
      }
      setInterim(interimText)
    }

    recognition.onerror = (e: SRErrorEvent) => {
      const name = e.error
      if (name === 'not-allowed' || name === 'service-not-allowed') {
        setError('麦克风权限被拒绝，请在浏览器地址栏允许麦克风后重试')
      } else if (name === 'network') {
        setError('语音识别服务不可达（浏览器识别需联网，可开启系统代理后重试），请改用文字作答')
      } else if (name === 'no-speech') {
        setError('未检测到语音，请重试或改用文字作答')
      } else if (name === 'audio-capture') {
        setError('未检测到麦克风设备')
      } else if (name !== 'aborted') {
        setError(`语音识别失败（${name}），请改用文字作答`)
      }
      userStoppedRef.current = true // onend 不再回填
      stopTick()
      setPhase('idle')
      setInterim('')
    }

    recognition.onend = () => {
      stopTick()
      setInterim('')
      if (unmountedRef.current) return
      const text = finalTextRef.current.trim()
      setPhase('idle')
      setElapsedMs(0)
      if (userStoppedRef.current && text) {
        onTranscript(text)
      } else if (userStoppedRef.current && !text) {
        setError('转写结果为空，请改用文字作答或重试')
      }
    }

    try {
      recognition.start()
      recognitionRef.current = recognition
      startedAtRef.current = Date.now()
      setElapsedMs(0)
      tickRef.current = window.setInterval(() => {
        setElapsedMs(Date.now() - startedAtRef.current)
      }, 500)
      setPhase('recording')
    } catch {
      setError('无法启动语音识别，请重试或改用文字作答')
    }
  }

  const stopBrowserASR = () => {
    userStoppedRef.current = true
    try {
      recognitionRef.current?.stop()
    } catch {
      /* noop */
    }
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
      {recording && interim && (
        <div
          className="dim"
          style={{
            fontSize: 13,
            padding: '8px 12px',
            background: 'var(--surface-2, #f7f7f9)',
            borderRadius: 8,
            border: '1px dashed var(--line)',
          }}
        >
          {interim}
        </div>
      )}
    </div>
  )
}
