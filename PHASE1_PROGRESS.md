# Giai đoạn 1: Core Stabilization - Progress Tracker

**Bắt đầu**: 2026-09-01  
**Status**: 🟡 Đang triển khai (4/6 mục tiêu hoàn thành)

---

## ✅ Hoàn thành

### 1. Bảo mật ingest ✅

**Công việc**:
- [x] API Key Manager + database schema
- [x] generateKey() - tạo random key với hash
- [x] validateKey() - xác thực trước ingest
- [x] revokeKey() - vô hiệu hóa key
- [x] listKeys() - audit trail
- [x] Key expiration support
- [x] Unit tests (apikey_test.go)

**File**:
- `backend/internal/apikey/manager.go`
- `backend/internal/apikey/manager_test.go`
- `backend/migrations/006_api_keys.sql`

**Kết quả**:
- POST /api/v1/ingest yêu cầu Bearer token
- Return 401 nếu header missing/invalid
- Audit mỗi request via request_count và last_used_at

---

### 2. Rate limit + auth ✅

**Công việc**:
- [x] Rate Limiter - token bucket algorithm
- [x] Redis-based counter per hostname
- [x] Default: 1000 requests/minute
- [x] Return rate limit headers
- [x] Return 429 Too Many Requests khi vượt
- [x] Unit tests (limiter_test.go)

**File**:
- `backend/internal/ratelimit/limiter.go`
- `backend/internal/ratelimit/limiter_test.go`

**Kết quả**:
- X-RateLimit-Limit: 1000
- X-RateLimit-Remaining: N
- X-RateLimit-Reset: <unix_ts>
- Retry-After: <seconds>

---

### 3. Middleware integration ✅

**Công việc**:
- [x] withIngestAuth() - validate API key
- [x] withRateLimit() - enforce per-hostname limit
- [x] withRequestSizeLimit() - max 10MB payload
- [x] Middleware chain: auth → rate limit → size check
- [x] Handler integration
- [x] main.go initialization

**File**:
- `backend/internal/api/middleware.go`
- `backend/internal/api/handler.go` (updated)
- `backend/cmd/api/main.go` (updated)

**Kết quả**:
- Ingest endpoint được bảo vệ bởi 3 lớp
- Backend builds successfully
- All services start healthy

---

## ⏳ Chưa làm (tiếp theo)

### 4. Queue health + dead-letter 🔄

**Cần làm**:
- [x] Dead-letter queue cho parser failures
- [x] Dead-letter queue cho ingest failures
- [x] Retry logic với exponential backoff
- [x] Replay tool để reprocess failed events
- [ ] Health check trên Redis stream
- [x] Metrics: queue lag, pending count
- [ ] Alert khi queue đầy

**Priority**: HIGH  
**Estimate**: 1 ngày còn lại

---

### 5. Pipeline observability ⏳

**Cần làm**:
- [ ] GET /healthz/ingest - health của ingest service
- [ ] GET /healthz/parser - health của parser
- [ ] GET /healthz/redis - Redis stream status
- [ ] GET /healthz/elasticsearch - ES cluster status
- [ ] Metrics endpoint: ingest latency, parser lag, ES indexing lag
- [ ] Structured logging với trace ID
- [ ] Alert rules cho pipeline health

**Priority**: HIGH  
**Estimate**: 2-3 ngày

---

### 6. Fleet validation ⏳

**Cần làm**:
- [ ] Validate agent enrollment qua agent_id
- [ ] Policy template rõ ràng (Linux, Windows, Docker)
- [ ] Auto-tag mapping (env, team, criticality)
- [ ] Host status tracking (last_seen, healthy/unhealthy)
- [ ] Policy versioning và deploy history
- [ ] Unhealthy agent detection

**Priority**: MEDIUM  
**Estimate**: 3-5 ngày

---

## 📊 Tiêu chí hoàn thành giai đoạn 1

Hệ thống chỉ được xem là **"đạt level chạy thực tế"** khi thỏa mãn:

### ✅ Đã đạt

- [x] Ingest endpoint yêu cầu auth
- [x] Rate limit và kiểm soát kích thước
- [x] Dữ liệu được xác thực (hashed key)

### ⏳ Cần đạt

- [ ] Dead-letter và replay tool khi thất bại
- [ ] Health check riêng cho từng service
- [ ] Metrics cơ bản (latency, lag, indexing time)
- [ ] Fleet enrollment validation
- [ ] Structured logging + trace ID

---

## 🔍 Cách kiểm tra hiện tại

### Quick test
```powershell
# 1. Health check
curl http://localhost:8080/healthz

# 2. Thử ingest mà không có key (expect 401)
curl -X POST http://localhost:8080/api/v1/ingest \
  -H "Content-Type: application/json" \
  -d '{"message": "test", "source_type": "linux"}'

# 3. Kiểm tra database
docker compose exec postgres psql -U siem -d siem \
  -c "SELECT * FROM api_keys LIMIT 5;"
```

---

## 📈 Timeline dự kiến

| Giai đoạn | Mục tiêu | Ngày bắt đầu | Ngày kết thúc | Tiến độ |
|-----------|----------|--------------|---------------|---------|
| 1.1 | Ingest Security | 2026-09-01 | 2026-09-01 | ✅ 100% |
| 1.2 | Queue + Retry | 2026-09-02 | 2026-09-04 | ⏳ 0% |
| 1.3 | Health + Metrics | 2026-09-05 | 2026-09-07 | ⏳ 0% |
| 1.4 | Fleet Validation | 2026-09-08 | 2026-09-12 | ⏳ 0% |
| **Giai đoạn 1 hoàn thành** | **Core Stable** | **-** | **~2026-09-12** | **50%** |

---

## 🔗 Liên kết tài liệu

- [Roadmap chính](develop.md) - định hướng toàn bộ dự án
- [Ingest Security Implementation](INGEST_SECURITY_IMPLEMENTATION.md) - chi tiết triển khai
- [README](README.md) - hướng dẫn chạy hệ thống

---

## 💡 Ghi chú

**Tại sao ưu tiên ingest security trước dead-letter?**
- Không có bảo mật → bất kỳ ai cũng có thể gửi log → phải reset
- Có bảo mật nhưng chưa robust → được phép thêm dead-letter sau
- Thứ tự: Auth trước, sau đó Reliability, rồi Observability

**Kế tiếp sau phase 1 là gì?**
- Phase 2: Operational hardening (ILM, backup, retry, alert dedup)
- Phase 3: Detection enhancement (Threat Intel, Correlation)
- Phase 4: SOAR + Workflow nâng cao
