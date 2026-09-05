# Mini SIEM — Kế hoạch phase 2–4

Ngày lập: 2026-09-05. Baseline: `caccc286718abbae20b29e0f68afa427bf02bb95`.
Phase 1 được đánh dấu hoàn thành trong `PHASE1_PROGRESS.md`. Các mục dưới đây là công việc tiếp theo, chưa phải tuyên bố đã triển khai.

## Phase 2 — Vận hành và kiểm thử đáng tin cậy

Thứ tự triển khai và tiêu chí nghiệm thu:

1. **2.1: CI và regression của phase 1.** Chạy trên pull request và push main: Go vet, test có PostgreSQL/Redis và migrations, race detector, frontend lint/typecheck/build, integration ingest → Redis → parser → Elasticsearch. Test phải thất bại khi thiếu dependency. Kiểm tra ingest thiếu/sai/key bị revoke trả 401; key hợp lệ trả 202. Build image chỉ chạy sau toàn bộ quality gate.
2. **2.2: Retention và khôi phục.** Thiết kế ILM phù hợp index `normalized_events` hiện tại; migration sang alias phải giữ khả năng tìm dữ liệu cũ. Chọn retention theo nhu cầu lưu giữ; có dry-run và hướng dẫn rollback. PostgreSQL backup dùng format custom, checksum, và restore thử sang database trống. Elasticsearch snapshot phải được thử restore; backup PostgreSQL không thay thế backup event.
3. **2.3: Retry, replay và idempotency.** Kiểm tra crash sau index nhưng trước ACK; replay cùng event không tạo thêm event/alert. Test backoff, giới hạn retry, DLQ và phục hồi sau ES/Redis gián đoạn. Giữ event ID ổn định qua retry.
4. **2.4: Tìm kiếm và KPI vận hành.** Filter theo thời gian, host, severity, source; phân trang có giới hạn và thứ tự ổn định. KPI: ingest/error rate, parser lag, pending/retry/DLQ, indexing latency. Có test dữ liệu rỗng, boundary thời gian và quyền viewer/analyst/admin.
5. **2.5: Pilot.** Kiểm tra enrollment/sync từ Linux, Windows và app log; E2E login → events → alerts → cases; chạy soak 24–72 giờ và lưu báo cáo loss/lag, tài nguyên, restart/recovery. Chốt ngân sách tải và SLO dựa trên phép đo trước khi tuyên bố đạt.

Điều kiện sang phase 3: các mục 2.1–2.5 có bằng chứng test, CI xanh, restore drill thành công, lỗi ingest/replay nghiêm trọng đã xử lý.

## Phase 3 — Nâng cấp phát hiện

1. **3.1: IOC và enrichment.** Schema rõ cho IP/domain/hash, provenance và thời điểm cập nhật; provider interface, cache TTL, timeout, quota. Kiểm thử bằng feed fixture; provider lỗi không chặn ingest.
2. **3.2: Threat intelligence/GeoIP.** Cấu hình nguồn và giấy phép; secret bên ngoài source code. Giữ nguyên dữ liệu gốc, ghi enrichment riêng. Test cache hit/miss, IP private, IPv6, dữ liệu hết hạn và timeout.
3. **3.3: Correlation.** Chuỗi SSH failure → success → privilege escalation theo host/user và time window. Test sự kiện sai thứ tự, đến muộn, duplicate, hết cửa sổ và restart consumer; tránh ghép khác host/user.
4. **3.4: Đánh giá detection.** Bộ dữ liệu benign/attack có nhãn, mapping ATT&CK theo rule, đo precision/recall và false positive; lưu rule version, event evidence và lý do severity.

Điều kiện sang phase 4: enrichment không làm mất log; correlation có test restart/dedup; có kết quả đánh giá trên fixture và dữ liệu pilot.

## Phase 4 — SOC workflow và SOAR

1. **4.1: Case nâng cao.** Assignee, disposition, evidence, timeline, export CSV/report; test quyền truy cập và audit cho mỗi mutation.
2. **4.2: Playbook dry-run.** Trigger/condition/action có version; timeout, retry, trạng thái execution và idempotency. Ban đầu chỉ mô phỏng bằng adapter giả.
3. **4.3: Approval và response.** Action như block IP/disable account cần analyst có quyền phê duyệt; allowlist, TTL/undo và audit đầy đủ. Test từ chối quyền, hết hạn phê duyệt, chạy trùng, timeout và rollback. Chỉ bật adapter thật sau thử nghiệm lab và phê duyệt cấu hình.
4. **4.4: Release.** E2E toàn luồng alert → case → approval → action → audit; runbook cài đặt, upgrade, backup/restore; báo cáo kết quả và giới hạn vận hành.

## Các phát hiện cần xử lý ở mốc 2.1

- Integration ingest hiện không truyền API key.
- Middleware route hiện gọi rate limit trước auth, trong khi rate limit cần hostname do auth gán.
- Test apikey và ratelimit cần database/Redis thật; job test-backend hiện chưa cung cấp chúng.
- Test Redis hiện gọi FlushDB: chỉ được chạy với Redis dành riêng cho test.
- Workflow hiện chưa chạy trên pull request và chưa có frontend quality gate.
- Chưa có bằng chứng CI xanh hoặc soak test chỉ từ trạng thái hoàn thành trong tracker.
- Cần mốc bảo mật riêng trước pilot: `hashSHA256` hiện dùng XOR thay vì SHA-256; phải chuyển sang hash mật mã chuẩn kèm kế hoạch rotate/reissue key đang tồn tại. Rà soát việc ràng buộc hostname/agent trong payload với asset của key và quyền gọi enrollment. PR CI chưa giải quyết các thay đổi tương thích credential này.

## Cách chia pull request

Một PR đầu cho 2.1 và roadmap; các PR 2.2–2.5 tách riêng theo tiêu chí trên. Phase 3/4 là roadmap đã xác định phạm vi, không gộp vào PR CI. Mỗi PR ghi test đã chạy, kết quả và mục còn thiếu; không đánh dấu xong dựa trên tài liệu đơn thuần.
