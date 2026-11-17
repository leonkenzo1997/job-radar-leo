package scrapers

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"job-radar-leo/internal/models"
)

var (
	reITTitleURL   = regexp.MustCompile(`(?s)<h3[^>]*data-search--job-selection-target=['"]jobTitle['"][^>]*data-url=['"]([^'"]+)['"][^>]*>\s*<a[^>]*>(.*?)</a>`)
	reITCompany    = regexp.MustCompile(`(?s)<span[^>]*class=['"][^']*text-hover-underline[^']*['"][^>]*>\s*<a[^>]*>(.*?)</a>`)
	reITLogo       = regexp.MustCompile(`data-src=['"](https://itviec\.com/rails/active_storage/[^'"]+)['"]`)
	reITPosted     = regexp.MustCompile(`(?s)Posted\s*(.*?)</span>`)
	reITTag        = regexp.MustCompile(`(?s)data-responsive-tag-list-target=["']tag["'][^>]*>(.*?)</a>`)
	reITLoc        = regexp.MustCompile(`(?s)#map-pin.*?<div[^>]*title=['"]([^'"]+)['"][^>]*>`)
	reITWorkMode   = regexp.MustCompile(`(?s)<div class=['"]text-rich-grey flex-shrink-0['"]>\s*(.*?)\s*</div>`)
	reITJDBlocks   = regexp.MustCompile(`(?s)<div class=['"]imy-5 paragraph['"]>(.*?)</div>`)
	reITJobSlugURL = regexp.MustCompile(`https://itviec\.com/it-jobs/([a-zA-Z0-9\-]+)`)
)

func toITviecSlug(query string) string {
	q := strings.ToLower(strings.TrimSpace(query))
	replacements := map[string]string{
		"node.js":        "nodejs",
		"node js":        "nodejs",
		"node.js backend": "nodejs",
		"react.js":       "reactjs",
		"react js":       "reactjs",
		"ai engineer":    "ai",
		"ai-native":      "ai",
		"senior backend": "backend-developer",
		"backend":        "backend-developer",
		"fullstack":      "full-stack-developer",
		"full-stack":     "full-stack-developer",
	}
	if s, ok := replacements[q]; ok {
		return s
	}
	q = strings.ReplaceAll(q, ".", "")
	q = strings.ReplaceAll(q, " ", "-")
	return url.PathEscape(q)
}

// ScrapeITviec parses public ITviec SSR search listings in Ho Chi Minh City.
func ScrapeITviec(client *http.Client, query string) ([]models.Job, error) {
	slug := toITviecSlug(query)
	u := fmt.Sprintf("https://itviec.com/it-jobs/%s/ho-chi-minh-hcm", slug)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", defaultUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,vi;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("itviec status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 3<<20))
	if err != nil {
		return nil, err
	}
	htmlStr := string(b)

	sep := "data-search--pagination-target='jobCard'"
	if !strings.Contains(htmlStr, sep) {
		sep = `data-search--pagination-target="jobCard"`
	}
	parts := strings.Split(htmlStr, sep)
	if len(parts) <= 1 {
		return nil, nil
	}

	var jobs []models.Job
	now := time.Now()
	for _, card := range parts[1:] {
		// Trim at next job-card wrapper if present
		if next := strings.Index(card, "data-job-impressions-target="); next != -1 {
			card = card[:next]
		}
		mTitle := reITTitleURL.FindStringSubmatch(card)
		if len(mTitle) < 3 {
			continue
		}
		rawURL := html.UnescapeString(mTitle[1])
		if idx := strings.Index(rawURL, "?"); idx != -1 {
			rawURL = rawURL[:idx]
		}
		title := cleanHTMLText(mTitle[2])
		if title == "" {
			continue
		}

		extID := rawURL
		if mSlug := reITJobSlugURL.FindStringSubmatch(rawURL); len(mSlug) >= 2 {
			extID = mSlug[1]
		}

		company := ""
		if m := reITCompany.FindStringSubmatch(card); len(m) >= 2 {
			company = cleanHTMLText(m[1])
		}
		logo := ""
		if m := reITLogo.FindStringSubmatch(card); len(m) >= 2 {
			logo = html.UnescapeString(m[1])
		}
		posted := ""
		if m := reITPosted.FindStringSubmatch(card); len(m) >= 2 {
			posted = "Posted " + cleanHTMLText(m[1])
		}
		location := "Vietnam"
		if m := reITLoc.FindStringSubmatch(card); len(m) >= 2 {
			location = cleanHTMLText(m[1])
		}
		workMode := "Unknown"
		if m := reITWorkMode.FindStringSubmatch(card); len(m) >= 2 {
			wm := cleanHTMLText(m[1])
			if wm != "" {
				workMode = wm
			}
		}

		var skills []string
		for _, tm := range reITTag.FindAllStringSubmatch(card, -1) {
			if len(tm) >= 2 {
				sk := cleanHTMLText(tm[1])
				if sk != "" {
					skills = append(skills, sk)
				}
			}
		}

		salary := "Sign in on ITviec"
		if strings.Contains(card, "VND") || strings.Contains(card, "USD") || strings.Contains(card, "$") {
			salary = "Xem chi tiết trên ITviec"
		}

		jobs = append(jobs, models.Job{
			ID:          "itviec-" + extID,
			ExternalID:  extID,
			Platform:    "itviec",
			Title:       title,
			Company:     company,
			CompanyLogo: logo,
			Location:    location,
			WorkMode:    workMode,
			Salary:      salary,
			PostedAt:    posted,
			URL:         rawURL,
			Skills:      skills,
			Status:      "new",
			QueryUsed:   query,
			FetchedAt:   now,
		})
	}
	return jobs, nil
}

// FetchITviecJobDetail fetches full JD blocks from an ITviec job detail page.
func FetchITviecJobDetail(client *http.Client, jobURL string) (string, error) {
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
		return "", fmt.Errorf("itviec detail status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	matches := reITJDBlocks.FindAllStringSubmatch(string(b), -1)
	var sections []string
	for _, m := range matches {
		if len(m) >= 2 {
			txt := cleanHTMLMultiline(m[1])
			if txt != "" {
				sections = append(sections, txt)
			}
		}
	}
	if len(sections) == 0 {
		return "", fmt.Errorf("itviec detail sections not found")
	}
	return strings.Join(sections, "\n\n"), nil
}
