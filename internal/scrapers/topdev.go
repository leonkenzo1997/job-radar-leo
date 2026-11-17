package scrapers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"job-radar-leo/internal/models"
)

var (
	reNextFChunk = regexp.MustCompile(`self\.__next_f\.push\(\[1,"((?:[^"\\]|\\.)*)"\]\)`)
	reTopDevJD   = regexp.MustCompile(`(?s)<(?:div|section)[^>]*class="[^"]*(?:prose|description|responsibilities)[^"]*"[^>]*>(.*?)</(?:div|section)>`)
)

type topDevCompany struct {
	ID          int    `json:"id"`
	DisplayName string `json:"display_name"`
	ImageLogo   string `json:"image_logo"`
}

type topDevSalary struct {
	Value string `json:"value"`
}

type topDevAddresses struct {
	SortAddresses     string `json:"sort_addresses"`
	AddressRegionList string `json:"address_region_list"`
}

type topDevDate struct {
	Date  string `json:"date"`
	Since string `json:"since"`
}

type topDevRawJob struct {
	ID             int             `json:"id"`
	OwnedID        int             `json:"owned_id"`
	Title          string          `json:"title"`
	DetailURL      string          `json:"detail_url"`
	Slug           string          `json:"slug"`
	Company        json.RawMessage `json:"company"`
	SkillsArr      []string        `json:"skills_arr"`
	SkillsStr      string          `json:"skills_str"`
	JobTypesStr    string          `json:"job_types_str"`
	JobLevelsStr   string          `json:"job_levels_str"`
	ExperiencesStr string          `json:"experiences_str"`
	Salary         *topDevSalary   `json:"salary"`
	Addresses      *topDevAddresses `json:"addresses"`
	Published      *topDevDate     `json:"published"`
	Refreshed      *topDevDate     `json:"refreshed"`
}

func extractJSONArrayAt(src string, startBracket int) (string, bool) {
	if startBracket < 0 || startBracket >= len(src) || src[startBracket] != '[' {
		return "", false
	}
	depth := 0
	inStr := false
	esc := false
	for i := startBracket; i < len(src); i++ {
		ch := src[i]
		if esc {
			esc = false
			continue
		}
		if ch == '\\' {
			esc = true
			continue
		}
		if ch == '"' {
			inStr = !inStr
			continue
		}
		if !inStr {
			if ch == '[' {
				depth++
			} else if ch == ']' {
				depth--
				if depth == 0 {
					return src[startBracket : i+1], true
				}
			}
		}
	}
	return "", false
}

