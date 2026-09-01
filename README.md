# A mini SIEM platform

Nền tảng SIEM local gom:

```text
Elastic Agent -> Logstash -> HTTP ingest -> Redis Stream -> Parser -> Elasticsearch
                                                   -> Rule Engine -> PostgreSQL alerts
                                                           -> Next.js dashboard
```

Elastic Agent thay thế Filebeat/Winlogbeat để quản lý thu thập log từ nhiều service/host theo một policy tập trung, hỗ trợ tốt hơn cho môi trường đa máy chủ, đa dịch vụ và dễ mở rộng ở giai đoạn sau. Đây là mô hình Fleet thật, không phải mock: Fleet Server luôn chạy cùng stack, agent enroll trực tiếp vào `fleet-server` trên cổng `8220`, và backend đồng bộ asset + `log_sources` theo hostname để nhiều client đều có thể gửi log lên cùng một server mà không cần khai báo thủ công từng host.

Dashboard doc du lieu that tu API Go. Overview, Alerts va Log Explorer tu tai lai moi 3 giay.

## Yeu cau

- Docker Desktop dang chay
- Docker Compose v2
- PowerShell 5.1 hoac PowerShell 7
- Cac port `3000`, `5432`, `6379`, `8080`, `9200`, `5044`, `8220` chua bi chiem

```powershell
docker --version
docker compose version
```

Go va Node.js khong can cai neu chi chay bang Docker.

## Khoi dong local tu dau

Mo PowerShell tai thu muc repository:

```powershell
Copy-Item .env.example .env
```

Mo `.env` va doi it nhat:

```dotenv
JWT_SECRET=mot_chuoi_bi_mat_dai_va_ngau_nhien
ADMIN_PASSWORD=mat_khau_admin_toi_thieu_8_ky_tu
# Chi bat du lieu demo khi can chay moi truong demo
MODE=production
```

`ADMIN_PASSWORD` chi duoc dung de tao admin lan dau. Neu PostgreSQL volume da ton tai, doi bien nay khong doi password cu.
Du lieu demo (tai khoan mau, assets mau va cac nut/kich ban demo tren UI) chi duoc bat khi dat `MODE=develop`.

De xoa toan bo du lieu local va khoi dong lai tu trang thai rong:

```powershell
docker compose down -v
docker compose up -d --build
```

Khoi dong:

```powershell
docker compose up -d --build
docker compose ps
```

Cho `postgres`, `redis`, `elasticsearch` va `backend` co trang thai `healthy`. Frontend khong co healthcheck rieng.

Kiem tra API va dashboard:

```powershell
Invoke-RestMethod http://localhost:8080/healthz
Start-Process http://localhost:3000
```

Ket qua health mong doi la `status: ok`.

Neu backend chua len:

```powershell
docker compose logs --tail=100 backend
```

## Dang nhap

Neu day la database moi, dang nhap bang credentials trong `.env`:

```powershell
$login = Invoke-RestMethod http://localhost:8080/api/v1/auth/login -Method Post -ContentType 'application/json' -Body (@{
  email = 'admin@example.com'
  password = 'change-me-now'
} | ConvertTo-Json)
$token = $login.token
$token
```

Doi `email` va `password` neu ban da sua `.env`.

Hoac mo `http://localhost:3000/accounts`. Frontend luu token trong trinh duyet voi key `siem_token`.

Kiem tra quyen admin:

```powershell
Invoke-RestMethod http://localhost:8080/api/v1/users -Headers @{ Authorization = "Bearer $token" }
```

Neu nhan `401`, dung password da tao cung PostgreSQL volume hien tai. Voi moi truong local co the xoa toan bo du lieu va khoi tao lai co chu dich:

```powershell
docker compose down -v
docker compose up -d --build
```

Lenh `down -v` xoa du lieu PostgreSQL, Redis va Elasticsearch.

## Gui log thu nghiem

```powershell
"Failed password for root from 192.0.2.10 port 55222 ssh2" | docker compose run --rm -T ingest `
  --source-type linux_sshd `
  --hostname web-01 `
  --agent-id manual
```

Kiem tra raw log trong Redis:

```powershell
docker compose exec redis redis-cli XRANGE siem:raw-logs - +
```

Phan biet log da di qua tung buoc:

```powershell
# Logstash da nhan Beats va co gui HTTP thanh cong hay khong
docker compose logs --tail=100 logstash

