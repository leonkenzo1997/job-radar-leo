package models

import "time"

// Job represents a normalized IT job listing across all 7 platforms.
type Job struct {
	ID                  string    `json:"id"`
	ExternalID          string    `json:"externalId,omitempty"`
	Platform            string    `json:"platform"` // linkedin, itviec, topdev, topcv, threads, facebook, zalo
	Title               string    `json:"title"`
	Company             string    `json:"company"`
	CompanyLogo         string    `json:"companyLogo,omitempty"`
	Location            string    `json:"location"`
	WorkMode            string    `json:"workMode"` // Remote, Hybrid, On-site, Unknown
	Salary              string    `json:"salary,omitempty"`
	PostedAt            string    `json:"postedAt,omitempty"`
	URL                 string    `json:"url"`
	Description         string    `json:"description,omitempty"`
	Skills              []string  `json:"skills,omitempty"`
	MatchedSkills       []string  `json:"matchedSkills"`
	MissingSkills       []string  `json:"missingSkills,omitempty"`
	MatchScore          int       `json:"matchScore"`
	MatchLevel          string    `json:"matchLevel"` // high, medium, low
	RecommendedTrack    string    `json:"recommendedTrack"`
	RecommendedProjects []string  `json:"recommendedProjects,omitempty"`
	MatchReasons        []string  `json:"matchReasons,omitempty"`
	ContactInfo         string    `json:"contactInfo,omitempty"`
	Status              string    `json:"status"` // new, saved, applied, interviewing, offer, ignored
	Notes               string    `json:"notes,omitempty"`
	QueryUsed           string    `json:"queryUsed,omitempty"`
	FetchedAt           time.Time `json:"fetchedAt"`
}

// FlagshipProject represents a highlight project from Leo's CV used in recommendations & HR pitches.
type FlagshipProject struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Role     string   `json:"role"`
	Period   string   `json:"period"`
	Keywords []string `json:"keywords"`
	Summary  string   `json:"summary"`
}

// CVProfile holds the candidate's CV data and weighted skill tiers.
type CVProfile struct {
	FullName       string            `json:"fullName"`
	Headline       string            `json:"headline"`
	Location       string            `json:"location"`
	YearsExp       int               `json:"yearsExp"`
	Email          string            `json:"email"`
	Phone          string            `json:"phone"`
	GitHub         string            `json:"github"`
	Portfolio      string            `json:"portfolio"`
	LinkedIn       string            `json:"linkedin"`
	TargetRoles    []string          `json:"targetRoles"`
	Tier1Skills    []string          `json:"tier1Skills"` // Core backend (Go, Node, NestJS, TS, Microservices)
	Tier2Skills    []string          `json:"tier2Skills"` // AI & Agentic (LLM, RAG, MCP, OpenAI, Gemini)
	Tier3Skills    []string          `json:"tier3Skills"` // Data, Messaging, Cloud & DevOps
	Tier4Skills    []string          `json:"tier4Skills"` // Frontend & Mobile (React, Flutter)
	Projects       []FlagshipProject `json:"projects"`
	DefaultQueries []string          `json:"defaultQueries"`
}

