# A mini SIEM platform

Nen tang SIEM local gom:

```text
Filebeat / Winlogbeat -> HTTP ingest -> Redis Stream -> Parser -> Elasticsearch
                                                       -> Rule Engine -> PostgreSQL alerts
                                                               -> Next.js dashboard
```

Dashboard doc du lieu that tu API Go. Overview, Alerts va Log Explorer tu tai lai moi 3 giay.

## Yeu cau

- Docker Desktop dang chay
- Docker Compose v2
- PowerShell 5.1 hoac PowerShell 7
- Cac port `3000`, `5432`, `6379`, `8080`, `9200`, `5044` chua bi chiem

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
```

`ADMIN_PASSWORD` chi duoc dung de tao admin lan dau. Neu PostgreSQL volume da ton tai, doi bien nay khong doi password cu.

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

## Cai theo doi may Windows

Dùng Winlogbeat tren may Windows can theo doi. Backend Docker va service `logstash` phai dang chay. Winlogbeat gui Beats protocol toi port `5044`; Logstash moi gui HTTP toi backend.

1. Cai Winlogbeat tu Elastic.
2. Chep `config/winlogbeat.yml` vao thu muc Winlogbeat. Khong dung ban cu co `output.http`; Winlogbeat khong ho tro output nay.
3. Neu Logstash chay tren cung may, giu mac dinh `localhost`. Neu Docker chay tren may khac, sua host trong config:

```yaml
output.logstash:
  hosts: ["IP_MAY_CHAY_SIEM:5044"]
```

4. Kiem tra cau hinh trong PowerShell Administrator:

```powershell
.\winlogbeat.exe test config -c .\winlogbeat.yml
```

5. Neu service da ton tai nhu ban dang co, chi can nap lai config va khoi dong lai:

```powershell
Restart-Service winlogbeat
Get-Service winlogbeat
```

Neu may bao khong tim thay service, chay PowerShell Administrator tai thu muc Winlogbeat:

```powershell
.\install-service-winlogbeat.ps1
Start-Service winlogbeat
```

6. Xem log neu chua gui duoc:

```powershell
Get-Content 'C:\ProgramData\winlogbeat\Logs\winlogbeat' -Tail 100
```

Config doc Windows `Security` va `System`, gui `source_type=windows_eventlog`, `host.name` va `agent.id` toi Logstash. Khoi dong bridge bang `docker compose up -d logstash`.

Winlogbeat phai ket noi duoc toi `IP_MAY_CHAY_SIEM:5044`. Khong gui Winlogbeat truc tiep toi port `8080`, vi port do nhan HTTP tu Logstash.

## Cai theo doi may Linux

Dùng Filebeat tren may Linux can theo doi. Chep `config/filebeat.yml` vao Filebeat va sua dia chi Logstash neu Docker chay tren may khac:

```yaml
output.logstash:
  hosts: ["IP_MAY_CHAY_SIEM:5044"]
```

Config doc `/var/log/auth.log` va `/var/log/syslog`, sau do gui `source_type=linux_sshd` toi API.

```bash
sudo filebeat test config -c /etc/filebeat/filebeat.yml
sudo systemctl enable --now filebeat
sudo systemctl status filebeat
sudo journalctl -u filebeat -f
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
- Kiem tra agent dang dung dung file config.
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
- `config/filebeat.yml`: theo doi log Linux.
- `config/winlogbeat.yml`: theo doi Windows Event Log.
- `frontend`: Next.js dashboard.
- `docker-compose.yml`: PostgreSQL, Redis, Elasticsearch, backend va frontend.

## Gioi han local

- `POST /api/v1/ingest` chua co API key hoac mTLS, chi phu hop local/demo.
- Dashboard dung polling 10 giay, chua co WebSocket.
- Correlation rule dung Redis counter don gian theo rule/hostname.
- Elasticsearch dang tat security trong Compose local.
