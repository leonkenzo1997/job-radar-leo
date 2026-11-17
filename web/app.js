const state = {
  jobs: [],
  profile: null,
  platformCounts: {},
  statusCounts: {},
  activePlatform: "all",
  activeTab: "feed",
  searchDebounce: null,
  autoScan: { enabled: true, intervalMinutes: 15, nextScanAt: null },
  cloudMode: false,
  apiBase: "",
  cloudProfile: null,
  cloudJobs: null,
};

const CLOUD_KEYS = {
  overrides: "leo_radar_overrides_v1",
  customJobs: "leo_radar_custom_jobs_v1",
  profile: "leo_radar_profile_v1",
};

const CANONICAL_ALIASES = {
  go: "Golang",
  golang: "Golang",
  "node.js": "Node.js",
  nodejs: "Node.js",
  "express.js": "Express.js",
  express: "Express.js",
  postgres: "PostgreSQL",
  postgresql: "PostgreSQL",
  k8s: "Kubernetes",
  kubernetes: "Kubernetes",
  reactjs: "ReactJS",
  react: "ReactJS",
  "react.js": "ReactJS",
  "flutter web": "Flutter",
  flutter: "Flutter",
  "apache kafka": "Kafka",
  kafka: "Kafka",
  "rest api": "RESTful API",
  restful: "RESTful API",
  "model context protocol": "MCP",
  mcp: "MCP",
  "ai-native": "AI-Native",
  "gitlab ci": "CI/CD",
  "github actions": "CI/CD",
  "ci/cd": "CI/CD",
  "azure event hubs": "Event Hubs",
  "event hubs": "Event Hubs",
};

const EXTERNAL_SKILL_CHECKS = [
  "Java", "Spring Boot", "Python", "Django", "FastAPI", "C#", ".NET", ".NET Core",
  "PHP", "Laravel", "Ruby", "Ruby on Rails", "Rust", "C++", "Scala", "Kotlin",
  "Swift", "iOS", "Android Native", "Angular", "VueJS", "Vue.js", "GraphQL",
  "Terraform", "Ansible", "GCP", "Snowflake", "Spark", "SAP", "Salesforce",
];

function isLocalhostHost() {
  return ["localhost", "127.0.0.1"].includes(window.location.hostname);
}

async function resolveBackendBase() {
  if (isLocalhostHost()) {
    state.apiBase = "";
    state.cloudMode = false;
    return;
  }
  state.apiBase = "";
  state.cloudMode = true;
}

function canonicalSkill(s) {
  const low = String(s || "").trim().toLowerCase();
  return CANONICAL_ALIASES[low] || s;
}

