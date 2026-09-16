import { useCallback, useEffect, useRef, useState } from 'react'
import { tokenStore } from '../api/client'
import { interviewApi } from '../api/endpoints'
import type { Answer, AnswerAnalysis, InterviewSession, Question } from '../api/types'
import { isAutoplayEnabled, speak } from '../interview/speech'

// WS 事件信封
interface Envelope<T = unknown> {
  type: string
  data: T
}

export type ConnStatus = 'connecting' | 'open' | 'closed'

// 对话气泡（面试间自由对话流）
export interface ChatBubble {
  id: string
  role: 'candidate' | 'interviewer'
  text: string
  speechUrl?: string
  pending?: boolean
}

// 结构化答题流水事件
export interface AnswerEvent {
  key: string
  questionId: string
  answer?: Answer
  analysis?: AnswerAnalysis
  analysisSkipped?: boolean // 分析服务超时/失败：回答已记录，本轮不再转圈
  followUp?: Question
}

interface SocketState {
  status: ConnStatus
  session: InterviewSession | null
  bubbles: ChatBubble[]
  streaming: string // 面试官流式增量累积
  answerEvents: AnswerEvent[]
  error: string | null
}

const initial: SocketState = {
  status: 'connecting',
  session: null,
  bubbles: [],
  streaming: '',
  answerEvents: [],
  error: null,
}

let seq = 0
const nextId = () => `${Date.now()}-${seq++}`

