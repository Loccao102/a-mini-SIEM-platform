# 🛠️ Cẩm Nang Phát Triển & Lộ Trình Tiến Hóa Hệ Thống (Development & Architecture Evolution)

Tài liệu này đóng vai trò là kim chỉ nam kỹ thuật (Technical Blueprint) cho các kỹ sư muốn tham gia phát triển, tùy biến hoặc mở rộng hệ thống **Sentinel (Mini-SIEM & SOAR)** từ quy mô hiện tại (PoC / 20-50 nodes) lên quy mô doanh nghiệp lớn (Enterprise Cloud-Native / Hàng nghìn nodes, hàng triệu EPS).

---

## 1. Triết Lý Thiết Kế (Design Philosophy)

Hệ thống Sentinel được thiết kế theo phương châm:
1. **Fit-for-Purpose (Tối ưu cho mục tiêu hiện tại)**: Vận hành trơn tru toàn bộ pipeline SIEM + SOAR trên máy trạm cấu hình tối thiểu (<1GB RAM, 0 đồng chi phí bản quyền/hạ tầng).
2. **Modular Decoupling (Tháo lắp linh hoạt)**: Áp dụng kiến trúc Clean Architecture & Event-Driven. Mỗi tầng (Ingest, Queue, Parser, Correlation, Storage, SOAR) đều được phân tách ranh giới rõ ràng thông qua Go Interfaces và REST/Streaming contracts.
3. **Pluggable Architecture**: Khi tải hệ thống vượt ngưỡng, bất kỳ thành phần nào cũng có thể được "rút phích cắm" và thay thế bằng giải pháp Big Data chuyên dụng mà **không phải đập đi xây lại toàn bộ codebase**.

---

## 2. Ma Trận Thay Thế & Nâng Cấp Module (Component Evolution Matrix)

Bảng tổng quan so sánh công nghệ hiện tại và phương án nâng cấp khi hệ thống scale lớn:

| Phân hệ (Subsystem) | Hiện tại (Minimal / PoC) | Phương án Nâng cấp (Enterprise Scale) | Khi nào cần thay thế? |
| :--- | :--- | :--- | :--- |
| **Hàng đợi đệm (Message Queue)** | **Redis Streams** | **Apache Kafka** hoặc **Redpanda** | Tải $> 50,000 \text{ EPS}$, cần lưu log trên đĩa nhiều ngày để replay |
| **Điều phối (Orchestration)** | **Docker Compose** | **Kubernetes (K8s / K3s) + KEDA** | Cần Auto-scaling tự động 100%, Multi-node cluster, High Availability |
| **Kho lưu trữ log (Datastore)** | **Elasticsearch 8** | **ClickHouse** hoặc **OpenSearch** | Log $> 10 \text{ triệu/ngày}$, muốn giảm 80% dung lượng đĩa và RAM |
| **Phân tích luồng (Stream Engine)** | **Go In-Memory Rules** | **Apache Flink** / **Vector / Benthos** | Cần Complex Event Processing (CEP) trên sliding windows phân tán |
| **Định danh & Truy cập (IAM)** | **Local JWT + Postgres** | **Keycloak / Authentik (OIDC/SAML)** | Cần SSO doanh nghiệp (Google Workspace, Active Directory, Azure AD) |
| **Giao thức Ingestion (Transport)** | **HTTP REST / JSON** | **gRPC / Protocol Buffers** hoặc **OTel** | Muốn giảm 60% băng thông mạng và chi phí đóng/mở kết nối TLS |
| **Threat Intelligence (CTI)** | **Local JSON Feed + Cache**| **MISP / OpenCTI (STIX/TAXII)** | Tự động đồng bộ CTI feeds toàn cầu từ các cơ quan an ninh mạng |

---

## 3. Phân Tích Kỹ Thuật Chi Tiết Các Hướng Thay Thế

### 🔄 3.1. Hàng đợi đệm: `Redis Streams` $\longrightarrow$ `Apache Kafka` / `Redpanda`

