package tts

// SpeechResult 面试官语音合成结果（返回临时下载 URL，音频存对象存储）
type SpeechResult struct {
	// Text 被合成的文本（仅在日志外返回给客户端本人）
	Text string `json:"text"`
	// DownloadURL 对象存储临时预签名 URL
	DownloadURL string `json:"download_url"`
	// Format 音频格式（mp3 / wav ...）
	Format string `json:"format"`
	// ContentType 音频 MIME 类型
	ContentType string `json:"content_type"`
	// SizeBytes 音频字节数
	SizeBytes int64 `json:"size_bytes"`
	// Cached 是否命中对象存储缓存（未实际调用 TTS Provider）
	Cached bool `json:"cached"`
	// DownloadExpireSeconds 下载 URL 有效期（秒）
	DownloadExpireSeconds int `json:"download_expire_seconds"`
}
