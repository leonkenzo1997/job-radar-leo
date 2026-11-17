package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"job-radar-leo/internal/matcher"
	"job-radar-leo/internal/models"
	"job-radar-leo/internal/radar"
	"job-radar-leo/internal/scrapers"
)

type AutoScanStatus struct {
	Enabled         bool      `json:"enabled"`
	IntervalMinutes int       `json:"intervalMinutes"`
	NextScanAt      time.Time `json:"nextScanAt"`
}

type Store struct {
	mu                  sync.RWMutex
	scanMu              sync.Mutex
	scanning            bool
	lastScan            *models.ScanResult
	autoScanEnabled     bool
	autoScanIntervalMin int
	nextScanAt          time.Time
	dataDir             string
	jobsFile            string
	profileFile         string
	jobs                map[string]models.Job
	profile             models.CVProfile
	httpClient          *http.Client
}

func NewStore(baseDir string) (*Store, error) {
	dataDir := filepath.Join(baseDir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{
		autoScanEnabled:     true,
		autoScanIntervalMin: 15,
		nextScanAt:          time.Now().Add(15 * time.Minute),
		dataDir:             dataDir,
		jobsFile:            filepath.Join(dataDir, "jobs.json"),
		profileFile:         filepath.Join(dataDir, "profile.json"),
		jobs:                make(map[string]models.Job),
		httpClient:          scrapers.NewHTTPClient(),
	}
	if err := s.loadProfile(); err != nil {
		return nil, err
	}
	if err := s.loadJobs(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) loadProfile() error {
	b, err := os.ReadFile(s.profileFile)
	if err != nil {
		if os.IsNotExist(err) {
			s.profile = models.DefaultLeoProfile()
			return s.saveProfileLocked()
		}
		return err
	}
	var p models.CVProfile
	if err := json.Unmarshal(b, &p); err != nil {
		s.profile = models.DefaultLeoProfile()
		return s.saveProfileLocked()
	}
	s.profile = p
	return nil
}

func (s *Store) saveProfileLocked() error {
	b, err := json.MarshalIndent(s.profile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.profileFile, b, 0o644)
}

func (s *Store) loadJobs() error {
	b, err := os.ReadFile(s.jobsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return s.saveJobsLocked()
		}
		return err
	}
	var list []models.Job
	if err := json.Unmarshal(b, &list); err != nil {
		return nil
	}
	pruned := false
	for _, j := range list {
		matcher.EvaluateJob(&j, s.profile)
		if !matcher.IsHCMOrRemote(j) {
			pruned = true
			continue
		}
		s.jobs[j.ID] = j
	}
	if pruned {
		return s.saveJobsLocked()
	}
	return nil
}

func (s *Store) saveJobsLocked() error {
	list := make([]models.Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		list = append(list, j)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].MatchScore != list[j].MatchScore {
			return list[i].MatchScore > list[j].MatchScore
		}
		return list[i].FetchedAt.After(list[j].FetchedAt)
	})
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.jobsFile, b, 0o644)
}

