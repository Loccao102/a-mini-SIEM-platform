# Mini-SIEM — checklist đưa lên VPS (01 node, HTTPS)

**Phạm vi:** Một VPS Linux chạy Docker Compose: PostgreSQL, Redis, Elasticsearch, Go backend, ingest, Next.js frontend và Caddy. Tất cả dịch vụ ứng dụng/database bind 127.0.0.1 ở host; chỉ Caddy xuất bản 80/443. **Không chạy script này trên HABI hoặc máy chủ đang có dữ liệu SIEM do một Compose project khác quản lý mà chưa di chuyển volume.**

## 1. Hạ tầng và DNS

- Khuyến nghị: **2 vCPU, RAM 4–8 GB, SSD >= 40 GB** (ngưỡng chặn tối thiểu của preflight: 3 GiB RAM, 8 GiB đĩa trống). Elasticsearch cần bộ nhớ và disk I/O; 2 GB không đủ an toàn.
- Linux x86_64, Docker Engine + Docker Compose plugin, Node.js 22+, git, firewall. Thiết lập `vm.max_map_count=262144` bền vững để Elasticsearch chạy ổn. Đồng bộ thời gian NTP.
- Trỏ A record `siem.example.org` đến public IPv4 VPS; có thể dùng tên miền DNS tương đương bạn sở hữu. Chưa có DNS + A record trỏ đúng sẽ không có chứng chỉ công khai.
- Firewall của VPS/cloud chỉ cho TCP 80, TCP 443 và UDP 443 (HTTP/3 tùy chọn); SSH 22 chỉ cho IP quản trị. Không mở PostgreSQL 5432, Redis 6379, Elasticsearch 9200, API 8080, frontend 3000, Fleet 8220, Beats 5044 ra Internet.
- Không chạy thêm reverse proxy khác chiếm port 80/443. Nếu VPS có Caddy/Nginx sẵn, không bật đồng thời dịch vụ Caddy profile `production`; dùng cấu hình tương đương ở `ops/Caddyfile` qua proxy hiện tại.

## 2. Chuẩn bị từ mã nguồn đã test

```bash
git clone https://github.com/Loccao102/a-mini-SIEM-platform.git
cd a-mini-SIEM-platform
cp .env.production.example .env.production
chmod 600 .env.production
```

Điền các biến trong `.env.production` bằng editor ngay trên VPS, **không paste secret vào chat hoặc commit lên Git**:

- `MODE=production`
- `SIEM_DOMAIN=siem.example.org` — hostname không có `https://` hay `/`.
- `ACME_EMAIL=admin@your-domain.example` — email thật của người vận hành.
- `NEXT_PUBLIC_API_URL=https://siem.example.org` — **chính xác origin**, không thêm `/api` hoặc dấu `/` cuối; frontend đã tự thêm `/api/v1/...`.
- `POSTGRES_PASSWORD`, `ADMIN_PASSWORD` là hai mật khẩu khác nhau, dài >= 16; `JWT_SECRET` dài >= 32. `POSTGRES_PASSWORD` phải chứa ký tự URL-safe `[A-Za-z0-9._~-]` do hiện được nhúng trực tiếp vào DSN của Compose. Cách tạo ngẫu nhiên: `openssl rand -hex 24` cho password, `openssl rand -hex 32` cho JWT secret.
- `ADMIN_EMAIL` phải là email hợp lệ, không dùng thông tin mặc định.
- Các biến `*_BIND_HOST=127.0.0.1` giữ nguyên. Không đưa `POSTGRES_URL`, `JWT_SECRET`, `ADMIN_PASSWORD` vào `NEXT_PUBLIC_*`.

**Lưu ý volume:** script dùng Compose project cố định `siem-prod`. Nếu máy đã có dữ liệu với project khác (ví dụ chạy `docker compose up` không `-p`), **không chạy song song với dữ liệu cũ** vì sẽ tạo named volume khác. Backup, xác minh và di chuyển thủ công trước.

## 3. Kiểm tra và khởi động (chỉ khi bạn quyết định deploy)

```bash
node scripts/vps-release.mjs --env .env.production
# Lệnh trên chỉ in kế hoạch; KHÔNG tạo container.
node scripts/vps-release.mjs --env .env.production --apply
```

