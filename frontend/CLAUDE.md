@AGENTS.md
Hướng dẫn cho AI Agent làm việc trên repo này

Đọc PROJECT_OVERVIEW.md trước khi bắt đầu bất kỳ task nào — file đó chứa đầy đủ bối cảnh kiến trúc và lý do lựa chọn công nghệ.

Quy tắc bắt buộc
Không đổi công nghệ nền tảng đã chốt: Redis Streams, Elasticsearch, PostgreSQL, Go, Next.js, Docker Compose. Nếu thấy có lựa chọn "tốt hơn", vẫn hỏi trước khi đổi, không tự ý sửa.
Không thêm Kubernetes/ArgoCD/Kafka vào code hoặc gợi ý deploy — nằm ngoài phạm vi đồ án hiện tại.
Không tích hợp Kibana — dashboard luôn là Next.js tự viết, gọi qua Go REST API.
Khi thêm loại log mới (ngoài Windows/Linux), field đặc thù đi vào extra_fields (JSONB/object), không thêm cột cứng vào schema Elasticsearch hay Postgres.
Backend Go giữ nguyên cấu trúc 1 binary, 3 module nội bộ (parser, ruleengine, api) chạy bằng goroutine — không tách thành nhiều service/container riêng trừ khi được yêu cầu.
Mọi thay đổi phải tương thích ngược với docker-compose.yml — nếu thêm service mới, cập nhật luôn file này.
Ưu tiên code dễ đọc, dễ giải thích trong báo cáo đồ án hơn là tối ưu hiệu năng cực hạn.
Khi không chắc chắn

Nếu yêu cầu không rõ ràng hoặc có vẻ mâu thuẫn với PROJECT_OVERVIEW.md, hỏi lại thay vì tự suy diễn hướng đi khác.