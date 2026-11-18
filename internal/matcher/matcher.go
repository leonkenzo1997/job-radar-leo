package matcher

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"job-radar-leo/internal/models"
)

var (
	reEmail      = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`)
	rePhone      = regexp.MustCompile(`(?:\+84|0)[35789][0-9.\-\s]{7,11}[0-9]`)
	reZaloLink   = regexp.MustCompile(`(?i)https?://zalo\.me/[a-zA-Z0-9_./\-]+`)
	reSalaryLine = regexp.MustCompile(`(?i)(?:up[ \t]*to|tới|đến|từ)?[ \t]*(?:\$[ \t]*\d[\d.,]*(?:[ \t]*(?:[-–~]|[ \t]+to[ \t]+)[ \t]*\$?[ \t]*\d[\d.,]*)?|\d[\d.,]*(?:[ \t]*(?:[-–~]|[ \t]+to[ \t]+)[ \t]*\d[\d.,]*)?[ \t]*(?:USD|\$|triệu|tr(?:$|[\s,.;)])|VNĐ|VND|M(?:$|[\s,.;)])))`)
	reGoWord     = regexp.MustCompile(`(?i)(?:^|[^a-z0-9_])(golang|go)(?:$|[^a-z0-9_])`)
	reGoGeneric  = regexp.MustCompile(`(?i)(?:^|[^a-z0-9_])go(?:$|[^a-z0-9_])`)
)

// canonicalSkill maps aliases to one canonical display label to avoid double counting.
var canonicalAliases = map[string]string{
	"go":                     "Golang",
	"golang":                 "Golang",
	"node.js":                "Node.js",
	"nodejs":                 "Node.js",
	"express.js":             "Express.js",
	"express":                "Express.js",
	"postgres":               "PostgreSQL",
	"postgresql":             "PostgreSQL",
	"k8s":                    "Kubernetes",
	"kubernetes":             "Kubernetes",
	"reactjs":                "ReactJS",
	"react":                  "ReactJS",
	"react.js":               "ReactJS",
	"flutter web":            "Flutter",
	"flutter":                "Flutter",
	"apache kafka":           "Kafka",
	"kafka":                  "Kafka",
	"rest api":               "RESTful API",
	"restful":                "RESTful API",
	"model context protocol": "MCP",
	"mcp":                    "MCP",
	"ai-native":              "AI-Native",
	"gitlab ci":              "CI/CD",
	"github actions":         "CI/CD",
	"ci/cd":                  "CI/CD",
	"azure event hubs":       "Event Hubs",
	"event hubs":             "Event Hubs",
}

// externalSkillChecks detects common stack skills outside Leo's primary core to highlight gaps.
var externalSkillChecks = []string{
	"Java", "Spring Boot", "Python", "Django", "FastAPI", "C#", ".NET", ".NET Core",
	"PHP", "Laravel", "Ruby", "Ruby on Rails", "Rust", "C++", "Scala", "Kotlin",
	"Swift", "iOS", "Android Native", "Angular", "VueJS", "Vue.js", "GraphQL",
	"Terraform", "Ansible", "GCP", "Snowflake", "Spark", "SAP", "Salesforce",
}

func canonicalName(skill string) string {
	low := strings.ToLower(strings.TrimSpace(skill))
	if c, ok := canonicalAliases[low]; ok {
		return c
	}
	return skill
}

