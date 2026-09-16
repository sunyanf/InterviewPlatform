import { useEffect, useRef, useState } from 'react'
import type { ChatBubble } from '../ws/useInterviewSocket'
import { speak, speechSupported, stopSpeaking, useSpeaking, useInterviewerVoice } from './speech'
import { VoiceRecorder } from './VoiceRecorder'

export function ChatPanel({
  bubbles,
  streaming,
  disabled,
  error,
  onDismissError,
  onSend,
}: {
  bubbles: ChatBubble[]
  streaming: string
  disabled: boolean
  error: string | null
  onDismissError: () => void
  onSend: (message: string, withTTS: boolean) => void
}) {
  const [draft, setDraft] = useState('')
  const [voiceOn, setVoiceOn] = useInterviewerVoice()
  const speaking = useSpeaking()
  const scrollRef = useRef<HTMLDivElement>(null)
  const replying = !!streaming

  // 新消息/流式增量时滚到底部
  useEffect(() => {
    const el = scrollRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [bubbles, streaming])

  const send = () => {
    const text = draft.trim()
    if (!text || disabled || replying) return
    onSend(text, voiceOn)
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
        {speechSupported() && (
          <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: 10 }}>
            <label
              className="dim"
              style={{ fontSize: 12, display: 'flex', alignItems: 'center', gap: 5, cursor: 'pointer' }}
              title="关闭后面试官只用文字回复，不会自动说话"
            >
              <input
                type="checkbox"
                checked={voiceOn}
                onChange={(e) => setVoiceOn(e.target.checked)}
                style={{ accentColor: 'var(--accent)' }}
              />
              面试官语音
            </label>
          </div>
        )}
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
            {b.role === 'interviewer' && speechSupported() && (
              <button
                type="button"
                onClick={() => speak(b.text)}
                title="朗读这条回复"
                className="dim"
                style={{
                  marginTop: 6,
                  border: 'none',
                  background: 'transparent',
                  padding: 0,
                  cursor: 'pointer',
                  fontSize: 12,
                }}
              >
                🔊 朗读
              </button>
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
        {speechSupported() && speaking && (
          <button
            type="button"
            onClick={stopSpeaking}
            title="立即停止面试官朗读"
            style={{
              width: '100%',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              gap: 10,
              marginBottom: 10,
              padding: '11px 16px',
              fontSize: 14.5,
              fontWeight: 600,
              color: '#fff',
              background: 'var(--accent)',
              border: 'none',
              borderRadius: 10,
              cursor: 'pointer',
              boxShadow: '0 2px 10px rgba(0,0,0,0.12)',
            }}
          >
            <span
              aria-hidden
              style={{
                width: 11,
                height: 11,
                borderRadius: 2,
                background: '#fff',
                display: 'inline-block',
              }}
            />
            面试官正在说话，点击停止
          </button>
        )}
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
        <div style={{ marginTop: 8 }}>
          <VoiceRecorder
            questionId="free-talk"
            disabled={disabled}
            onTranscript={(t) => setDraft((prev) => (prev.trim() ? `${prev.trimEnd()}\n${t}` : t))}
          />
        </div>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginTop: 8 }}>
          <span className="dim" style={{ fontSize: 12 }}>
            {replying ? '考官正在回复…' : 'Enter 发送，Shift+Enter 换行'}
          </span>
          <button
            className="btn btn-primary"
            style={{ padding: '7px 18px', fontSize: 13.5 }}
            onClick={send}
            disabled={disabled || replying || !draft.trim()}
          >
            发送
          </button>
        </div>
      </div>
    </>
  )
}
