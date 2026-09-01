# GIAI ĐOẠN 1: Core Stabilization - Ingest Security Implementation

**Status**: ✅ Hoàn thành (Phase 1 của roadmap)
**Date**: 2026-09-01
**Components Implemented**: API Key Manager + Rate Limiter + Middleware

---

## 📋 Tóm tắt công việc

Đã triển khai **3 lớp bảo mật** cho ingest endpoint `POST /api/v1/ingest`:

1. **API Key Authentication** - yêu cầu Bearer token để phân biệt agent hợp lệ
2. **Rate Limiting** - giới hạn 1000 requests/phút per hostname để chống DDoS
3. **Request Size Limit** - tối đa 10MB per request để chống payload bombs

---

## 🔐 Các thành phần đã tạo

### 1. API Key Manager (`internal/apikey/manager.go`)

**Mục đích**: Tạo, xác thực và quản lý API key cho từng agent/asset

**Tính năng**:
- Tạo key ngẫu nhiên với format `sk_<32_hex_chars>`
- Lưu hash SHA256 (không lưu plain key)
- Display key an toàn: `sk_<8_first>...<4_last>`
- Hỗ trợ expiration tuỳ chọn
- Track last_used_at và request_count cho auditing
- Revoke individual keys

**Public Methods**:
```go
GenerateKey(ctx, assetID, expiresIn) -> (*APIKey, string, error)
ValidateKey(ctx, rawKey) -> (*APIKey, error)
RevokeKey(ctx, keyID) -> error
ListKeys(ctx, assetID) -> ([]APIKey, error)
```

**Database Schema**:
```sql
CREATE TABLE api_keys (
    api_key_id BIGSERIAL PRIMARY KEY,
    asset_id BIGINT REFERENCES assets,
    key_hash VARCHAR(255) UNIQUE,  -- SHA256 hash
    display_key VARCHAR(50),       -- safe display
    status VARCHAR(20),            -- active/revoked/expired
    created_at TIMESTAMP,
    expires_at TIMESTAMP,
    last_used_at TIMESTAMP,
    request_count BIGINT
)
```

**Tests**: ✅ Có test cho: generate, validate, revoke, list, expiration

---

### 2. Rate Limiter (`internal/ratelimit/limiter.go`)

**Mục đích**: Chống DDoS và phòng ngừa abuse bằng cách giới hạn request per hostname

**Tính năng**:
- Token bucket algorithm sử dụng Redis
- Default: 1000 requests/phút per hostname
- TTL tự động reset cửa sổ
- Trả về remaining count và reset time

**Public Methods**:
```go
Allow(ctx, identifier) -> (bool, int64, time.Time, error)
Reset(ctx, identifier) -> error
GetStatus(ctx, identifier) -> (int64, int64, time.Time, error)
```

**Response Headers**:
```
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: <unix_timestamp>
Retry-After: <seconds> (nếu exceeded)
```

**Tests**: ✅ Có test cho: allow, multiple requests, different users, reset, status

---

### 3. Middleware (`internal/api/middleware.go`)

**3 middleware được thêm vào Routes**:

#### a) `withIngestAuth` - API Key validation
```go
// Check Authorization header format: "Bearer sk_..."
// Validate key từ database via apiKeyManager
// Store asset_id và hostname vào X-Asset-ID / X-Hostname headers
// Return 401 nếu key invalid/expired/revoked
```

#### b) `withRateLimit` - Per-hostname rate limiting
```go
// Use hostname từ auth middleware
// Call limiter.Allow(hostname)
// Set rate limit headers
// Return 429 Too Many Requests nếu vượt limit
```

#### c) `withRequestSizeLimit` - Payload size guard
```go
// Kiểm tra Content-Length header
// Max size: 10MB (MaxPayloadSize)
// Wrap body với io.LimitReader
// Return 413 Request Entity Too Large nếu vượt
```

---

## 🚀 Triển khai trong Handler

**File**: `internal/api/handler.go`

```go
type Handler struct {
    // ... existing fields ...
    apiKeyManager *apikey.Manager
    rateLimiter   *ratelimit.Limiter
}

func (handler *Handler) Routes() http.Handler {
    mux := http.NewServeMux()
    
    // Protected ingest endpoint with middleware chain
    ingestSecured := handler.withRequestSizeLimit(MaxPayloadSize)(
        handler.withRateLimit(
            handler.withIngestAuth(
                http.HandlerFunc(handler.ingestLog),
            ),
        ),
    )
    mux.Handle("POST /api/v1/ingest", ingestSecured)
    
    // ... rest of routes ...
    return withCORS(mux)
}
```

---

## 🔧 Khởi tạo trong `cmd/api/main.go`

```go
// API Key Manager
apiKeyMgr := apikey.New(postgres)

// Rate Limiter (1000 requests/minute per hostname)
rateLimiter := ratelimit.New(redisClient, 1000, 1*time.Minute)

// Handler với cả hai
handler := api.New(postgres, elastic, ingestClient, authManager, 
    dedupManager, apiKeyMgr, rateLimiter)
```

---

## 📊 Database Migration

**File**: `backend/migrations/006_api_keys.sql`

Tạo bảng `api_keys` với:
- Unique index trên `key_hash` (nhanh validate)
- Index trên `asset_id` + `status` (nhanh list + status check)
- Partial index trên `expires_at` khi `status = 'active'` (nhanh cleanup)

---