func containsSkill(textLower string, skill string, skillTagsLower []string) bool {
	sLower := strings.ToLower(strings.TrimSpace(skill))
	if sLower == "" {
		return false
	}
	for _, tag := range skillTagsLower {
		if tag == sLower || canonicalName(tag) == canonicalName(skill) {
			return true
		}
	}

	// Special care for short tokens that match unrelated English words.
	switch sLower {
	case "go":
		// Only match standalone "Go" if context suggests programming, or exact tag/title match.
		if !reGoGeneric.MatchString(textLower) {
			return false
		}
		// Reject phrases like "go to", "go live", "on the go", "go-to-market"
		cleaned := strings.ReplaceAll(textLower, "go to", " ")
		cleaned = strings.ReplaceAll(cleaned, "go-to", " ")
		cleaned = strings.ReplaceAll(cleaned, "go live", " ")
		cleaned = strings.ReplaceAll(cleaned, "on the go", " ")
		cleaned = strings.ReplaceAll(cleaned, "go-live", " ")
		cleaned = strings.ReplaceAll(cleaned, "go ahead", " ")
		return reGoGeneric.MatchString(cleaned) && (strings.Contains(cleaned, "golang") ||
			strings.Contains(cleaned, "go/") ||
			strings.Contains(cleaned, "/go") ||
			strings.Contains(cleaned, "go,") ||
			strings.Contains(cleaned, ", go") ||
			strings.Contains(cleaned, "go developer") ||
			strings.Contains(cleaned, "go engineer") ||
			strings.Contains(cleaned, "go backend") ||
			strings.Contains(cleaned, "(go)") ||
			strings.Contains(cleaned, "(go,") ||
			strings.Contains(cleaned, ", go)"))
	case "ai":
		return containsWordBoundary(textLower, "ai")
	case "s3", "ec2", "sqs", "aks", "iot", "mcp", "rag", "llm", "jwt", "gin", "pm2", "zod", "2fa":
		return containsWordBoundary(textLower, sLower)
	case ".net", "c#", "c++":
		return strings.Contains(textLower, sLower)
	default:
		if len(sLower) <= 4 {
			return containsWordBoundary(textLower, sLower)
		}
		return strings.Contains(textLower, sLower)
	}
}

func containsWordBoundary(textLower, word string) bool {
	escaped := regexp.QuoteMeta(word)
	re := regexp.MustCompile(`(?:^|[^a-z0-9_])` + escaped + `(?:$|[^a-z0-9_])`)
	return re.MatchString(textLower)
}