function escapeRegex(s) {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function containsWordBoundary(textLower, word) {
  const re = new RegExp(`(?:^|[^a-z0-9_])${escapeRegex(word)}(?:$|[^a-z0-9_])`, "i");
  return re.test(textLower);
}

function containsSkillClient(textLower, skill, tagsLower) {
  const sLow = String(skill || "").trim().toLowerCase();
  if (!sLow) return false;
  for (const tag of tagsLower) {
    if (tag === sLow || canonicalSkill(tag) === canonicalSkill(skill)) return true;
  }
  if (sLow === "go") {
    const cleaned = textLower
      .replaceAll("go to", " ")
      .replaceAll("go-to", " ")
      .replaceAll("go live", " ")
      .replaceAll("on the go", " ")
      .replaceAll("go-live", " ")
      .replaceAll("go ahead", " ");
    return (
      containsWordBoundary(cleaned, "go") &&
      (cleaned.includes("golang") ||
        cleaned.includes("go/") ||
        cleaned.includes("/go") ||
        cleaned.includes("go,") ||
        cleaned.includes(", go") ||
        cleaned.includes("go developer") ||
        cleaned.includes("go engineer") ||
        cleaned.includes("go backend") ||
        cleaned.includes("(go)") ||
        cleaned.includes("(go,") ||
        cleaned.includes(", go)"))
    );
  }
  if (sLow === ".net" || sLow === "c#" || sLow === "c++") return textLower.includes(sLow);
  if (sLow.length <= 4) return containsWordBoundary(textLower, sLow);
  return textLower.includes(sLow);
}

function evaluateJobClient(job, profile) {
  const titleLower = (job.title || "").toLowerCase();
  const fullText = `${job.title || ""} ${job.description || ""} ${(job.skills || []).join(" ")}`.trim();
  const textLower = fullText.toLowerCase();
  const tagsLower = (job.skills || []).map((s) => String(s).trim().toLowerCase());

  const seen = new Set();
  const matched = [];
  let t1 = 0, t2 = 0, t3 = 0, t4 = 0;

  const addMatch = (raw, cap, per, bucketSetter, cur) => {
    const canon = canonicalSkill(raw);
    if (seen.has(canon)) return cur;
    seen.add(canon);
    matched.push(canon);
    return Math.min(cap, cur + per);
  };

  for (const s of profile.tier1Skills || []) {
    if (containsSkillClient(textLower, s, tagsLower)) t1 = addMatch(s, 42, 12, null, t1);
  }
  for (const s of profile.tier2Skills || []) {
    if (containsSkillClient(textLower, s, tagsLower)) t2 = addMatch(s, 25, 10, null, t2);
  }
  for (const s of profile.tier3Skills || []) {
    if (containsSkillClient(textLower, s, tagsLower)) t3 = addMatch(s, 20, 4, null, t3);
  }
  for (const s of profile.tier4Skills || []) {
    if (containsSkillClient(textLower, s, tagsLower)) t4 = addMatch(s, 10, 5, null, t4);
  }

  let score = 0;
  const reasons = [];
  const roleSignals = ["backend", "back-end", "fullstack", "full-stack", "full stack", "golang", "node", "nestjs", "software engineer", "ai engineer", "agentic", "platform engineer", "distributed", "microservices"];
  if (roleSignals.some((sig) => titleLower.includes(sig))) {
    score += 25;
    reasons.push("Đúng vị trí mục tiêu (Backend / Fullstack / AI-Native)");
  } else if (titleLower.includes("developer") || titleLower.includes("engineer") || titleLower.includes("lập trình")) {
    score += 12;
  }

  const seniorSignals = ["senior", "sr.", "lead", "staff", "middle", "mid-senior", "principal", "architect", "chuyên viên cao cấp"];
  const juniorSignals = ["intern", "fresher", "thực tập", "junior only"];
  const hasSenior = seniorSignals.some((sig) => titleLower.includes(sig)) || ["3+ years", "4+ years", "5+ years", "3 năm", "5 năm"].some((y) => textLower.includes(y));
  const hasJunior = juniorSignals.some((sig) => titleLower.includes(sig));
  if (hasJunior && !hasSenior) {
    score -= 25;
    reasons.push("Cấp bậc Intern/Fresher thấp hơn kinh nghiệm 5+ năm");
  } else if (hasSenior) {
    score += 10;
    reasons.push("Khớp cấp bậc Middle/Senior (5+ năm kinh nghiệm)");
  }

  score += t1 + t2 + t3 + t4;
  if (t1 >= 12) reasons.push("Trùng khớp Core Backend (Golang / Node.js / NestJS / TypeScript)");
  if (t2 >= 10) reasons.push("Lợi thế mạnh về AI/LLM/RAG/Agentic");

  const locLower = `${job.location || ""} ${job.workMode || ""} ${textLower}`.toLowerCase();
  if (["ho chi minh", "hồ chí minh", "hcm", "remote", "hybrid"].some((k) => locLower.includes(k))) {
    score += 8;
    reasons.push("Phù hợp địa điểm làm việc (TP.HCM / Remote / Hybrid)");
  }

  const hasCore = ["Golang", "Node.js", "NestJS", "TypeScript", "AI", "LLM", "RAG", "ReactJS", "Flutter"].some((k) => seen.has(k));
  if (!hasCore) {
    const offMarkers = [".net", "c#", "laravel", "wordpress", "cobol", "swift", "unity", "ruby on rails", "sap ", "abap", "magento", "drupal"];
    if (offMarkers.some((off) => titleLower.includes(off) || containsSkillClient(textLower, off, tagsLower))) {
      score -= 30;
      reasons.push("Công nghệ chính không phải Go/Node.js/AI trong CV");
    }
  }

  const missing = [];
  for (const ext of EXTERNAL_SKILL_CHECKS) {
    if (containsSkillClient(textLower, ext, tagsLower)) {
      const canon = canonicalSkill(ext);
      if (!seen.has(canon)) missing.push(canon);
    }
  }

  score = Math.max(5, Math.min(99, score));
  job.matchScore = score;
  job.matchLevel = score >= 75 ? "high" : score >= 50 ? "medium" : "low";
  job.matchedSkills = matched.sort();
  job.missingSkills = missing;
  job.matchReasons = reasons;

  const hasGo = seen.has("Golang") || /(?:^|[^a-z0-9_])(golang|go)(?:$|[^a-z0-9_])/i.test(titleLower);
  const hasNode = seen.has("Node.js") || seen.has("NestJS") || seen.has("Express.js") || seen.has("TypeScript");
  const hasAI = t2 >= 10 || titleLower.includes("ai ") || titleLower.includes("llm") || titleLower.includes("agent") || titleLower.includes("rag");
  const hasFE = seen.has("ReactJS") || seen.has("Flutter") || titleLower.includes("fullstack") || titleLower.includes("full-stack") || titleLower.includes("full stack");

  if (hasAI && (hasGo || hasNode || titleLower.includes("ai"))) job.recommendedTrack = "AI-Native (LLM / RAG / Agents)";
  else if (hasFE && (hasGo || hasNode)) job.recommendedTrack = "Fullstack (Go/Node + React/Flutter)";
  else if (hasGo) job.recommendedTrack = "Backend Golang & Microservices";
  else if (hasNode) job.recommendedTrack = "Node.js / NestJS Backend";
  else if (t3 >= 8) job.recommendedTrack = "Cloud / DevOps & Distributed Systems";
  else job.recommendedTrack = "Software Engineering (Tham khảo)";

  const projRanked = (profile.projects || []).map((p) => ({
    name: p.name,
    score: (p.keywords || []).filter((kw) => textLower.includes(kw.toLowerCase())).length,
  }));
  projRanked.sort((a, b) => b.score - a.score);
  job.recommendedProjects = projRanked.slice(0, 2).filter((p, idx) => p.score > 0 || idx === 0).map((p) => p.name);

  if (!job.workMode || job.workMode === "Unknown") {
    if (locLower.includes("remote")) job.workMode = "Remote";
    else if (locLower.includes("hybrid")) job.workMode = "Hybrid";
    else if (locLower.includes("on-site") || locLower.includes("onsite") || locLower.includes("at office") || locLower.includes("trực tiếp")) job.workMode = "On-site";
    else job.workMode = "Unknown";
  }
  return job;
}

function isHCMOrRemoteClient(job) {
  const hay = `${job.location || ""} ${job.workMode || ""} ${job.title || ""}`.toLowerCase();
  const markers = [
    "ho chi minh", "hồ chí minh", "hcm", "tphcm", "tp.hcm",
    "sài gòn", "saigon", "thủ đức", "thu duc",
    "tân bình", "tan binh", "bình thạnh", "binh thanh", "phú nhuận", "phu nhuan",
    "quận 1", "quận 2", "quận 3", "quận 4", "quận 5", "quận 7", "quận 10",
    "district 1", "district 3", "district 7",
    "remote", "work from home", "wfh", "từ xa",
  ];
  return markers.some((m) => hay.includes(m));
}

function generatePitchClient(job, profile) {
  const skillsStr = (job.matchedSkills && job.matchedSkills.length)
    ? job.matchedSkills.slice(0, 6).join(", ")
    : "Golang, Node.js (NestJS), TypeScript, Microservices, PostgreSQL/Kafka và AI-Native (LLM/RAG)";
  let projSummary = "";
  if (job.recommendedProjects && job.recommendedProjects.length) {
    const found = (profile.projects || []).find((p) => p.name === job.recommendedProjects[0]);
    if (found) projSummary = found.summary;
  }
  if (!projSummary && profile.projects && profile.projects.length) projSummary = profile.projects[0].summary;

  return {
    vietnameseShort: `Chào anh/chị HR bên ${job.company},\n\nMình là Trương Tăng Quang (Leo), Senior Backend & Fullstack Engineer (5+ năm kinh nghiệm). Mình vừa đọc tin tuyển dụng vị trí ${job.title} và thấy rất khớp với kinh nghiệm thực chiến của mình (${skillsStr}).\n\nĐiểm nổi bật gần đây: ${projSummary}\n\nAnh/chị xem thêm hồ sơ & 13 dự án production của mình tại:\n- Portfolio & Web CV: ${profile.portfolio}\n- LinkedIn: ${profile.linkedin}\n- SĐT / Zalo: ${profile.phone} | Email: ${profile.email}\n\nRất mong được trao đổi thêm với team!`,
    englishPro: `Hi ${job.company} Hiring Team,\n\nI am Truong Tang Quang (Leo), a Senior Backend & Fullstack Software Engineer with 5+ years of production experience building distributed systems in Golang, Node.js (NestJS/TypeScript), React/Flutter, and AI-Native LLM/RAG pipelines.\n\nI am very interested in the ${job.title} role, especially given my direct hands-on alignment with ${skillsStr}. Most recently, I architected production microservices, multi-LLM/RAG orchestration, Kafka event-driven pipelines, and Kubernetes/ArgoCD GitOps deployments.\n\nYou can review my full architecture archive (13 production projects) and CV here:\n- Portfolio: ${profile.portfolio}\n- GitHub: ${profile.github}\n- LinkedIn: ${profile.linkedin}\n- Contact: ${profile.email} | ${profile.phone}\n\nBest regards,\nTruong Tang Quang (Leo)`,
  };
}

function parseRawJobPostClient(req, profile) {
  const raw = (req.rawText || "").trim();
  const urlLower = (req.url || "").toLowerCase();
  const rawLower = raw.toLowerCase();

  let platform = (req.platform || "").trim().toLowerCase();
  if (!platform || platform === "auto") {
    if (urlLower.includes("threads.net") || urlLower.includes("threads.com")) platform = "threads";
    else if (urlLower.includes("facebook.com") || urlLower.includes("fb.com")) platform = "facebook";
    else if (urlLower.includes("zalo.me") || rawLower.includes("zalo")) platform = "zalo";
    else if (urlLower.includes("topcv.vn")) platform = "topcv";
    else if (urlLower.includes("linkedin.com")) platform = "linkedin";
    else if (urlLower.includes("itviec.com")) platform = "itviec";
    else if (urlLower.includes("topdev.vn") || urlLower.includes("saramin.vn")) platform = "topdev";
    else if (rawLower.includes("threads")) platform = "threads";
    else if (rawLower.includes("facebook") || rawLower.includes("group")) platform = "facebook";
    else if (rawLower.includes("topcv")) platform = "topcv";
    else if (rawLower.includes("saramin")) platform = "topdev";
    else platform = "zalo";
  }

  let title = (req.title || "").trim();
  if (!title && raw) {
    const firstLine = raw.split("\n").map((l) => l.trim()).find((l) => l && !l.toLowerCase().startsWith("http"));
    title = firstLine ? (firstLine.length > 90 ? firstLine.slice(0, 87) + "..." : firstLine) : "";
  }
  if (!title) title = `IT Job Opportunity (${platform.toUpperCase()})`;

  let company = (req.company || "").trim();
  if (!company) {
    for (const line of raw.split("\n")) {
      const low = line.trim().toLowerCase();
      for (const prefix of ["công ty:", "company:", "tại công ty", "team "]) {
        if (low.startsWith(prefix)) {
          const val = line.trim().slice(prefix.length).trim();
          if (val.length > 2 && val.length < 80) {
            company = val;
            break;
          }
        }
      }
      if (company) break;
    }
  }
  if (!company) company = `HR / Nguồn ${platform.toUpperCase()}`;

  let salary = (req.salary || "").trim();
  if (!salary) {
    const salRe = /(?:up[ \t]*to|tới|đến|từ)?[ \t]*(?:\$[ \t]*\d[\d.,]*(?:[ \t]*(?:[-–~]|[ \t]+to[ \t]+)[ \t]*\$?[ \t]*\d[\d.,]*)?|\d[\d.,]*(?:[ \t]*(?:[-–~]|[ \t]+to[ \t]+)[ \t]*\d[\d.,]*)?[ \t]*(?:USD|\$|triệu|tr(?:$|[\s,.;)])|VNĐ|VND|M(?:$|[\s,.;)])))/i;
    const lines = raw.split("\n");
    for (const l of lines) {
      if (/(lương|salary|thu nhập|offer|package|upto|up to)/i.test(l)) {
        const m = l.match(salRe);
        if (m) { salary = m[0].trim(); break; }
      }
    }
    if (!salary) {
      for (const l of lines) {
        const m = l.match(salRe);
        if (m) { salary = m[0].trim(); break; }
      }
    }
  }

  let contact = (req.contactInfo || "").trim();
  if (!contact) {
    const parts = [];
    const zm = raw.match(/https?:\/\/zalo\.me\/[a-zA-Z0-9_./\-]+/i);
    if (zm) parts.push(zm[0]);
    const pm = raw.match(/(?:\+84|0)[35789][0-9.\-\s]{7,11}[0-9]/);
    if (pm) parts.push(`SĐT/Zalo: ${pm[0].trim()}`);
    const em = raw.match(/[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}/i);
    if (em) parts.push(`Email: ${em[0]}`);
    contact = parts.join(" | ");
  }

  let location = (req.location || "").trim();
  if (!location) {
    if (["hồ chí minh", "ho chi minh", "hcm", "sài gòn"].some((k) => rawLower.includes(k))) location = "Ho Chi Minh City";
    else if (["hà nội", "ha noi"].some((k) => rawLower.includes(k))) location = "Ha Noi";
    else if (["đà nẵng", "da nang"].some((k) => rawLower.includes(k))) location = "Da Nang";
    else if (rawLower.includes("remote")) location = "Remote";
    else location = "Vietnam";
  }

  const seed = req.url || `${platform}|${title}|${raw}`;
  let hash = 0;
  for (let i = 0; i < seed.length; i++) hash = ((hash << 5) - hash + seed.charCodeAt(i)) | 0;
  const id = `${platform}-${Math.abs(hash).toString(16)}`;

  const job = {
    id,
    platform,
    title,
    company,
    location,
    workMode: req.workMode || "Unknown",
    salary,
    postedAt: "Vừa nhập",
    url: (req.url || "").trim(),
    description: raw,
    contactInfo: contact,
    status: req.status || "saved",
    fetchedAt: new Date().toISOString(),
  };
  return evaluateJobClient(job, profile);
}