## ✅ Tiêu chí hoàn thành (đã đạt)

- [x] API key generation và storage (hashed)
- [x] Validate key trước khi ingest
- [x] Rate limit per hostname (1000/min)
- [x] Request size limit (10MB max)
- [x] Audit trail (last_used_at, request_count)
- [x] Key revocation support
- [x] Key expiration support
- [x] Database migration
- [x] Unit tests cho apikey manager
- [x] Unit tests cho rate limiter
- [x] Middleware integration
- [x] API server builds successfully
- [x] All services start healthy

---

## 🧪 Cách kiểm tra

### 1. Login và lấy token admin
```powershell
$login = Invoke-RestMethod http://localhost:8080/api/v1/auth/login -Method Post `
  -ContentType 'application/json' `
  -Body (@{
    email = 'admin@example.com'
    password = 'admin'
  } | ConvertTo-Json)
$token = $login.token
```

### 2. Tạo API key cho asset
```powershell
# Trước tiên, tạo asset nếu chưa có
Invoke-RestMethod http://localhost:8080/api/v1/assets -Method Post `
  -ContentType 'application/json' `
  -Headers @{ Authorization = "Bearer $token" } `
  -Body (@{
    hostname = 'web-01'
    ip_address = '192.168.1.100'
    os_type = 'linux'
    criticality = 'high'
  } | ConvertTo-Json)
```

### 3. Thử ingest mà không có API key (phải fail 401)
```powershell
$payload = @{
  event = @{ original = "Test log" }
  source_type = 'linux'
  host = @{ name = 'web-01' }
} | ConvertTo-Json -Depth 4

Invoke-RestMethod http://localhost:8080/api/v1/ingest -Method Post `
  -ContentType 'application/json' `
  -Body $payload
# Kết quả: 401 Unauthorized (missing Authorization header)
```

### 4. Tạo API key qua database (dạng test)
```bash
# Generate key bằng app code
# Hoặc có thể thêm endpoint POST /api/v1/keys (chưa làm, để sau)
```

### 5. Gửi log với API key hợp lệ
```powershell
$payload = @{
  event = @{ original = "Failed password for root" }
  source_type = 'linux'
  host = @{ name = 'web-01' }
} | ConvertTo-Json -Depth 4

Invoke-RestMethod http://localhost:8080/api/v1/ingest -Method Post `
  -ContentType 'application/json' `
  -Headers @{ Authorization = "Bearer sk_<your_key_here>" } `
  -Body $payload
# Kết quả: 202 Accepted + stream_id
# Headers: X-RateLimit-Limit: 1000, X-RateLimit-Remaining: 999
```

### 6. Kiểm tra rate limit
```powershell
# Gửi 1001 request (vượt limit) -> request thứ 1001+ return 429
# Header Retry-After sẽ nói khi nào có thể retry
```

---

## 🎯 Tiếp theo (giai đoạn 2)

Sau khi ingest security ổn:

1. **Dead-Letter Queue + Retry Logic**
   - Catch failed events từ parser/ingest
   - Store vào DLQ stream
   - Retry với exponential backoff

2. **Health Checks & Observability**
   - GET /healthz cho từng service riêng
   - Metrics cho queue lag, parser lag, ingest latency
   - Alert khi pipeline chậm

3. **Fleet/Agent Management**
   - Validate agent enrollment
   - Policy template rõ ràng
   - Status tracking (last_seen, healthy/unhealthy)

---

## 🔑 API Key Generation Workflow (Future)

Khi user muốn tạo key cho agent mới:

```
1. Admin tạo asset trong UI/API (hostname, os_type, etc.)
   → asset được lưu với asset_id

2. Admin request API key generation
   POST /api/v1/assets/{asset_id}/keys
   → apikey.Manager.GenerateKey(asset_id, expiration)
   → Tạo random key `sk_<hash>`
   → Lưu key_hash vào database
   → Return raw key (duy nhất lần này)

3. Admin chia sẻ key cho team/agent owner
   → Agent cấu hình header: Authorization: Bearer sk_<key>

4. Agent gửi log
   → withIngestAuth validate key
   → withRateLimit check hostname limit
   → withRequestSizeLimit check payload
   → ingestLog xử lý bình thường
```

---

## 📝 Cấu hình Environment (optional)

Có thể thêm vào `.env` nếu muốn tuning:

```dotenv
# Rate limit (requests per minute)
RATE_LIMIT_REQUESTS=1000
RATE_LIMIT_WINDOW=1m

# Request size limit (bytes)
MAX_PAYLOAD_SIZE=10485760  # 10MB
```

---

## 🏁 Kết luận

**Giai đoạn 1: Core Stabilization** đã hoàn thành **3/5 mục tiêu**:

- ✅ Bảo mật ingest endpoint
- ✅ Rate limit + auth
- ⏳ Queue health + dead-letter (tiếp theo)
- ⏳ Pipeline observability (tiếp theo)
- ⏳ Fleet validation (tiếp theo)

Hệ thống giờ đây:
- ✅ Không còn public ingest endpoint
- ✅ Agent/client phải xác thực bằng API key
- ✅ Chống DDoS qua rate limiting
- ✅ Audit trail cho mỗi request
- ✅ Ready cho production-like pilot

**Tiêu chí "đạt level chạy thực tế"** có thể được kiểm tra sau khi hoàn thành:
- Dead-letter + retry
- Health check endpoints
- Fleet agent validation

