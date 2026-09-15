import { useEffect, useRef, useState } from 'react'
import type { ChatBubble } from '../ws/useInterviewSocket'

export function ChatPanel({
  bubbles,
  streaming,
  disabled,
  defaultTTS,
  error,
  onDismissError,
  onSend,
}: {
  bubbles: ChatBubble[]
  streaming: string
  disabled: boolean
  defaultTTS: boolean
  error: string | null
  onDismissError: () => void
  onSend: (message: string, withTTS: boolean) => void
}) {
  const [draft, setDraft] = useState('')
  const [withTTS, setWithTTS] = useState(defaultTTS)
  const scrollRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    setWithTTS(defaultTTS)
  }, [defaultTTS])

  // 新消息/流式增量时滚到底部
  useEffect(() => {
    const el = scrollRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [bubbles, streaming])

  const send = () => {
    const text = draft.trim()
    if (!text || disabled) return
    onSend(text, withTTS)
    setDraft('')
  }

  return (
    <>
      <div
        style={{
          padding: '16px 18px',
          borderBottom: '1px solid var(--line)',
          display: 'flex',
          alignItems: 'center',
          gap: 8,
        }}
      >
        <span className="mono dim" style={{ fontSize: 10.5, letterSpacing: '0.28em' }}>
          FREE TALK
        </span>
        <span className="muted" style={{ fontSize: 12.5 }}>和考官自由追问</span>
      </div>

      <div className="iv-chat-scroll" ref={scrollRef}>
        {bubbles.length === 0 && !streaming && (
          <p className="dim" style={{ fontSize: 13, textAlign: 'center', margin: 'auto' }}>
            对题目有疑问、想要求提示或深入讨论，
            <br />
            都可以直接和考官对话。
          </p>
        )}

        {bubbles.map((b) => (
          <div key={b.id} className={`bubble bubble-${b.role}`}>
            <div>{b.text}</div>
            {b.role === 'interviewer' && b.speechUrl && (
              <audio
                controls
                preload="none"
                src={b.speechUrl}
                style={{ marginTop: 8, width: '100%', height: 32 }}
              />
            )}
          </div>
        ))}

        {streaming && (
          <div className="bubble bubble-interviewer streaming-caret">{streaming}</div>
        )}
      </div>

      {error && (
        <div className="notice notice-error" style={{ margin: 10, padding: '8px 12px', fontSize: 12.5 }}>
          <span onClick={onDismissError} style={{ float: 'right', cursor: 'pointer' }}>
            ✕
          </span>
          {error}
        </div>
      )}

      <div style={{ borderTop: '1px solid var(--line)', padding: 12 }}>
        <textarea
          className="input"
          style={{ minHeight: 64, maxHeight: 140, resize: 'none', fontSize: 14 }}
          placeholder={disabled ? '面试未在进行中…' : '输入你想对考官说的话…'}
          value={draft}
          disabled={disabled}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault()
              send()
            }
          }}
        />
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginTop: 8 }}>
          <label
            className="dim"
            style={{ fontSize: 12.5, display: 'flex', alignItems: 'center', gap: 6, cursor: 'pointer' }}
          >
            <input
              type="checkbox"
              checked={withTTS}
              onChange={(e) => setWithTTS(e.target.checked)}
              style={{ accentColor: 'var(--accent)' }}
            />
            语音回复
          </label>
          <button
            className="btn btn-primary"
            style={{ padding: '7px 18px', fontSize: 13.5 }}
            onClick={send}
            disabled={disabled || !draft.trim()}
          >
            发送
          </button>
        </div>
      </div>
    </>
  )
}
