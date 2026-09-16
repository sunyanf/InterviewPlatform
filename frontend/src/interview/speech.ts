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
  const voice = pickZhVoice()
  if (voice) utter.voice = voice
  synth.speak(utter)
}

export function stopSpeaking(): void {
  if (speechSupported()) window.speechSynthesis.cancel()
}