Chỉ khi có `--apply`, script mới kiểm tra file secrets (mode 0600), RAM/đĩa/DNS cấu hình, Docker Compose, cú pháp Caddyfile, rồi chạy `docker compose up -d --build --wait`. Sau khi khởi động, smoke test qua **HTTPS public**: trang web trả HTML, `/healthz` healthy, các route chẩn đoán admin trả 401 nếu chưa đăng nhập và health public không lộ thông tin chi tiết.

Để chỉ kiểm tra lại không làm thay đổi môi trường:

```bash
node scripts/production-preflight.mjs --env .env.production
node scripts/vps-smoke.mjs --origin https://siem.example.org
docker compose --env-file .env.production --profile production -p siem-prod ps
docker compose --env-file .env.production --profile production -p siem-prod logs --tail 80 backend
```

**Không chạy `docker compose down -v` hoặc `docker volume prune` trên production.** Khi cập nhật commit, chạy lại script sau khi có backup hợp lệ; `--build` đảm bảo Next frontend bundle sử dụng hostname mới.

## 4. Sao lưu và thử khôi phục

```bash
node scripts/vps-backup.mjs --env .env.production
# Chỉ hiển thị dry-run.
node scripts/vps-backup.mjs --env .env.production --apply
```

- Sao lưu PostgreSQL bằng `pg_dump -Fc` vào file chmod 0600 kèm SHA-256.
- Elasticsearch snapshot qua repository `fs` có path thuộc `/snapshots` (named volume). Script xuất **toàn bộ thư mục snapshot repository** bằng `docker compose cp` ra thư mục backup. Đảm bảo volume `/snapshots` đủ quyền ghi cho UID Elasticsearch, có đủ disk trước khi snapshot.
- Backup nằm mặc định trong `~/siem-backups/siem-<UTC-timestamp>/` với quyền riêng tư. **Sao chép sang ổ/địa điểm khác**: backup trên cùng VPS không bảo vệ được khi ổ đĩa/VPS hỏng.
- Repository Elasticsearch có thể tích lũy nhiều snapshots; lên lịch retention sau khi đã có phương án khôi phục được kiểm chứng. Tuyệt đối không xóa file riêng lẻ trong repository snapshot; xóa snapshot bằng API Elasticsearch theo quy trình.
- Khôi phục chỉ nên thử trên một Compose project/VPS **cô lập**, không restore trực tiếp đè production. Có thể dùng `scripts/restore-drill.mjs` trên test stack tương ứng để đo tính toàn vẹn; kiểm tra số tài sản và sự kiện đã index.
- **Giới hạn:** script mới chưa tự tạo lịch backup và chưa tự diễn tập recovery full-stack trên VPS thực. Nhất thiết chạy thử một bản snapshot trước khi lưu log quan trọng.

## 5. Xử lý sự cố đầu tiên

| Hiện tượng | Hướng kiểm tra |
|---|---|
| Caddy chưa cấp TLS | Kiểm tra DNS trỏ IP, cổng 80/443 và log `docker compose ... logs caddy`; không dùng `https://` trong SIEM_DOMAIN |
| Giao diện mở được nhưng API 404 | NEXT_PUBLIC_API_URL phải đúng `https://SIEM_DOMAIN`, không chứa `/api`. Kiểm tra `/api/v1/auth/me` trả 401 khi chưa đăng nhập |
| API trả 503 | Kiểm tra trang Admin `/system`, các health probe và `docker compose ... ps` |
| Elasticsearch không khởi động | Kiểm tra RAM, `vm.max_map_count`, dung lượng và log Elasticsearch |
| Đăng nhập fail sau đổi env | `POSTGRES_PASSWORD` env mới không tự thay đổi password của database đã khởi tạo; phải thay đổi credential trong DB có kế hoạch, không xóa volume |
| Không thấy sự kiện từ agent | Agent ngoài VPS phải gọi URL HTTPS public `/api/v1/ingest`; các cổng 8080/Redis luôn giữ riêng tư; kiểm tra API key và stream |
| Snapshot thất bại | Kiểm tra permissions volume snapshot, free disk, trạng thái Elasticsearch và full output; không xóa snapshot hiện có trực tiếp trên filesystem |

**Kiến trúc 2 VPS:** không dùng nguyên cấu hình Compose này để chia backend/DB sang 2 máy. Muốn tách node cần mạng riêng mã hóa (WireGuard/Tailscale), service discovery, TLS và cấu hình secrets/ports khác; cấu hình hiện tại được thiết kế cho **một VPS**.
