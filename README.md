# A mini SIEM platform

Monorepo cho đồ án SIEM: Redis Streams tiếp nhận log, Go xử lý Parser/Rule Engine/REST API, Elasticsearch lưu event chuẩn hoá, PostgreSQL lưu rule và alert, còn Next.js cung cấp dashboard.

## Chạy local từ đầu

### 1. Kiểm tra yêu cầu

Chỉ cần Docker Engine và Docker Compose v2. Go/Node.js chỉ cần khi chạy test hoặc phát triển ngoài container.

```bash
docker --version
docker compose version
```

Đảm bảo các port sau chưa bị chương trình khác sử dụng: `3000`, `5432`, `6379`, `8080`, `9200`.

### 2. Tạo file môi trường

Chạy tại thư mục gốc repository:

```bash
cp .env.example .env
```

Mở `.env` và đổi ít nhất hai giá trị trước khi dùng thật:

```dotenv
JWT_SECRET=mot_chuoi_bi_mat_dai_va_ngau_nhien
ADMIN_PASSWORD=mat_khau_admin_toi_thieu_8_ky_tu
```

`TELEGRAM_BOT_TOKEN` và `TELEGRAM_CHAT_ID` có thể để trống khi chạy local; khi đó alert vẫn được lưu PostgreSQL nhưng không gửi Telegram.

### 3. Khởi động toàn bộ hệ thống

```bash
docker compose up -d --build
docker compose ps
```

Chờ `postgres`, `redis`, `elasticsearch` và `backend` có trạng thái `healthy`. Xem log nếu backend chưa lên:

```bash
docker compose logs -f backend
```

Kiểm tra API:

```bash
curl http://localhost:8080/healthz
```

Kết quả mong đợi là `{"status":"ok"}`. Mở dashboard tại `http://localhost:3000`.

### 4. Đăng nhập và lấy JWT

Tài khoản admin được tạo tự động từ `ADMIN_EMAIL` và `ADMIN_PASSWORD` trong `.env` ở lần khởi động đầu tiên:

```bash
curl -s http://localhost:8080/api/v1/auth/login \
	-H 'Content-Type: application/json' \
	-d '{"email":"admin@example.com","password":"change-me-now"}'
```

Copy giá trị `token` trong response và dùng ở các API cần quyền:

```bash
curl http://localhost:8080/api/v1/users \
	-H 'Authorization: Bearer <token>'
```

Hoặc mở `http://localhost:3000/accounts`, nhập email/password admin; trang này tự lưu JWT vào trình duyệt.

### 5. Gửi log thử nghiệm

Mở terminal khác, tại thư mục gốc repository:

```bash
printf '%s\n' 'Failed password for root from 192.0.2.10 port 55222 ssh2' \
	| docker compose run --rm -T ingest \
			--source-type linux_sshd \
			--hostname web-01 \
			--agent-id manual
```

Kiểm tra raw log trong Redis:

```bash
docker compose exec redis redis-cli XRANGE siem:raw-logs - +
```

Parser trong backend sẽ chuẩn hoá log và ghi vào Elasticsearch. Kiểm tra event:

```bash
curl 'http://localhost:9200/normalized_events/_search?pretty'
```

### 6. Tạo rule và kiểm tra alert

```bash
docker compose exec postgres psql -U siem -d siem -c \
	"INSERT INTO rules (name, regex_pattern, target_field, severity, category) VALUES ('SSH failure', 'Failed password', 'message', 'high', 'authentication');"
```

Gửi lại log thử nghiệm ở bước 5, sau đó kiểm tra alert:

```bash
docker compose exec postgres psql -U siem -d siem -c \
	'SELECT alert_id, severity, status, summary FROM alerts ORDER BY alert_id DESC LIMIT 10;'
```

### 7. Dừng hoặc xoá môi trường

Dừng container nhưng giữ dữ liệu:

```bash
docker compose down
```

Xoá cả volumes để chạy lại hoàn toàn từ đầu. Lệnh này xoá database, Redis Stream và Elasticsearch index local:

```bash
docker compose down -v
```

Dữ liệu PostgreSQL, Redis và Elasticsearch được giữ trong Docker volumes nếu chỉ dùng `docker compose down`.

### Volume database đã tồn tại

Nếu backend báo `relation "users" does not exist`, volume PostgreSQL được tạo trước migration users. Chạy một lần:

```bash
docker compose exec -T postgres psql -U siem -d siem < backend/migrations/002_users.sql
docker compose restart backend
```

## Đẩy raw log vào Redis Stream

Producer đọc log dạng mỗi dòng một event từ stdin (hoặc `--file`) và ghi vào stream `siem:raw-logs`:

```bash
printf '%s\n' 'Failed password for root from 192.0.2.10' \
	| docker compose run --rm -T ingest \
			--redis-url redis://redis:6379/0 \
			--source-type linux_sshd --hostname web-01 --agent-id manual
```

Kiểm tra event đã nhận:

```bash
docker compose exec redis redis-cli XRANGE siem:raw-logs - +
```

Trong môi trường thật, Winlogbeat/Filebeat sẽ thay producer CLI và gửi các field tương đương vào Redis Stream.

## Parser, Elasticsearch và Rule Engine

Khi chạy service `backend`, một Go process khởi động HTTP API, Parser và Rule Engine bằng goroutine. Parser dùng consumer group `siem-parser`, chuẩn hoá log, ghi vào index `normalized_events`, rồi Rule Engine đọc các rule enabled và tạo alert trong PostgreSQL.

