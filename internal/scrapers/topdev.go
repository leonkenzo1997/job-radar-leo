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
	reRSCTextRef = regexp.MustCompile(`(?s)^([0-9a-fA-F]+):T[0-9a-fA-F]+,(.*)$`)
	reTopDevJD   = regexp.MustCompile(`(?s)<(?:div|section)[^>]*class="[^"]*(?:prose|description|responsibilities)[^"]*"[^>]*>(.*?)</(?:div|section)>`)
	reSaraminMeta = regexp.MustCompile(`^,"meta":\{"total":\d+,"per_page":\d+,"current_page":\d+,"last_page":(\d+)`)
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
	ID                       int              `json:"id"`
	OwnedID                  int              `json:"owned_id"`
	Title                    string           `json:"title"`
	DetailURL                string           `json:"detail_url"`
	Slug                     string           `json:"slug"`
	Company                  json.RawMessage  `json:"company"`
	SkillsArr                []string         `json:"skills_arr"`
	SkillsStr                string           `json:"skills_str"`
	JobTypesStr              string           `json:"job_types_str"`
	JobLevelsStr             string           `json:"job_levels_str"`
	ExperiencesStr           string           `json:"experiences_str"`
	Salary                   *topDevSalary    `json:"salary"`
	Addresses                *topDevAddresses `json:"addresses"`
	Published                *topDevDate      `json:"published"`
	Refreshed                *topDevDate      `json:"refreshed"`
	ResponsibilitiesOriginal string           `json:"responsibilities_original"`
	RequirementsOriginal     string           `json:"requirements_original"`
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

func normalizeSaraminURL(u string) string {
	u = strings.TrimSpace(u)
	u = strings.ReplaceAll(u, "https://topdev.vn/detail-jobs/", "https://saramin.vn/detail-jobs/")
	u = strings.ReplaceAll(u, "http://topdev.vn/detail-jobs/", "https://saramin.vn/detail-jobs/")
	return u
}

func resolveRSCRef(val string, refs map[string]string) string {
	val = strings.TrimSpace(val)
	if val == "" {
		return ""
	}
	if strings.HasPrefix(val, "$") {
		if resolved, ok := refs[val]; ok {
			return cleanHTMLMultiline(resolved)
		}
		return ""
	}
	return cleanHTMLMultiline(val)
}