func (s *Store) RunAndMergeScan(req models.ScanRequest) models.ScanResult {
	s.scanMu.Lock()
	s.mu.Lock()
	s.scanning = true
	profile := s.profile
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.scanning = false
		s.mu.Unlock()
		s.scanMu.Unlock()
	}()

	scannedJobs, summary := scrapers.RunScan(req, profile)

	s.mu.Lock()
	defer s.mu.Unlock()

	scannedSaramin := false
	for _, st := range summary.Stats {
		if st.Platform == "topdev" && st.Error == "" && st.Fetched > 0 {
			scannedSaramin = true
		}
	}

	preservedTopDev := make(map[string]models.Job)
	for id, existing := range s.jobs {
		if !matcher.IsHCMOrRemote(existing) {
			delete(s.jobs, id)
			continue
		}
		if scannedSaramin && existing.Platform == "topdev" {
			preservedTopDev[id] = existing
			delete(s.jobs, id)
		}
	}

	addedByPlat := make(map[string]int)
	updatedByPlat := make(map[string]int)

	for _, incoming := range scannedJobs {
		if !matcher.IsHCMOrRemote(incoming) {
			continue
		}
		existing, ok := s.jobs[incoming.ID]
		if !ok {
			existing, ok = preservedTopDev[incoming.ID]
		}
		if ok {
			// Preserve user workflow fields and richer fetched JD description
			if existing.Status != "" {
				incoming.Status = existing.Status
			}
			if existing.Notes != "" {
				incoming.Notes = existing.Notes
			}
			if len(existing.Description) > len(incoming.Description) {
				incoming.Description = existing.Description
			}
			if existing.Salary != "" && (incoming.Salary == "" || incoming.Salary == "Sign in on ITviec") {
				incoming.Salary = existing.Salary
			}
			matcher.EvaluateJob(&incoming, s.profile)
			s.jobs[incoming.ID] = incoming
			summary.UpdatedJobs++
			updatedByPlat[incoming.Platform]++
		} else {
			s.jobs[incoming.ID] = incoming
			summary.NewJobs++
			addedByPlat[incoming.Platform]++
		}
	}

	for i := range summary.Stats {
		p := summary.Stats[i].Platform
		summary.Stats[i].Added = addedByPlat[p]
		summary.Stats[i].Updated = updatedByPlat[p]
	}

	s.lastScan = &summary
	if s.autoScanIntervalMin > 0 {
		s.nextScanAt = time.Now().Add(time.Duration(s.autoScanIntervalMin) * time.Minute)
	}
	_ = s.saveJobsLocked()
	return summary
}

func (s *Store) autoScanSnapshotLocked() AutoScanStatus {
	return AutoScanStatus{
		Enabled:         s.autoScanEnabled,
		IntervalMinutes: s.autoScanIntervalMin,
		NextScanAt:      s.nextScanAt,
	}
}