* **Hiện tại**: `siem:raw-logs` dùng Redis Streams với `XADD ... MAXLEN ~ 50000`.
  * *Ưu điểm*: Chiếm $<30\text{MB}$ RAM, độ trễ $<1\text{ms}$, hỗ trợ Consumer Groups chia việc cho nhiều parser workers.
  * *Hạn chế*: Lưu trên RAM nên giới hạn dung lượng đệm nếu parser bị nghẽn trong thời gian dài.
* **Kế hoạch chuyển đổi**:
  * Tách module queue trong Go: Định nghĩa interface `QueueProducer` và `QueueConsumer`.
  * Sử dụng thư viện `segmentio/kafka-go` để triển khai adapter Kafka/Redpanda.
  * *Khuyến nghị công nghệ*: Ưu tiên dùng **Redpanda** thay vì Apache Kafka truyền thống. Redpanda viết bằng C++, tương thích 100% Kafka API nhưng không cần chạy JVM/Zookeeper, tiết kiệm RAM gấp 3 lần và khởi động chỉ mất 2 giây.

```go
// Ví dụ Interface trừu tượng hóa Queue:
type QueueProducer interface {
    Publish(ctx context.Context, topic string, payload []byte) error
}

type QueueConsumer interface {
    Subscribe(ctx context.Context, topic string, group string, handler func(msg []byte) error) error
}
```

---

### 🔄 3.2. Điều phối hạ tầng: `Docker Compose` $\longrightarrow$ `Kubernetes` + `KEDA`

