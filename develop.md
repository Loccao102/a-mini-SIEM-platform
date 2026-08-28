# SIEM Platform Development Roadmap

Bản định hướng và kế hoạch phát triển mở rộng hệ thống Mini SIEM Platform lên tầm Enterprise SIEM/SOAR.

---

## 🚀 6 Hướng phát triển chiến lược

### 1. Threat Intelligence & GeoIP Enrichment (Làm giàu dữ liệu sự kiện)

- **GeoIP Lookup**: Tự động xác định Quốc gia, Thành phố, Cờ quốc gia (Country Flag) từ `src_ip`.
- **Threat Intel Feeds**: Tra cứu danh tiếng IP (`src_ip`) đối soát với dữ liệu AbuseIPDB, VirusTotal, Tor Exit Nodes, C2 Botnet. Gắn nhãn độc hại (`is_malicious`, `reputation_score`) và tự động nâng Severity lên `CRITICAL`.
- **Còn thiếu**: GeoIP/threat intel hiện vẫn là mapping mô phỏng trong parser; chưa có database GeoIP, feed cập nhật, cache, timeout, retry hoặc provenance cho kết quả.

### 2. Multi-Stage Attack Chain Correlation Rules (Chuỗi tấn công liên hoàn)

- Phát hiện kịch bản tấn công theo chuỗi hành vi (Kill Chain):
  `SSH Brute Force` ➔ `SSH Login Success` ➔ `Sudo Privilege Escalation`.
- Tự động kích hoạt Alert tổng hợp `CRITICAL: Full System Compromise Detected`.
- **Còn thiếu**: Rule engine hiện mới đếm theo một rule/entity trong Redis; chưa có state machine theo chuỗi, thứ tự thời gian, correlation window đa sự kiện hoặc alert tổng hợp.

### 3. SOAR - Automated Response & Playbooks (Phản ứng sự cố tự động)

- **1-Click / Auto Block Attacker IP**: Nút bấm trên giao diện Alert để khóa IP qua `iptables`, `firewall` hoặc Cloudflare WAF API.
- **Auto Account Lockout**: Tự động vô hiệu hóa tài khoản khi phát hiện hành vi leo thang đặc quyền bất thường.
- **Còn thiếu**: Chưa có playbook model, approval gate, dry-run, adapter firewall/Cloudflare, idempotency, rollback, audit kết quả hoặc giới hạn quyền thực thi.

### 4. SOC Case Management & Audit Trail (Quản lý incident & Nhật ký thao tác)

- **Timeline & Case Notes**: Ghi chép tiến trình điều tra của Analyst, gắn nhãn `False Positive`, `True Positive`, `Resolved`.
- **Audit Logs**: Ghi nhật ký thao tác người dùng (ai sửa luật, ai vô hiệu hóa tài khoản, ai đóng alert).
- **Đã triển khai**: Case lifecycle `open/investigating/resolved/closed`, notes, liên kết alert, timeline và audit log atomic cho case, alert, rule, user, asset.
- **Còn thiếu**: Chưa có phân quyền theo case/tenant, chỉnh sửa hoặc xóa note, phân công user qua UI, liên kết case trực tiếp từ màn hình alert, export báo cáo và pagination/audit retention.

### 5. High Throughput & Batch Pipeline Scaling (Tối ưu hóa hiệu năng)

- **Batch Bulk Indexing**: Gom log ghi theo lô sang Elasticsearch (`_bulk` API) tăng tốc độ ghi > 10.000 events/sec.
- **Go Worker Pool**: Mở rộng Parser Consumer xử lý đa luồng song song trên CPU multi-core.
- **Đã triển khai**: Elasticsearch NDJSON bulk indexing có kiểm tra lỗi từng item; parser dùng batch configurable và worker pool giới hạn qua `PARSER_BATCH_SIZE`/`PARSER_WORKERS`, chỉ ACK sau khi xử lý thành công.
- **Còn thiếu**: Chưa có benchmark throughput/latency, retry backoff theo item, dead-letter stream, reclaim pending message sau crash của consumer khác, flush interval và metrics/alerts về queue lag.

### 6. Advanced Analytics & Chart Visualizations (Trực quan hóa chỉ số)

- Biểu đồ xu hướng sự kiện theo mốc thời gian (Event Trend Line Chart).
- Biểu đồ phân bố loại tấn công (SQLi vs SSH vs LFI vs Scanners).
- Biểu đồ Top 5 IP tấn công nhiều nhất (Top Attacking IPs).
- Bản đồ nguồn tấn công GeoIP và xuất báo cáo an ninh.
- **Đã triển khai một phần**: Biểu đồ hiện lấy aggregation thực từ Elasticsearch cho severity, category, attacker IP, targeted user và timeline 24 giờ.
- **Còn thiếu**: Chưa có bản đồ GeoIP thực, bộ lọc thời gian/asset/source, drill-down từ chart về event, export CSV/PDF và kiểm thử trực quan responsive.

---

## Xử lý log info lặp

### Phương án đề xuất: lưu đầy đủ, gom nhóm khi đọc

Không nên xóa log `info` ngay lúc ingest vì sẽ mất bằng chứng điều tra và làm sai số liệu audit. Thay vào đó:

1. Giữ nguyên raw event trong Redis/Elasticsearch để bảo toàn dữ liệu.
2. Ghi nhận fingerprint ổn định theo `hostname + source_type + normalized_message` trong cửa sổ 5 phút, không dùng timestamp chính xác trong fingerprint.
3. API Log Explorer trả về nhóm đại diện gồm `count`, `first_seen`, `last_seen`, `sample_event_id`; UI hiển thị một dòng và cho phép mở các event gốc.
4. Mặc định gom các event `info` lặp; không gom `high`/`critical` nếu khác event ID, và vẫn tăng alert/correlation theo chính sách riêng.
5. Đặt giới hạn nhóm và TTL Redis, đo `dedup_ratio`, queue lag, API latency trước/sau để tránh giảm khả năng phát hiện.

### Tiêu chí hoàn thành

- Một chuỗi log giống nhau không còn chiếm nhiều dòng trên Log Explorer nhưng vẫn truy xuất được event gốc.
- Tổng số raw event và alert không bị thay đổi ngoài chính sách đã công bố.
- Có test cho khác hostname, khác source, khác cửa sổ 5 phút và event severity cao.
- Có benchmark trước/sau và dashboard hiển thị tỷ lệ gom nhóm.

## Các hạng mục nền tảng còn thiếu

- **Bảo mật ingest**: API key/mTLS cho agent, rate limiting, request size limit, TLS và allowlist mạng.
- **Quan sát hệ thống**: metrics Prometheus/OpenTelemetry, structured logging, trace ID, queue lag alert và health/readiness tách biệt.
- **Độ tin cậy dữ liệu**: dead-letter queue, retry có backoff, idempotency end-to-end, replay tool và migration runner có version tracking.
- **Quản trị production**: secret manager, Elasticsearch security, backup/restore PostgreSQL, retention/ILM, resource limits và CI quality gates.
- **Kiểm thử**: integration test hoàn chỉnh cho Logstash, API, Redis, parser, Elasticsearch; E2E Playwright cho login, asset, alert và case.
- **Trải nghiệm vận hành**: pipeline status trên UI, tìm kiếm nâng cao, pagination, export, empty/error/loading states và tài liệu agent đầy đủ.

---