# Redis stream co message, consumer group dang xu ly bao nhieu message
docker compose exec redis redis-cli XLEN siem:raw-logs
docker compose exec redis redis-cli XINFO GROUPS siem:raw-logs
```

Hoac xem JSON trang thai da tong hop (can JWT viewer+):

```powershell
Invoke-RestMethod http://localhost:8080/api/v1/pipeline/status -Headers @{ Authorization = "Bearer $token" }
```

`POST /api/v1/ingest` tra `202` khi backend da ghi log vao Redis. Trong ket qua status, `pending > 0` nghia la Redis da nhan nhung parser chua ACK; `pending = 0` nghia la consumer group da ACK het batch. Kiem tra Elasticsearch de xac nhan event da duoc index.

Lenh producer tren chi kiem tra truc tiep Redis. De kiem tra dung luong monitoring qua HTTP API va dang ky asset, dung:

```powershell
$payload = @{
  event = @{ original = "Failed password for root from 192.0.2.10" }
  source_type = 'linux_sshd'
  host = @{ name = 'web-01' }
  agent = @{ id = 'manual-http' }
} | ConvertTo-Json -Depth 4
Invoke-RestMethod http://localhost:8080/api/v1/ingest -Method Post -ContentType 'application/json' -Body $payload
```

Doi vai giay roi kiem tra event da chuan hoa trong Elasticsearch:

```powershell
Invoke-RestMethod 'http://localhost:9200/normalized_events/_search?pretty'
```

Kiem tra host da duoc ghi vao PostgreSQL:

```powershell
docker compose exec postgres psql -U siem -d siem -c "SELECT hostname, os_type FROM assets ORDER BY asset_id DESC LIMIT 10;"
```

Frontend:

- `http://localhost:3000/events`
- `http://localhost:3000/assets`

## Tao rule va kiem tra alert

Tao rule:

```powershell
docker compose exec postgres psql -U siem -d siem -c "INSERT INTO rules (name, regex_pattern, target_field, severity, category) VALUES ('SSH failure', 'Failed password', 'message', 'high', 'authentication');"
```

Gui lai log thu nghiem:

```powershell
"Failed password for root from 192.0.2.10 port 55222 ssh2" | docker compose run --rm -T ingest `
  --source-type linux_sshd `
  --hostname web-01 `
  --agent-id manual
```

Kiem tra alert:

```powershell
docker compose exec postgres psql -U siem -d siem -c "SELECT alert_id, severity, status, summary FROM alerts ORDER BY alert_id DESC LIMIT 10;"
```

Frontend:

- `http://localhost:3000/alerts`
- `http://localhost:3000/rules`

## Chuyển sang Elastic Agent + Fleet

Với Elastic Agent + Fleet, mỗi máy chủ/host chỉ cần cài một agent duy nhất, rồi đăng ký với Fleet Server để nhận policy thu thập log từ nhiều service khác nhau như Linux syslog, Windows Event Log, Docker, NGINX, SSH, v.v. Điều này rõ ràng mạnh hơn Filebeat/Winlogbeat khi ta cần quản lý nhiều serviço/host trong một hệ thống SIEM kiểu enterprise.

Backend cũng được bổ sung một route `POST /api/v1/fleet/agents` để đồng bộ asset: khi một agent enroll, server sẽ upsert vào bảng `assets` và `log_sources`, từ đó nhiều client có thể gửi log lên cùng server mà không cần tạo asset thủ công trước. Về mặt kiến trúc, đây là một SIEM mini nhưng chạy theo đúng nguyên tắc của Elastic Fleet: Fleet Server là thành phần nền của hệ thống, không phải phụ kiện tùy chọn.

1. Tải Elastic Agent theo đúng OS/arch.
2. Dùng policy mẫu trong `config/elastic-agent.yml`.
3. Chỉnh sửa host Logstash nếu chạy ngoài cùng máy:

```yaml
outputs:
  default:
    type: logstash
    hosts: ["IP_MAY_CHAY_SIEM:5044"]
```

4. Đối với Linux host:

```bash
sudo elastic-agent install -f -c /etc/elastic-agent/elastic-agent.yml
sudo systemctl enable --now elastic-agent
sudo systemctl status elastic-agent
sudo journalctl -u elastic-agent -f
```

5. Với Fleet, agent sẽ tự đăng ký qua `fleet-server` ở port `8220`; backend sẽ nhận metadata `hostname`, `os_type`, `agent_id`, và `source_types` rồi upsert vào bảng assets/log_sources. Trong môi trường thực tế, Fleet Server là thành phần luôn chạy và agent liên tục poll policy từ đây.

