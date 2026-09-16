import { api, uploadFile } from './client'
import type {
  AudioAnswer,
  Category,
  CreateSessionRequest,
  Evaluation,
  InterviewSession,
  Job,
  JobList,
  LoginResponse,
  MatchResult,
  RegisterRequest,
  Report,
  Resume,
  SpeechResult,
  SubmitAnswerResult,
  TaskAccepted,
  TaskInfo,
  User,
} from './types'

export const authApi = {
  login: (email: string, password: string) =>
    api.post<LoginResponse>('/auth/login', { email, password }),
  register: (req: RegisterRequest) => api.post<LoginResponse>('/auth/register', req),
  refresh: (refreshToken: string) =>
    api.post<LoginResponse>('/auth/refresh', { refresh_token: refreshToken }),
  logout: (refreshToken: string) =>
    api.post<null>('/auth/logout', { refresh_token: refreshToken }),
  me: () => api.get<User>('/me'),
}

export interface CreateJobRequest {
  category_id: string
  title: string
  description: string
  requirements: string[]
  skills: string[]
  source?: string
  source_url?: string
}

export const jobApi = {
  categories: () => api.get<Category[]>('/jobs/categories'),
  list: (params: { category_id?: string; page?: number; page_size?: number } = {}) => {
    const q = new URLSearchParams()
    if (params.category_id) q.set('category_id', params.category_id)
    if (params.page) q.set('page', String(params.page))
    if (params.page_size) q.set('page_size', String(params.page_size))
    const qs = q.toString()
    return api.get<JobList>(`/jobs${qs ? `?${qs}` : ''}`)
  },
  get: (id: string) => api.get<Job>(`/jobs/${id}`),
  create: (req: CreateJobRequest) => api.post<Job>('/jobs', req),
}

export const resumeApi = {
  upload: (file: File) => uploadFile<Resume>('/resumes', 'file', file),
  list: () => api.get<Resume[]>('/resumes'),
  get: (id: string) => api.get<Resume>(`/resumes/${id}`),
  parse: (id: string) => api.post<TaskAccepted>(`/resumes/${id}/parse`),
  match: (id: string, jobId: string) =>
    api.post<MatchResult>(`/resumes/${id}/match?job_id=${encodeURIComponent(jobId)}`),
}

export const interviewApi = {
  create: (req: CreateSessionRequest) => api.post<InterviewSession>('/interviews', req),
  list: () => api.get<InterviewSession[]>('/interviews'),
  get: (id: string) => api.get<InterviewSession>(`/interviews/${id}`),
  start: (id: string) => api.post<InterviewSession>(`/interviews/${id}/start`),
  finish: (id: string) => api.post<InterviewSession>(`/interviews/${id}/finish`),
  submitAnswer: (id: string, body: { question_id: string; text_content: string; duration_ms: number }) =>
    api.post<SubmitAnswerResult>(`/interviews/${id}/answer`, body),
  openingSpeech: (id: string) => api.get<SpeechResult>(`/interviews/${id}/speech/opening`),
}

export const evaluationApi = {
  // 异步：返回 202 + task_id，凭 taskApi.waitFor 等待完成
  run: (sessionId: string) =>
    api.post<TaskAccepted>(`/evaluations/sessions/${encodeURIComponent(sessionId)}`),
  get: (sessionId: string) =>
    api.get<Evaluation>(`/evaluations/sessions/${encodeURIComponent(sessionId)}`),
}

export const reportApi = {
  generate: (sessionId: string) =>
    api.post<TaskAccepted>(`/reports/sessions/${encodeURIComponent(sessionId)}`),
  get: (sessionId: string) =>
    api.get<Report>(`/reports/sessions/${encodeURIComponent(sessionId)}`),
}

export const taskApi = {
  get: (taskId: string) => api.get<TaskInfo>(`/tasks/${encodeURIComponent(taskId)}`),
}

export const audioApi = {
  // 上传答题录音（multipart：file + duration_ms）
  upload: (questionId: string, file: File, durationMs: number) =>
    uploadFile<AudioAnswer>(
      `/answers/${encodeURIComponent(questionId)}/audio`,
      'file',
      file,
      { duration_ms: String(durationMs) },
    ),
  // 触发 ASR 转写，返回含 transcript 的音频记录
  transcribe: (questionId: string) =>
    api.post<AudioAnswer>(
      `/answers/${encodeURIComponent(questionId)}/audio/transcribe`,
    ),
}
