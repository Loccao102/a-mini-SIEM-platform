# Kiểm thử và CI

## Môi trường riêng

Các test `internal/apikey` và `internal/ratelimit` dùng PostgreSQL/Redis ở localhost:5432/6379. Test rate limit gọi `FlushDB`; **không chạy chúng trên Redis đang chứa dữ liệu ứng dụng**. Dừng stack ứng dụng trước, hoặc chạy trên máy/runner riêng. CI dùng runner mới và project Compose riêng cho mỗi job.

Từ root repo, dùng database mới với migrations 001–007:

```powershell
docker compose -p siem-ci-phase2 --env-file .env.example up -d --wait postgres redis
Set-Location backend
go vet ./...
go test -count=1 -timeout=5m ./...
```

Trên Linux có C compiler, chạy thêm `go test -race -count=1 -timeout=5m -coverprofile=coverage.out ./...`. Trên Windows cần CGO và toolchain C phù hợp; không xem việc chưa chạy được race detector là test đã pass.

## Integration pipeline

Từ root repo:

```powershell
docker compose -p siem-ci-phase2 --env-file .env.example up -d --build --wait --wait-timeout 300 postgres redis elasticsearch backend
Set-Location backend
go test -tags=integration -count=1 -timeout=3m -v ./integration
```

Suite tạo key bằng API key manager trong database test, xác minh HTTP 202, nội dung Redis Stream, event được parser normalize và index vào Elasticsearch, và HTTP 401 với key thiếu/sai/bị thu hồi. Test authentication kiểm tra login admin và quyền truy cập users. Redis hoặc ES không sẵn sàng làm test fail, không skip.

Biến tùy chọn: `INTEGRATION_API_URL`, `INTEGRATION_REDIS_URL`, `INTEGRATION_POSTGRES_URL`, `INTEGRATION_ELASTICSEARCH_URL`, `INTEGRATION_ADMIN_EMAIL`, `INTEGRATION_ADMIN_PASSWORD`. Mặc định khớp `.env.example`; chỉ trỏ chúng tới stack test. Fixture key được xóa sau test; asset/event test còn trong stack riêng cho chẩn đoán.

Sau khi dùng xong, từ root repo có thể xóa **chỉ dữ liệu stack test** bằng `docker compose -p siem-ci-phase2 --env-file .env.example down -v`. Không thay project này bằng project ứng dụng.

## Frontend

Từ `frontend`, với Node 22 (khớp Dockerfile):

```powershell
npm ci
npm run lint
npx tsc --noEmit
npm run build
```

Đây là lint/typecheck/build, chưa phải browser E2E. E2E login/events/alerts/cases và Fleet nhiều host nằm trong roadmap phase 2.

## GitHub Actions

- `ci.yml`: pull request, push nhánh ngoài main, chạy thủ công, và reusable workflow.
- Backend: dependencies/migrations thật → vet → test/race/coverage.
- Frontend: locked install → lint → typecheck → build trên Node 22.
- Integration: chờ hai job trên xanh, chạy pipeline Docker và integration tests.
- `deploy.yml`: push main hoặc chạy thủ công gọi lại toàn bộ CI qua job `quality`; chỉ build/push image khi quality thành công. SSH deployment giữ điều kiện có VPS_HOST, kiểm tra ở step thông qua env theo cú pháp GitHub Actions.

CI không tự bật branch protection. Có thể chọn các job backend/frontend/integration làm required checks trong repository ruleset sau khi lần chạy đầu xuất hiện. Việc có workflow không đồng nghĩa toàn bộ phase 2–4 đã hoàn thành.

Tham chiếu: [GitHub reusable workflows](https://docs.github.com/en/actions/how-tos/reuse-automations/reuse-workflows), [secrets trong workflow](https://docs.github.com/en/actions/how-tos/write-workflows/choose-what-workflows-do/use-secrets).