function buildRadarMatrixClient(keyword, location, days) {
  const kw = (keyword || "Golang").trim();
  const loc = (location || "Ho Chi Minh").trim();
  const dNum = Number(days) || 7;
  const googleTbs = dNum === 1 ? "qdr:d" : dNum === 30 ? "qdr:m" : "qdr:w";
  const liTPR = dNum === 1 ? "r86400" : dNum === 30 ? "r2592000" : "r604800";
  const timeLabel = dNum === 1 ? "24 giờ qua" : dNum === 30 ? "30 ngày qua" : "7 ngày qua";
  const slug = kw.toLowerCase().replaceAll(".", "").replaceAll("/", "-").replaceAll(" ", "-");
  const itviecSlug = slug === "node-js" || slug === "node" ? "nodejs" : slug;
  const enc = encodeURIComponent;

  return [
    {
      platform: "threads",
      name: "Threads",
      badge: "Mạng xã hội / HR săn trực tiếp",
      tip: "Threads là nơi rất nhiều Tech Lead, CTO và HR đăng job Golang/Node/AI ẩn. Bấm link mở tab mới và dùng Bookmarklet nếu muốn lưu job.",
      links: [
        { label: `Threads Search: tuyển ${kw} (Mới nhất)`, description: "Tìm trực tiếp bài đăng mới nhất trên Threads với từ khóa tuyển dụng", url: `https://www.threads.net/search?q=${enc("tuyển " + kw)}&serp_type=default&filter=recent` },
        { label: `Threads Search: hiring ${kw} (English)`, description: "Tìm bài hiring bằng tiếng Anh từ các startup / công ty product", url: `https://www.threads.net/search?q=${enc("hiring " + kw)}&serp_type=default&filter=recent` },
        { label: "Threads Tag: #tuyendungit (Recent)", description: "Theo dõi hashtag tuyển dụng IT phổ biến nhất trên Threads", url: `https://www.threads.net/search?q=${enc("#tuyendungit")}&serp_type=default&filter=recent` },
        { label: `Google X-Ray Threads (${timeLabel})`, description: "Quét Google các bài Threads công khai có chữ tuyển / hiring", url: `https://www.google.com/search?q=${enc(`(site:threads.net OR site:threads.com) ("tuyển" OR "hiring" OR "tuyển dụng") ${kw}`)}&tbs=${googleTbs}` },
      ],
    },
    {
      platform: "facebook",
      name: "Facebook",
      badge: "Hội nhóm Dev & HR Freelance",
      tip: "Mở trực tiếp tab Bài viết (Posts) trên Facebook hoặc quét Google X-Ray nhóm công khai. Gặp bài ngon, copy text hoặc dùng Bookmarklet.",
      links: [
        { label: `Facebook Posts: tuyển dụng ${kw} ${loc}`, description: "Tìm bài đăng tuyển dụng mới trên feed & các group công nghệ", url: `https://www.facebook.com/search/posts/?q=${enc(`"tuyển" ${kw} ${loc}`)}` },
        { label: `Facebook Posts: tuyển ${kw} Remote`, description: "Săn các job làm việc từ xa (Remote) được đăng trong group dev Việt", url: `https://www.facebook.com/search/posts/?q=${enc(`"tuyển" ${kw} Remote`)}` },
        { label: "Facebook Groups: Tuyển dụng IT / Golang / NodeJS", description: "Khám phá các group tuyển dụng IT chuyên ngành đang hoạt động", url: `https://www.facebook.com/search/groups/?q=${enc("tuyển dụng IT " + kw)}` },
        { label: `Google X-Ray Facebook (${timeLabel})`, description: "Tìm bài đăng Facebook công khai từ các fanpage / HR tech", url: `https://www.google.com/search?q=${enc(`site:facebook.com ("tuyển dụng" OR "we are hiring") ${kw} (${loc} OR Remote)`)}&tbs=${googleTbs}` },
      ],
    },
    {
      platform: "zalo",
      name: "Zalo",
      badge: "Nhóm kín & Liên hệ HR trực tiếp",
      tip: "Quét link mời nhóm Zalo IT (zalo.me/g/) và tìm các bài HR để lại SĐT Zalo tuyển gấp.",
      links: [
        { label: `Săn Nhóm Zalo Tuyển Dụng IT (${kw})`, description: "Tìm link tham gia các nhóm Zalo tuyển dụng Dev / Tech (zalo.me/g/)", url: `https://www.google.com/search?q=${enc(`"zalo.me/g/" ("tuyển dụng" OR "job" OR "IT" OR "dev") ${kw}`)}` },
        { label: `Săn HR để lại Zalo tuyển ${kw} (${timeLabel})`, description: "Tìm các bài tuyển dụng mới có kèm SĐT Zalo hoặc link zalo.me", url: `https://www.google.com/search?q=${enc(`("zalo" OR "zalo.me") ("tuyển" OR "hiring") ${kw} ("${loc}" OR "HCM" OR "Remote")`)}&tbs=${googleTbs}` },
        { label: "Nhóm Zalo Cộng đồng Backend / Golang / NodeJS VN", description: "Tìm các group chia sẻ kèo job, freelance và tuyển dụng nội bộ", url: `https://www.google.com/search?q=${enc(`"zalo.me/g/" (Golang OR NodeJS OR Backend OR "AI Engineer")`)}` },
      ],
    },
    {
      platform: "topcv",
      name: "TopCV",
      badge: "Cloudflare Protected + X-Ray",
      tip: "Mở thẳng trang tìm kiếm TopCV TP.HCM (-kl2) khi bạn đã đăng nhập trên Chrome, kết hợp Bookmarklet hoặc Extension.",
      links: [
        { label: `Mở TopCV: Việc làm ${kw}`, description: "Mở trực tiếp trang tìm kiếm theo từ khóa trên TopCV.vn", url: `https://www.topcv.vn/tim-viec-lam-${encodeURIComponent(slug)}` },
        { label: `TopCV TP.HCM: ${kw}`, description: "Lọc sẵn theo từ khóa & khu vực TP. Hồ Chí Minh trên TopCV", url: `https://www.topcv.vn/tim-viec-lam-${encodeURIComponent(slug)}-tai-ho-chi-minh-kl2` },
        { label: `Google X-Ray TopCV (${timeLabel})`, description: "Lọc nhanh các trang chi tiết việc làm TopCV mới cập nhật", url: `https://www.google.com/search?q=${enc(`site:topcv.vn/viec-lam ${kw} ("${loc}" OR "Hồ Chí Minh" OR "Remote")`)}&tbs=${googleTbs}` },
      ],
    },
    {
      platform: "linkedin",
      name: "LinkedIn",
      badge: "Quét tự động + Săn bài viết HR",
      tip: "Dùng thêm link dưới để săn bài viết HR đăng thẳng trên Feed (rất dễ gửi tin nhắn trực tiếp).",
      links: [
        { label: `LinkedIn Hiring Posts: ${kw} (Mới nhất)`, description: "Xem bài đăng của Recruiter / Tech Lead trên Feed", url: `https://www.linkedin.com/search/results/content/?keywords=${enc(kw + " hiring Vietnam")}&origin=SWITCH_SEARCH_VERTICAL&sortBy=%22date_posted%22` },
        { label: `LinkedIn Jobs (${timeLabel}): ${kw}`, description: "Mở LinkedIn Jobs có lọc sẵn theo khoảng thời gian đã chọn", url: `https://www.linkedin.com/jobs/search/?keywords=${enc(kw)}&location=Vietnam&f_TPR=${liTPR}` },
        { label: `LinkedIn Remote Worldwide: ${kw}`, description: "Tìm các vị trí Remote toàn cầu / APAC trả lương USD", url: `https://www.linkedin.com/jobs/search/?keywords=${enc(kw)}&f_WT=2&f_TPR=${liTPR}` },
      ],
    },
    {
      platform: "itviec",
      name: "ITviec",
      badge: "Quét tự động (Live)",
      tip: "ITviec đã được quét tự động ở TP.HCM. Mở link dưới nếu muốn đăng nhập xem mức lương ẩn.",
      links: [
        { label: `ITviec: Việc làm ${kw} tại Việt Nam`, description: "Mở trang danh sách ITviec theo kỹ năng", url: `https://itviec.com/it-jobs/${encodeURIComponent(itviecSlug)}` },
        { label: `ITviec TP.HCM: ${kw}`, description: "Lọc riêng việc làm tại Hồ Chí Minh trên ITviec", url: `https://itviec.com/it-jobs/${encodeURIComponent(itviecSlug)}/ho-chi-minh-hcm` },
      ],
    },
    {
      platform: "topdev",
      name: "Saramin (TopDev)",
      badge: "Quét tự động (Live + Full JD)",
      tip: "TopDev đã chuyển sang Saramin Vietnam (saramin.vn). Hệ thống tự động quét TP.HCM (region_ids=79) kèm Full JD.",
      links: [
        { label: `Saramin TP.HCM: ${kw}`, description: "Tìm trực tiếp tại TP.HCM trên Saramin.vn", url: `https://saramin.vn/jobs/search?keyword=${enc(kw)}&region_ids=79` },
        { label: "Saramin AI & Software Engineering TP.HCM", description: "Xem các vị trí AI / Software Engineer tại TP.HCM trên Saramin", url: "https://saramin.vn/jobs/search?keyword=AI&region_ids=79" },
        { label: "Trang chủ Việc làm IT Saramin", description: "Khám phá danh mục việc làm công nghệ mới nhất trên saramin.vn", url: "https://saramin.vn/viec-lam-it" },
      ],
    },
  ];
}

