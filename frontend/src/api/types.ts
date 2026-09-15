// 后端 DTO 的 TypeScript 镜像（字段名与 backend json tag 严格对齐）

export interface ApiEnvelope<T> {
  code: string
  message: string
  data: T
}

// ---------- user ----------
export interface User {
  id: string
  email: string
  nickname: string
  avatar_url?: string
  status: string
  created_at: string
  updated_at: string
}

export interface LoginResponse {
  token: string
  refresh_token: string
  expires_in: number
  user: User
}

export interface RefreshRequest {
  refresh_token: string
}

export interface RegisterRequest {
  email: string
  password: string
  nickname: string
}

// ---------- job ----------
export interface Category {
  id: string
  name: string
  description?: string
  sort_order: number
}

export interface Job {
  id: string
  category_id: string
  title: string
  description?: string
  requirements: string[]
  skills: string[]
  source?: string
  status: string
  created_at: string
}

export interface JobList {
  list: Job[]
  total: number
  page: number
  size: number
}

// ---------- resume ----------
export interface Resume {
  id: string
  user_id: string
  file_url: string
  file_name: string
  file_type: string
  file_size: number
  parsed_content?: string
  structured_data?: StructuredResume
  skills: string[]
  status: string
  created_at: string
}

export interface StructuredResume {
  name: string
  email: string
  phone: string
  education: { school: string; major: string; degree: string }[]
  work_experience: { company: string; position: string; duration: string }[]
  skills: string[]
  summary: string
  target_position: string
}

export interface MatchResult {
  job_id: string
  job_title: string
  match_score: number
  matched_skills: string[]
  missing_skills: string[]
}

// ---------- interview ----------
export type SessionStatus =
  | 'INIT'
  | 'READY'
  | 'RUNNING'
  | 'PAUSED'
  | 'FINISHING'
  | 'EVALUATING'
  | 'COMPLETED'
  | 'FAILED'

export interface SessionConfig {
  question_count: number
  duration_minutes: number
}

export interface AnswerAnalysis {
  claims: string[]
  correct_points: string[]
  wrong_points: string[]
  missing_points: string[]
  knowledge_gaps: string[]
  prompt_version: string
  model: string
}

export interface Answer {
  id: string
  session_id: string
  question_id: string
  input_type: string
  text_content: string
  duration_ms: number
  analysis?: AnswerAnalysis
  created_at: string
}

export interface Question {
  id: string
  session_id: string
  seq: number
  question_type: string
  question: string
  difficulty: string
  source: string
  expected_points: string[]
  metadata?: Record<string, unknown>
  answered: boolean
  answer_id?: string
  answer?: Answer
}

export interface InterviewSession {
  id: string
  user_id: string
  job_id: string
  resume_id?: string
  interview_type: string
  mode: string
  status: SessionStatus
  config: SessionConfig
  started_at?: string
  ended_at?: string
  created_at: string
  updated_at: string
  job_title?: string
  questions?: Question[]
  answered_count: number
  metadata?: Record<string, unknown>
}

export interface CreateSessionRequest {
  job_id: string
  resume_id: string
  interview_type: 'technical' | 'behavioral' | 'mixed'
  mode: 'text' | 'voice'
  question_count: number
  duration_minutes: number
}

export interface SubmitAnswerResult {
  answer: Answer
  analysis?: AnswerAnalysis
  follow_up?: Question
}

// ---------- evaluation ----------
export interface EvidenceItem {
  question_seq: number
  issue: string
  evidence: string
  reference: string
}

export interface RubricDimension {
  name: string
  label: string
  weight: number
}

export interface Evaluation {
  id: string
  session_id: string
  user_id: string
  dimensions: Record<string, number>
  evidence: EvidenceItem[]
  recommendations: string[]
  rubric: { dimensions: RubricDimension[] }
  total_score: number
  prompt_version: string
  model: string
  created_at: string
}

// ---------- report ----------
export interface FocusArea {
  topic: string
  reason: string
  suggestions: string[]
}

export interface NextTraining {
  focus: string
  suggested_question_type: string
  suggested_difficulty: string
  suggested_topics: string[]
}

export interface Report {
  id: string
  session_id: string
  user_id: string
  total_score: number
  capability_profile: {
    dimensions: Record<string, number>
    total_score: number
    history?: {
      compared_count: number
      avg_total_score: number
      avg_dimensions: Record<string, number>
      delta_total_score: number
      delta_dimensions: Record<string, number>
    }
  }
  strengths: string[]
  weaknesses: string[]
  knowledge_gaps: string[]
  learning_plan: {
    focus_areas: FocusArea[]
    next_training: NextTraining
  }
  prompt_version: string
  model: string
  created_at: string
}

// ---------- tts ----------
export interface SpeechResult {
  text: string
  download_url: string
  format: string
  content_type: string
  size_bytes: number
  cached: boolean
  download_expire_seconds: number
}

// ---------- async task ----------
export interface TaskAccepted {
  task_id: string
  status: 'pending' | 'running'
}

export type TaskStatus = 'pending' | 'running' | 'succeeded' | 'failed'

export interface TaskInfo {
  id: string
  type: string
  status: TaskStatus
  attempts: number
  max_attempts: number
  last_error: string
  created_at: string
  updated_at: string
}
