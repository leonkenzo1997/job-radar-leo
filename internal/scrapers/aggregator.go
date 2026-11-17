package scrapers

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"job-radar-leo/internal/matcher"
	"job-radar-leo/internal/models"
)

const defaultUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

// NewHTTPClient builds a shared HTTP client with safe timeouts.
func NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 16 * time.Second,
	}
}

// RunScan queries LinkedIn, ITviec, and TopDev concurrently and scores all discovered jobs against the CV profile.
func RunScan(req models.ScanRequest, profile models.CVProfile) ([]models.Job, models.ScanResult) {
	keywords := req.Keywords
	usingDefault := len(keywords) == 0
	if usingDefault {
		keywords = profile.DefaultQueries
	}
	if len(keywords) == 0 {
		keywords = []string{"Golang", "NodeJS", "NestJS", "AI Engineer"}
	}

	saraminKeywords := keywords
	if usingDefault {
		saraminKeywords = []string{
			"Golang", "NodeJS", "TypeScript", "React", "Backend", "Fullstack",
			"AI", "Engineer", "Developer", "Software", "DevOps", "Cloud", "Lập trình",
		}
	}

	platformSet := map[string]bool{
		"linkedin": true,
		"itviec":   true,
		"topdev":   true,
	}
	if len(req.Platforms) > 0 {
		platformSet = make(map[string]bool)
		for _, p := range req.Platforms {
			platformSet[strings.ToLower(strings.TrimSpace(p))] = true
		}
	}

	location := strings.TrimSpace(req.Location)
	if location == "" {
		location = "Ho Chi Minh City, Vietnam"
	}

	client := NewHTTPClient()
	sem := make(chan struct{}, 5) // limit concurrent outbound requests
	var linkedinMu sync.Mutex

	type taskResult struct {
		platform string
		jobs     []models.Job
		err      error
	}

	var tasks []func() taskResult
	for _, kw := range keywords {
		q := strings.TrimSpace(kw)
		if q == "" {
			continue
		}
		if platformSet["linkedin"] {
			query := q
			tasks = append(tasks, func() taskResult {
				linkedinMu.Lock()
				defer func() {
					time.Sleep(450 * time.Millisecond)
					linkedinMu.Unlock()
				}()
				jobs, err := ScrapeLinkedIn(client, query, location)
				return taskResult{platform: "linkedin", jobs: jobs, err: err}
			})
		}
		if platformSet["itviec"] {
			query := q
			tasks = append(tasks, func() taskResult {
				jobs, err := ScrapeITviec(client, query)
				return taskResult{platform: "itviec", jobs: jobs, err: err}
			})
		}
	}
	if platformSet["topdev"] {
		for _, kw := range saraminKeywords {
			q := strings.TrimSpace(kw)
			if q == "" {
				continue
			}
			query := q
			tasks = append(tasks, func() taskResult {
				jobs, err := ScrapeTopDev(client, query)
				return taskResult{platform: "topdev", jobs: jobs, err: err}
			})
		}
	}

	resultsCh := make(chan taskResult, len(tasks))
	var wg sync.WaitGroup
	for _, t := range tasks {
		wg.Add(1)
		fn := t
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			resultsCh <- fn()
		}()
	}
	wg.Wait()
	close(resultsCh)

	statMap := map[string]*models.PlatformScanStat{}
	for p := range platformSet {
		statMap[p] = &models.PlatformScanStat{Platform: p}
	}

	dedup := make(map[string]models.Job)
	for res := range resultsCh {
		st, ok := statMap[res.platform]
		if !ok {
			st = &models.PlatformScanStat{Platform: res.platform}
			statMap[res.platform] = st
		}
		if res.err != nil {
			if st.Error == "" {
				st.Error = res.err.Error()
			}
			continue
		}
		st.Fetched += len(res.jobs)
		for _, j := range res.jobs {
			if existing, exists := dedup[j.ID]; exists {
				// Merge skills if discovered via multiple queries
				existing.Skills = mergeUniqueStrings(existing.Skills, j.Skills)
				if existing.Description == "" && j.Description != "" {
					existing.Description = j.Description
				}
				dedup[j.ID] = existing
			} else {
				dedup[j.ID] = j
			}
		}
	}

	var scored []models.Job
	for _, j := range dedup {
		matcher.EvaluateJob(&j, profile)
		if !matcher.IsHCMOrRemote(j) {
			continue
		}
		scored = append(scored, j)
	}

	var stats []models.PlatformScanStat
	for _, p := range []string{"linkedin", "itviec", "topdev"} {
		if st, ok := statMap[p]; ok {
			stats = append(stats, *st)
		}
	}

	return scored, models.ScanResult{
		ScannedAt:  time.Now(),
		TotalFound: len(scored),
		Stats:      stats,
	}
}

func mergeUniqueStrings(a, b []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range append(append([]string{}, a...), b...) {
		k := strings.ToLower(strings.TrimSpace(s))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, s)
	}
	return out
}
