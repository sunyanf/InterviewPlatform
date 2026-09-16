import { useSyncExternalStore, useState, useCallback } from 'react'

// 浏览器原生语音合成（SpeechSynthesis）：面试官语音真实朗读，零后端依赖。
// Windows/macOS 自带中文语音；voice 列表异步加载，需监听 voiceschanged。

let cachedVoices: SpeechSynthesisVoice[] = []

function refreshVoices() {
  if (typeof window === 'undefined' || !('speechSynthesis' in window)) return
  cachedVoices = window.speechSynthesis.getVoices()
}

if (typeof window !== 'undefined' && 'speechSynthesis' in window) {
  refreshVoices()
  window.speechSynthesis.onvoiceschanged = refreshVoices
}

export function speechSupported(): boolean {
  return typeof window !== 'undefined' && 'speechSynthesis' in window
}

function pickZhVoice(): SpeechSynthesisVoice | null {
  if (cachedVoices.length === 0) refreshVoices()
  const voices = cachedVoices.length > 0 ? cachedVoices : window.speechSynthesis.getVoices()
  // 优先普通话（中国大陆），其次任意中文语音
  return (
    voices.find((v) => /zh[-_]CN/i.test(v.lang)) ||
    voices.find((v) => /^zh/i.test(v.lang)) ||
    null
  )
}

// ---------- 朗读中状态（供 UI 显示“停止朗读”打断键） ----------
let speaking = false
const speakingListeners = new Set<(v: boolean) => void>()

function setSpeaking(v: boolean) {
  if (speaking === v) return
  speaking = v
  speakingListeners.forEach((l) => l(v))
}

function subscribeSpeaking(listener: (v: boolean) => void): () => void {
  speakingListeners.add(listener)
  return () => speakingListeners.delete(listener)
}

export function useSpeaking(): boolean {
  return useSyncExternalStore(
    subscribeSpeaking,
    () => speaking,
    () => false,
  )
}

// speak 朗读文本：先取消正在进行的朗读，避免多条消息重叠
export function speak(text: string): void {
  if (!speechSupported()) return
  const content = text.trim()
  if (!content) return

  const synth = window.speechSynthesis
  synth.cancel()

  const utter = new SpeechSynthesisUtterance(content)
  utter.lang = 'zh-CN'
  utter.rate = 1
  utter.pitch = 1
  utter.onstart = () => setSpeaking(true)
  utter.onend = () => setSpeaking(false)
  utter.onerror = () => setSpeaking(false)
  const voice = pickZhVoice()
  if (voice) utter.voice = voice
  synth.speak(utter)
  // 部分浏览器在 cancel 后不触发 start 事件，兜底同步标记
  setSpeaking(true)
}

export function stopSpeaking(): void {
  if (!speechSupported()) return
  window.speechSynthesis.cancel()
  setSpeaking(false)
}

// ---------- 面试官自动朗读偏好（纯文字模式可关闭，本地持久化） ----------
const AUTOPLAY_KEY = 'iv.interviewer.autoplay'

export function isAutoplayEnabled(): boolean {
  try {
    return localStorage.getItem(AUTOPLAY_KEY) !== '0'
  } catch {
    return true
  }
}

function persistAutoplay(enabled: boolean) {
  try {
    localStorage.setItem(AUTOPLAY_KEY, enabled ? '1' : '0')
  } catch {
    /* 隐私模式等场景下静默忽略 */
  }
}

// useInterviewerVoice 管理“面试官语音”开关：关闭即纯文字，且立即停止当前朗读
export function useInterviewerVoice(): [boolean, (enabled: boolean) => void] {
  const [enabled, setEnabled] = useState<boolean>(() => isAutoplayEnabled())
  const update = useCallback((v: boolean) => {
    setEnabled(v)
    persistAutoplay(v)
    if (!v) stopSpeaking()
  }, [])
  return [enabled, update]
}
