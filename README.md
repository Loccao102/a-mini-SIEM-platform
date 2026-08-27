# A mini SIEM platform

Monorepo cho đồ án SIEM: Redis Streams tiếp nhận log, Go xử lý Parser/Rule Engine/REST API, Elasticsearch lưu event chuẩn hoá, PostgreSQL lưu rule và alert, còn Next.js cung cấp dashboard.

## Chạy local

Yêu cầu: Docker Compose, Go 1.26+ và Node.js 22+.

```bash
cp .env.example .env
docker compose up --build
```

Dashboard chạy tại `http://localhost:3000`, API health check tại `http://localhost:8080/healthz`. Dữ liệu PostgreSQL, Redis và Elasticsearch được giữ trong Docker volumes.

## Cấu trúc

- `backend/cmd/api`: entrypoint Go; các goroutine Parser, Rule Engine và API sẽ được nối vào đây.
- `backend/internal`: các package xử lý parser, rule engine, REST API và storage.
- `backend/migrations`: schema PostgreSQL khởi tạo tự động khi volume database mới.
- `frontend`: Next.js dashboard.
- `docker-compose.yml`: PostgreSQL, Redis, Elasticsearch, backend và frontend.

## CI/CD

`.github/workflows/deploy.yml` chạy test Go, build/push hai image lên GHCR, sau đó SSH tới VPS để chỉ pull và restart `backend`/`frontend`. Cấu hình các repository secrets: `VPS_HOST`, `VPS_USER`, `VPS_SSH_KEY`, `VPS_APP_PATH`. Database và các dịch vụ dữ liệu không bị restart trong bước deploy.