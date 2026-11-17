(() => {
  const API_BATCH = "http://localhost:8090/api/jobs/import-batch";
  const API_SINGLE = "http://localhost:8090/api/jobs/import";
  const STORAGE_KEY_AUTO = "leo_topcv_auto_reload_v1";
  const RELOAD_INTERVAL_MS = 10 * 60 * 1000; // 10 minutes

  let autoTimer = null;
  let countdownTimer = null;
  let nextReloadAt = 0;

  function cleanUrl(rawHref) {
    try {
      const u = new URL(rawHref, location.origin);
      return u.origin + u.pathname;
    } catch {
      return rawHref || "";
    }
  }

  function pickText(root, selectors) {
    for (const sel of selectors) {
      const el = root.querySelector(sel);
      if (el && el.innerText && el.innerText.trim()) {
        return el.innerText.trim().replace(/\s+/g, " ");
      }
    }
    return "";
  }

  function extractJobsFromListPage() {
    const cardSelectors = [
      ".job-item-search-result",
      ".job-item-2",
      ".job-item",
      ".box-job-item",
      "[data-job-id]"
    ];
    const seenUrls = new Set();
    const cards = [];

    for (const sel of cardSelectors) {
      document.querySelectorAll(sel).forEach((el) => cards.push(el));
    }

    // Fallback: locate anchors pointing to TopCV job detail pages
    if (cards.length === 0) {
      document.querySelectorAll('a[href*="/viec-lam/"]').forEach((a) => {
        const container = a.closest("div[class*='job'], li, article") || a.parentElement;
        if (container) cards.push(container);
      });
    }

    const jobs = [];
    for (const card of cards) {
      const linkEl =
        card.querySelector('h3 a[href*="/viec-lam/"], .title a[href*="/viec-lam/"], a.job-title[href*="/viec-lam/"], a[href*="/viec-lam/"]');
      if (!linkEl) continue;

      const url = cleanUrl(linkEl.href);
      if (!url || !url.includes("/viec-lam/") || seenUrls.has(url)) continue;
      seenUrls.add(url);

      const title =
        pickText(card, ["h3.title a span", "h3.title a", ".job-title", "h3 a", "h3"]) ||
        linkEl.innerText.trim().replace(/\s+/g, " ");
      if (!title || title.length < 3) continue;

      const company = pickText(card, [".company-name", ".company a", ".company", "[class*='company']"]);
      const salary = pickText(card, [".title-salary", ".salary", "[class*='salary']"]);
      const location = pickText(card, [".address", ".city", ".location", "[class*='address']"]);
      const rawText = (card.innerText || "").trim().slice(0, 2500);

      jobs.push({
        platform: "topcv",
        title,
        company,
        url,
        salary,
        location,
        rawText,
        status: "new"
      });
    }
    return jobs;
  }

  function extractSingleDetailPage() {
    if (!location.pathname.includes("/viec-lam/")) return null;
    const title = pickText(document, ["h1.job-detail__info--title", "h1", ".job-title"]);
    if (!title) return null;
    const company = pickText(document, [".company-name-label", ".company-title", ".company-name"]);
    const salary = pickText(document, [".job-detail__info--section-content-value", ".box-main .salary"]);
    const locationText = pickText(document, [".job-detail__info--sections", ".box-address", ".address"]);
    const descEl = document.querySelector(".job-detail__information-detail, .job-data, main") || document.body;
    const rawText = (descEl.innerText || "").trim().slice(0, 6000);
    return {
      platform: "topcv",
      title,
      company,
      url: cleanUrl(location.href),
      salary,
      location: locationText,
      rawText,
      status: "saved"
    };
  }

  function ensureWidget() {
    let box = document.getElementById("leo-radar-sync-widget");
    if (box) return box;

    box = document.createElement("div");
    box.id = "leo-radar-sync-widget";
    box.style.cssText = [
      "position:fixed",
      "bottom:16px",
      "left:16px",
      "z-index:2147483647",
      "background:#110f1c",
      "color:#f5f3ff",
      "border:1px solid rgba(167,139,250,0.45)",
      "border-radius:12px",
      "padding:10px 14px",
      "font-family:-apple-system,BlinkMacSystemFont,Inter,Segoe UI,sans-serif",
      "font-size:12px",
      "box-shadow:0 12px 30px rgba(0,0,0,0.45)",
      "min-width:270px",
      "max-width:340px"
    ].join(";");

    box.innerHTML = `
      <div style="display:flex;justify-content:space-between;align-items:center;gap:8px;margin-bottom:6px;">
        <strong style="color:#a78bfa;">⚡ Leo Job Radar • TopCV Sync</strong>
        <div style="display:flex;gap:8px;align-items:center;">
          <a href="https://leonkenzo1997.github.io/job-radar-leo/" target="_blank" style="color:#a78bfa;text-decoration:none;font-weight:700;">Domain ↗</a>
          <a href="http://localhost:8090" target="_blank" style="color:#34d399;text-decoration:none;font-weight:700;">Local ↗</a>
        </div>
      </div>
      <div id="leo-sync-status" style="color:#d4d0e8;margin-bottom:8px;">Đang chờ quét trang TopCV...</div>
      <div style="display:flex;gap:6px;align-items:center;flex-wrap:wrap;">
        <button id="leo-sync-now" type="button" style="background:#7c3aed;color:#fff;border:none;border-radius:6px;padding:5px 10px;font-size:12px;font-weight:600;cursor:pointer;">🔄 Đồng bộ ngay</button>
        <label style="display:flex;align-items:center;gap:5px;cursor:pointer;color:#d4d0e8;">
          <input id="leo-auto-toggle" type="checkbox" />
          <span id="leo-auto-label">Auto-reload 10p</span>
        </label>
      </div>
    `;
    document.body.appendChild(box);

    document.getElementById("leo-sync-now").addEventListener("click", () => syncNow(true));
    const toggle = document.getElementById("leo-auto-toggle");
    toggle.checked = localStorage.getItem(STORAGE_KEY_AUTO) === "1";
    toggle.addEventListener("change", () => {
      localStorage.setItem(STORAGE_KEY_AUTO, toggle.checked ? "1" : "0");
      scheduleAutoReload();
    });
    scheduleAutoReload();
    return box;
  }

  function setStatus(html) {
    ensureWidget();
    const el = document.getElementById("leo-sync-status");
    if (el) el.innerHTML = html;
  }

  function scheduleAutoReload() {
    clearTimeout(autoTimer);
    clearInterval(countdownTimer);
    const enabled = localStorage.getItem(STORAGE_KEY_AUTO) === "1";
    const label = document.getElementById("leo-auto-label");
    if (!enabled) {
      if (label) label.textContent = "Auto-reload 10p";
      return;
    }
    nextReloadAt = Date.now() + RELOAD_INTERVAL_MS;
    autoTimer = setTimeout(() => {
      location.reload();
    }, RELOAD_INTERVAL_MS);

    countdownTimer = setInterval(() => {
      const rem = Math.max(0, Math.floor((nextReloadAt - Date.now()) / 1000));
      const m = String(Math.floor(rem / 60)).padStart(2, "0");
      const s = String(rem % 60).padStart(2, "0");
      if (label) label.textContent = `Tự tải lại (${m}:${s})`;
    }, 1000);
  }

  async function syncNow(manual = false) {
    ensureWidget();
    const jobs = extractJobsFromListPage();
    if (jobs.length > 0) {
      setStatus(`⏳ Đang gửi ${jobs.length} job TopCV về Leo Radar...`);
      try {
        const res = await fetch(API_BATCH, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ platform: "topcv", jobs })
        });
        const data = await res.json();
        setStatus(
          `✅ Đã quét <strong>${data.received}</strong> job TopCV → lưu <strong style="color:#34d399;">${data.imported}</strong> job TP.HCM/Remote (bỏ ${data.skippedNonHCM} ngoài KV).`
        );
      } catch (err) {
        setStatus(`⚠️ Không kết nối được <code>localhost:8090</code> (hãy bật server Leo Job Radar).`);
      }
      return;
    }

    const single = extractSingleDetailPage();
    if (single) {
      try {
        const res = await fetch(API_SINGLE, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(single)
        });
        const data = await res.json();
        if (data.job) {
          setStatus(`✅ Đã lưu JD chi tiết (${data.job.matchScore}% khớp CV).`);
          return;
        }
      } catch {
        setStatus(`⚠️ Không kết nối được <code>localhost:8090</code>.`);
        return;
      }
    }

    if (manual) {
      setStatus("ℹ️ Hãy mở trang danh sách việc làm TopCV (vd: Việc làm phù hợp hoặc Tìm việc làm) rồi bấm Đồng bộ.");
    } else {
      setStatus("ℹ️ Đang theo dõi tab TopCV (hãy mở danh sách việc làm để tự động lấy job).");
    }
  }

  // Wait briefly for TopCV dynamic job cards to render, then auto-sync
  setTimeout(() => syncNow(false), 1800);
})();
