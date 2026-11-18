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
	reNextData24h = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__" type="application/json">(.*?)</script>`)
	re24hDetail   = regexp.MustCompile(`(?s)<(?:div|section)[^>]*class="[^"]*(?:job-description|job-requirement|description)[^"]*"[^>]*>(.*?)</(?:div|section)>`)
)

type vl24hEmployer struct {
	Name string `json:"name"`
	Logo string `json:"logo"`
}

type vl24hSmartTag struct {
	Name     string `json:"name"`
	Category string `json:"category"`
}

type vl24hPlace struct {
	ProvinceID int    `json:"province_id"`
	Address    string `json:"address"`
}

type vl24hItem struct {
	ID               int             `json:"id"`
	Title            string          `json:"title"`
	TitleSlug        string          `json:"title_slug"`
	ProvinceIDs      []int           `json:"province_ids"`
	EmployerInfo     *vl24hEmployer  `json:"employer_info"`
	SalaryMin        int64           `json:"salary_min"`
	SalaryMax        int64           `json:"salary_max"`
	JobRequirement   string          `json:"job_requirement"`
	OtherRequirement string          `json:"other_requirement"`
	Places           string          `json:"places"`
	SmartTags        []vl24hSmartTag `json:"smart_tags"`
	UpdatedAt        int64           `json:"updated_at"`
	RefreshAt        int64           `json:"refresh_at"`
}

type vl24hNextData struct {
	Props struct {
		InitialProps struct {
			PageProps struct {
				JobsResponse struct {
					Items      []vl24hItem `json:"items"`
					TotalPages int         `json:"total_pages"`
				} `json:"jobsResponse"`
			} `json:"pageProps"`
		} `json:"initialProps"`
	} `json:"props"`
}

func format24hSalary(minVal, maxVal int64) string {
	if minVal <= 0 && maxVal <= 0 {
		return "Thỏa thuận"
	}
	toMillion := func(v int64) string {
		if v >= 1_000_000 {
			if v%1_000_000 == 0 {
				return strconv.FormatInt(v/1_000_000, 10)
			}
			return fmt.Sprintf("%.1f", float64(v)/1_000_000.0)
		}
		return strconv.FormatInt(v, 10)
	}
	if minVal > 0 && maxVal > 0 {
		return fmt.Sprintf("%s - %s triệu", toMillion(minVal), toMillion(maxVal))
	}
	if minVal > 0 {
		return fmt.Sprintf("Từ %s triệu", toMillion(minVal))
	}
	return fmt.Sprintf("Tới %s triệu", toMillion(maxVal))
}