func (s *Store) StartAutoScanLoop() {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			s.mu.Lock()
			due := s.autoScanEnabled && !s.scanning && s.autoScanIntervalMin > 0 && time.Now().After(s.nextScanAt)
			if due {
				s.nextScanAt = time.Now().Add(time.Duration(s.autoScanIntervalMin) * time.Minute)
			}
			s.mu.Unlock()
			if due {
				log.Println("AutoScan triggered — scanning LinkedIn, ITviec, TopDev (TP.HCM / Remote)...")
				res := s.RunAndMergeScan(models.ScanRequest{})
				log.Printf("AutoScan finished: found=%d new=%d updated=%d", res.TotalFound, res.NewJobs, res.UpdatedJobs)
			}
		}
	}()
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Private-Network", "true")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func main() {
	portFlag := flag.String("port", "", "HTTP listen port (overrides PORT env)")
	scanOnceFlag := flag.Bool("scan-once", false, "Run one live scan, update data/jobs.json, and exit")
	flag.Parse()

	port := strings.TrimSpace(*portFlag)
	if port == "" {
		port = strings.TrimSpace(os.Getenv("PORT"))
	}
	if port == "" {
		port = "8090"
	}

	baseDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(baseDir, "web")); os.IsNotExist(err) {
		if exe, e := os.Executable(); e == nil {
			exeDir := filepath.Dir(exe)
			if _, e2 := os.Stat(filepath.Join(exeDir, "web")); e2 == nil {
				baseDir = exeDir
			}
		}
	}

	store, err := NewStore(baseDir)
	if err != nil {
		log.Fatalf("init store: %v", err)
	}

	if *scanOnceFlag {
		log.Println("Running one-shot CV job scan (-scan-once)...")
		res := store.RunAndMergeScan(models.ScanRequest{})
		log.Printf("Scan-once complete: found=%d new=%d updated=%d", res.TotalFound, res.NewJobs, res.UpdatedJobs)
		return
	}

	// Trigger a background live scan if jobs store is initially empty
	store.mu.RLock()
	initialCount := len(store.jobs)
	store.mu.RUnlock()
	if initialCount == 0 {
		go func() {
			log.Println("Initial store empty — running background CV job scan (LinkedIn, ITviec, TopDev)...")
			res := store.RunAndMergeScan(models.ScanRequest{})
			log.Printf("Initial scan complete: found=%d new=%d", res.TotalFound, res.NewJobs)
		}()
	}

	store.StartAutoScanLoop()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		store.mu.RLock()
		defer store.mu.RUnlock()
		writeJSON(w, http.StatusOK, map[string]any{
			"status":    "ok",
			"port":      port,
			"jobsCount": len(store.jobs),
			"scanning":  store.scanning,
			"lastScan":  store.lastScan,
			"autoScan":  store.autoScanSnapshotLocked(),
		})
	})

	mux.HandleFunc("/api/autoscan", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			store.mu.RLock()
			snap := store.autoScanSnapshotLocked()
			store.mu.RUnlock()
			writeJSON(w, http.StatusOK, snap)
		case http.MethodPost:
			var body struct {
				Enabled         *bool `json:"enabled"`
				IntervalMinutes *int  `json:"intervalMinutes"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
				return
			}
			store.mu.Lock()
			if body.Enabled != nil {
				store.autoScanEnabled = *body.Enabled
			}
			if body.IntervalMinutes != nil {
				iv := *body.IntervalMinutes
				if iv < 1 {
					iv = 1
				}
				if iv > 1440 {
					iv = 1440
				}
				store.autoScanIntervalMin = iv
			}
			if store.autoScanEnabled {
				store.nextScanAt = time.Now().Add(time.Duration(store.autoScanIntervalMin) * time.Minute)
			}
			snap := store.autoScanSnapshotLocked()
			store.mu.Unlock()
			writeJSON(w, http.StatusOK, snap)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/profile", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			store.mu.RLock()
			defer store.mu.RUnlock()
			writeJSON(w, http.StatusOK, store.profile)
		case http.MethodPut:
			var p models.CVProfile
			if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
				return
			}
			store.mu.Lock()
			store.profile = p
			for id, j := range store.jobs {
				matcher.EvaluateJob(&j, store.profile)
				store.jobs[id] = j
			}
			_ = store.saveProfileLocked()
			_ = store.saveJobsLocked()
			store.mu.Unlock()
			writeJSON(w, http.StatusOK, p)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()
		platformFilter := strings.ToLower(strings.TrimSpace(q.Get("platform")))
		trackFilter := strings.ToLower(strings.TrimSpace(q.Get("track")))
		statusFilter := strings.ToLower(strings.TrimSpace(q.Get("status")))
		workModeFilter := strings.ToLower(strings.TrimSpace(q.Get("workMode")))
		searchQuery := strings.ToLower(strings.TrimSpace(q.Get("q")))
		minScore, _ := strconv.Atoi(q.Get("minScore"))

		store.mu.RLock()
		all := make([]models.Job, 0, len(store.jobs))
		platformCounts := map[string]int{
			"linkedin": 0, "itviec": 0, "topdev": 0,
			"topcv": 0, "threads": 0, "facebook": 0, "zalo": 0,
		}
		statusCounts := map[string]int{}
		highMatchCount := 0
		for _, j := range store.jobs {
			platformCounts[j.Platform]++
			statusCounts[j.Status]++
			if j.MatchScore >= 75 && j.Status != "ignored" {
				highMatchCount++
			}
			all = append(all, j)
		}
		scanning := store.scanning
		lastScan := store.lastScan
		autoScan := store.autoScanSnapshotLocked()
		store.mu.RUnlock()

		var filtered []models.Job
		for _, j := range all {
			if statusFilter == "" {
				if j.Status == "ignored" {
					continue
				}
			} else if statusFilter != "all" && strings.ToLower(j.Status) != statusFilter {
				continue
			}
			if platformFilter != "" && platformFilter != "all" && strings.ToLower(j.Platform) != platformFilter {
				continue
			}
			if trackFilter != "" && trackFilter != "all" && !strings.Contains(strings.ToLower(j.RecommendedTrack), trackFilter) {
				continue
			}
			if workModeFilter != "" && workModeFilter != "all" && strings.ToLower(j.WorkMode) != workModeFilter {
				continue
			}
			if minScore > 0 && j.MatchScore < minScore {
				continue
			}
			if searchQuery != "" {
				hay := strings.ToLower(j.Title + " " + j.Company + " " + j.Location + " " + j.Description + " " + strings.Join(j.Skills, " ") + " " + strings.Join(j.MatchedSkills, " "))
				if !strings.Contains(hay, searchQuery) {
					continue
				}
			}
			filtered = append(filtered, j)
		}

		sort.Slice(filtered, func(i, j int) bool {
			if filtered[i].MatchScore != filtered[j].MatchScore {
				return filtered[i].MatchScore > filtered[j].MatchScore
			}
			return filtered[i].FetchedAt.After(filtered[j].FetchedAt)
		})

		writeJSON(w, http.StatusOK, map[string]any{
			"jobs":           filtered,
			"totalStored":    len(all),
			"highMatchCount": highMatchCount,
			"platformCounts": platformCounts,
			"statusCounts":   statusCounts,
			"scanning":       scanning,
			"lastScan":       lastScan,
			"autoScan":       autoScan,
		})
	})

	mux.HandleFunc("/api/scan", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req models.ScanRequest
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&req)
		}
		res := store.RunAndMergeScan(req)
		writeJSON(w, http.StatusOK, res)
	})

	mux.HandleFunc("/api/jobs/import", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req models.ImportJobRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Dữ liệu JSON không hợp lệ"})
			return
		}
		if strings.TrimSpace(req.RawText) == "" && strings.TrimSpace(req.Title) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Vui lòng nhập tiêu đề hoặc nội dung bài tuyển dụng"})
			return
		}

		store.mu.Lock()
		job := matcher.ParseRawJobPost(req, store.profile)
		if existing, ok := store.jobs[job.ID]; ok {
			if existing.Notes != "" {
				job.Notes = existing.Notes
			}
		}
		store.jobs[job.ID] = job
		_ = store.saveJobsLocked()
		pitch := matcher.GeneratePitch(job, store.profile)
		store.mu.Unlock()

		writeJSON(w, http.StatusOK, map[string]any{
			"job":   job,
			"pitch": pitch,
		})
	})

	mux.HandleFunc("/api/jobs/import-batch", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Platform string                    `json:"platform"`
			Jobs     []models.ImportJobRequest `json:"jobs"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Dữ liệu JSON batch không hợp lệ"})
			return
		}

		store.mu.Lock()
		imported := 0
		skippedNonHCM := 0
		for _, item := range req.Jobs {
			if strings.TrimSpace(item.RawText) == "" && strings.TrimSpace(item.Title) == "" {
				continue
			}
			if strings.TrimSpace(item.Platform) == "" {
				if strings.TrimSpace(req.Platform) != "" {
					item.Platform = req.Platform
				} else {
					item.Platform = "auto"
				}
			}
			if strings.TrimSpace(item.Status) == "" {
				item.Status = "new"
			}
			job := matcher.ParseRawJobPost(item, store.profile)
			if !matcher.IsHCMOrRemote(job) {
				skippedNonHCM++
				continue
			}
			if existing, ok := store.jobs[job.ID]; ok {
				if existing.Status != "" {
					job.Status = existing.Status
				}
				if existing.Notes != "" {
					job.Notes = existing.Notes
				}
			}
			store.jobs[job.ID] = job
			imported++
		}
		if imported > 0 {
			_ = store.saveJobsLocked()
		}
		totalStored := len(store.jobs)
		store.mu.Unlock()

		writeJSON(w, http.StatusOK, map[string]any{
			"received":      len(req.Jobs),
			"imported":      imported,
			"skippedNonHCM": skippedNonHCM,
			"totalStored":   totalStored,
		})
	})

	mux.HandleFunc("/api/jobs/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
		if path == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Handle /api/jobs/{id}/detail and /api/jobs/{id}/pitch
		if strings.HasSuffix(path, "/detail") && r.Method == http.MethodGet {
			id := strings.TrimSuffix(path, "/detail")
			store.mu.RLock()
			job, ok := store.jobs[id]
			profile := store.profile
			store.mu.RUnlock()
			if !ok {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
				return
			}

			// Fetch live JD text on demand if description is missing or short
			if len(strings.TrimSpace(job.Description)) < 160 {
				var fetchedDesc string
				var fetchErr error
				switch job.Platform {
				case "linkedin":
					if job.ExternalID != "" {
						fetchedDesc, fetchErr = scrapers.FetchLinkedInJobDetail(store.httpClient, job.ExternalID)
					}
				case "itviec":
					if job.URL != "" {
						fetchedDesc, fetchErr = scrapers.FetchITviecJobDetail(store.httpClient, job.URL)
					}
				case "topdev":
					if job.URL != "" {
						fetchedDesc, fetchErr = scrapers.FetchTopDevJobDetail(store.httpClient, job.URL)
					}
				}
				if fetchErr == nil && strings.TrimSpace(fetchedDesc) != "" {
					store.mu.Lock()
					if cur, exists := store.jobs[id]; exists {
						if cur.Description != "" && !strings.Contains(fetchedDesc, cur.Description) {
							cur.Description = cur.Description + "\n\n" + fetchedDesc
						} else {
							cur.Description = fetchedDesc
						}
						matcher.EvaluateJob(&cur, store.profile)
						store.jobs[id] = cur
						job = cur
						_ = store.saveJobsLocked()
					}
					store.mu.Unlock()
				}
			}

			pitch := matcher.GeneratePitch(job, profile)
			writeJSON(w, http.StatusOK, map[string]any{
				"job":   job,
				"pitch": pitch,
			})
			return
		}

		if strings.HasSuffix(path, "/pitch") && r.Method == http.MethodPost {
			id := strings.TrimSuffix(path, "/pitch")
			store.mu.RLock()
			job, ok := store.jobs[id]
			profile := store.profile
			store.mu.RUnlock()
			if !ok {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
				return
			}
			writeJSON(w, http.StatusOK, matcher.GeneratePitch(job, profile))
			return
		}

		id := path
		switch r.Method {
		case http.MethodPatch:
			var body struct {
				Status *string `json:"status"`
				Notes  *string `json:"notes"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
				return
			}
			store.mu.Lock()
			job, ok := store.jobs[id]
			if !ok {
				store.mu.Unlock()
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
				return
			}
			if body.Status != nil {
				job.Status = strings.TrimSpace(*body.Status)
			}
			if body.Notes != nil {
				job.Notes = *body.Notes
			}
			store.jobs[id] = job
			_ = store.saveJobsLocked()
			store.mu.Unlock()
			writeJSON(w, http.StatusOK, job)
		case http.MethodDelete:
			store.mu.Lock()
			delete(store.jobs, id)
			_ = store.saveJobsLocked()
			store.mu.Unlock()
			writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/radar", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()
		keyword := q.Get("keyword")
		location := q.Get("location")
		days, _ := strconv.Atoi(q.Get("days"))
		if days <= 0 {
			days = 7
		}
		matrix := radar.BuildRadarMatrix(keyword, location, days)
		writeJSON(w, http.StatusOK, map[string]any{
			"keyword":   keyword,
			"location":  location,
			"days":      days,
			"platforms": matrix,
		})
	})

	webDir := filepath.Join(baseDir, "web")
	fileServer := http.FileServer(http.Dir(webDir))
	mux.Handle("/", fileServer)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           withCORS(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("Leo Job Radar running at http://localhost:%s (workspace: %s)", port, baseDir)
	log.Fatal(srv.ListenAndServe())
}