async function ensureCloudData(forceReload = false) {
  if (!state.cloudProfile || forceReload) {
    const savedProf = localStorage.getItem(CLOUD_KEYS.profile);
    if (savedProf && !forceReload) {
      state.cloudProfile = JSON.parse(savedProf);
    } else {
      const r = await fetch("./data/profile.json", { cache: "no-store" });
      state.cloudProfile = await r.json();
    }
  }
  if (!state.cloudJobs || forceReload) {
    const r = await fetch("./data/jobs.json", { cache: "no-store" });
    const baseJobs = await r.json();
    const overrides = JSON.parse(localStorage.getItem(CLOUD_KEYS.overrides) || "{}");
    const customJobs = JSON.parse(localStorage.getItem(CLOUD_KEYS.customJobs) || "[]");

    const map = new Map();
    for (const j of baseJobs) map.set(j.id, j);
    for (const cj of customJobs) map.set(cj.id, cj);
    for (const [id, ov] of Object.entries(overrides)) {
      if (ov.deleted) { map.delete(id); continue; }
      if (map.has(id)) {
        const cur = map.get(id);
        if (ov.status !== undefined) cur.status = ov.status;
        if (ov.notes !== undefined) cur.notes = ov.notes;
      }
    }
    state.cloudJobs = Array.from(map.values()).map((j) => evaluateJobClient({ ...j }, state.cloudProfile));
  }
}

async function cloudApi(path, opts = {}) {
  await ensureCloudData(false);
  const method = (opts.method || "GET").toUpperCase();
  const body = opts.body ? JSON.parse(opts.body) : null;
  const u = new URL(path, "http://cloud.local");

  if (u.pathname === "/api/profile") {
    if (method === "PUT" && body) {
      state.cloudProfile = body;
      localStorage.setItem(CLOUD_KEYS.profile, JSON.stringify(body));
      state.cloudJobs = state.cloudJobs.map((j) => evaluateJobClient({ ...j }, state.cloudProfile));
    }
    return state.cloudProfile;
  }

  if (u.pathname === "/api/autoscan") {
    return { enabled: true, intervalMinutes: 360, cloudMode: true };
  }

  if (u.pathname === "/api/radar") {
    const kw = u.searchParams.get("keyword") || "Golang";
    const loc = u.searchParams.get("location") || "Ho Chi Minh";
    const days = u.searchParams.get("days") || "7";
    return { keyword: kw, location: loc, days: Number(days), platforms: buildRadarMatrixClient(kw, loc, days) };
  }

  if (u.pathname === "/api/scan") {
    await ensureCloudData(true);
    return {
      scannedAt: new Date().toISOString(),
      totalFound: state.cloudJobs.length,
      newJobs: 0,
      updatedJobs: state.cloudJobs.length,
      cloudMode: true,
      stats: [],
    };
  }

  if (u.pathname === "/api/jobs/import") {
    const job = parseRawJobPostClient(body || {}, state.cloudProfile);
    const customJobs = JSON.parse(localStorage.getItem(CLOUD_KEYS.customJobs) || "[]").filter((x) => x.id !== job.id);
    customJobs.push(job);
    localStorage.setItem(CLOUD_KEYS.customJobs, JSON.stringify(customJobs));
    state.cloudJobs = [job, ...state.cloudJobs.filter((x) => x.id !== job.id)];
    return { job, pitch: generatePitchClient(job, state.cloudProfile) };
  }

  if (u.pathname === "/api/jobs/import-batch") {
    const customJobs = JSON.parse(localStorage.getItem(CLOUD_KEYS.customJobs) || "[]");
    const customMap = new Map(customJobs.map((x) => [x.id, x]));
    let imported = 0, skippedNonHCM = 0;
    for (const item of (body?.jobs || [])) {
      const parsed = parseRawJobPostClient({ ...item, platform: item.platform || body.platform || "auto", status: item.status || "new" }, state.cloudProfile);
      if (!isHCMOrRemoteClient(parsed)) { skippedNonHCM++; continue; }
      customMap.set(parsed.id, parsed);
      state.cloudJobs = [parsed, ...state.cloudJobs.filter((x) => x.id !== parsed.id)];
      imported++;
    }
    localStorage.setItem(CLOUD_KEYS.customJobs, JSON.stringify(Array.from(customMap.values())));
    return { received: (body?.jobs || []).length, imported, skippedNonHCM, totalStored: state.cloudJobs.length };
  }

  if (u.pathname.startsWith("/api/jobs/")) {
    const rest = u.pathname.slice("/api/jobs/".length);
    if (rest.endsWith("/detail")) {
      const id = decodeURIComponent(rest.slice(0, -"/detail".length));
      const job = state.cloudJobs.find((j) => j.id === id);
      if (!job) throw new Error("job not found");
      return { job, pitch: generatePitchClient(job, state.cloudProfile) };
    }
    const id = decodeURIComponent(rest);
    const overrides = JSON.parse(localStorage.getItem(CLOUD_KEYS.overrides) || "{}");
    if (method === "PATCH") {
      overrides[id] = { ...(overrides[id] || {}), ...(body || {}) };
      localStorage.setItem(CLOUD_KEYS.overrides, JSON.stringify(overrides));
      const job = state.cloudJobs.find((j) => j.id === id);
      if (job) {
        if (body?.status !== undefined) job.status = body.status;
        if (body?.notes !== undefined) job.notes = body.notes;
      }
      return job || { id };
    }
    if (method === "DELETE") {
      overrides[id] = { deleted: true };
      localStorage.setItem(CLOUD_KEYS.overrides, JSON.stringify(overrides));
      state.cloudJobs = state.cloudJobs.filter((j) => j.id !== id);
      return { deleted: true };
    }
  }

  if (u.pathname === "/api/jobs") {
    const pFilter = (u.searchParams.get("platform") || "").toLowerCase();
    const tFilter = (u.searchParams.get("track") || "").toLowerCase();
    const sFilter = (u.searchParams.get("status") || "").toLowerCase();
    const wFilter = (u.searchParams.get("workMode") || "").toLowerCase();
    const qFilter = (u.searchParams.get("q") || "").toLowerCase();
    const minScore = Number(u.searchParams.get("minScore") || 0);

    const platformCounts = { linkedin: 0, itviec: 0, topdev: 0, topcv: 0, threads: 0, facebook: 0, zalo: 0 };
    const statusCounts = {};
    let highMatchCount = 0;

    for (const j of state.cloudJobs) {
      platformCounts[j.platform] = (platformCounts[j.platform] || 0) + 1;
      statusCounts[j.status] = (statusCounts[j.status] || 0) + 1;
      if (j.matchScore >= 75 && j.status !== "ignored") highMatchCount++;
    }

    const filtered = state.cloudJobs.filter((j) => {
      if (!sFilter) { if (j.status === "ignored") return false; }
      else if (sFilter !== "all" && (j.status || "").toLowerCase() !== sFilter) return false;
      if (pFilter && pFilter !== "all" && (j.platform || "").toLowerCase() !== pFilter) return false;
      if (tFilter && tFilter !== "all" && !(j.recommendedTrack || "").toLowerCase().includes(tFilter)) return false;
      if (wFilter && wFilter !== "all" && (j.workMode || "").toLowerCase() !== wFilter) return false;
      if (minScore > 0 && j.matchScore < minScore) return false;
      if (qFilter) {
        const hay = `${j.title} ${j.company} ${j.location} ${j.description || ""} ${(j.skills || []).join(" ")} ${(j.matchedSkills || []).join(" ")}`.toLowerCase();
        if (!hay.includes(qFilter)) return false;
      }
      return true;
    }).sort((a, b) => b.matchScore - a.matchScore || String(b.fetchedAt).localeCompare(String(a.fetchedAt)));

    return {
      jobs: filtered,
      totalStored: state.cloudJobs.length,
      highMatchCount,
      platformCounts,
      statusCounts,
      scanning: false,
      autoScan: { enabled: true, intervalMinutes: 360, cloudMode: true },
    };
  }

  throw new Error("Unsupported cloud route: " + u.pathname);
}

const PLATFORM_LABELS = {
  linkedin: "LinkedIn",
  itviec: "ITviec",
  topdev: "Saramin (TopDev)",
  topcv: "TopCV",
  threads: "Threads",
  facebook: "Facebook",
  zalo: "Zalo",
};

const STATUS_OPTIONS = [
  { value: "new", label: "Mới" },
  { value: "saved", label: "⭐ Đã lưu" },
  { value: "applied", label: "📨 Đã ứng tuyển" },
  { value: "interviewing", label: "🎤 Phỏng vấn" },
  { value: "offer", label: "🎉 Offer" },
  { value: "ignored", label: "🚫 Ẩn" },
];

function $(id) {
  return document.getElementById(id);
}

