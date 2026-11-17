package radar

import (
	"fmt"
	"net/url"
	"strings"

	"job-radar-leo/internal/models"
)

// BuildRadarMatrix generates Smart Deep-Links and Google X-Ray searches across all 7 requested platforms.
func BuildRadarMatrix(keyword, location string, days int) []models.PlatformRadar {
	kw := strings.TrimSpace(keyword)
	if kw == "" {
		kw = "Golang"
	}
	loc := strings.TrimSpace(location)
	if loc == "" {
		loc = "Ho Chi Minh"
	}

	googleTbs := "qdr:w"
	liTPR := "r604800"
	timeLabel := "7 ngày qua"
	switch days {
	case 1:
		googleTbs = "qdr:d"
		liTPR = "r86400"
		timeLabel = "24 giờ qua"
	case 30:
		googleTbs = "qdr:m"
		liTPR = "r2592000"
		timeLabel = "30 ngày qua"
	}

	topcvSlug := strings.ToLower(kw)
	topcvSlug = strings.ReplaceAll(topcvSlug, ".", "")
	topcvSlug = strings.ReplaceAll(topcvSlug, "/", "-")
	topcvSlug = strings.ReplaceAll(topcvSlug, " ", "-")

	itviecSlug := topcvSlug
	if itviecSlug == "node-js" || itviecSlug == "node" {
		itviecSlug = "nodejs"
	}

	return []models.PlatformRadar{
		{
			Platform:   "threads",
			Name:       "Threads",
			AccessMode: "xray-radar",
			Badge:      "Mạng xã hội / HR săn trực tiếp",
			Tip:        "Threads là nơi rất nhiều Tech Lead, CTO và HR đăng job Golang/Node/AI ẩn (chưa kịp lên sàn). Bấm link mở tab mới và dùng Bookmarklet nếu muốn lưu job.",
			Links: []models.RadarLink{
				{
					Label:       fmt.Sprintf("Threads Search: tuyển %s (Mới nhất)", kw),
					Description: "Tìm trực tiếp bài đăng mới nhất trên Threads với từ khóa tuyển dụng",
					URL:         "https://www.threads.net/search?q=" + url.QueryEscape("tuyển "+kw) + "&serp_type=default&filter=recent",
					Kind:        "direct",
				},
				{
					Label:       fmt.Sprintf("Threads Search: hiring %s (English)", kw),
					Description: "Tìm bài hiring bằng tiếng Anh từ các startup / công ty product",
					URL:         "https://www.threads.net/search?q=" + url.QueryEscape("hiring "+kw) + "&serp_type=default&filter=recent",
					Kind:        "direct",
				},
				{
					Label:       "Threads Tag: #tuyendungit (Recent)",
					Description: "Theo dõi hashtag tuyển dụng IT phổ biến nhất trên Threads",
					URL:         "https://www.threads.net/search?q=" + url.QueryEscape("#tuyendungit") + "&serp_type=default&filter=recent",
					Kind:        "direct",
				},
				{
					Label:       fmt.Sprintf("Google X-Ray Threads (%s)", timeLabel),
					Description: "Quét Google các bài Threads công khai có chữ tuyển / hiring không cần đăng nhập",
					URL:         fmt.Sprintf("https://www.google.com/search?q=%s&tbs=%s", url.QueryEscape(fmt.Sprintf(`(site:threads.net OR site:threads.com) ("tuyển" OR "hiring" OR "tuyển dụng") %s`, kw)), googleTbs),
					Kind:        "xray",
				},
			},
		},
		{
			Platform:   "facebook",
			Name:       "Facebook",
			AccessMode: "login-required",
			Badge:      "Hội nhóm Dev & HR Freelance",
			Tip:        "Mở trực tiếp tab Bài viết (Posts) trên Facebook hoặc quét Google X-Ray nhóm công khai. Gặp bài ngon, copy text hoặc dùng Bookmarklet ném vào Tab 'Phân tích JD'.",
			Links: []models.RadarLink{
				{
					Label:       fmt.Sprintf("Facebook Posts: tuyển dụng %s %s", kw, loc),
					Description: "Tìm bài đăng tuyển dụng mới trên feed & các group công nghệ",
					URL:         "https://www.facebook.com/search/posts/?q=" + url.QueryEscape(fmt.Sprintf(`"tuyển" %s %s`, kw, loc)),
					Kind:        "post",
				},
				{
					Label:       fmt.Sprintf("Facebook Posts: tuyển %s Remote", kw),
					Description: "Săn các job làm việc từ xa (Remote) được đăng trong group dev Việt",
					URL:         "https://www.facebook.com/search/posts/?q=" + url.QueryEscape(fmt.Sprintf(`"tuyển" %s Remote`, kw)),
					Kind:        "post",
				},
				{
					Label:       "Facebook Groups: Tuyển dụng IT / Golang / NodeJS",
					Description: "Khám phá các group tuyển dụng IT chuyên ngành đang hoạt động",
					URL:         "https://www.facebook.com/search/groups/?q=" + url.QueryEscape("tuyển dụng IT "+kw),
					Kind:        "group",
				},
				{
					Label:       fmt.Sprintf("Google X-Ray Facebook (%s)", timeLabel),
					Description: "Tìm bài đăng Facebook công khai từ các fanpage / HR tech",
					URL:         fmt.Sprintf("https://www.google.com/search?q=%s&tbs=%s", url.QueryEscape(fmt.Sprintf(`site:facebook.com ("tuyển dụng" OR "we are hiring") %s (%s OR Remote)`, kw, loc)), googleTbs),
					Kind:        "xray",
				},
			},
		},
		{
			Platform:   "zalo",
			Name:       "Zalo",
			AccessMode: "xray-radar",
			Badge:      "Nhóm kín & Liên hệ HR trực tiếp",
			Tip:        "Zalo không có trang tìm việc công khai; cách hiệu quả nhất là quét link mời nhóm Zalo IT (`zalo.me/g/`) và tìm các bài HR để lại SĐT Zalo tuyển gấp.",
			Links: []models.RadarLink{
				{
					Label:       fmt.Sprintf("Săn Nhóm Zalo Tuyển Dụng IT (%s)", kw),
					Description: "Tìm link tham gia các nhóm Zalo tuyển dụng Dev / Tech (zalo.me/g/)",
					URL:         "https://www.google.com/search?q=" + url.QueryEscape(fmt.Sprintf(`"zalo.me/g/" ("tuyển dụng" OR "job" OR "IT" OR "dev") %s`, kw)),
					Kind:        "group",
				},
				{
					Label:       fmt.Sprintf("Săn HR để lại Zalo tuyển %s (%s)", kw, timeLabel),
					Description: "Tìm các bài tuyển dụng mới có kèm SĐT Zalo hoặc link zalo.me để nhắn trực tiếp",
					URL:         fmt.Sprintf("https://www.google.com/search?q=%s&tbs=%s", url.QueryEscape(fmt.Sprintf(`("zalo" OR "zalo.me") ("tuyển" OR "hiring") %s ("%s" OR "HCM" OR "Remote")`, kw, loc)), googleTbs),
					Kind:        "xray",
				},
				{
					Label:       "Nhóm Zalo Cộng đồng Backend / Golang / NodeJS VN",
					Description: "Tìm các group chia sẻ kèo job, freelance và tuyển dụng nội bộ",
					URL:         "https://www.google.com/search?q=" + url.QueryEscape(`"zalo.me/g/" (Golang OR NodeJS OR Backend OR "AI Engineer")`),
					Kind:        "group",
				},
			},
		},
		{
			Platform:   "topcv",
			Name:       "TopCV",
			AccessMode: "xray-radar",
			Badge:      "Cloudflare Protected + X-Ray",
			Tip:        "TopCV bật Cloudflare chống bot tự động (HTTP 403). Dùng các link dưới đây để mở thẳng trang tìm kiếm TopCV hoặc tra Google X-Ray theo giờ.",
			Links: []models.RadarLink{
				{
					Label:       fmt.Sprintf("Mở TopCV: Việc làm %s", kw),
					Description: "Mở trực tiếp trang tìm kiếm theo từ khóa trên TopCV.vn",
					URL:         "https://www.topcv.vn/tim-viec-lam-" + url.PathEscape(topcvSlug),
					Kind:        "direct",
				},
				{
					Label:       fmt.Sprintf("TopCV TP.HCM: %s", kw),
					Description: "Lọc sẵn theo từ khóa & khu vực TP. Hồ Chí Minh trên TopCV",
					URL:         "https://www.topcv.vn/tim-viec-lam-" + url.PathEscape(topcvSlug) + "-tai-ho-chi-minh-kl2",
					Kind:        "direct",
				},
				{
					Label:       fmt.Sprintf("Google X-Ray TopCV (%s)", timeLabel),
					Description: "Lọc nhanh các trang chi tiết việc làm TopCV mới cập nhật",
					URL:         fmt.Sprintf("https://www.google.com/search?q=%s&tbs=%s", url.QueryEscape(fmt.Sprintf(`site:topcv.vn/viec-lam %s ("%s" OR "Hồ Chí Minh" OR "Remote")`, kw, loc)), googleTbs),
					Kind:        "xray",
				},
			},
		},
		{
			Platform:   "linkedin",
			Name:       "LinkedIn",
			AccessMode: "auto-scrape",
			Badge:      "Quét tự động + Săn bài viết HR",
			Tip:        "Tab 'Bảng Tin Job' đã tự động quét LinkedIn Jobs API. Dùng thêm link dưới để săn bài viết HR đăng thẳng trên Feed (rất dễ gửi tin nhắn trực tiếp).",
			Links: []models.RadarLink{
				{
					Label:       fmt.Sprintf("LinkedIn Hiring Posts: %s (Mới nhất)", kw),
					Description: "Xem bài đăng của Recruiter / Tech Lead trên Feed có từ khóa hiring",
					URL:         fmt.Sprintf("https://www.linkedin.com/search/results/content/?keywords=%s&origin=SWITCH_SEARCH_VERTICAL&sortBy=%%22date_posted%%22", url.QueryEscape(kw+" hiring Vietnam")),
					Kind:        "post",
				},
				{
					Label:       fmt.Sprintf("LinkedIn Jobs (%s): %s", timeLabel, kw),
					Description: "Mở LinkedIn Jobs có lọc sẵn theo khoảng thời gian đã chọn",
					URL:         fmt.Sprintf("https://www.linkedin.com/jobs/search/?keywords=%s&location=Vietnam&f_TPR=%s", url.QueryEscape(kw), liTPR),
					Kind:        "direct",
				},
				{
					Label:       fmt.Sprintf("LinkedIn Remote Worldwide: %s", kw),
					Description: "Tìm các vị trí Remote toàn cầu / APAC trả lương USD",
					URL:         fmt.Sprintf("https://www.linkedin.com/jobs/search/?keywords=%s&f_WT=2&f_TPR=%s", url.QueryEscape(kw), liTPR),
					Kind:        "direct",
				},
			},
		},
		{
			Platform:   "itviec",
			Name:       "ITviec",
			AccessMode: "auto-scrape",
			Badge:      "Quét tự động (Live)",
			Tip:        "ITviec đã được quét tự động trong Tab 'Bảng Tin Job'. Mở link bên dưới nếu bạn muốn đăng nhập để xem mức lương ẩn trên ITviec.",
			Links: []models.RadarLink{
				{
					Label:       fmt.Sprintf("ITviec: Việc làm %s tại Việt Nam", kw),
					Description: "Mở trang danh sách ITviec theo kỹ năng",
					URL:         "https://itviec.com/it-jobs/" + url.PathEscape(itviecSlug),
					Kind:        "direct",
				},
				{
					Label:       fmt.Sprintf("ITviec TP.HCM: %s", kw),
					Description: "Lọc riêng việc làm tại Hồ Chí Minh trên ITviec",
					URL:         "https://itviec.com/it-jobs/" + url.PathEscape(itviecSlug) + "/ho-chi-minh-hcm",
					Kind:        "direct",
				},
			},
		},
		{
			Platform:   "topdev",
			Name:       "Saramin (TopDev)",
			AccessMode: "auto-scrape",
			Badge:      "Quét tự động (Live + Full JD)",
			Tip:        "TopDev đã chuyển sang Saramin Vietnam (saramin.vn). Hệ thống tự động quét khu vực TP.HCM (region_ids=79) kèm đầy đủ Mô tả & Yêu cầu.",
			Links: []models.RadarLink{
				{
					Label:       fmt.Sprintf("Saramin TP.HCM: %s", kw),
					Description: "Tìm trực tiếp tại TP.HCM trên Saramin.vn",
					URL:         "https://saramin.vn/jobs/search?keyword=" + url.QueryEscape(kw) + "&region_ids=79",
					Kind:        "direct",
				},
				{
					Label:       "Saramin AI & Software Engineering TP.HCM",
					Description: "Xem các vị trí AI / Software Engineer tại TP.HCM trên Saramin",
					URL:         "https://saramin.vn/jobs/search?keyword=AI&region_ids=79",
					Kind:        "direct",
				},
				{
					Label:       "Trang chủ Việc làm IT Saramin",
					Description: "Khám phá danh mục việc làm công nghệ mới nhất trên saramin.vn",
					URL:         "https://saramin.vn/viec-lam-it",
					Kind:        "direct",
				},
			},
		},
	}
}