func scrape24hPage(client *http.Client, query string, page int) ([]models.Job, int, error) {
	u := fmt.Sprintf(
		"https://vieclam24h.vn/tim-kiem-viec-lam-nhanh?occupation_ids=8&province_ids=210000187&q=%s&page=%d",
		url.QueryEscape(query),
		page,
	)
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
		return nil, 1, fmt.Errorf("vieclam24h status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, 1, err
	}
	htmlStr := string(b)

	m := reNextData24h.FindStringSubmatch(htmlStr)
	if len(m) < 2 {
		return nil, 1, fmt.Errorf("vieclam24h __NEXT_DATA__ not found")
	}
	var nd vl24hNextData
	if err := json.Unmarshal([]byte(m[1]), &nd); err != nil {
		return nil, 1, err
	}
	jr := nd.Props.InitialProps.PageProps.JobsResponse

	// Extract actual detail URLs rendered on page keyed by job ID
	linkByID := make(map[int]string)
	reHref := regexp.MustCompile(`href="(/[^"]*id(\d+)\.html)[^"]*"`)
	for _, hm := range reHref.FindAllStringSubmatch(htmlStr, -1) {
		if len(hm) >= 3 {
			if idNum, err := strconv.Atoi(hm[2]); err == nil {
				linkByID[idNum] = "https://vieclam24h.vn" + hm[1]
			}
		}
	}

	now := time.Now()
	var jobs []models.Job
	for _, it := range jr.Items {
		if it.ID == 0 || strings.TrimSpace(it.Title) == "" {
			continue
		}

		company := "Việc Làm 24h Employer"
		logo := ""
		if it.EmployerInfo != nil {
			if strings.TrimSpace(it.EmployerInfo.Name) != "" {
				company = strings.TrimSpace(it.EmployerInfo.Name)
			}
			logo = strings.TrimSpace(it.EmployerInfo.Logo)
		}

		location := "Hồ Chí Minh"
		if strings.TrimSpace(it.Places) != "" {
			var places []vl24hPlace
			if err := json.Unmarshal([]byte(it.Places), &places); err == nil && len(places) > 0 {
				if addr := strings.TrimSpace(places[0].Address); addr != "" && !strings.Contains(strings.ToLower(addr), "hồ chí minh") {
					location = addr + ", Hồ Chí Minh"
				}
			}
		}

		var skills []string
		for _, tag := range it.SmartTags {
			if strings.TrimSpace(tag.Name) != "" {
				skills = append(skills, strings.TrimSpace(tag.Name))
			}
		}

		var descParts []string
		if reqHTML := cleanHTMLMultiline(it.JobRequirement); reqHTML != "" {
			descParts = append(descParts, "Mô tả công việc:\n"+reqHTML)
		}
		if otherHTML := cleanHTMLMultiline(it.OtherRequirement); otherHTML != "" {
			descParts = append(descParts, "Yêu cầu ứng viên:\n"+otherHTML)
		}

		jobURL := linkByID[it.ID]
		if jobURL == "" {
			slug := strings.TrimSpace(it.TitleSlug)
			if slug == "" {
				slug = "viec-lam-it"
			}
			jobURL = fmt.Sprintf("https://vieclam24h.vn/it-phan-mem/%s-c8p210000187id%d.html", slug, it.ID)
		}

		posted := ""
		ts := it.RefreshAt
		if ts <= 0 {
			ts = it.UpdatedAt
		}
		if ts > 0 {
			posted = time.Unix(ts, 0).Format("02/01/2006")
		}

		jobs = append(jobs, models.Job{
			ID:          "vieclam24h-" + strconv.Itoa(it.ID),
			ExternalID:  strconv.Itoa(it.ID),
			Platform:    "vieclam24h",
			Title:       strings.TrimSpace(it.Title),
			Company:     company,
			CompanyLogo: logo,
			Location:    location,
			WorkMode:    "Unknown",
			Salary:      format24hSalary(it.SalaryMin, it.SalaryMax),
			PostedAt:    posted,
			URL:         jobURL,
			Description: strings.Join(descParts, "\n\n"),
			Skills:      skills,
			Status:      "new",
			QueryUsed:   query,
			FetchedAt:   now,
		})
	}
	return jobs, jr.TotalPages, nil
}

// ScrapeViecLam24h extracts IT Software jobs in TP.HCM from vieclam24h.vn (Tìm Việc 24h).
func ScrapeViecLam24h(client *http.Client, query string) ([]models.Job, error) {
	allJobs, totalPages, err := scrape24hPage(client, query, 1)
	if err != nil {
		return nil, err
	}
	maxPages := 2
	if totalPages < maxPages {
		maxPages = totalPages
	}
	for p := 2; p <= maxPages; p++ {
		more, _, err := scrape24hPage(client, query, p)
		if err != nil {
			break
		}
		allJobs = append(allJobs, more...)
	}
	return allJobs, nil
}

// FetchViecLam24hJobDetail fetches extra text from a vieclam24h.vn detail page if needed.
func FetchViecLam24hJobDetail(client *http.Client, jobURL string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, jobURL, nil)
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
		return "", fmt.Errorf("vieclam24h detail status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	var sections []string
	for _, m := range re24hDetail.FindAllStringSubmatch(string(b), -1) {
		if len(m) >= 2 {
			clean := cleanHTMLMultiline(m[1])
			if len(clean) > 40 {
				sections = append(sections, clean)
			}
		}
	}
	if len(sections) == 0 {
		return "", fmt.Errorf("vieclam24h detail sections not found")
	}
	return strings.Join(sections, "\n\n"), nil
}
