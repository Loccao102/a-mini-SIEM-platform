# SIEM Platform Development Roadmap

Bản định hướng và kế hoạch phát triển hệ thống Mini SIEM Platform theo hướng production-ready, nhưng vẫn phù hợp với quy mô đồ án tốt nghiệp / PoC SIEM thực tế.

---

## 1. Tình trạng hiện tại của hệ thống

### Core đã có

Hệ thống hiện tại đã hình thành được một stack SIEM cơ bản với các thành phần chính sau:

- `Elastic Agent + Fleet Server`: thu thập log từ nhiều host/service theo policy tập trung.
- `Logstash`: nhận log từ agent và forward sang backend ingest.
- `Backend API`: xử lý auth, ingest, asset, rule, alert, analytics, cases.
- `Redis Stream`: hàng đợi trung gian cho raw log.
- `Parser`: chuyển raw log thành normalized event.
- `Elasticsearch`: lưu trữ event và dữ liệu analytics.
- `PostgreSQL`: lưu asset, rule, alert, user, case, log_sources.
- `Frontend Next.js`: dashboard tổng quan, alert, rule, events, assets.
- `Deduplication`: gom nhóm event lặp để giảm noise và tăng hiệu quả điều tra.
- `Case management`: lifecycle case, notes, timeline, audit trail.

### Dễ thấy: project đã có core SIEM thực chứ chưa chỉ là demo

- Có pipeline log thực từ source -> ingest -> queue -> parser -> ES.
- Có asset inventory và log_source tracking.
- Có user roles và auth.
- Có rule engine và alert management.
- Có Fleet-style architecture nâng cấp từ Filebeat/Winlogbeat sang Elastic Agent.
- Có quản lý case và audit cơ bản.

Vì vậy, mục tiêu phát triển không còn là “xây thêm nhiều chức năng mới” mà là làm cho core này ổn định, an toàn và triển khai được như một hệ thống SIEM thật.

---

## 2. Core cần làm ngay: ưu tiên thực tế

### 2.1. Bảo mật core pipeline

- **Cần làm**: API key / mTLS / allowlist cho ingest endpoint.
- **Cần làm**: rate limiting, request size limit, TLS cho agent.
- **Cần làm**: xác thực agent theo host, agent_id, source_type.
- **Vì sao**: hiện `POST /api/v1/ingest` vẫn là public local-demo style, chưa an toàn cho môi trường multi-client.

### 2.2. Tăng độ tin cậy dữ liệu

- **Cần làm**: dead-letter queue cho parser / ingest thất bại.
- **Cần làm**: retry backoff và idempotency.
- **Cần làm**: replay tool / reprocessing tool.
- **Cần làm**: soft-delete hoặc retention policy cho raw logs.
- **Vì sao**: SIEM phải chịu được loss/delay, không được mất evidence khi consumer lỗi.

### 2.3. Quan sát hệ thống

- **Cần làm**: metrics cho queue lag, parser lag, ingest latency, ES indexing latency.
- **Cần làm**: health/readiness riêng cho từng service.
- **Cần làm**: structured logging + trace ID.
- **Cần làm**: alert cho pipeline health (redis đầy, parser chậm, ES lỗi).
- **Vì sao**: system là pipeline phân tán, nếu không quan sát thì không biết đâu là điểm nghẽn.

### 2.4. Fleet/Agent management hơn thực tế

- **Cần làm**: policy template rõ ràng cho Linux, Windows, Docker, app logs.
- **Cần làm**: auto-enrollment validation và tag mapping.
- **Cần làm**: check host status / last_seen / unhealthy agent.
- **Cần làm**: policy versioning và deploy lịch sử.
- **Vì sao**: hiện Fleet đã có cấu hình và endpoint đồng bộ asset, nhưng chưa hoàn chỉnh như môi trường thực tế.

### 2.5. Dữ liệu và retention

- **Cần làm**: ILM / retention policy cho Elasticsearch.
- **Cần làm**: backup/restore cho PostgreSQL.
- **Cần làm**: log archival và compliance rule.
- **Vì sao**: SIEM phải có cơ chế lưu giữ sự kiện theo thời hạn và tuân thủ.