export function useInterviewSocket(sessionId: string) {
  const [state, setState] = useState<SocketState>(initial)
  const wsRef = useRef<WebSocket | null>(null)
  const retryRef = useRef(0)
  const manualCloseRef = useRef(false)
  const pendingSpeechRef = useRef<string | undefined>(undefined)
  const streamingRef = useRef('')

  const send = useCallback((type: string, data: unknown) => {
    const ws = wsRef.current
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type, data }))
    }
  }, [])

  const sendChat = useCallback(
    (message: string, withTTS: boolean) => {
      streamingRef.current = ''
      pendingSpeechRef.current = undefined
      setState((s) => ({
        ...s,
        streaming: '',
        bubbles: [...s.bubbles, { id: nextId(), role: 'candidate', text: message }],
      }))
      send('chat', { message, tts: withTTS })
    },
    [send],
  )

  const sendAnswer = useCallback(
    (questionId: string, textContent: string, durationMs: number) => {
      const key = nextId()
      setState((s) => ({
        ...s,
        answerEvents: [...s.answerEvents, { key, questionId: questionId }],
      }))
      send('answer', { question_id: questionId, text_content: textContent, duration_ms: durationMs })
      return key
    },
    [send],
  )

  useEffect(() => {
    manualCloseRef.current = false
    let pingTimer: ReturnType<typeof setInterval>
    let reconnectTimer: ReturnType<typeof setTimeout>
    let statusTimer: ReturnType<typeof setTimeout>
    let pollTimers: ReturnType<typeof setTimeout>[] = []
    let pollsArmed = false // 频繁重连下 snapshot 会反复到达，补拉只能排一次，避免定时器被无限重置
    let everOpened = false

    const clearPolls = () => {
      pollTimers.forEach((t) => clearTimeout(t))
      pollTimers = []
      pollsArmed = false
    }

    // 用服务端会话数据对账答题事件：重连后恢复 answer/analysis，
    // 复用已存在的占位事件 key，避免重复气泡。
    // 返回是否仍有悬而未决的答题（需要 REST 补拉）。
    const reconcileFromSession = (sess: InterviewSession, finalize: boolean): boolean => {
      let shouldPoll = false
      setState((s) => {
        const answeredQs = (sess.questions ?? []).filter((q) => q.answered && q.answer)
        let events = s.answerEvents
        for (const q of answeredQs) {
          const answer = q.answer as Answer
          const idx = events.findIndex((e) => e.questionId === q.id)
          if (idx === -1) {
            events = [
              ...events,
              {
                key: nextId(),
                questionId: q.id,
                answer,
                analysis: answer.analysis,
                analysisSkipped: finalize && !answer.analysis ? true : undefined,
              },
            ]
          } else {
            const cur = events[idx]
            const analysis = cur.analysis ?? answer.analysis
            events = events.map((e, i) =>
              i === idx
                ? {
                    ...e,
                    answer: cur.answer ?? answer,
                    analysis,
                    analysisSkipped: analysis
                      ? undefined
                      : cur.analysisSkipped ?? (finalize ? true : undefined),
                  }
                : e,
            )
          }
          if (!answer.analysis && !events.find((e) => e.questionId === q.id)?.analysisSkipped) {
            shouldPoll = true
          }
        }
        // finalize：补拉截止后仍无 answer 的占位，说明提交在断连时丢失
        if (finalize) {
          const answeredIds = new Set(answeredQs.map((q) => q.id))
          const lost = events.some((e) => !e.answer && !answeredIds.has(e.questionId))
          if (lost) {
            events = events.filter((e) => e.answer || answeredIds.has(e.questionId))
            return {
              ...s,
              session: sess,
              answerEvents: events,
              error: '上一条回答因网络中断未送达，请重新提交',
            }
          }
        } else if (events.some((e) => !e.answer)) {
          shouldPoll = true
        }
        return { ...s, session: sess, answerEvents: events }
      })
      return shouldPoll
    }

    // snapshot 到达时后台分析可能仍在进行（脱离连接 ctx 最多 30s）：
    // 安排两轮 REST 补拉，把迟到的 analysis 对账回来。
    const scheduleSnapshotPolls = (needPoll: boolean) => {
      if (!needPoll || pollsArmed) return
      pollsArmed = true
      const fetchAndReconcile = (finalize: boolean, delayMs: number) => {
        const t = setTimeout(() => {
          interviewApi
            .get(sessionId)
            .then((fresh) => {
              if (manualCloseRef.current) return
              const stillPending = reconcileFromSession(fresh, finalize)
              if (!finalize && !stillPending) clearPolls()
            })
            .catch(() => {
              /* 补拉失败保持现状，不打断面试 */
            })
        }, delayMs)
        pollTimers.push(t)
      }
      fetchAndReconcile(false, 10_000)
      fetchAndReconcile(true, 28_000)
    }

    const connect = () => {
      const token = tokenStore.get()
      if (!token) {
        setState((s) => ({ ...s, status: 'closed', error: '未登录' }))
        return
      }
      const proto = location.protocol === 'https:' ? 'wss' : 'ws'
      const url = `${proto}://${location.host}/api/v1/interviews/${sessionId}/ws?token=${encodeURIComponent(token)}`
      const ws = new WebSocket(url)
      wsRef.current = ws
      // 首次连接立即显示“连接中”；断线重连给 500ms 宽限，瞬断不闪黄
      if (!everOpened) setState((s) => ({ ...s, status: 'connecting' }))

      ws.onopen = () => {
        if (wsRef.current !== ws) return // 严格模式/快速重连下的陈旧连接
        everOpened = true
        retryRef.current = 0
        clearTimeout(statusTimer)
        setState((s) => ({ ...s, status: 'open', error: null }))
        pingTimer = setInterval(() => send('ping', {}), 30_000)
      }

      ws.onmessage = (ev) => {
        if (wsRef.current !== ws) return
        const env = JSON.parse(ev.data as string) as Envelope
        switch (env.type) {
          case 'snapshot': {
            const sess = env.data as InterviewSession
            // 重连全量恢复：先对账 answerEvents（含迟到 analysis 的 REST 补拉）
            const needPoll = reconcileFromSession(sess, false)
            scheduleSnapshotPolls(needPoll)
            break
          }
          case 'answer_saved': {
            const answer = env.data as Answer
            setState((s) => ({
              ...s,
              session: patchAnswered(s.session, answer),
              answerEvents: fillLatest(s.answerEvents, (e) => !e.answer, (e) => ({
                ...e,
                answer,
              })),
            }))
            break
          }
          case 'analysis': {
            const analysis = env.data as AnswerAnalysis
            setState((s) => ({
              ...s,
              answerEvents: fillLatest(s.answerEvents, (e) => !e.analysis, (e) => ({
                ...e,
                analysis,
                analysisSkipped: undefined,
              })),
            }))
            break
          }
          case 'analysis_skipped': {
            const data = env.data as { answer_id: string; question_id: string }
            setState((s) => ({
              ...s,
              answerEvents: fillLatest(
                s.answerEvents,
                (e) => e.questionId === data.question_id && !e.analysis && !e.analysisSkipped,
                (e) => ({ ...e, analysisSkipped: true }),
              ),
            }))
            break
          }
          case 'follow_up': {
            const followUp = env.data as Question
            setState((s) => {
              // 把追问题目并入会话题目列表（若不存在）
              const exists = s.session?.questions?.some((q) => q.id === followUp.id)
              const session =
                s.session && !exists
                  ? { ...s.session, questions: [...(s.session.questions ?? []), followUp] }
                  : s.session
              return {
                ...s,
                session,
                answerEvents: fillLatest(s.answerEvents, (e) => !e.followUp, (e) => ({
                  ...e,
                  followUp,
                })),
              }
            })
            break
          }
          case 'chat_delta': {
            const { delta } = env.data as { delta: string }
            streamingRef.current += delta
            setState((s) => ({ ...s, streaming: s.streaming + delta }))
            break
          }
          case 'chat_speech': {
            const data = env.data as { download_url: string }
            pendingSpeechRef.current = data.download_url
            break
          }
          case 'chat_done': {
            const url = pendingSpeechRef.current
            pendingSpeechRef.current = undefined
            const text = streamingRef.current.trim()
            streamingRef.current = ''
            if (text) {
              setState((s) => ({
                ...s,
                streaming: '',
                bubbles: [
                  ...s.bubbles,
                  { id: nextId(), role: 'interviewer', text, speechUrl: url },
                ],
              }))
              // 用户请求了语音（后端返回 speech url）且未静音时，浏览器朗读真实回复
              if (url && isAutoplayEnabled()) speak(text)
            } else {
              setState((s) => ({ ...s, streaming: '' }))
            }
            break
          }
          case 'pong':
            break
          case 'error': {
            const data = env.data as { code: string; message: string }
            streamingRef.current = ''
            pendingSpeechRef.current = undefined
            setState((s) => ({
              ...s,
              streaming: '',
              error: `${data.code}: ${data.message}`,
              // 提交失败：移除尚未落库的答题占位，避免永久“解析中”
              answerEvents: s.answerEvents.filter((e) => e.answer),
            }))
            break
          }
        }
      }

      ws.onclose = () => {
        if (wsRef.current !== ws) return // 已被新连接取代，事件忽略
        clearInterval(pingTimer)
        if (manualCloseRef.current) {
          setState((s) => ({ ...s, status: 'closed' }))
          return
        }
        // 指数退避重连（1s → 16s 封顶）；重连后 snapshot 全量恢复 UI
        const delay = Math.min(1000 * 2 ** retryRef.current, 16_000)
        retryRef.current += 1
        // 曾成功连接过：延迟 500ms 再提示“重连中”，快速重连成功时用户无感知
        clearTimeout(statusTimer)
        statusTimer = setTimeout(() => {
          if (wsRef.current === ws) setState((s) => ({ ...s, status: 'connecting' }))
        }, 500)
        reconnectTimer = setTimeout(connect, delay)
      }

      ws.onerror = () => {
        if (wsRef.current !== ws) return
        // 错误细节由 onclose/error 帧处理，此处仅确保关闭触发重连
        ws.close()
      }
    }

    connect()
    return () => {
      manualCloseRef.current = true
      clearInterval(pingTimer)
      clearTimeout(reconnectTimer)
      clearTimeout(statusTimer)
      clearPolls()
      wsRef.current?.close()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionId, send])

  const dismissError = useCallback(
    () => setState((s) => ({ ...s, error: null })),
    [],
  )

  return { ...state, sendChat, sendAnswer, dismissError }
}

// 回答落库后，把对应题目标记为已回答并挂摘要 answer
function patchAnswered(session: InterviewSession | null, answer: Answer): InterviewSession | null {
  if (!session) return session
  return {
    ...session,
    answered_count: (session.answered_count ?? 0) + 1,
    questions: session.questions?.map((q) =>
      q.id === answer.question_id
        ? { ...q, answered: true, answer_id: answer.id, answer }
        : q,
    ),
  }
}

// fillLatest 找到最后一个满足 cond 的事件并打补丁（一轮答题对应一个占位事件）
function fillLatest<T>(
  list: T[],
  cond: (e: T) => boolean,
  patch: (e: T) => T,
): T[] {
  for (let i = list.length - 1; i >= 0; i--) {
    if (cond(list[i])) {
      const next = [...list]
      next[i] = patch(list[i])
      return next
    }
  }
  return list
}
