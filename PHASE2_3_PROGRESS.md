# Giai đoạn 2 & 3: Operational Hardening & Threat Detection - Progress Tracker

**Bắt đầu**: 2026-09-05  
**Trạng thái**: ✅ Hoàn thành (Nghiệm thu toàn bộ tiêu chí Phase 2 & Phase 3)

---

## ✅ Giai đoạn 2: Vận hành và kiểm thử đáng tin cậy

### 2.1: CI và regression của phase 1 ✅
- [x] Chạy CI trên pull request và push: `Go vet`, race detector, `backend` isolated PostgreSQL/Redis tests.
- [x] Frontend quality gate: ESLint, TypeScript check (`tsc --noEmit`), Next.js production build.
- [x] Tích hợp kiểm thử ingest: từ chối request thiếu key (401), key không hợp lệ (401), key bị thu hồi (401), cấp phát key thành công (202).
- [x] Từ chối giả mạo hostname và agent chưa enroll: `POST /api/v1/ingest` so sánh `X-Hostname` và `log_sources` (403).
- [x] Yêu cầu quyền admin cho `POST /api/v1/fleet/agents`.
- [x] Chuyển đổi mã băm API key sang chuẩn mật mã SHA-256 (`sha256:<digest>`), vô hiệu hóa các key XOR cũ.

### 2.2: Retention và khôi phục ✅
- [x] Cấu hình Elasticsearch ILM policy `siem-events-retention` theo số ngày lưu trữ `EVENT_RETENTION_DAYS`.
- [x] Định tuyến ghi log theo index ngày: `siem-events-YYYY.MM.DD`.
- [x] Hỗ trợ tìm kiếm kết hợp `normalized_events,siem-events-*` với `ignore_unavailable=true`.
- [x] Kịch bản diễn tập khôi phục `scripts/restore-drill.mjs`:
  - PostgreSQL custom dump (`pg_dump -Fc`), xác thực SHA-256 checksum, khôi phục vào DB riêng biệt và kiểm tra số lượng bản ghi `assets`.
  - Elasticsearch snapshot repository và restore vào index riêng biệt, kiểm tra khớp số lượng events.
- [x] Cấu hình Docker volume `elasticsearch_snapshots` và `path.repo: /snapshots`.

### 2.3: Retry, replay và idempotency ✅
- [x] Bảng `rule_event_receipts` với khóa chính `(rule_id, event_id)` lưu vết tiếp nhận sự kiện.
- [x] Atomic locking với `pg_advisory_xact_lock` ngăn race condition khi nhiều consumer xử lý song song.
- [x] Idempotent alert aggregation: Replay sự kiện đã ghi nhận không tạo alert mới hoặc tăng sai số đếm occurrences.
- [x] Test tích hợp `TestConcurrentReplayIsIdempotentAcrossEngineRestart` xác nhận tính toàn vẹn qua các lần khởi động lại.

### 2.4: Tìm kiếm và KPI vận hành ✅
- [x] Module `eventSearchQuery`: Giới hạn tối đa 10,000 bản ghi, kiểm soát kích thước trang (25, 50, 100).
- [x] Giới hạn độ dài query string tối đa 512 ký tự, ngăn chặn DoS bằng regex/wildcard quá dài.
- [x] Kiểm tra tính hợp lệ của mốc thời gian `from <= to` theo chuẩn RFC3339.
- [x] Sắp xếp ổn định với tiebreaker `event_id.keyword asc`.
- [x] Endpoint `POST /api/v1/assets/{id}/keys` cấp API key cho asset kèm ghi nhận `audit_logs`.

### 2.5: Pilot & E2E Testing ✅
- [x] Cấu hình Playwright `frontend/playwright.config.ts`.
- [x] Kịch bản E2E `frontend/e2e/workflow.spec.ts`: Đăng nhập, tải dữ liệu thật trên các màn hình `/events`, `/alerts`, `/cases` mà không gặp lỗi API `>= 400`.

---

## ✅ Giai đoạn 3: Nâng cấp phát hiện an ninh

### 3.1 & 3.2: IOC và Threat Intelligence ✅
- [x] Module `backend/internal/intel`: Trích xuất IOC từ log (`ip`, `domain`, `sha256`).
- [x] Định dạng chuẩn `Observation` kèm `source`, `license`, `updated_at`, `expires_at`, `malicious`, vị trí địa lý.
- [x] Provider interface hỗ trợ nguồn Feed file cục bộ và HTTP Provider có token.
- [x] Bounded cache và cơ chế giới hạn tần suất quota (`MaxPerMinute`) tránh quá tải API bên ngoài.
- [x] Cơ chế timeout độc lập (500ms), lỗi provider hoặc quá quota không bao giờ làm gián đoạn hay drop log tại pipeline ingest.
- [x] Phân tích IP chính xác bằng `netip`: Phân định rõ ràng Private LAN / Loopback / Link-local vs Public IP.

### 3.3: Correlation & Attack Chain Detection ✅
- [x] Động cơ tương quan `backend/internal/correlation`: Phát hiện chuỗi tấn công:
  `SSH Brute Force (≥3 failures)` → `SSH Login Success` → `Privilege Escalation`
- [x] Khung thời gian trượt 10 phút, phân vùng nghiêm ngặt theo từng cặp `(hostname, username)`.
- [x] Migration `008_security_detection.sql`: Bảng `detection_events` và `detection_findings`.
- [x] Ánh xạ kỹ thuật MITRE ATT&CK: `T1110` (Brute Force), `T1078` (Valid Accounts), `T1548.003` (Sudo / Privilege Escalation).
- [x] Endpoint `GET /api/v1/detection/findings` cho phép SOC analyst điều tra chuỗi tấn công kèm bằng chứng JSON chi tiết.
- [x] Xử lý sự kiện sai thứ tự, sự kiện đến muộn (bounded lateness) và loại trừ duplicate.