// EvaluateJob scores a job against Leo's CV profile and fills matched skills, track, and project suggestions.
func EvaluateJob(job *models.Job, profile models.CVProfile) {
	titleLower := strings.ToLower(job.Title)
	fullText := strings.TrimSpace(job.Title + " " + job.Description + " " + strings.Join(job.Skills, " "))
	textLower := strings.ToLower(fullText)

	var skillTagsLower []string
	for _, s := range job.Skills {
		skillTagsLower = append(skillTagsLower, strings.ToLower(strings.TrimSpace(s)))
	}

	seenMatched := make(map[string]bool)
	var matched []string

	addMatched := func(raw string, points *int, capPts int, perSkill int) {
		canon := canonicalName(raw)
		if seenMatched[canon] {
			return
		}
		seenMatched[canon] = true
		matched = append(matched, canon)
		if *points < capPts {
			*points += perSkill
			if *points > capPts {
				*points = capPts
			}
		}
	}

	tier1Pts, tier2Pts, tier3Pts, tier4Pts := 0, 0, 0, 0
	for _, s := range profile.Tier1Skills {
		if containsSkill(textLower, s, skillTagsLower) {
			addMatched(s, &tier1Pts, 42, 12)
		}
	}
	for _, s := range profile.Tier2Skills {
		if containsSkill(textLower, s, skillTagsLower) {
			addMatched(s, &tier2Pts, 25, 10)
		}
	}
	for _, s := range profile.Tier3Skills {
		if containsSkill(textLower, s, skillTagsLower) {
			addMatched(s, &tier3Pts, 20, 4)
		}
	}
	for _, s := range profile.Tier4Skills {
		if containsSkill(textLower, s, skillTagsLower) {
			addMatched(s, &tier4Pts, 10, 5)
		}
	}

	score := 0
	var reasons []string

	// 1. Role alignment (+25)
	roleSignals := []string{"backend", "back-end", "fullstack", "full-stack", "full stack", "golang", "node", "nestjs", "software engineer", "ai engineer", "agentic", "platform engineer", "distributed", "microservices"}
	roleMatched := false
	for _, sig := range roleSignals {
		if strings.Contains(titleLower, sig) {
			roleMatched = true
			break
		}
	}
	if roleMatched {
		score += 25
		reasons = append(reasons, "Đúng vị trí mục tiêu (Backend / Fullstack / AI-Native)")
	} else if strings.Contains(titleLower, "developer") || strings.Contains(titleLower, "engineer") || strings.Contains(titleLower, "lập trình") {
		score += 12
	}

	// 2. Seniority alignment (+10 or -25)
	seniorSignals := []string{"senior", "sr.", "lead", "staff", "middle", "mid-senior", "principal", "architect", "chuyên viên cao cấp"}
	juniorSignals := []string{"intern", "fresher", "thực tập", "junior only"}
	hasSenior := false
	for _, sig := range seniorSignals {
		if strings.Contains(titleLower, sig) || strings.Contains(textLower, "3+ years") || strings.Contains(textLower, "4+ years") || strings.Contains(textLower, "5+ years") || strings.Contains(textLower, "3 năm") || strings.Contains(textLower, "5 năm") {
			hasSenior = true
			break
		}
	}
	hasJunior := false
	for _, sig := range juniorSignals {
		if strings.Contains(titleLower, sig) {
			hasJunior = true
			break
		}
	}
	if hasJunior && !hasSenior {
		score -= 25
		reasons = append(reasons, "Cấp bậc Intern/Fresher thấp hơn kinh nghiệm 5+ năm")
	} else if hasSenior {
		score += 10
		reasons = append(reasons, "Khớp cấp bậc Middle/Senior (5+ năm kinh nghiệm)")
	}

	// 3. Skill tier points
	score += tier1Pts + tier2Pts + tier3Pts + tier4Pts
	if tier1Pts >= 12 {
		reasons = append(reasons, "Trùng khớp Core Backend (Golang / Node.js / NestJS / TypeScript)")
	}
	if tier2Pts >= 10 {
		reasons = append(reasons, "Lợi thế mạnh về AI/LLM/RAG/Agentic")
	}

	// 4. Location / WorkMode bonus (+8)
	locLower := strings.ToLower(job.Location + " " + job.WorkMode + " " + textLower)
	if strings.Contains(locLower, "ho chi minh") || strings.Contains(locLower, "hồ chí minh") || strings.Contains(locLower, "hcm") || strings.Contains(locLower, "remote") || strings.Contains(locLower, "hybrid") {
		score += 8
		reasons = append(reasons, "Phù hợp địa điểm làm việc (TP.HCM / Remote / Hybrid)")
	}

	// 5. Off-stack penalty (-30)
	offStackMarkers := []string{".net", "c#", "laravel", "wordpress", "cobol", "swift", "unity", "ruby on rails", "sap ", "abap", "magento", "drupal"}
	hasCoreLeo := seenMatched["Golang"] || seenMatched["Node.js"] || seenMatched["NestJS"] || seenMatched["TypeScript"] || seenMatched["AI"] || seenMatched["LLM"] || seenMatched["RAG"] || seenMatched["ReactJS"] || seenMatched["Flutter"]
	if !hasCoreLeo {
		for _, off := range offStackMarkers {
			if strings.Contains(titleLower, off) || containsSkill(textLower, off, skillTagsLower) {
				score -= 30
				reasons = append(reasons, "Công nghệ chính không phải Go/Node.js/AI trong CV")
				break
			}
		}
	}

	// 6. Missing skills detection
	var missing []string
	for _, ext := range externalSkillChecks {
		if containsSkill(textLower, ext, skillTagsLower) {
			canon := canonicalName(ext)
			if !seenMatched[canon] {
				missing = append(missing, canon)
			}
		}
	}

	if score < 5 {
		score = 5
	}
	if score > 99 {
		score = 99
	}

	job.MatchScore = score
	if score >= 75 {
		job.MatchLevel = "high"
	} else if score >= 50 {
		job.MatchLevel = "medium"
	} else {
		job.MatchLevel = "low"
	}

	sort.Strings(matched)
	job.MatchedSkills = matched
	job.MissingSkills = missing
	job.MatchReasons = reasons

	// Determine RecommendedTrack
	hasGo := seenMatched["Golang"] || reGoWord.MatchString(titleLower)
	hasNode := seenMatched["Node.js"] || seenMatched["NestJS"] || seenMatched["Express.js"] || seenMatched["TypeScript"]
	hasAI := tier2Pts >= 10 || strings.Contains(titleLower, "ai ") || strings.Contains(titleLower, "llm") || strings.Contains(titleLower, "agent") || strings.Contains(titleLower, "rag")
	hasFE := seenMatched["ReactJS"] || seenMatched["Flutter"] || strings.Contains(titleLower, "fullstack") || strings.Contains(titleLower, "full-stack") || strings.Contains(titleLower, "full stack")

	switch {
	case hasAI && (hasGo || hasNode || strings.Contains(titleLower, "ai")):
		job.RecommendedTrack = "AI-Native (LLM / RAG / Agents)"
	case hasFE && (hasGo || hasNode):
		job.RecommendedTrack = "Fullstack (Go/Node + React/Flutter)"
	case hasGo:
		job.RecommendedTrack = "Backend Golang & Microservices"
	case hasNode:
		job.RecommendedTrack = "Node.js / NestJS Backend"
	case tier3Pts >= 8:
		job.RecommendedTrack = "Cloud / DevOps & Distributed Systems"
	default:
		job.RecommendedTrack = "Software Engineering (Tham khảo)"
	}

	// Pick top 2 Flagship Projects from CV
	type projScore struct {
		name  string
		score int
	}
	var pScores []projScore
	for _, p := range profile.Projects {
		ps := 0
		for _, kw := range p.Keywords {
			if strings.Contains(textLower, strings.ToLower(kw)) {
				ps++
			}
		}
		pScores = append(pScores, projScore{name: p.Name, score: ps})
	}
	sort.Slice(pScores, func(i, j int) bool {
		return pScores[i].score > pScores[j].score
	})
	var recProjs []string
	for i := 0; i < len(pScores) && i < 2; i++ {
		if pScores[i].score > 0 || i == 0 {
			recProjs = append(recProjs, pScores[i].name)
		}
	}
	job.RecommendedProjects = recProjs

	// Infer WorkMode if unknown
	if job.WorkMode == "" || job.WorkMode == "Unknown" {
		switch {
		case strings.Contains(locLower, "remote"):
			job.WorkMode = "Remote"
		case strings.Contains(locLower, "hybrid"):
			job.WorkMode = "Hybrid"
		case strings.Contains(locLower, "on-site") || strings.Contains(locLower, "onsite") || strings.Contains(locLower, "at office") || strings.Contains(locLower, "trực tiếp"):
			job.WorkMode = "On-site"
		default:
			job.WorkMode = "Unknown"
		}
	}
}