func scrapeSaraminPage(client *http.Client, query string, page int) ([]models.Job, int, error) {
	u := fmt.Sprintf("https://saramin.vn/jobs/search?keyword=%s&region_ids=79&page=%d", url.QueryEscape(query), page)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, 1, err
	}
	req.Header.Set("User-Agent", defaultUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en-US;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, 1, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, 1, fmt.Errorf("saramin status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, 1, err
	}
	htmlStr := string(b)

	// Unescape Next.js RSC chunks and build $ref -> HTML text map
	var chunks []string
	var sb strings.Builder
	for _, m := range reNextFChunk.FindAllStringSubmatch(htmlStr, -1) {
		if len(m) < 2 {
			continue
		}
		var unescaped string
		if err := json.Unmarshal([]byte(`"`+m[1]+`"`), &unescaped); err == nil {
			chunks = append(chunks, unescaped)
			sb.WriteString(unescaped)
		}
	}
	full := sb.String()
	if full == "" {
		return nil, 1, nil
	}

	rscRefs := make(map[string]string)
	for i, ch := range chunks {
		if m := reRSCTextRef.FindStringSubmatch(ch); len(m) >= 3 {
			refID := "$" + m[1]
			body := m[2]
			if body == "" && i+1 < len(chunks) {
				body = chunks[i+1]
			}
			rscRefs[refID] = body
		}
	}

	// Collect company lookup across all job arrays, but keep ONLY actual search results (followed by ,"meta":{"total":)
	companyByID := make(map[int]topDevCompany)
	type jobCompanyLookup struct {
		ID      int            `json:"id"`
		Company *topDevCompany `json:"company"`
	}

	var parsedJobs []topDevRawJob
	lastPage := 1
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

		// Record lightweight {id, company} lookup table
		var lookupList []jobCompanyLookup
		if err := json.Unmarshal([]byte(arrJSON), &lookupList); err == nil {
			for _, item := range lookupList {
				if item.Company != nil && item.Company.ID > 0 && item.Company.DisplayName != "" {
					companyByID[item.Company.ID] = *item.Company
				}
			}
		}

		var rawList []topDevRawJob
		if err := json.Unmarshal([]byte(arrJSON), &rawList); err == nil && len(rawList) > 0 {
			for _, rj := range rawList {
				if len(rj.Company) > 0 && rj.Company[0] == '{' {
					var comp topDevCompany
					if err := json.Unmarshal(rj.Company, &comp); err == nil && comp.ID > 0 && comp.DisplayName != "" {
						companyByID[comp.ID] = comp
					}
				}
			}
			after := full[arrStart+len(arrJSON):]
			if mMeta := reSaraminMeta.FindStringSubmatch(after); len(mMeta) >= 2 {
				if lp, err := strconv.Atoi(mMeta[1]); err == nil && lp > lastPage {
					lastPage = lp
				}
				for _, rj := range rawList {
					if rj.Title != "" && rj.DetailURL != "" {
						parsedJobs = append(parsedJobs, rj)
					}
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
			companyName = "Saramin Employer"
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

		location := "Hồ Chí Minh"
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
		metaLine := []string{}
		if rj.JobLevelsStr != "" {
			metaLine = append(metaLine, "Level: "+rj.JobLevelsStr)
		}
		if rj.ExperiencesStr != "" {
			metaLine = append(metaLine, "Kinh nghiệm: "+rj.ExperiencesStr)
		}
		if rj.SkillsStr != "" {
			metaLine = append(metaLine, "Kỹ năng: "+rj.SkillsStr)
		}
		if len(metaLine) > 0 {
			descParts = append(descParts, strings.Join(metaLine, " | "))
		}
		if respText := resolveRSCRef(rj.ResponsibilitiesOriginal, rscRefs); respText != "" {
			descParts = append(descParts, "Mô tả công việc:\n"+respText)
		}
		if reqText := resolveRSCRef(rj.RequirementsOriginal, rscRefs); reqText != "" {
			descParts = append(descParts, "Yêu cầu ứng viên:\n"+reqText)
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
			URL:         normalizeSaraminURL(rj.DetailURL),
			Description: strings.Join(descParts, "\n\n"),
			Skills:      skills,
			Status:      "new",
			QueryUsed:   query,
			FetchedAt:   now,
		})
	}
	return jobs, lastPage, nil
}

// ScrapeTopDev extracts keyword-matched TP.HCM IT jobs (with full JD) from Saramin Vietnam (saramin.vn).
func ScrapeTopDev(client *http.Client, query string) ([]models.Job, error) {
	allJobs, lastPage, err := scrapeSaraminPage(client, query, 1)
	if err != nil {
		return nil, err
	}
	maxPages := 3
	if lastPage < maxPages {
		maxPages = lastPage
	}
	for page := 2; page <= maxPages; page++ {
		more, _, err := scrapeSaraminPage(client, query, page)
		if err != nil {
			break
		}
		allJobs = append(allJobs, more...)
	}
	return allJobs, nil
}

// FetchTopDevJobDetail tries to extract additional JD content from a Saramin/TopDev detail URL.
func FetchTopDevJobDetail(client *http.Client, detailURL string) (string, error) {
	detailURL = normalizeSaraminURL(detailURL)
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
		return "", fmt.Errorf("saramin detail status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	htmlStr := string(b)

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
		return "", fmt.Errorf("saramin detail sections not found")
	}
	return strings.Join(sections, "\n\n"), nil
}