```powershell
Invoke-RestMethod http://localhost:8080/api/v1/fleet/agents -Method Post -ContentType 'application/json' -Body (@{
  agent_id = 'linux-fleet-01'
  hostname = 'web-02'
  os_type = 'linux'
  ip_address = '10.0.0.12'
  source_types = @('system','docker','elastic_agent')
  tags = @{ env = 'prod'; team = 'platform' }
} | ConvertTo-Json -Depth 4)
```

5. Đối với Windows host:

```powershell
.\elastic-agent.exe install -f -c .\elastic-agent.yml
Restart-Service elastic-agent
Get-Service elastic-agent
```

6. Với Elastic Agent, ta có thể gắn nhiều input trong cùng một policy: `logfile`, `winlog`, `docker`, `system`, `custom`, v.v. Điều này giúp quản lý log từ nhiều service trong một agent thay vì phải cài Filebeat + Winlogbeat riêng cho từng loại host.

> Lưu ý: luồng dữ liệu vẫn giữ nguyên mô hình hiện tại: Elastic Agent -> Logstash -> backend HTTP ingest -> Redis -> parser -> Elasticsearch. Chỉ thay đổi tầng thu thập, không cần sửa pipeline nghiệp vụ trong SIEM.

## Cai theo doi may Windows

Dùng Elastic Agent tren may Windows can theo doi. Backend Docker va service `logstash` phai dang chay. Agent gui log den port `5044` qua Logstash; Logstash moi gui HTTP toi backend.

1. Cai Elastic Agent tu Elastic.
2. Sử dụng file mẫu `config/elastic-agent.yml` và bật input `winlog` trong policy.
3. Nếu Logstash chay tren cung may, giu mac dinh `localhost`. Neu Docker chay tren may khac, sua host trong config:

```yaml
outputs:
  default:
    type: logstash
    hosts: ["IP_MAY_CHAY_SIEM:5044"]
```

4. Kiem tra cau hinh trong PowerShell Administrator:

```powershell
.\elastic-agent.exe inspect -c .\elastic-agent.yml
```

5. Ngoai ra, co the tai lai policy va khoi dong lai service:

```powershell
Restart-Service elastic-agent
Get-Service elastic-agent
```

6. Xem log neu chua gui duoc:

```powershell
Get-Content 'C:\ProgramData\Elastic\Agent\logs\elastic-agent-*' -Tail 100
```

Elastic Agent cho phep gom `Security`, `System` va cac log daemon/phu trong cung mot policy, đồng thời dễ lan truyen/quan ly dọc theo nhiều host va service.

## Cai theo doi may Linux

Dùng Elastic Agent tren may Linux can theo doi. Chep `config/elastic-agent.yml` vao Elastic Agent va sua dia chi Logstash neu Docker chay tren may khac.

```yaml
outputs:
  default:
    type: logstash
    hosts: ["IP_MAY_CHAY_SIEM:5044"]
```

Agency thu thập `/var/log/auth.log`, `/var/log/syslog`, log Docker, log nginx, v.v. trong một policy duy nhất.

```bash
sudo elastic-agent inspect -c /etc/elastic-agent/elastic-agent.yml
sudo systemctl enable --now elastic-agent
sudo systemctl status elastic-agent
sudo journalctl -u elastic-agent -f
```

Neu Filebeat chay truc tiep tren may, dung IP hoac `localhost` cua may chay Docker. `backend:8080` chi dung cho container noi bo, khong dung trong cau hinh agent chay tren Windows/Linux host.

## Xac minh monitoring cap nhat

Sau khi agent chay, tao mot su kien moi tren may duoc theo doi. Kiem tra theo thu tu:

```powershell
docker compose exec redis redis-cli XREVRANGE siem:raw-logs + - COUNT 5
Invoke-RestMethod 'http://localhost:9200/normalized_events/_search?pretty'
docker compose exec postgres psql -U siem -d siem -c "SELECT a.hostname, l.source_type, l.last_seen FROM assets a LEFT JOIN log_sources l ON l.asset_id=a.asset_id ORDER BY a.asset_id DESC LIMIT 10;"
```