* **Hiện tại**: File [docker-compose.yml](file:///c:/Users/Admin/a-mini-SIEM-platform/docker-compose.yml) và [docker-compose.scale.yml](file:///c:/Users/Admin/a-mini-SIEM-platform/docker-compose.scale.yml) cho phép scale thủ công `docker compose ... up -d --scale backend=3`.
* **Kế hoạch chuyển đổi lên Cloud-Native K8s**:
  1. **Deployments**:
     - `siem-backend`: Stateless Deployment chạy nhiều Replicas.
     - `siem-frontend`: Next.js SSR Deployment.
     - `siem-gateway`: Ingress Nginx Controller có cert-manager tự cấp chứng chỉ HTTPS.
  2. **Event-Driven Auto-Scaling (KEDA)**:
     - Cài đặt KEDA trên cụm Kubernetes.
     - Cấu hình `ScaledObject` theo dõi Redis Stream lag (hoặc Kafka consumer lag):
       - Khi hàng đợi pending logs $> 2,000$: KEDA tự động tăng backend pods từ 2 lên 10 pods.
       - Khi hàng đợi về 0: KEDA tự thu hồi về 2 pods để tiết kiệm tài nguyên cloud.
  3. **Khuyến nghị môi trường tối thiểu**: Bắt đầu bằng **K3s** (Lightweight Kubernetes do Rancher phát triển) chỉ tốn ~512MB RAM cho Control Plane.

---

### 🔄 3.3. Kho lưu trữ Log: `Elasticsearch` $\longrightarrow$ `ClickHouse`

* **Hiện tại**: Elasticsearch 8 lưu trữ index theo ngày `siem-events-YYYY.MM.DD` với ILM Retention.
  * *Hạn chế*: Elasticsearch dựa trên JVM và Inverted Index, tốn khá nhiều RAM (~700MB - 1GB) và đĩa phình to khi log lớn.
* **Kế hoạch chuyển đổi sang ClickHouse**:
  * ClickHouse là DBMS dạng cột (Columnar Database) nhanh nhất thế giới hiện nay cho việc ghi nhận và truy vấn logs thời gian thực.
  * *Tỷ lệ nén*: Tỷ lệ nén dữ liệu từ $5:1$ đến $10:1$ (100GB raw log chỉ tốn 10-15GB SSD).
  * *Tốc độ truy vấn*: Nhanh gấp 10-50 lần Elasticsearch đối với các truy vấn tổng hợp SOC (Top 10 IP tấn công, lượng log theo giờ, thống kê brute-force).
  * *Cơ chế triển khai*: Tạo bảng với engine `MergeTree` hoặc `ReplacingMergeTree` phân vùng theo ngày (`PARTITION BY toYYYYMMDD(timestamp)`).

---

### 🔄 3.4. Giao thức thu thập: `HTTP REST/JSON` $\longrightarrow$ `gRPC / Protobuf` & `OpenTelemetry`

* **Hiện tại**: Agent gửi log qua HTTP POST `/api/v1/ingest` định dạng JSON.
* **Kế hoạch chuyển đổi**:
  1. **gRPC Streaming**:
     - Định nghĩa file `siem.proto` cho thông điệp log.
     - Giảm kích thước payload đến **60%** so với chuỗi JSON thô.
     - Duy trì kết nối HTTP/2 persistent connection, loại bỏ chi phí handshake TCP/TLS cho mỗi batch log.
  2. **OpenTelemetry (OTel) Collector**:
     - Tích hợp endpoint nhận dữ liệu chuẩn OTLP (OpenTelemetry Protocol).
     - Cho phép nhận log trực tiếp từ Kubernetes DaemonSets, Fluent Bit, AWS CloudWatch, hoặc Azure Monitor mà không cần viết custom agent.

---

## 4. Hướng Dẫn Thiết Lập Môi Trường Phát Triển Cục Bộ (Local Dev Setup)

Dành cho nhà phát triển muốn chạy debug trực tiếp mã nguồn trên máy:

### 4.1. Khởi động các dịch vụ phụ trợ (Dependencies only)
Thay vì chạy toàn bộ stack bằng Docker, chỉ khởi động các database và queue:
```bash
docker compose up -d postgres redis elasticsearch
```

### 4.2. Chạy Backend Go (Hot-reload / Debug)
```bash
cd backend

# Tải dependencies
go mod download

# Chạy trực tiếp Backend API Server
go run ./cmd/api/main.go

# Chạy Ingest & Parser Worker (nếu chạy phân tán)
go run ./cmd/ingest/main.go
go run ./cmd/parser/main.go
```

### 4.3. Chạy Frontend Next.js
```bash
cd frontend

# Cài đặt dependencies
npm install

# Chạy server phát triển Next.js với Turbopack
npm run dev
# Mở trình duyệt tại http://localhost:3000
```

---

## 5. Quy Chuẩn Kiểm Thử & Đóng Góp Mã Nguồn (Testing & Quality Gates)

Trước khi commit bất kỳ tính năng hoặc refactor nào, mã nguồn **bắt buộc** phải vượt qua bộ kiểm chuẩn sau:

```bash
# 1. Kiểm tra tĩnh Backend Go
cd backend
go vet ./...

# 2. Chạy toàn bộ Unit Tests & Data Race Detector
go test -race -count=1 ./...

# 3. Chạy Integration Tests xác thực pipeline thực tế (yêu cầu Docker)
go test -v -tags=integration ./integration

# 4. Kiểm tra Lint & Type Check Frontend
cd ../frontend
npm run lint
npx tsc --noEmit

# 5. Build kiểm tra Production Frontend
npm run build
```

---

## 6. Lộ Trình Tính Năng Tương Lai (Roadmap Milestones)

- [ ] **v1.1 (Detection as Code)**: Hỗ trợ import/export Sigma Rules trực tiếp (Sigma format $\rightarrow$ Go regex rule).
- [ ] **v1.2 (Multi-Tenancy)**: Tách biệt dữ liệu log và alert giữa các chi nhánh / khách hàng (Mỗi Tenant một không gian lưu trữ và dashboard riêng).
- [ ] **v1.3 (eBPF Agent)**: Bổ sung agent giám sát nhân Linux bằng eBPF (BumbleBee / Cilium Tetragon) để phát hiện mã độc rootkit ngay tại tầng kernel.
- [ ] **v1.4 (AI SOC Copilot)**: Tích hợp mô hình ngôn ngữ lớn cục bộ (Ollama / Llama-3) để tự động tóm tắt chuỗi sự kiện điều tra và đề xuất bước khắc phục sự cố.
