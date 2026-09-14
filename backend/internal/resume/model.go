package resume

import "time"

// Resume 简历
type Resume struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	FileURL        string    `json:"file_url"`
	FileName       string    `json:"file_name"`
	FileType       string    `json:"file_type"`
	FileSize       int64     `json:"file_size"`
	ParsedContent  string    `json:"parsed_content,omitempty"`
	StructuredData any       `json:"structured_data,omitempty"`
	Skills         []string  `json:"skills"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// StructuredResume LLM 解析后的结构化简历
type StructuredResume struct {
	Name           string           `json:"name"`
	Email          string           `json:"email"`
	Phone          string           `json:"phone"`
	Education      []Education      `json:"education"`
	WorkExperience []WorkExperience `json:"work_experience"`
	Skills         []string         `json:"skills"`
	Summary        string           `json:"summary"`
	TargetPosition string           `json:"target_position"`
}

// Education 教育经历
type Education struct {
	School string `json:"school"`
	Major  string `json:"major"`
	Degree string `json:"degree"`
}

// WorkExperience 工作经历
type WorkExperience struct {
	Company  string `json:"company"`
	Position string `json:"position"`
	Duration string `json:"duration"`
}

// MatchResult 岗位-简历匹配结果
type MatchResult struct {
	JobID         string   `json:"job_id"`
	JobTitle      string   `json:"job_title"`
	MatchScore    float64  `json:"match_score"`
	MatchedSkills []string `json:"matched_skills"`
	MissingSkills []string `json:"missing_skills"`
}