// ParseRawJobPost converts pasted text from Zalo, Facebook, Threads, or TopCV into a scored Job.
func ParseRawJobPost(req models.ImportJobRequest, profile models.CVProfile) models.Job {
	raw := strings.TrimSpace(req.RawText)
	urlLower := strings.ToLower(req.URL)
	rawLower := strings.ToLower(raw)

	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	if platform == "" || platform == "auto" {
		switch {
		case strings.Contains(urlLower, "threads.net") || strings.Contains(urlLower, "threads.com"):
			platform = "threads"
		case strings.Contains(urlLower, "facebook.com") || strings.Contains(urlLower, "fb.com"):
			platform = "facebook"
		case strings.Contains(urlLower, "zalo.me") || strings.Contains(rawLower, "zalo"):
			platform = "zalo"
		case strings.Contains(urlLower, "topcv.vn"):
			platform = "topcv"
		case strings.Contains(urlLower, "linkedin.com"):
			platform = "linkedin"
		case strings.Contains(urlLower, "itviec.com"):
			platform = "itviec"
		case strings.Contains(urlLower, "topdev.vn") || strings.Contains(urlLower, "saramin.vn"):
			platform = "topdev"
		case strings.Contains(urlLower, "vieclam24h.vn") || strings.Contains(urlLower, "timviec24h"):
			platform = "vieclam24h"
		case strings.Contains(rawLower, "threads"):
			platform = "threads"
		case strings.Contains(rawLower, "facebook") || strings.Contains(rawLower, "group"):
			platform = "facebook"
		case strings.Contains(rawLower, "topcv"):
			platform = "topcv"
		case strings.Contains(rawLower, "saramin"):
			platform = "topdev"
		case strings.Contains(rawLower, "vieclam24h") || strings.Contains(rawLower, "timviec24h"):
			platform = "vieclam24h"
		default:
			platform = "zalo"
		}
	}

	title := strings.TrimSpace(req.Title)
	if title == "" && raw != "" {
		lines := strings.Split(raw, "\n")
		for _, line := range lines {
			clean := strings.TrimSpace(line)
			if clean == "" || strings.HasPrefix(strings.ToLower(clean), "http") {
				continue
			}
			// Clean decorative hashtags/brackets at start if short
			if len([]rune(clean)) > 90 {
				runes := []rune(clean)
				clean = string(runes[:87]) + "..."
			}
			title = clean
			break
		}
	}
	if title == "" {
		title = "IT Job Opportunity (" + strings.ToUpper(platform) + ")"
	}

	company := strings.TrimSpace(req.Company)
	if company == "" {
		company = extractCompanyHint(raw)
	}
	if company == "" {
		company = "HR / Nguồn " + strings.ToUpper(platform)
	}

	salary := strings.TrimSpace(req.Salary)
	if salary == "" {
		salary = extractSalaryHint(raw)
	}

	contact := strings.TrimSpace(req.ContactInfo)
	if contact == "" {
		var parts []string
		if m := reZaloLink.FindString(raw); m != "" {
			parts = append(parts, m)
		}
		if m := rePhone.FindString(raw); m != "" {
			parts = append(parts, "SĐT/Zalo: "+strings.TrimSpace(m))
		}
		if m := reEmail.FindString(raw); m != "" {
			parts = append(parts, "Email: "+strings.TrimSpace(m))
		}
		contact = strings.Join(parts, " | ")
	}

	location := strings.TrimSpace(req.Location)
	if location == "" {
		switch {
		case strings.Contains(rawLower, "hồ chí minh") || strings.Contains(rawLower, "ho chi minh") || strings.Contains(rawLower, "hcm") || strings.Contains(rawLower, "sài gòn"):
			location = "Ho Chi Minh City"
		case strings.Contains(rawLower, "hà nội") || strings.Contains(rawLower, "ha noi"):
			location = "Ha Noi"
		case strings.Contains(rawLower, "đà nẵng") || strings.Contains(rawLower, "da nang"):
			location = "Da Nang"
		case strings.Contains(rawLower, "remote"):
			location = "Remote"
		default:
			location = "Vietnam"
		}
	}

	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "saved"
	}

	idSeed := req.URL
	if idSeed == "" {
		idSeed = platform + "|" + title + "|" + raw
	}
	sum := sha1.Sum([]byte(idSeed))
	id := platform + "-" + hex.EncodeToString(sum[:6])

	job := models.Job{
		ID:          id,
		Platform:    platform,
		Title:       title,
		Company:     company,
		Location:    location,
		WorkMode:    req.WorkMode,
		Salary:      salary,
		PostedAt:    "Vừa nhập (" + time.Now().Format("02/01 15:04") + ")",
		URL:         strings.TrimSpace(req.URL),
		Description: raw,
		ContactInfo: contact,
		Status:      status,
		FetchedAt:   time.Now(),
	}

	EvaluateJob(&job, profile)
	return job
}