function escapeHtml(str) {
  if (!str) return "";
  return String(str)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function showToast(msg) {
  const t = $("toast");
  t.textContent = msg;
  t.classList.remove("hidden");
  clearTimeout(showToast._timer);
  showToast._timer = setTimeout(() => t.classList.add("hidden"), 3200);
}

async function api(url, opts = {}) {
  if (state.cloudMode) {
    return cloudApi(url, opts);
  }
  try {
    const res = await fetch(`${state.apiBase}${url}`, {
      headers: { "Content-Type": "application/json", ...(opts.headers || {}) },
      ...opts,
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      throw new Error(data.error || `HTTP ${res.status}`);
    }
    return data;
  } catch (err) {
    if (!isLocalhostHost()) {
      state.cloudMode = true;
      return cloudApi(url, opts);
    }
    throw err;
  }
}

function switchTab(tabName) {
  state.activeTab = tabName;
  document.querySelectorAll(".tab-btn").forEach((btn) => {
    const active = btn.dataset.tab === tabName;
    btn.classList.toggle("active", active);
    btn.setAttribute("aria-selected", active ? "true" : "false");
  });
  document.querySelectorAll(".tab-panel").forEach((panel) => {
    panel.classList.toggle("active", panel.id === `tab-${tabName}`);
  });
}

async function loadProfile() {
  try {
    const p = await api("/api/profile");
    state.profile = p;
    $("profileHeadline").textContent = `${p.fullName} • ${p.headline}`;
    $("profTier1").value = (p.tier1Skills || []).join(", ");
    $("profTier2").value = (p.tier2Skills || []).join(", ");
    $("profTier3").value = (p.tier3Skills || []).join(", ");
    $("profTier4").value = (p.tier4Skills || []).join(", ");
  } catch (e) {
    console.error("loadProfile error:", e);
  }
}

async function loadJobs() {
  const params = new URLSearchParams();
  if (state.activePlatform !== "all") params.set("platform", state.activePlatform);
  const track = $("filterTrack").value;
  if (track !== "all") params.set("track", track);
  const wm = $("filterWorkMode").value;
  if (wm !== "all") params.set("workMode", wm);
  const minScore = $("filterMinScore").value;
  if (Number(minScore) > 0) params.set("minScore", minScore);
  const status = $("filterStatus").value;
  if (status) params.set("status", status);
  const q = $("globalSearch").value.trim();
  if (q) params.set("q", q);

  try {
    const data = await api(`/api/jobs?${params.toString()}`);
    state.jobs = data.jobs || [];
    state.platformCounts = data.platformCounts || {};
    state.statusCounts = data.statusCounts || {};
    if (data.autoScan) {
      state.autoScan = data.autoScan;
      renderAutoScanStatus();
    }
    renderKPIs(data);
    renderPlatformCounts();
    renderJobsFeed();
    renderKanban(data);

    // If initial background scan is still running on first boot, poll briefly
    if (data.scanning && (data.totalStored || 0) === 0) {
      $("scanBanner").classList.remove("hidden");
      $("scanBanner").textContent = "⏳ Đang tự động quét job thực tế lần đầu từ LinkedIn, ITviec và TopDev theo CV của Leo...";
      setTimeout(loadJobs, 2500);
    } else if (!data.scanning) {
      if ($("scanBanner").textContent.includes("lần đầu")) {
        $("scanBanner").classList.add("hidden");
      }
    }
  } catch (e) {
    showToast("Lỗi tải danh sách job: " + e.message);
  }
}

function renderAutoScanStatus() {
  const a = state.autoScan || {};
  const dot = $("autoScanDot");
  const label = $("autoScanLabel");
  const ivSel = $("autoScanInterval");
  const btn = $("btnAutoScanToggle");
  if (!dot || !label || !ivSel || !btn) return;

  if (state.cloudMode || a.cloudMode) {
    dot.classList.add("on");
    label.innerHTML = `<strong>☁️ GitHub Pages Cloud Mode (TP.HCM &amp; Remote):</strong> <span class="text-emerald">Tự cập nhật mỗi 6h qua GitHub Actions</span>`;
    $("autoScanCountdown").textContent = "Đang dùng dữ liệu đồng bộ từ GitHub";
    ivSel.classList.add("hidden");
    btn.classList.add("hidden");
    return;
  }

  ivSel.classList.remove("hidden");
  btn.classList.remove("hidden");
  dot.classList.toggle("on", Boolean(a.enabled));
  label.innerHTML = a.enabled
    ? `<strong>Quét liên tục 24/7 (TP.HCM &amp; Remote):</strong> <span class="text-emerald">ĐANG BẬT</span>`
    : `<strong>Quét liên tục 24/7 (TP.HCM &amp; Remote):</strong> <span class="muted">ĐANG TẠM DỪNG</span>`;
  if (a.intervalMinutes && Array.from(ivSel.options).some((o) => Number(o.value) === a.intervalMinutes)) {
    ivSel.value = String(a.intervalMinutes);
  }
  btn.textContent = a.enabled ? "Tạm dừng quét ngầm" : "Bật quét liên tục";
  updateAutoScanCountdown();
}

function updateAutoScanCountdown() {
  const el = $("autoScanCountdown");
  if (!el) return;
  const a = state.autoScan || {};
  if (state.cloudMode || a.cloudMode) return;
  if (!a.enabled || !a.nextScanAt) {
    el.textContent = "Đã tạm dừng";
    return;
  }
  const diffSec = Math.max(0, Math.floor((new Date(a.nextScanAt).getTime() - Date.now()) / 1000));
  const m = String(Math.floor(diffSec / 60)).padStart(2, "0");
  const s = String(diffSec % 60).padStart(2, "0");
  el.textContent = `Lần quét tiếp theo: ${m}:${s}`;
}

async function updateAutoScanConfig(payload) {
  try {
    const snap = await api("/api/autoscan", {
      method: "POST",
      body: JSON.stringify(payload),
    });
    state.autoScan = snap;
    renderAutoScanStatus();
    showToast(snap.enabled ? `Đã bật quét liên tục mỗi ${snap.intervalMinutes} phút` : "Đã tạm dừng quét tự động");
  } catch (e) {
    showToast("Lỗi cấu hình quét tự động: " + e.message);
  }
}

function renderKPIs(data) {
  const pc = data.platformCounts || {};
  const sc = data.statusCounts || {};
  $("kpiTotal").textContent = data.totalStored || 0;
  $("kpiHighMatch").textContent = data.highMatchCount || 0;

  const autoTotal = (pc.linkedin || 0) + (pc.itviec || 0) + (pc.topdev || 0);
  $("kpiAutoSources").textContent = autoTotal;
  $("kpiAutoBreakdown").textContent = `LI: ${pc.linkedin || 0} • ITviec: ${pc.itviec || 0} • Saramin: ${pc.topdev || 0}`;

  const saved = sc.saved || 0;
  const applied = sc.applied || 0;
  const iv = sc.interviewing || 0;
  const offer = sc.offer || 0;
  $("kpiPipeline").textContent = saved + applied + iv + offer;
  $("kpiPipelineMeta").textContent = `Lưu: ${saved} • Đã nộp: ${applied} • PV: ${iv}`;

  if (data.lastScan && data.lastScan.scannedAt) {
    const dt = new Date(data.lastScan.scannedAt);
    $("kpiLastScan").textContent = `Quét gần nhất: ${dt.toLocaleTimeString("vi-VN")} (${data.lastScan.totalFound} job)`;
  }
}

function renderPlatformCounts() {
  const pc = state.platformCounts || {};
  let total = 0;
  for (const k of Object.keys(PLATFORM_LABELS)) {
    const count = pc[k] || 0;
    total += count;
    const el = $(`cnt-${k}`);
    if (el) el.textContent = count;
  }
  const allEl = $("cnt-all");
  if (allEl) allEl.textContent = total;
}

function renderJobsFeed() {
  const container = $("jobsList");
  if (!state.jobs.length) {
    container.innerHTML = `
      <div class="empty-state" style="grid-column: 1 / -1;">
        Chưa có job nào khớp bộ lọc hiện tại. Hãy bấm <strong>"🔄 Quét Job Tự Động"</strong> ở góc trên hoặc nới lỏng bộ lọc!
      </div>`;
    return;
  }

  container.innerHTML = state.jobs
    .map((job) => {
      const scoreClass = job.matchLevel === "high" ? "score-high" : job.matchLevel === "medium" ? "score-medium" : "score-low";
      const platLabel = PLATFORM_LABELS[job.platform] || job.platform;
      const initial = (job.company || "J").trim().charAt(0).toUpperCase();
      const logoHtml = job.companyLogo
        ? `<img src="${escapeHtml(job.companyLogo)}" alt="${escapeHtml(job.company)}" class="company-logo" onerror="this.style.display='none'" />`
        : `<div class="company-logo-fallback">${escapeHtml(initial)}</div>`;

      const matchedHtml = (job.matchedSkills || [])
        .slice(0, 7)
        .map((s) => `<span class="skill-chip skill-match">✓ ${escapeHtml(s)}</span>`)
        .join("");

      const missingHtml = (job.missingSkills || [])
        .slice(0, 3)
        .map((s) => `<span class="skill-chip skill-miss">+ ${escapeHtml(s)}</span>`)
        .join("");

      const rawSkillsHtml =
        (!job.matchedSkills || job.matchedSkills.length === 0) && (job.skills || []).length
          ? job.skills
              .slice(0, 5)
              .map((s) => `<span class="skill-chip skill-raw">${escapeHtml(s)}</span>`)
              .join("")
          : "";

      const projectsHint =
        job.recommendedProjects && job.recommendedProjects.length
          ? `<div class="project-hint">💡 <strong>Dự án nên khoe từ CV:</strong> ${escapeHtml(job.recommendedProjects.join(" • "))}</div>`
          : "";

      const statusOptions = STATUS_OPTIONS.map(
        (opt) => `<option value="${opt.value}" ${job.status === opt.value ? "selected" : ""}>${opt.label}</option>`
      ).join("");

      const openBtn = job.url
        ? `<a href="${escapeHtml(job.url)}" target="_blank" rel="noopener noreferrer" class="btn btn-sm btn-secondary">Mở bài gốc ↗</a>`
        : "";

      return `
        <article class="job-card" data-id="${escapeHtml(job.id)}">
          <div class="job-card-top">
            <div class="job-main-head">
              ${logoHtml}
              <div>
                <a href="${escapeHtml(job.url || "#")}" target="_blank" rel="noopener noreferrer" class="job-title">${escapeHtml(job.title)}</a>
                <div class="job-company-row">
                  <span>🏢 ${escapeHtml(job.company || "Đang cập nhật")}</span>
                  <span>📍 ${escapeHtml(job.location || "Vietnam")}</span>
                  ${job.workMode && job.workMode !== "Unknown" ? `<span>💼 ${escapeHtml(job.workMode)}</span>` : ""}
                </div>
              </div>
            </div>
            <div class="score-pill ${scoreClass}" title="Độ khớp với CV của Leo">
              ${job.matchScore}% Khớp
            </div>
          </div>

          <div class="job-meta-row">
            <span class="plat-badge plat-${escapeHtml(job.platform)}">${escapeHtml(platLabel)}</span>
            <span class="track-badge">${escapeHtml(job.recommendedTrack)}</span>
            ${job.salary ? `<span>💰 ${escapeHtml(job.salary)}</span>` : ""}
            ${job.postedAt ? `<span>🕒 ${escapeHtml(job.postedAt)}</span>` : ""}
            ${job.contactInfo ? `<span>📞 ${escapeHtml(job.contactInfo)}</span>` : ""}
          </div>

          <div class="skills-wrap">
            ${matchedHtml}
            ${missingHtml}
            ${rawSkillsHtml}
          </div>

          ${projectsHint}

          <div class="job-actions">
            <div class="job-actions-left">
              <button type="button" class="btn btn-sm btn-primary btn-inspect" data-id="${escapeHtml(job.id)}">
                🔍 Phân tích sâu &amp; Tin nhắn HR
              </button>
              ${openBtn}
            </div>
            <select class="select-ctrl btn-sm status-select" data-id="${escapeHtml(job.id)}" aria-label="Trạng thái ứng tuyển">
              ${statusOptions}
            </select>
          </div>
        </article>`;
    })
    .join("");
}

async function runLiveScan(customKeywords = []) {
  const btn = $("btnScanCV");
  const spinner = $("scanSpinner");
  btn.disabled = true;
  spinner.classList.remove("hidden");
  $("scanBanner").classList.remove("hidden");
  $("scanBanner").textContent = state.cloudMode
    ? "☁️ Đang làm mới dữ liệu mới nhất từ GitHub Pages (data/jobs.json)..."
    : "⏳ Đang quét song song LinkedIn, ITviec và TopDev theo CV của Trương Tăng Quang...";

  try {
    const payload = customKeywords.length ? { keywords: customKeywords } : {};
    const res = await api("/api/scan", {
      method: "POST",
      body: JSON.stringify(payload),
    });
    if (res.cloudMode) {
      $("scanBanner").textContent = `☁️ Đã tải bản mới nhất (${res.totalFound} job TP.HCM & Remote). Hệ thống tự quét bằng GitHub Actions mỗi 6h; muốn quét Live theo từ khóa tức thì hãy bật ./start.sh trên máy local.`;
      showToast(`Đã làm mới ${res.totalFound} job từ GitHub!`);
    } else {
      const statText = (res.stats || [])
        .map((s) => `${PLATFORM_LABELS[s.platform] || s.platform}: ${s.fetched} quét được (+${s.added} mới)${s.error ? ` [lỗi: ${s.error}]` : ""}`)
        .join(" | ");
      $("scanBanner").textContent = `✅ Hoàn tất lúc ${new Date(res.scannedAt).toLocaleTimeString("vi-VN")}: Tổng ${res.totalFound} job (${res.newJobs} mới, ${res.updatedJobs} cập nhật). ${statText}`;
      showToast(`Đã quét xong ${res.totalFound} job (${res.newJobs} mới)!`);
    }
    await loadJobs();
  } catch (e) {
    $("scanBanner").textContent = "❌ Quét thất bại: " + e.message;
    showToast("Lỗi khi quét job: " + e.message);
  } finally {
    btn.disabled = false;
    spinner.classList.add("hidden");
  }
}

async function openJobModal(jobId) {
  const modal = $("jobModal");
  modal.classList.remove("hidden");
  $("modalTitle").textContent = "Đang tải chi tiết JD & chấm điểm sâu...";
  $("modalSub").textContent = "";
  $("modalBadges").innerHTML = "";
  $("modalBody").innerHTML = `<div class="empty-state">⏳ Đang kéo nội dung JD đầy đủ từ nguồn gốc và soạn tin nhắn ứng tuyển...</div>`;

  try {
    const data = await api(`/api/jobs/${encodeURIComponent(jobId)}/detail`);
    const job = data.job;
    const pitch = data.pitch;

    $("modalTitle").textContent = job.title;
    $("modalSub").textContent = `${job.company} • ${job.location} ${job.salary ? "• " + job.salary : ""} ${job.postedAt ? "• " + job.postedAt : ""}`;
    $("modalBadges").innerHTML = `
      <span class="plat-badge plat-${escapeHtml(job.platform)}">${escapeHtml(PLATFORM_LABELS[job.platform] || job.platform)}</span>
      <span class="track-badge">${escapeHtml(job.recommendedTrack)}</span>
      <span class="score-pill ${job.matchLevel === "high" ? "score-high" : job.matchLevel === "medium" ? "score-medium" : "score-low"}">${job.matchScore}% Khớp CV</span>
    `;

    const reasonsHtml = (job.matchReasons || []).map((r) => `<li>${escapeHtml(r)}</li>`).join("");
    const matchedHtml = (job.matchedSkills || []).map((s) => `<span class="skill-chip skill-match">✓ ${escapeHtml(s)}</span>`).join("");
    const missingHtml = (job.missingSkills || []).map((s) => `<span class="skill-chip skill-miss">+ ${escapeHtml(s)}</span>`).join("");

    $("modalBody").innerHTML = `
      <div class="skills-wrap mb-12">${matchedHtml} ${missingHtml}</div>
      ${reasonsHtml ? `<div class="project-hint mb-12"><strong>Lý do chấm điểm:</strong><ul style="margin-left:18px;margin-top:4px;">${reasonsHtml}</ul></div>` : ""}
      ${job.recommendedProjects?.length ? `<div class="project-hint mb-12"><strong>Dự án trong CV nên nhấn mạnh:</strong> ${escapeHtml(job.recommendedProjects.join(" • "))}</div>` : ""}

      <h3 style="font-size:14px;margin-bottom:6px;">📄 Nội dung mô tả công việc (JD)</h3>
      <div class="jd-box mb-12">${escapeHtml(job.description || "Nguồn này không có văn bản mô tả chi tiết công khai; bấm 'Mở bài gốc' để xem đầy đủ trên web gốc.")}</div>

      <div class="pitch-box">
        <div class="card-head-between" style="margin-bottom:4px;">
          <strong>💬 Tin nhắn ứng tuyển Tiếng Việt (Dành cho Zalo / Threads / Facebook DM)</strong>
          <button type="button" class="btn btn-sm btn-secondary btn-copy-text" data-target="pitchVi">Copy Tiếng Việt</button>
        </div>
        <pre id="pitchVi" class="pitch-pre">${escapeHtml(pitch.vietnameseShort)}</pre>
      </div>

      <div class="pitch-box">
        <div class="card-head-between" style="margin-bottom:4px;">
          <strong>✉️ Cover Message Tiếng Anh (Dành cho LinkedIn / Email / ITviec)</strong>
          <button type="button" class="btn btn-sm btn-secondary btn-copy-text" data-target="pitchEn">Copy Tiếng Anh</button>
        </div>
        <pre id="pitchEn" class="pitch-pre">${escapeHtml(pitch.englishPro)}</pre>
      </div>

      <div style="display:flex;gap:10px;justify-content:flex-end;margin-top:14px;flex-wrap:wrap;">
        ${job.url ? `<a href="${escapeHtml(job.url)}" target="_blank" rel="noopener noreferrer" class="btn btn-primary">Mở trang ứng tuyển gốc ↗</a>` : ""}
        <button type="button" class="btn btn-secondary" onclick="updateJobStatus('${escapeHtml(job.id)}', 'applied')">Đánh dấu 📨 Đã ứng tuyển</button>
      </div>
    `;

    // Refresh feed in case deep JD fetch updated MatchScore
    await loadJobs();
  } catch (e) {
    $("modalBody").innerHTML = `<div class="empty-state">Lỗi tải chi tiết: ${escapeHtml(e.message)}</div>`;
  }
}

async function updateJobStatus(id, status) {
  try {
    await api(`/api/jobs/${encodeURIComponent(id)}`, {
      method: "PATCH",
      body: JSON.stringify({ status }),
    });
    showToast("Đã cập nhật trạng thái job");
    await loadJobs();
  } catch (e) {
    showToast("Lỗi cập nhật trạng thái: " + e.message);
  }
}

async function updateJobNotes(id, notes) {
  try {
    await api(`/api/jobs/${encodeURIComponent(id)}`, {
      method: "PATCH",
      body: JSON.stringify({ notes }),
    });
    showToast("Đã lưu ghi chú");
  } catch (e) {
    showToast("Lỗi lưu ghi chú: " + e.message);
  }
}

// RADAR 7 PLATFORMS
async function loadRadar() {
  const kw = $("radarKeyword").value.trim() || "Golang";
  const loc = $("radarLocation").value.trim() || "Ho Chi Minh";
  const days = $("radarDays").value || "7";
  try {
    const data = await api(`/api/radar?keyword=${encodeURIComponent(kw)}&location=${encodeURIComponent(loc)}&days=${encodeURIComponent(days)}`);
    const platforms = data.platforms || [];
    $("radarGrid").innerHTML = platforms
      .map((p) => {
        const linksHtml = (p.links || [])
          .map(
            (l) => `
          <a href="${escapeHtml(l.URL || l.url)}" target="_blank" rel="noopener noreferrer" class="radar-link-item">
            <div class="radar-link-title">
              <span>${escapeHtml(l.label)}</span>
              <span>↗</span>
            </div>
            <div class="radar-link-desc">${escapeHtml(l.description)}</div>
          </a>`
          )
          .join("");
        return `
          <div class="radar-card">
            <div class="radar-card-head">
              <div style="display:flex;align-items:center;gap:8px;">
                <span class="plat-badge plat-${escapeHtml(p.platform)}">${escapeHtml(p.name)}</span>
                <span class="muted">${escapeHtml(p.badge)}</span>
              </div>
            </div>
            <p class="muted">${escapeHtml(p.tip)}</p>
            <div style="display:flex;flex-direction:column;gap:8px;">
              ${linksHtml}
            </div>
          </div>`;
      })
      .join("");
  } catch (e) {
    showToast("Lỗi tải Radar 7 nền tảng: " + e.message);
  }
}

function buildBookmarkletCode() {
  const basePath = window.location.pathname.replace(/\/index\.html$/, "").replace(/\/$/, "");
  const origin = window.location.origin + basePath;
  const fn = function (targetOrigin) {
    if (location.hostname.indexOf("topcv.vn") !== -1) {
      var cards = Array.from(document.querySelectorAll(".job-item-search-result,.job-item-2,.job-item,.box-job-item"));
      var seen = {};
      var batch = [];
      for (var i = 0; i < cards.length && batch.length < 30; i++) {
        var c = cards[i];
        var a = c.querySelector('a[href*="/viec-lam/"]');
        if (!a) continue;
        var u = a.href.split("?")[0];
        if (seen[u]) continue;
        seen[u] = 1;
        var titleEl = c.querySelector("h3,.job-title") || a;
        var compEl = c.querySelector(".company-name,.company");
        var salEl = c.querySelector(".title-salary,.salary");
        var locEl = c.querySelector(".address,.city,.location");
        batch.push({
          platform: "topcv",
          title: (titleEl.innerText || "").trim().replace(/\s+/g, " ").slice(0, 120),
          company: compEl ? (compEl.innerText || "").trim().replace(/\s+/g, " ").slice(0, 80) : "",
          salary: salEl ? (salEl.innerText || "").trim().replace(/\s+/g, " ").slice(0, 60) : "",
          location: locEl ? (locEl.innerText || "").trim().replace(/\s+/g, " ").slice(0, 80) : "",
          url: u,
          rawText: (c.innerText || "").trim().replace(/\s+/g, " ").slice(0, 260)
        });
      }
      if (batch.length > 0) {
        window.open(targetOrigin + "/#import_batch=" + encodeURIComponent(JSON.stringify({ platform: "topcv", jobs: batch })), "_blank");
        return;
      }
    }
    var d = { title: document.title, url: location.href, rawText: (window.getSelection().toString() || document.body.innerText || "").slice(0, 3500) };
    window.open(targetOrigin + "/#import=" + encodeURIComponent(JSON.stringify(d)), "_blank");
  };
  return "javascript:(" + fn.toString() + ")(" + JSON.stringify(origin) + ");";
}

function setupBookmarklet() {
  const code = buildBookmarkletCode();
  $("bookmarkletLink").setAttribute("href", code);
  $("btnCopyBookmarklet").addEventListener("click", async () => {
    try {
      await navigator.clipboard.writeText(code);
      showToast("Đã copy mã Bookmarklet! Tạo 1 Bookmark mới và dán vào mục URL.");
    } catch {
      showToast("Không copy tự động được — hãy kéo nút tím lên thanh Bookmarks.");
    }
  });
}

// IMPORTER (Zalo / FB / Threads / TopCV)
async function submitImportPayload(payload) {
  try {
    const res = await api("/api/jobs/import", {
      method: "POST",
      body: JSON.stringify(payload),
    });
    renderImportAnalysis(res.job, res.pitch);
    showToast(`Đã phân tích & lưu job (${res.job.matchScore}% khớp)!`);
    await loadJobs();
  } catch (e) {
    showToast("Lỗi phân tích tin tuyển dụng: " + e.message);
  }
}

function renderImportAnalysis(job, pitch) {
  const scoreClass = job.matchLevel === "high" ? "score-high" : job.matchLevel === "medium" ? "score-medium" : "score-low";
  const matchedHtml = (job.matchedSkills || []).map((s) => `<span class="skill-chip skill-match">✓ ${escapeHtml(s)}</span>`).join("");
  const missingHtml = (job.missingSkills || []).map((s) => `<span class="skill-chip skill-miss">+ ${escapeHtml(s)}</span>`).join("");

  $("importResultBody").className = "";
  $("importResultBody").innerHTML = `
    <div style="display:flex;justify-content:space-between;align-items:flex-start;gap:10px;margin-bottom:10px;">
      <div>
        <span class="plat-badge plat-${escapeHtml(job.platform)}">${escapeHtml(PLATFORM_LABELS[job.platform] || job.platform)}</span>
        <span class="track-badge" style="margin-left:6px;">${escapeHtml(job.recommendedTrack)}</span>
        <h3 style="margin-top:6px;">${escapeHtml(job.title)}</h3>
        <div class="muted">${escapeHtml(job.company)} • ${escapeHtml(job.location)} ${job.salary ? "• 💰 " + escapeHtml(job.salary) : ""}</div>
        ${job.contactInfo ? `<div style="margin-top:4px;font-size:13px;color:var(--sky);">📞 Liên hệ trích xuất: <strong>${escapeHtml(job.contactInfo)}</strong></div>` : ""}
      </div>
      <div class="score-pill ${scoreClass}">${job.matchScore}% Khớp</div>
    </div>

    <div class="skills-wrap mb-12">${matchedHtml || '<span class="muted">Chưa thấy từ khóa kỹ năng khớp rõ ràng</span>'} ${missingHtml}</div>
    ${job.recommendedProjects?.length ? `<div class="project-hint mb-12">💡 <strong>Dự án nên khoe:</strong> ${escapeHtml(job.recommendedProjects.join(" • "))}</div>` : ""}

    <div class="pitch-box">
      <div class="card-head-between" style="margin-bottom:4px;">
        <strong>💬 Tin nhắn gửi HR (Zalo / FB / Threads)</strong>
        <button type="button" class="btn btn-sm btn-secondary btn-copy-text" data-target="impPitchVi">Copy</button>
      </div>
      <pre id="impPitchVi" class="pitch-pre">${escapeHtml(pitch.vietnameseShort)}</pre>
    </div>

    <div class="pitch-box">
      <div class="card-head-between" style="margin-bottom:4px;">
        <strong>✉️ Cover Message (English)</strong>
        <button type="button" class="btn btn-sm btn-secondary btn-copy-text" data-target="impPitchEn">Copy</button>
      </div>
      <pre id="impPitchEn" class="pitch-pre">${escapeHtml(pitch.englishPro)}</pre>
    </div>
  `;
}

async function checkHashImport() {
  if (window.location.hash.startsWith("#import_batch=")) {
    try {
      const raw = decodeURIComponent(window.location.hash.slice("#import_batch=".length));
      const payload = JSON.parse(raw);
      history.replaceState(null, "", window.location.pathname);
      const res = await api("/api/jobs/import-batch", {
        method: "POST",
        body: JSON.stringify(payload),
      });
      state.activePlatform = payload.platform || "topcv";
      document.querySelectorAll("#platformFilterPills .pill").forEach((p) => {
        p.classList.toggle("active", p.dataset.platform === state.activePlatform);
      });
      switchTab("feed");
      showToast(`Đã đồng bộ ${res.imported}/${res.received} job TopCV (TP.HCM/Remote)!`);
      await loadJobs();
    } catch (e) {
      console.error("Batch bookmarklet import error:", e);
      showToast("Lỗi nhập batch từ Bookmarklet: " + e.message);
    }
    return;
  }
  if (!window.location.hash.startsWith("#import=")) return;
  try {
    const raw = decodeURIComponent(window.location.hash.slice("#import=".length));
    const payload = JSON.parse(raw);
    history.replaceState(null, "", window.location.pathname);
    switchTab("importer");
    $("impUrl").value = payload.url || "";
    $("impTitle").value = payload.title || "";
    $("impRawText").value = payload.rawText || "";
    await submitImportPayload({
      platform: "auto",
      url: payload.url || "",
      title: payload.title || "",
      rawText: payload.rawText || "",
      status: "saved",
    });
  } catch (e) {
    console.error("Bookmarklet import error:", e);
  }
}

// KANBAN PIPELINE
async function renderKanban(currentData) {
  // Fetch all statuses if current filter excluded some
  let allJobs = currentData.jobs || [];
  if ($("filterStatus").value !== "all") {
    try {
      const full = await api("/api/jobs?status=all");
      allJobs = full.jobs || [];
    } catch {
      // ignore fallback
    }
  }
  const cols = { saved: [], applied: [], interviewing: [], offer: [] };
  for (const j of allJobs) {
    if (cols[j.status]) cols[j.status].push(j);
  }

  for (const status of Object.keys(cols)) {
    const list = cols[status];
    $(`kcnt-${status}`).textContent = list.length;
    const colEl = $(`kcol-${status}`);
    if (!list.length) {
      colEl.innerHTML = `<div class="muted" style="font-size:12px;">Trống</div>`;
      continue;
    }
    colEl.innerHTML = list
      .map(
        (j) => `
      <div class="kanban-item">
        <div style="display:flex;justify-content:space-between;gap:6px;align-items:center;">
          <span class="plat-badge plat-${escapeHtml(j.platform)}">${escapeHtml(PLATFORM_LABELS[j.platform] || j.platform)}</span>
          <span class="score-pill ${j.matchLevel === "high" ? "score-high" : "score-medium"}" style="padding:2px 6px;font-size:11px;">${j.matchScore}%</span>
        </div>
        <a href="${escapeHtml(j.url || "#")}" target="_blank" rel="noopener noreferrer" style="font-weight:700;font-size:13px;">${escapeHtml(j.title)}</a>
        <div class="muted" style="font-size:12px;">${escapeHtml(j.company)}</div>
        <textarea class="textarea-ctrl kanban-note" rows="2" placeholder="Ghi chú (ngày PV, tên HR, deal lương)..." data-id="${escapeHtml(j.id)}">${escapeHtml(j.notes || "")}</textarea>
        <div style="display:flex;justify-content:space-between;gap:6px;">
          <button type="button" class="btn btn-sm btn-secondary btn-inspect" data-id="${escapeHtml(j.id)}">Chi tiết</button>
          <select class="select-ctrl btn-sm status-select" data-id="${escapeHtml(j.id)}">
            ${STATUS_OPTIONS.map((o) => `<option value="${o.value}" ${j.status === o.value ? "selected" : ""}>${o.label}</option>`).join("")}
          </select>
        </div>
      </div>`
      )
      .join("");
  }
}

async function saveProfileSkills() {
  if (!state.profile) return;
  const splitCSV = (val) =>
    val
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);
  const updated = {
    ...state.profile,
    tier1Skills: splitCSV($("profTier1").value),
    tier2Skills: splitCSV($("profTier2").value),
    tier3Skills: splitCSV($("profTier3").value),
    tier4Skills: splitCSV($("profTier4").value),
  };
  try {
    await api("/api/profile", {
      method: "PUT",
      body: JSON.stringify(updated),
    });
    showToast("Đã cập nhật CV và chấm điểm lại toàn bộ Job!");
    await loadProfile();
    await loadJobs();
  } catch (e) {
    showToast("Lỗi lưu CV: " + e.message);
  }
}

function wireEvents() {
  // Tabs
  document.querySelectorAll(".tab-btn").forEach((btn) => {
    btn.addEventListener("click", () => switchTab(btn.dataset.tab));
  });

  $("btnQuickImport").addEventListener("click", () => switchTab("importer"));
  $("btnScanCV").addEventListener("click", () => runLiveScan());
  $("btnCustomScan").addEventListener("click", () => {
    const raw = $("customScanKeyword").value.trim();
    if (!raw) return runLiveScan();
    const kws = raw
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);
    runLiveScan(kws);
  });

  // Platform filter pills
  $("platformFilterPills").addEventListener("click", (e) => {
    const pill = e.target.closest(".pill");
    if (!pill) return;
    state.activePlatform = pill.dataset.platform;
    document.querySelectorAll("#platformFilterPills .pill").forEach((p) => p.classList.toggle("active", p === pill));
    loadJobs();
  });

  // Dropdown filters + search
  ["filterTrack", "filterWorkMode", "filterMinScore", "filterStatus"].forEach((id) => {
    $(id).addEventListener("change", loadJobs);
  });

  $("globalSearch").addEventListener("input", () => {
    clearTimeout(state.searchDebounce);
    state.searchDebounce = setTimeout(loadJobs, 220);
  });

  // Cmd+K shortcut
  window.addEventListener("keydown", (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
      e.preventDefault();
      $("globalSearch").focus();
    }
    if (e.key === "Escape") {
      $("jobModal").classList.add("hidden");
    }
  });

  // Delegate clicks inside main (Inspect / Copy / Status / Notes)
  document.addEventListener("click", async (e) => {
    const inspectBtn = e.target.closest(".btn-inspect");
    if (inspectBtn) {
      openJobModal(inspectBtn.dataset.id);
      return;
    }
    const copyBtn = e.target.closest(".btn-copy-text");
    if (copyBtn) {
      const targetEl = $(copyBtn.dataset.target);
      if (targetEl) {
        await navigator.clipboard.writeText(targetEl.textContent);
        showToast("Đã copy nội dung tin nhắn!");
      }
    }
  });

  document.addEventListener("change", (e) => {
    const sel = e.target.closest(".status-select");
    if (sel) {
      updateJobStatus(sel.dataset.id, sel.value);
      return;
    }
    const noteEl = e.target.closest(".kanban-note");
    if (noteEl) {
      updateJobNotes(noteEl.dataset.id, noteEl.value);
    }
  });

  // Modal close
  $("modalClose").addEventListener("click", () => $("jobModal").classList.add("hidden"));
  $("modalBackdrop").addEventListener("click", () => $("jobModal").classList.add("hidden"));

  // Radar events
  $("radarPresets").addEventListener("click", (e) => {
    const chip = e.target.closest(".preset-chip");
    if (!chip) return;
    document.querySelectorAll(".preset-chip").forEach((c) => c.classList.toggle("active", c === chip));
    $("radarKeyword").value = chip.dataset.kw;
    loadRadar();
  });
  $("btnRefreshRadar").addEventListener("click", loadRadar);
  $("radarDays").addEventListener("change", loadRadar);

  // Importer form
  $("btnFillSamplePost").addEventListener("click", () => {
    $("impPlatform").value = "zalo";
    $("impUrl").value = "";
    $("impTitle").value = "";
    $("impCompany").value = "";
    $("impRawText").value = `[HCM - HYBRID] TUYỂN GẤP SENIOR BACKEND (GOLANG / NODEJS NESTJS)
Công ty: FinTech AI Startup Quận 1
Mức lương: 2500 - 3500 USD (thỏa thuận theo năng lực)
Yêu cầu:
- 4+ năm kinh nghiệm xây dựng Microservices bằng Golang (Gin/GORM) hoặc Node.js (NestJS, TypeScript)
- Thành thạo PostgreSQL, Redis, Kafka, Docker, Kubernetes (AKS/EKS)
- Có kinh nghiệm làm AI / LLM / RAG / OpenAI / Gemini là điểm cộng rất lớn!
Liên hệ gửi CV trực tiếp qua Zalo: 0901234567 hoặc email hr@fintech-ai.vn`;
  });

  $("importForm").addEventListener("submit", async (e) => {
    e.preventDefault();
    await submitImportPayload({
      platform: $("impPlatform").value,
      url: $("impUrl").value.trim(),
      title: $("impTitle").value.trim(),
      company: $("impCompany").value.trim(),
      rawText: $("impRawText").value.trim(),
      status: "saved",
    });
  });

  // Save CV profile
  $("btnSaveProfile").addEventListener("click", saveProfileSkills);

  // Auto-scan controls
  $("btnAutoScanToggle").addEventListener("click", () => {
    updateAutoScanConfig({ enabled: !state.autoScan.enabled });
  });
  $("autoScanInterval").addEventListener("change", () => {
    updateAutoScanConfig({
      enabled: true,
      intervalMinutes: Number($("autoScanInterval").value) || 15,
    });
  });
}

window.updateJobStatus = updateJobStatus;

async function boot() {
  await resolveBackendBase();
  wireEvents();
  setupBookmarklet();
  await loadProfile();
  await Promise.all([loadJobs(), loadRadar()]);
  await checkHashImport();

  setInterval(updateAutoScanCountdown, 1000);
  // Silent UI refresh every 20s when connected to local server
  setInterval(() => {
    if (!state.cloudMode && !document.hidden && state.activeTab === "feed") {
      loadJobs();
    }
  }, 20000);
}

boot();
