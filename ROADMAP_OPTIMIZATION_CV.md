# Mini-SIEM & SOAR: Chiến lược Tối ưu hóa 0 đồng & Nâng tầm CV Đồ án

> **Định hướng chiến lược**: Tối đa hóa giá trị kỹ thuật cho **Đồ án tốt nghiệp** và **Hồ sơ xin việc (CV / Portfolio)** với **CHI PHÍ 0 ĐỒNG**, tối ưu tài nguyên tối đa để chạy mượt trên laptop cá nhân (8GB – 16GB RAM).

---

## 1. Giá trị cốt lõi khi đưa vào CV & Phỏng vấn tuyển dụng

Khi nhà tuyển dụng (SOC Manager / Security Lead / Backend Tech Lead) xem CV của bạn, một dự án SIEM thông thường chỉ dừng lại ở mức "thu thập log và hiển thị biểu đồ". Dự án này sẽ tạo sự khác biệt vượt trội nhờ 4 điểm sáng:

| Điểm nhấn trong CV | Giá trị thực tiễn chứng minh năng lực |
| :--- | :--- |
| **Full-cycle SIEM + SOAR (Closed-loop Defense)** | Không chỉ dừng ở phát hiện (Detection), mà tự động hóa phản ứng (Incident Response: Chặn IP, cô lập tài khoản) có phê duyệt của con người (*Human-in-the-loop*). |
| **Chuẩn hóa MITRE ATT&CK & Threat Intel** | Ánh xạ chính xác các kỹ thuật tấn công (`T1110`, `T1078`, `T1548`) và làm giàu dữ liệu từ nguồn mở 0đ (AbuseIPDB, Tor Exit Nodes, netip LAN isolation). |
| **Báo cáo điều tra sự cố chuẩn SOC (Incident Report)** | Khả năng xuất bản báo cáo điều tra sự cố hoàn chỉnh (Executive & Technical Incident Report) kèm bằng chứng pháp lý và dấu vết kiểm toán (`audit_logs`). |
| **Engineering xuất sắc & Khả năng chịu lỗi (Resilience)** | Thiết kế Idempotency chống trùng lặp, Dead-letter queue, TTL Rollback tự hủy block, Connection Pool tuning chống deadlock, và kiểm thử tự động với CI/CD Quality Gate. |

---

## 2. Tiêu chí thiết kế "0 ĐỒNG & SIÊU NHẸ" (Zero-Cost & Low-Footprint)

1. **Không dùng bất kỳ dịch vụ đám mây hay API trả phí nào**:
   - Sử dụng Threat Feeds mã nguồn mở offline hoặc API miễn phí.
   - Sử dụng Telegram Bot API miễn phí để thông báo cảnh báo và phê duyệt hành động (thay thế cho PagerDuty/Slack Enterprise tốn phí).
   - Tận dụng SQLite/PostgreSQL và Elasticsearch mã nguồn mở cục bộ.
2. **Tối ưu tài nguyên chạy cục bộ trên Laptop**:
   - Khống chế RAM toàn bộ cụm Docker Compose dưới **2.0 GB RAM**:
     - Elasticsearch: JVM cap `-Xms256m -Xmx384m` (vẫn đủ chạy mượt hàng vạn sự kiện).
     - Redis: `maxmemory 128mb` kèm chính sách LRU eviction.
     - PostgreSQL: Cấu hình `shared_buffers = 128MB`.
     - Backend Go: Biên dịch binary Native siêu nhẹ (~20-40MB RAM).
     - Frontend Next.js: Tối ưu hóa bundle và render static.

---

## 3. Các hạng mục triển khai (Phase 4: SOAR & Advanced SOC)

### Hạng mục 1: Module SOAR — Động cơ phản ứng tự động & Phê duyệt (Zero-cost SOAR)
- **Bảng cơ sở dữ liệu**:
  - `playbooks`: Định nghĩa trigger (khi alert severity = critical hoặc category = correlation), điều kiện và danh sách actions.
  - `playbook_executions`: Nhật ký thực thi playbook, trạng thái (`pending_approval`, `approved`, `executed`, `rejected`, `rolled_back`).
  - `blocked_entities`: Danh sách các IP/User đang bị khóa kèm thời gian hết hạn (`expires_at / TTL`).
