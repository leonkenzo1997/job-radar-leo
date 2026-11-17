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
	reLIJobID    = regexp.MustCompile(`urn:li:jobPosting:(\d+)`)
	reLIURL      = regexp.MustCompile(`class="base-card__full-link[^"]*"[^>]*href="([^"]+)"`)
	reLITitle    = regexp.MustCompile(`(?s)<h3[^>]*class="base-search-card__title"[^>]*>(.*?)</h3>`)
	reLICompany  = regexp.MustCompile(`(?s)<h4[^>]*class="base-search-card__subtitle"[^>]*>(.*?)</h4>`)
	reLILocation = regexp.MustCompile(`(?s)<span[^>]*class="job-search-card__location"[^>]*>(.*?)</span>`)
	reLIDate     = regexp.MustCompile(`(?s)<time[^>]*class="job-search-card__listdate[^"]*"[^>]*>(.*?)</time>`)
	reLILogo     = regexp.MustCompile(`data-delayed-url="(https://media\.licdn\.com[^"]+)"`)
	reLIJDMarkup = regexp.MustCompile(`(?s)<div[^>]*class="[^"]*show-more-less-html__markup[^"]*"[^>]*>(.*?)</div>`)
	reHTMLTags   = regexp.MustCompile(`<[^>]+>`)
)

func cleanHTMLText(s string) string {
	s = reHTMLTags.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

func cleanHTMLMultiline(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = strings.ReplaceAll(s, "</p>", "\n")
	s = strings.ReplaceAll(s, "</li>", "\n")
	s = strings.ReplaceAll(s, "<li>", "• ")
	s = reHTMLTags.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	lines := strings.Split(s, "\n")
	var clean []string
	for _, l := range lines {
		f := strings.Join(strings.Fields(l), " ")
		if f != "" {
			clean = append(clean, f)
		}
	}
	return strings.Join(clean, "\n")
}

// ScrapeLinkedIn queries LinkedIn's public guest jobs API.
func ScrapeLinkedIn(client *http.Client, query, location string) ([]models.Job, error) {
	if strings.TrimSpace(location) == "" {
		location = "Vietnam"
	}
	u := fmt.Sprintf(
		"https://www.linkedin.com/jobs-guest/jobs/api/seeMoreJobPostings/search?keywords=%s&location=%s&start=0",
		url.QueryEscape(query),
		url.QueryEscape(location),
	)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", defaultUA)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,vi;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("linkedin status %d", resp.StatusCode)
	}
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	body := string(bodyBytes)

	cards := strings.Split(body, "<li>")
	var jobs []models.Job
	now := time.Now()
	for _, card := range cards[1:] {
		end := strings.Index(card, "</li>")
		if end != -1 {
			card = card[:end]
		}
		mID := reLIJobID.FindStringSubmatch(card)
		if len(mID) < 2 {
			continue
		}
		extID := mID[1]
		title := ""
		if m := reLITitle.FindStringSubmatch(card); len(m) >= 2 {
			title = cleanHTMLText(m[1])
		}
		if title == "" {
			continue
		}
		company := ""
		if m := reLICompany.FindStringSubmatch(card); len(m) >= 2 {
			company = cleanHTMLText(m[1])
		}
		loc := location
		if m := reLILocation.FindStringSubmatch(card); len(m) >= 2 {
			loc = cleanHTMLText(m[1])
		}
		posted := ""
		if m := reLIDate.FindStringSubmatch(card); len(m) >= 2 {
			posted = cleanHTMLText(m[1])
		}
		jobURL := fmt.Sprintf("https://www.linkedin.com/jobs/view/%s", extID)
		if m := reLIURL.FindStringSubmatch(card); len(m) >= 2 {
			rawURL := html.UnescapeString(m[1])
			if idx := strings.Index(rawURL, "?"); idx != -1 {
				rawURL = rawURL[:idx]
			}
			jobURL = rawURL
		}
		logo := ""
		if m := reLILogo.FindStringSubmatch(card); len(m) >= 2 {
			logo = html.UnescapeString(m[1])
		}

		jobs = append(jobs, models.Job{
			ID:          "linkedin-" + extID,
			ExternalID:  extID,
			Platform:    "linkedin",
			Title:       title,
			Company:     company,
			CompanyLogo: logo,
			Location:    loc,
			WorkMode:    "Unknown",
			PostedAt:    posted,
			URL:         jobURL,
			Status:      "new",
			QueryUsed:   query,
			FetchedAt:   now,
		})
	}
	return jobs, nil
}

// FetchLinkedInJobDetail fetches the full JD description for a LinkedIn job ID.
func FetchLinkedInJobDetail(client *http.Client, externalID string) (string, error) {
	u := fmt.Sprintf("https://www.linkedin.com/jobs-guest/jobs/api/jobPosting/%s", url.PathEscape(externalID))
	req, err := http.NewRequest(http.MethodGet, u, nil)
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
		return "", fmt.Errorf("linkedin detail status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	if m := reLIJDMarkup.FindStringSubmatch(string(b)); len(m) >= 2 {
		return cleanHTMLMultiline(m[1]), nil
	}
	return "", fmt.Errorf("linkedin detail markup not found")
}