// RadarLink represents an actionable search/X-Ray link for one of the 7 platforms.
type RadarLink struct {
	Label       string `json:"label"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Kind        string `json:"kind"` // direct, xray, group, post
}

// PlatformRadar groups radar links for a specific platform.
type PlatformRadar struct {
	Platform   string      `json:"platform"`
	Name       string      `json:"name"`
	AccessMode string      `json:"accessMode"` // auto-scrape, xray-radar, login-required
	Badge      string      `json:"badge"`
	Tip        string      `json:"tip"`
	Links      []RadarLink `json:"links"`
}

// ScanRequest configures a live scan across direct-scrape platforms.
type ScanRequest struct {
	Keywords  []string `json:"keywords"`
	Platforms []string `json:"platforms"`
	Location  string   `json:"location"`
}

// PlatformScanStat captures scan outcome per platform.
type PlatformScanStat struct {
	Platform string `json:"platform"`
	Fetched  int    `json:"fetched"`
	Added    int    `json:"added"`
	Updated  int    `json:"updated"`
	Error    string `json:"error,omitempty"`
}

// ScanResult returns summary statistics after running a live scan.
type ScanResult struct {
	ScannedAt   time.Time          `json:"scannedAt"`
	TotalFound  int                `json:"totalFound"`
	NewJobs     int                `json:"newJobs"`
	UpdatedJobs int                `json:"updatedJobs"`
	Stats       []PlatformScanStat `json:"stats"`
}

// ImportJobRequest represents a manual or bookmarklet import (Zalo/FB/Threads/TopCV/any URL).
type ImportJobRequest struct {
	Title       string `json:"title"`
	Company     string `json:"company"`
	URL         string `json:"url"`
	Platform    string `json:"platform"`
	Location    string `json:"location"`
	Salary      string `json:"salary"`
	WorkMode    string `json:"workMode"`
	RawText     string `json:"rawText"`
	ContactInfo string `json:"contactInfo"`
	Status      string `json:"status"`
}

// PitchResponse returns tailored Vietnamese and English messages for applying/reaching out to HR.
type PitchResponse struct {
	VietnameseShort string `json:"vietnameseShort"`
	EnglishPro      string `json:"englishPro"`
}

// DefaultLeoProfile returns the exact CV profile extracted from CV-English-Truong-Tang-Quang.pdf.
func DefaultLeoProfile() CVProfile {
	return CVProfile{
		FullName:  "Truong Tang Quang (Leo)",
		Headline:  "Senior Backend & Fullstack Software Engineer (Golang, Node.js, React, Flutter) | AI-Native Engineering",
		Location:  "Ho Chi Minh City, Vietnam",
		YearsExp:  5,
		Email:     "truongtangquang1997@gmail.com",
		Phone:     "+84 799 759 531",
		GitHub:    "https://github.com/leonkenzo1997",
		Portfolio: "https://leonkenzo1997.github.io/portfolio-leo",
		LinkedIn:  "https://www.linkedin.com/in/quang-truong-tang-1ba617157",
		TargetRoles: []string{
			"Senior Backend Engineer",
			"Golang Backend Developer",
			"Node.js / NestJS Backend Engineer",
			"AI-Native Software Engineer",
			"Senior Fullstack Engineer",
		},
		Tier1Skills: []string{
			"Golang", "Go", "Gin", "GORM", "Wire", "Goose",
			"Node.js", "NodeJS", "NestJS", "Express", "Express.js", "TypeScript", "JavaScript",
			"Microservices", "Event-Driven", "RESTful", "REST API", "WebSocket", "gRPC",
			"OAuth2", "JWT", "PASETO", "2FA", "Anti-Corruption Layer", "RBAC", "Zod",
		},
		Tier2Skills: []string{
			"AI", "AI-Native", "LLM", "RAG", "Agentic", "Multi-Agent", "MCP", "Model Context Protocol",
			"OpenAI", "GPT-4o", "Gemini", "Groq", "FLUX", "Vector Search", "Azure AI Search",
			"Prompt Engineering", "Multimodal", "Embedding", "Revideo", "FFmpeg",
		},
		Tier3Skills: []string{
			"PostgreSQL", "Postgres", "CockroachDB", "MySQL", "MariaDB", "MongoDB", "Mongoose", "TypeORM", "Sequelize",
			"Redis", "Kafka", "Apache Kafka", "Event Hubs", "RabbitMQ", "Elasticsearch", "SQS",
			"Azure", "AKS", "AWS", "EC2", "S3", "Lambda", "Kubernetes", "K8s", "Docker",
			"ArgoCD", "GitOps", "Helm", "CI/CD", "GitHub Actions", "GitLab CI", "Nginx", "HAProxy", "Grafana", "Loki", "Linux", "PM2",
		},
		Tier4Skills: []string{
			"React", "ReactJS", "React.js", "Next.js", "Flutter", "Flutter Web", "Riverpod", "BLoC", "Jest", "Supertest", "Swagger", "OpenAPI",
		},
		Projects: []FlagshipProject{
			{
				ID:       "cosmo8",
				Name:     "COSMO8 — In-Store AI Skin Curator Platform",
				Role:     "AI-Native Software Engineer (Core Backend)",
				Period:   "07/2026 – Present | MTS Corporation VN",
				Keywords: []string{"golang", "go", "gin", "gorm", "rag", "llm", "gpt-4o", "openai", "azure ai search", "postgresql", "redis", "kafka", "aks", "argocd", "microservices", "ai"},
				Summary:  "Kiến trúc monorepo 9 Go microservices (7 PostgreSQL DBs, Redis, Kafka), tích hợp GPT-4o Vision/Text + Azure AI Search RAG (<5s P95), SmartPOS ACL (5 adapters), PASETO/Redis & AES-256-GCM TOTP 2FA, AKS + ArgoCD GitOps.",
			},
			{
				ID:       "rmn",
				Name:     "RMN — Retail Media Network Platform",
				Role:     "AI-Native Software Engineer (Core Backend)",
				Period:   "01/2026 – Present | MTS Corporation VN",
				Keywords: []string{"golang", "go", "node", "nodejs", "gemini", "openai", "groq", "llm", "websocket", "cockroachdb", "redis", "kafka", "aks", "helm", "argocd", "ffmpeg"},
				Summary:  "Hệ sinh thái 17 Go microservices + Node.js 20 Revideo/FFmpeg worker, Multi-LLM/image failover gateway (Gemini/OpenAI/Groq/FLUX), WebSocket điều phối Android TV signage thời gian thực, CockroachDB, Kafka, Helm/ArgoCD.",
			},
			{
				ID:       "giaphuoc",
				Name:     "GiaPhuoc-Express — Express Freight & Debt Reconciliation",
				Role:     "Freelance Backend Developer",
				Period:   "06/2025 – 03/2026 | Gia Phuoc Express",
				Keywords: []string{"node", "nodejs", "typescript", "express", "mongodb", "mongoose", "zod", "jwt", "rbac", "pm2", "zalo", "jest", "ci/cd"},
				Summary:  "Backend Node.js 20 + TypeScript strict mode, MongoDB bulkWrite atomic transactions, engine đối soát công nợ liên trạm tự động, 4-tier JWT RBAC, tích hợp Zalo ZNS/SMS & Jest/Supertest CI/CD.",
			},
			{
				ID:       "busk-bems",
				Name:     "Busk 3PL Logistics & Kumho Electric BEMS IoT",
				Role:     "Backend Developer",
				Period:   "07/2022 – 12/2024 | Brickmate Group VN",
				Keywords: []string{"nestjs", "node", "nodejs", "typescript", "mysql", "typeorm", "redis", "aws", "sqs", "ec2", "s3", "azure", "iot", "amqp", "docker", "shopee", "ecommerce"},
				Summary:  "Backend NestJS 9 & TypeScript cho 7 sản phẩm quốc tế (Smart Building IoT/BEMS Kumho Electric, 3PL Busk giảm 20% API latency, Shopee Open Platform v2, AWS SQS/S3/MediaConvert DRM, Azure IoT Hub).",
			},
			{
				ID:       "pulse",
				Name:     "Pulse-SDMS & Pulseim Enterprise Messaging",
				Role:     "Backend Developer",
				Period:   "04/2025 – 08/2025 | Link Technology Crossborder",
				Keywords: []string{"golang", "go", "gin", "mysql", "aws", "ec2", "s3", "docker", "messaging", "chat", "im"},
				Summary:  "Xây dựng từ đầu Pulse-SDMS (Golang Gin + MySQL) quản lý phát hành server & phát triển hệ thống nhắn tin phân tán Pulseim (gateway-server, im-server, chat-server) trên AWS & Docker.",
			},
		},
		DefaultQueries: []string{
			"Golang",
			"NodeJS",
			"NestJS",
			"AI Engineer",
			"Senior Backend",
			"Fullstack",
		},
	}
}