Sau khi Parser xu ly xong, mo `http://localhost:3000`. Overview, Alerts va Log Explorer tu goi API lai moi 3 giay. Day la polling, khong phai WebSocket realtime; do tre thuong la vai giay den khoang 3 giay cong thoi gian xu ly queue. Neu can cap nhat tung event ngay lap tuc, can them SSE/WebSocket sau.

## REST API

| Endpoint | Quyen | Muc dich |
| --- | --- | --- |
| `GET /healthz` | public | Health check |
| `POST /api/v1/auth/login` | public | Dang nhap, tra JWT 12 gio |
| `POST /api/v1/ingest` | public local | Nhan log tu agent |
| `GET /api/v1/summary` | viewer+ | Metrics dashboard |
| `GET /api/v1/pipeline/status` | viewer+ | Redis stream length, pending messages va consumer count |
| `GET /api/v1/events?limit=100` | viewer+ | Event tu Elasticsearch |
| `GET /api/v1/assets` | viewer+ | Host va log source |
| `GET /api/v1/rules` | viewer+ | Danh sach rule |
| `POST /api/v1/rules` | admin | Tao rule |
| `PUT /api/v1/rules/{id}` | admin | Cap nhat rule |
| `DELETE /api/v1/rules/{id}` | admin | Xoa rule |
| `GET /api/v1/alerts` | viewer+ | Danh sach alert |
| `PATCH /api/v1/alerts/{id}` | analyst+ | Doi trang thai/gan alert |
| `GET /api/v1/users` | admin | Danh sach user |
| `POST /api/v1/users` | admin | Tao user |
| `DELETE /api/v1/users/{id}` | admin | Vo hieu hoa user |

## Troubleshooting

### Port da duoc su dung

Doi port trong `.env`:

```dotenv
FRONTEND_PORT=3001
API_PORT=8081
NEXT_PUBLIC_API_URL=http://localhost:8081
```

Sau do rebuild:

```powershell
docker compose up -d --build backend frontend
```

### `relation "users" does not exist`

Volume duoc tao truoc migration users:

```powershell
Get-Content .\backend\migrations\002_users.sql | docker compose exec -T postgres psql -U siem -d siem
docker compose restart backend
```

### Login tra `401`

Admin chi duoc tao o lan khoi tao database dau tien. Dung password cu cua volume hoac reset local bang `docker compose down -v`.

### Agent khong gui duoc log

- Kiem tra `http://IP_MAY_CHAY_SIEM:8080/healthz` tu may agent.
- Kiem tra firewall cho TCP `8080`.
- Kiem tra Fleet Server da chay o port `8220` va agent da enroll thanh cong.
- Kiem tra asset da ton tai trong PostgreSQL: `SELECT asset_id, hostname, os_type FROM assets ORDER BY asset_id DESC LIMIT 10;`
- Kiem tra Redis Stream va Elasticsearch theo phan “Xac minh monitoring cap nhat”.

## Test va phat trien

Backend:

```powershell
Push-Location backend
go test ./...
Pop-Location
```

Frontend:

```powershell
Push-Location frontend
npm ci
npm run lint
npm run build
Pop-Location
```

Integration test yeu cau stack healthy va credentials admin khop PostgreSQL volume:

```powershell
docker compose up -d --build postgres redis elasticsearch backend
Push-Location backend
go test -tags=integration ./integration
Pop-Location
```

## Dung moi truong

Giu du lieu:

```powershell
docker compose down
```

Xoa ca du lieu local:

```powershell
docker compose down -v
```

## Cau truc

- `backend/cmd/api`: khoi dong HTTP API, Parser va Rule Engine.
- `backend/cmd/ingest`: producer test doc stdin/file va ghi Redis Stream.
- `backend/cmd/parser`: parser CLI doc lap de debug.
- `backend/internal`: API, auth, ingest, parser, rule engine va storage.
- `backend/migrations`: schema PostgreSQL.
- `config/elastic-agent.yml`: policy thu thap log tập trung cho nhiều service/host.
- `config/logstash-pipeline.conf`: nhận Beats/Elastic Agent input và forward sang backend.
- `frontend`: Next.js dashboard.
- `docker-compose.yml`: PostgreSQL, Redis, Elasticsearch, Logstash, Elastic Agent, backend va frontend.

## Gioi han local

- `POST /api/v1/ingest` chua co API key hoac mTLS, chi phu hop local/demo.
- Dashboard dung polling 10 giay, chua co WebSocket.
- Correlation rule dung Redis counter don gian theo rule/hostname.
- Elasticsearch dang tat security trong Compose local.
