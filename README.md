# Leo Job Radar — 7-Platform IT Job Aggregator & CV Matcher

Live Web App (GitHub Pages): **https://leonkenzo1997.github.io/job-radar-leo/**

Hệ thống tìm kiếm, tổng hợp và chấm điểm độ khớp CV (0–100%) cho **Trương Tăng Quang (Leo)** trên 7 nền tảng tuyển dụng IT tại **TP.HCM & Remote**:
**LinkedIn • ITviec • TopDev • TopCV • Threads • Facebook • Zalo**.

## Tính năng chính
- **Live Automated Scraping (Go Backend & GitHub Actions)**: Quét định kỳ trên **LinkedIn**, **ITviec (TP.HCM)**, **TopDev** và tự động lọc giữ lại các job **TP.HCM hoặc Remote**.
- **Chấm điểm khớp CV (0–100%)**: Phân tích kỹ năng theo 4 tầng từ CV (`Golang`, `Node.js/NestJS`, `TypeScript`, `AI-Native LLM/RAG/MCP`, `PostgreSQL/Kafka/AKS`, `ReactJS/Flutter`), gợi ý hướng CV phù hợp, chọn 2 flagship projects (*COSMO8*, *RMN*, *GiaPhuoc-Express*, *Busk 3PL*) và soạn sẵn lời chào HR (Tiếng Việt / Tiếng Anh).
- **Radar 7 Nền Tảng & Bookmarklet 1-Click**: Hệ thống Deep-Link + Google X-Ray theo thời gian thực cho **Threads, Facebook, Zalo, TopCV, LinkedIn, ITviec, TopDev**.
- **Chrome Extension đồng bộ TopCV**: Tại thư mục [`chrome-extension/`](./chrome-extension), tự động trích xuất job từ tài khoản TopCV đang đăng nhập (kể cả chế độ auto-reload mỗi 10 phút) đẩy về `localhost:8090`.
- **Dual-Mode Architecture**: Hoạt động hoàn chỉnh cả trên GitHub Pages (`https://leonkenzo1997.github.io/job-radar-leo/`) lẫn khi chạy local server (`http://localhost:8090`).

## Chạy Local Server
```bash
./start.sh
# hoặc
go run . -port 8090
```