### 2.6. Kiểm thử và CI

- **Cần làm**: integration test cho ingest + redis + parser + elasticsearch.
- **Cần làm**: test cho Fleet enrollment và asset synchronization.
- **Cần làm**: E2E test cho login, events, alerts, cases.
- **Cần làm**: CI quality gate cho backend/frontend.
- **Vì sao**: hệ thống đang có nhiều thành phần phụ thuộc lẫn nhau, cần automation để giảm bug.

---

## 3. Hướng phát triển addon mạnh nhất

### 3.1. Threat Intelligence & GeoIP

**Mục tiêu**: làm giàu sự kiện bằng thông tin đen/đỏ, tăng accuracy cảnh báo.

- GeoIP lookup cho `src_ip`.
- Detect malicious IP từ AbuseIPDB / VirusTotal / Tor / C2 feed.
- Tự động nâng severity khi IP độc hại.
- Gắn tag `reputation_score`, `is_malicious`, `country`, `city`.
- Cache kết quả để hạn chế rate limit.

**Ưu tiên**: High
**Lý do**: tăng giá trị cảnh báo và giúp SOC thao tác nhanh hơn.

### 3.2. SOAR / Automated Response

**Mục tiêu**: không chỉ phát hiện, mà còn phản ứng tự động.

- Block IP bằng firewall / WAF / cloud API.
- Disable account khi có activity đổi đặc quyền bất thường.
- Runbook / playbook cho từng loại alert.
- Approval gate cho hành động nguy hiểm.
- Audit log cho mỗi action đã thực thi.

**Ưu tiên**: Medium-High
**Lý do**: rất hấp dẫn nhưng chỉ nên làm sau khi core pipeline và alerting đã ổn.

### 3.3. Correlation / Attack Chain

**Mục tiêu**: phát hiện chuỗi tấn công thay vì phát hiện đơn lẻ.

- `SSH Brute Force` -> `SSH Login Success` -> `Privilege Escalation`
- Mở rộng từ rule đơn lẻ sang multi-event correlation.
- Dùng state machine hoặc time-window correlation.
- Tổng hợp alert theo incident chain.

**Ưu tiên**: High
**Lý do**: đây là bước tiến từ SIEM cơ bản lên SOC full-scale.

### 3.4. Case Management nâng cao

**Mục tiêu**: chuyển alert thành workflow điều tra thực tế.

- Gắn alert -> case.
- Assign analyst.
- Timeline notes, evidence, status, resolution.
- Đánh dấu `False Positive`, `True Positive`, `Resolved`.
- Export report / PDF / CSV.

**Ưu tiên**: Medium
**Lý do**: core case management đã có, cần làm đi sâu để dùng hiệu quả.

### 3.5. Advanced Analytics & Visualization

**Mục tiêu**: báo cáo và drill-down tốt hơn.

- Trend chart theo thời gian.
- Top attacking IP / user / source type.
- Geo map.
- Facets và drill-down trên dashboard.
- Tự động report theo ngày / tuần.

**Ưu tiên**: Medium
**Lý do**: đã có nền analytics cơ bản, cần nâng cấp trải nghiệm observability.

---

## 4. Ưu tiên phát triển theo thời gian

### Giai đoạn 1: Core stabilization (0-2 tuần)

- Bảo mật ingest
- Rate limit + auth
- Queue health + dead-letter
- Pipeline observability
- Validation cho Fleet enrollment
- Dữ liệu đầu vào được xác thực và có baseline retention

**Mục tiêu**: hệ thống đạt mức “chạy được thực tế” cho môi trường pilot / lab production, không phải chỉ là demo. Khi core này ổn, mới mở rộng thêm tính năng nâng cao.

**Tiêu chí để coi là đã đạt level chạy thực tế**

Hệ thống chỉ được xem là “đủ mức chạy thực tế” khi thỏa mãn đủ các điều kiện sau:

- Agent/host có thể enroll và hoạt động với Fleet mà không cần xử lý thủ công từng lần.
- Ingest endpoint yêu cầu auth hợp lệ; có giới hạn tần suất và kiểm soát kích thước payload.
- Dữ liệu từ ít nhất 2-3 source khác nhau (ví dụ: Linux, Windows, app logs) có thể vào pipeline mà không lỗi định kỳ.
- Queue không bị đầy do consumer lỗi; có dead-letter và replay tool khi parser/ingest thất bại.
- Có health check riêng cho từng service: ingest, parser, redis, elasticsearch, fleet-server.
- Có metrics cơ bản về ingest latency, queue lag, parser lag, ES indexing latency.
- Có alert khi pipeline chậm hoặc mất dữ liệu.
- Có cơ chế ghi log theo cấu trúc và trace ID để debug dễ hơn.
- Các event đã được normalize đủ để dashboard và rule engine hoạt động với dữ liệu đáng tin cậy.
- Hệ thống có thể chạy liên tục trong ít nhất 24-72 giờ mà không phải restart thủ công.

**Không nên coi là “đã đạt chuẩn” nếu**

- Vẫn đang cắm data bằng cách gọi API bằng curl thủ công mà không có auth/ACL.
- Pipeline còn phụ thuộc heavy vào manual recovery khi parser hoặc redis lỗi.
- Chưa có check health và monitoring cơ bản.
- Fleet enrollment chưa được validate qua nhiều host/service.
- Chưa có dead-letter/retry hoặc bắt lỗi rõ ràng.

> Nói ngắn gọn: “đạt level chạy thực tế” là lúc hệ thống có thể nhận, xử lý, giám sát và phục hồi dữ liệu đáng tin cậy trong môi trường pilot, chứ chưa phải lúc đáp ứng toàn bộ tất cả yêu cầu SOC/Soar/Threat Intel nâng cao.

### Giai đoạn 2: Operational hardening (2-4 tuần)

- ILM retention
- Retry / replay / idempotency
- Alert dedup tốt hơn
- Search + filter nâng cao
- KPI dashboard cho pipeline

**Mục tiêu**: hệ thống dễ vận hành trong môi trường nhiều host.

### Giai đoạn 3: Detection enhancement (4-8 tuần)

- Threat Intel & GeoIP
- Correlation chain
- IOC extraction
- Alert enrichment

**Mục tiêu**: chuyển từ dữ liệu raw sang tri thức an ninh.

### Giai đoạn 4: SOAR + SOC workflow (8-12 tuần)

- Playbook engine
- Auto-response
- Case workflow nâng cao
- Report automation
- Analyst action audit

**Mục tiêu**: hệ thống bắt đầu có tính tự động hóa như SOAR thực thụ.

---

## 5. Khuyến nghị triển khai hợp lý cho đồ án / mini-SIEM

### Nên ưu tiên theo thứ tự sau:

1. Core hardening
2. Fleet policy maturity
3. Dedup + filter + search
4. Threat intel
5. Attack chain correlation
6. SOAR playbooks
7. Dashboard và report nâng cao

### Không nên làm thẳng addon SOAR trước khi core ổn

SOAR rất mạnh nhưng nếu pipeline log chưa chắc chắn, các playbook sẽ làm tăng rủi ro và việc triển khai sẽ không đáng tin. Nếu hệ thống core không ổn, dữ liệu đầu vào sẽ thiếu tin cậy.

---

## 6. Kết luận

Hệ thống hiện tại đã có một lớp nền SIEM đầy đủ: collect, normalize, analyze, alert, case, dashboard, asset management, Fleet-based agent architecture. Đây là nền tảng tốt để phát triển lên mức enterprise.

Nhưng điểm cần tập trung không phải là “thêm nhiều addon” ngay, mà là:

- ổn định core,
- tăng bảo mật,
- tăng độ tin cậy,
- tăng khả năng vận hành,
- rồi mới triển khai các addon như Threat Intel, Correlation, SOAR.

Lý tưởng cho đồ án / dự án mini-SIEM thực tế là:

- core SIEM phải chạy ổn định trước,
- các addon làm sau nhưng theo roadmap rõ ràng và có thứ tự ưu tiên.

---