- **Các hành động phản ứng (Actions)**:
  - `block_ip`: Mô phỏng hoặc thực thi script firewall cục bộ (`iptables`/`netsh`) với cơ chế tự động mở khóa (Auto-Rollback) sau TTL (mặc định 1 giờ) để tránh khóa nhầm.
  - `isolate_user`: Vô hiệu hóa quyền của user đang bị nghi ngờ bị chiếm đoạt.
  - `telegram_approval`: Gửi tin nhắn Telegram kèm liên kết hoặc mã phê duyệt để Analyst duyệt trước khi thực thi hành động nguy hiểm.
- **Cơ chế kiểm soát an toàn**:
  - Danh sách trắng (Allowlist: `127.0.0.1`, dải IP Gateway, máy Analyst) để hệ thống **không bao giờ tự khóa chính mình**.

### Hạng mục 2: Báo cáo sự cố chuyên nghiệp (Incident Report Generator)
- **API `GET /api/v1/cases/{id}/report`**:
  - Tự động kết xuất toàn bộ dữ liệu của vụ việc (Case) thành bản báo cáo hoàn chỉnh (định dạng HTML chuẩn in ấn / Printable View hoặc Markdown):
    1. **Tóm tắt vụ việc (Executive Summary)**: Tiêu đề, mức độ ưu tiên, thời gian phát hiện, thời gian xử lý (MTTD, MTTR).
    2. **Bằng chứng kỹ thuật (Technical Evidence)**: Danh sách IP nguồn, hostname mục tiêu, chuỗi tấn công ATT&CK liên quan.
    3. **Lịch sử điều tra (Investigation Timeline)**: Toàn bộ ghi chú của Analyst theo trục thời gian.
    4. **Hành động khắc phục (Mitigation & Response)**: Các lệnh SOAR đã chạy, tình trạng khóa IP/User.
    5. **Xác nhận của Điều tra viên (Analyst Sign-off & Disposition)**: Đánh dấu `True Positive`, bài học kinh nghiệm và kiến nghị bảo mật.

### Hạng mục 3: Phân tích hành vi bất thường cơ bản (Statistical UEBA 0đ)
- Không cần AI/Machine Learning tốn tiền GPU hay API OpenAI/Gemini trả phí:
  - **Impossible Travel**: Phát hiện 1 tài khoản đăng nhập từ 2 IP có vị trí địa lý cách nhau quá xa trong thời gian ngắn (< 1 giờ).
  - **Off-hours Access**: Phát hiện hoạt động đăng nhập hoặc truy cập dữ liệu nhạy cảm vào khung giờ bất thường (từ 0h đến 5h sáng).

### Hạng mục 4: Tối ưu hóa tài nguyên Docker & Tinh chỉnh hiệu năng
- Thêm `deploy.resources.limits` vào `docker-compose.yml`.
- Tối ưu chỉ mục PostgreSQL và template Elasticsearch để máy cá nhân chạy êm ái, quạt không kêu to, tiết kiệm pin khi demo.

---

## 4. Kịch bản Demo thực chiến ấn tượng khi phỏng vấn / bảo vệ

1. **Bước 1 (Giả lập tấn công)**:
   Chạy 1 lệnh script giả lập Brute Force SSH liên tục vào máy chủ từ một IP lạ.
2. **Bước 2 (SIEM phát hiện)**:
   Pipeline nhận log qua Elastic Agent → Ingest xác thực API Key → Redis → Parser phân tích → Correlation Engine xâu chuỗi phát hiện đợt tấn công mức `critical` → Alert tự động sinh ra trên Dashboard và Telegram.
3. **Bước 3 (SOAR phản ứng có kiểm soát)**:
   Playbook kích hoạt: Trạng thái chuyển sang `pending_approval`, Analyst nhận thông báo phê duyệt → Bấm Duyệt → Hệ thống đưa IP vào danh sách `blocked_entities`, ghi nhật ký kiểm toán.
4. **Bước 4 (Đóng Case & Xuất Báo cáo)**:
   Analyst mở Case trên giao diện Web, gắn kết luận `True Positive`, bấm **Export Incident Report** tải về file báo cáo đầy đủ chứng cứ để nộp cho cấp trên / hội đồng chấm điểm.

