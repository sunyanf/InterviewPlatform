import { useEffect, useRef, useState } from 'react'
import { ApiError } from '../api/client'
import { audioApi } from '../api/endpoints'

type Phase = 'idle' | 'recording' | 'processing'

// 候选编码：优先 webm/opus（Chrome/Edge/Firefox），Safari 退回 mp4
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
  return { mime: '', ext: 'webm' } // 交给浏览器默认编码
}

function formatDuration(ms: number): string {
  const total = Math.max(0, Math.round(ms / 1000))
  const m = Math.floor(total / 60)
  const s = total % 60
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

// VoiceRecorder 录制答题语音：MediaRecorder → 上传 → ASR 转写 → 回填文本。
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

  const recorderRef = useRef<MediaRecorder | null>(null)
  const streamRef = useRef<MediaStream | null>(null)
  const chunksRef = useRef<Blob[]>([])
  const mimeRef = useRef<{ mime: string; ext: string } | null>(null)
  const startedAtRef = useRef(0)
  const tickRef = useRef<number | null>(null)

  const supported = typeof navigator !== 'undefined' && !!navigator.mediaDevices && pickMime() !== null

  const cleanupStream = () => {
    streamRef.current?.getTracks().forEach((t) => t.stop())
    streamRef.current = null
    if (tickRef.current) {
      window.clearInterval(tickRef.current)
      tickRef.current = null
    }
  }

  // 卸载/切题时释放麦克风
  useEffect(() => {
    return () => {
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

  const start = async () => {
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

  const stop = () => {
    if (recorderRef.current && recorderRef.current.state !== 'inactive') {
      recorderRef.current.stop()
    }
  }

  if (!supported) return null

  const busy = !!disabled
  const recording = phase === 'recording'
  const processing = phase === 'processing'

  return (
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
  )
}