```bash
docker compose run --rm --no-deps \
	-e REDIS_URL=redis://redis:6379/0 \
	--entrypoint /siem-parser backend
```

CLI parser riêng chỉ dùng để debug hoặc chạy độc lập. Parser chính đã nằm trong API binary.

Tạo rule thử nghiệm:

```bash
docker compose exec postgres psql -U siem -d siem -c \
	"INSERT INTO rules (name, regex_pattern, target_field, severity, category) VALUES ('SSH failure', 'Failed password', 'message', 'high', 'authentication');"
```

Sau đó gửi log bằng lệnh ingest ở trên. Event nằm trong Elasticsearch và alert tương ứng nằm trong PostgreSQL.

## REST API

- `GET /healthz`: health check.
- `POST /api/v1/auth/login`: đăng nhập bằng `{ "email", "password" }`, trả JWT 12 giờ.
- `GET /api/v1/events?limit=100`: viewer/analyst/admin, tìm event mới nhất trong Elasticsearch.
- `GET /api/v1/rules`: viewer/analyst/admin, danh sách rule.
- `POST|PUT|DELETE /api/v1/rules[/{id}]`: admin, CRUD rule.
- `GET /api/v1/alerts`: viewer/analyst/admin, danh sách alert mới nhất.
- `PATCH /api/v1/alerts/{id}`: analyst/admin, cập nhật `open`, `acknowledged`, `closed` và assignee.
- `GET|POST /api/v1/users`: admin, xem/tạo tài khoản.
- `DELETE /api/v1/users/{id}`: admin, vô hiệu hoá tài khoản.

Đăng nhập bằng tài khoản admin mặc định lấy từ `ADMIN_EMAIL`/`ADMIN_PASSWORD`, sau đó gửi token:

```bash
curl -s http://localhost:8080/api/v1/auth/login \
	-H 'Content-Type: application/json' \
	-d '{"email":"admin@example.com","password":"change-me-now"}'
curl http://localhost:8080/api/v1/users -H 'Authorization: Bearer <token>'
```

Trang quản lý tài khoản nằm tại `http://localhost:3000/accounts`; frontend đọc token từ `localStorage` với key `siem_token`.

Frontend chỉ gọi các endpoint Go này, không truy cập trực tiếp database.

## Cấu trúc

- `backend/cmd/api`: entrypoint Go, khởi động Parser, Rule Engine và HTTP API trong cùng process.
- `backend/cmd/ingest`: producer demo đọc stdin/file và ghi raw log vào Redis Stream.
- `backend/cmd/parser`: parser CLI độc lập để debug consumer.
- `backend/internal`: các package xử lý parser, rule engine, REST API và storage.
- `backend/migrations`: schema PostgreSQL khởi tạo tự động khi volume database mới.
- `frontend`: Next.js dashboard.
- `docker-compose.yml`: PostgreSQL, Redis, Elasticsearch, backend và frontend.

## CI/CD

`.github/workflows/deploy.yml` chạy test Go, build/push hai image lên GHCR, sau đó SSH tới VPS để chỉ pull và restart `backend`/`frontend`. Cấu hình các repository secrets: `VPS_HOST`, `VPS_USER`, `VPS_SSH_KEY`, `VPS_APP_PATH`. Database và các dịch vụ dữ liệu không bị restart trong bước deploy.

## Checklist phát triển

Đã hoàn thành:

- [x] Docker Compose cho PostgreSQL, Redis, Elasticsearch, backend và frontend.
- [x] Producer raw log vào Redis Stream.
- [x] Redis consumer group và regex parser Linux SSH.
- [x] Normalized event vào Elasticsearch.
- [x] Rule regex cơ bản và alert transaction trong PostgreSQL.
- [x] REST API đọc events, rules và alerts.
- [x] Next.js dashboard khởi đầu và CI/CD GHCR/VPS.

Cần làm tiếp trước khi demo hoàn chỉnh:

- [x] Thêm Filebeat/Winlogbeat config gửi log vào HTTP bridge rồi Redis Stream.
- [x] Thêm regex cho Windows Event Log và Nginx access log.
- [x] Implement correlation rule “N lần trong M phút” qua `condition` JSON.
- [x] Thêm cooldown Redis để tránh alert trùng.
- [x] Thêm Telegram `sendMessage` và retry/backoff.
- [x] Thêm API CRUD rule, cập nhật trạng thái alert và authentication/RBAC.
- [x] Thêm dashboard quản lý tài khoản và phân quyền admin/analyst/viewer.
- [x] Thêm test integration với Docker Compose cho ingest API, Redis Stream và authentication/RBAC.

Chạy integration test thủ công sau khi stack đã healthy:

```bash
docker compose up -d --build postgres redis elasticsearch backend
cd backend && go test -tags=integration ./integration
```

Integration test gồm `POST /api/v1/ingest` -> kiểm tra `XRANGE` trên Redis và login admin -> kiểm tra RBAC endpoint users. Nếu database volume đã tồn tại từ trước migration users, chạy một lần: `docker compose exec -T postgres psql -U siem -d siem < backend/migrations/002_users.sql`.

Lưu ý: Filebeat/Winlogbeat config nằm trong `config/` và gửi native payload qua HTTP bridge; bridge nhận `message` hoặc `event.original`, cùng `source_type`, `host.name`, `agent.id`. Rule Engine xử lý từng normalized event trong consumer callback; correlation hiện dùng Redis counter đơn giản theo rule/hostname, phù hợp đồ án nhưng chưa phải cơ chế phân tán production. Integration test Compose vẫn là việc còn lại trước demo cuối.