// ScrapeTopDev extracts keyword-matched and featured IT jobs from TopDev's Next.js RSC stream.
func ScrapeTopDev(client *http.Client, query string) ([]models.Job, error) {
	u := fmt.Sprintf("https://topdev.vn/jobs/search?keyword=%s", url.QueryEscape(query))
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", defaultUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en-US;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("topdev status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	htmlStr := string(b)

	// Unescape Next.js RSC chunks into one stream
	var sb strings.Builder
	for _, m := range reNextFChunk.FindAllStringSubmatch(htmlStr, -1) {
		if len(m) < 2 {
			continue
		}
		var unescaped string
		if err := json.Unmarshal([]byte(`"`+m[1]+`"`), &unescaped); err == nil {
			sb.WriteString(unescaped)
		}
	}
	full := sb.String()
	if full == "" {
		return nil, nil
	}

	// Pass 1: collect company objects keyed by company ID and job ID
	companyByID := make(map[int]topDevCompany)
	type jobCompanyLookup struct {
		ID      int            `json:"id"`
		Company *topDevCompany `json:"company"`
	}

	var parsedJobs []topDevRawJob
	searchFrom := 0
	for {
		idx := strings.Index(full[searchFrom:], `"jobs":[`)
		if idx == -1 {
			break
		}
		arrStart := searchFrom + idx + len(`"jobs":`)
		arrJSON, ok := extractJSONArrayAt(full, arrStart)
		searchFrom = arrStart + 1
		if !ok {
			continue
		}

		// Try decoding as full job array
		var rawList []topDevRawJob
		if err := json.Unmarshal([]byte(arrJSON), &rawList); err == nil && len(rawList) > 0 {
			for _, rj := range rawList {
				// Record any inline company object
				if len(rj.Company) > 0 && rj.Company[0] == '{' {
					var comp topDevCompany
					if err := json.Unmarshal(rj.Company, &comp); err == nil && comp.ID > 0 && comp.DisplayName != "" {
						companyByID[comp.ID] = comp
					}
				}
				if rj.Title != "" && rj.DetailURL != "" {
					parsedJobs = append(parsedJobs, rj)
				}
			}
		}

		// Also try decoding lightweight {id, company} lookup table
		var lookupList []jobCompanyLookup
		if err := json.Unmarshal([]byte(arrJSON), &lookupList); err == nil {
			for _, item := range lookupList {
				if item.Company != nil && item.Company.ID > 0 && item.Company.DisplayName != "" {
					companyByID[item.Company.ID] = *item.Company
				}
			}
		}
	}

	seen := make(map[int]bool)
	var jobs []models.Job
	now := time.Now()
	for _, rj := range parsedJobs {
		if rj.ID == 0 || seen[rj.ID] {
			continue
		}
		seen[rj.ID] = true

		companyName := ""
		companyLogo := ""
		if len(rj.Company) > 0 && rj.Company[0] == '{' {
			var comp topDevCompany
			if err := json.Unmarshal(rj.Company, &comp); err == nil {
				companyName = comp.DisplayName
				companyLogo = comp.ImageLogo
			}
		}
		if companyName == "" && rj.OwnedID > 0 {
			if comp, ok := companyByID[rj.OwnedID]; ok {
				companyName = comp.DisplayName
				companyLogo = comp.ImageLogo
			}
		}
		if companyName == "" {
			companyName = "TopDev Employer"
		}

		skills := rj.SkillsArr
		if len(skills) == 0 && rj.SkillsStr != "" {
			for _, part := range strings.Split(rj.SkillsStr, ",") {
				if p := strings.TrimSpace(part); p != "" {
					skills = append(skills, p)
				}
			}
		}

		salary := ""
		if rj.Salary != nil {
			salary = strings.TrimSpace(rj.Salary.Value)
		}

		location := "Vietnam"
		if rj.Addresses != nil {
			if strings.TrimSpace(rj.Addresses.SortAddresses) != "" {
				location = strings.TrimSpace(rj.Addresses.SortAddresses)
			} else if strings.TrimSpace(rj.Addresses.AddressRegionList) != "" {
				location = strings.TrimSpace(rj.Addresses.AddressRegionList)
			}
		}

		posted := ""
		if rj.Refreshed != nil && rj.Refreshed.Since != "" {
			posted = rj.Refreshed.Since
		} else if rj.Published != nil && rj.Published.Since != "" {
			posted = rj.Published.Since
		}

		workMode := "Unknown"
		jtLow := strings.ToLower(rj.JobTypesStr)
		switch {
		case strings.Contains(jtLow, "remote"):
			workMode = "Remote"
		case strings.Contains(jtLow, "hybrid"):
			workMode = "Hybrid"
		case strings.Contains(jtLow, "trực tiếp") || strings.Contains(jtLow, "on-site"):
			workMode = "On-site"
		}

		descParts := []string{}
		if rj.JobLevelsStr != "" {
			descParts = append(descParts, "Level: "+rj.JobLevelsStr)
		}
		if rj.ExperiencesStr != "" {
			descParts = append(descParts, "Kinh nghiệm: "+rj.ExperiencesStr)
		}
		if rj.SkillsStr != "" {
			descParts = append(descParts, "Kỹ năng: "+rj.SkillsStr)
		}

		jobs = append(jobs, models.Job{
			ID:          "topdev-" + strconv.Itoa(rj.ID),
			ExternalID:  strconv.Itoa(rj.ID),
			Platform:    "topdev",
			Title:       rj.Title,
			Company:     companyName,
			CompanyLogo: companyLogo,
			Location:    location,
			WorkMode:    workMode,
			Salary:      salary,
			PostedAt:    posted,
			URL:         rj.DetailURL,
			Description: strings.Join(descParts, " | "),
			Skills:      skills,
			Status:      "new",
			QueryUsed:   query,
			FetchedAt:   now,
		})
	}
	return jobs, nil
}

// FetchTopDevJobDetail tries to extract additional JD content from a TopDev detail URL.
func FetchTopDevJobDetail(client *http.Client, detailURL string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, detailURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", defaultUA)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("topdev detail status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	htmlStr := string(b)

	// Extract RSC text payloads (> 120 chars containing HTML paragraphs/lists)
	var sections []string
	for _, m := range reNextFChunk.FindAllStringSubmatch(htmlStr, -1) {
		if len(m) < 2 {
			continue
		}
		var unescaped string
		if err := json.Unmarshal([]byte(`"`+m[1]+`"`), &unescaped); err == nil {
			if (strings.Contains(unescaped, "<li") || strings.Contains(unescaped, "<p")) && len(unescaped) > 100 && !strings.Contains(unescaped, "NodeJS và Golang") {
				clean := cleanHTMLMultiline(unescaped)
				if clean != "" {
					sections = append(sections, clean)
				}
			}
		}
	}
	if len(sections) == 0 {
		for _, m := range reTopDevJD.FindAllStringSubmatch(htmlStr, -1) {
			if len(m) >= 2 {
				clean := cleanHTMLMultiline(m[1])
				if len(clean) > 60 {
					sections = append(sections, clean)
				}
			}
		}
	}
	if len(sections) == 0 {
		return "", fmt.Errorf("topdev detail sections not found")
	}
	return strings.Join(sections, "\n\n"), nil
}