func extractSalaryHint(raw string) string {
	lines := strings.Split(raw, "\n")
	// Pass 1: prioritize lines explicitly mentioning salary/income keywords
	for _, line := range lines {
		low := strings.ToLower(line)
		if strings.Contains(low, "lương") || strings.Contains(low, "salary") || strings.Contains(low, "thu nhập") || strings.Contains(low, "offer") || strings.Contains(low, "package") || strings.Contains(low, "upto") || strings.Contains(low, "up to") {
			if m := reSalaryLine.FindString(line); m != "" {
				return strings.TrimSpace(m)
			}
		}
	}
	// Pass 2: fallback scan per line without crossing newlines
	for _, line := range lines {
		if m := reSalaryLine.FindString(line); m != "" {
			return strings.TrimSpace(m)
		}
	}
	return ""
}

func extractCompanyHint(raw string) string {
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		low := strings.ToLower(strings.TrimSpace(line))
		for _, prefix := range []string{"công ty:", "company:", "tại công ty", "team "} {
			if strings.HasPrefix(low, prefix) {
				val := strings.TrimSpace(line[len(prefix):])
				if len(val) > 2 && len(val) < 80 {
					return val
				}
			}
		}
	}
	return ""
}

// GeneratePitch crafts tailored Vietnamese & English application messages for a job.
func GeneratePitch(job models.Job, profile models.CVProfile) models.PitchResponse {
	skillsStr := "Golang, Node.js (NestJS), TypeScript, Microservices, PostgreSQL/Kafka và AI-Native (LLM/RAG)"
	if len(job.MatchedSkills) > 0 {
		limit := len(job.MatchedSkills)
		if limit > 6 {
			limit = 6
		}
		skillsStr = strings.Join(job.MatchedSkills[:limit], ", ")
	}

	projSummary := ""
	if len(job.RecommendedProjects) > 0 {
		for _, p := range profile.Projects {
			if p.Name == job.RecommendedProjects[0] {
				projSummary = p.Summary
				break
			}
		}
	}
	if projSummary == "" && len(profile.Projects) > 0 {
		projSummary = profile.Projects[0].Summary
	}

	vi := fmt.Sprintf(
		"Chào anh/chị HR bên %s,\n\n"+
			"Mình là Trương Tăng Quang (Leo), Senior Backend & Fullstack Engineer (5+ năm kinh nghiệm). "+
			"Mình vừa đọc tin tuyển dụng vị trí %s và thấy rất khớp với kinh nghiệm thực chiến của mình (%s).\n\n"+
			"Điểm nổi bật gần đây: %s\n\n"+
			"Anh/chị xem thêm hồ sơ & 13 dự án production của mình tại:\n"+
			"- Portfolio & Web CV: %s\n"+
			"- LinkedIn: %s\n"+
			"- SĐT / Zalo: %s | Email: %s\n\n"+
			"Rất mong được trao đổi thêm với team!",
		job.Company, job.Title, skillsStr, projSummary, profile.Portfolio, profile.LinkedIn, profile.Phone, profile.Email,
	)

	en := fmt.Sprintf(
		"Hi %s Hiring Team,\n\n"+
			"I am Truong Tang Quang (Leo), a Senior Backend & Fullstack Software Engineer with 5+ years of production experience building distributed systems in Golang, Node.js (NestJS/TypeScript), React/Flutter, and AI-Native LLM/RAG pipelines.\n\n"+
			"I am very interested in the %s role, especially given my direct hands-on alignment with %s. Most recently, I architected production microservices, multi-LLM/RAG orchestration, Kafka event-driven pipelines, and Kubernetes/ArgoCD GitOps deployments.\n\n"+
			"You can review my full architecture archive (13 production projects) and CV here:\n"+
			"- Portfolio: %s\n"+
			"- GitHub: %s\n"+
			"- LinkedIn: %s\n"+
			"- Contact: %s | %s\n\n"+
			"Best regards,\nTruong Tang Quang (Leo)",
		job.Company, job.Title, skillsStr, profile.Portfolio, profile.GitHub, profile.LinkedIn, profile.Email, profile.Phone,
	)

	return models.PitchResponse{
		VietnameseShort: vi,
		EnglishPro:      en,
	}
}

// IsHCMOrRemote returns true when a job is located in Ho Chi Minh City or supports Remote work.
func IsHCMOrRemote(job models.Job) bool {
	hay := strings.ToLower(job.Location + " " + job.WorkMode + " " + job.Title)
	markers := []string{
		"ho chi minh", "hồ chí minh", "hcm", "tphcm", "tp.hcm",
		"sài gòn", "saigon", "thủ đức", "thu duc",
		"tân bình", "tan binh", "bình thạnh", "binh thanh", "phú nhuận", "phu nhuan",
		"quận 1", "quận 2", "quận 3", "quận 4", "quận 5", "quận 7", "quận 10",
		"district 1", "district 3", "district 7",
		"remote", "work from home", "wfh", "từ xa",
	}
	for _, m := range markers {
		if strings.Contains(hay, m) {
			return true
		}
	}
	return false
}
