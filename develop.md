# SIEM Platform Development Roadmap

Bản định hướng và kế hoạch phát triển mở rộng hệ thống Mini SIEM Platform lên tầm Enterprise SIEM/SOAR.

---

## 🚀 6 Hướng phát triển chiến lược

### 1. Threat Intelligence & GeoIP Enrichment (Làm giàu dữ liệu sự kiện)
- **GeoIP Lookup**: Tự động xác định Quốc gia, Thành phố, Cờ quốc gia (Country Flag) từ `src_ip`.
- **Threat Intel Feeds**: Tra cứu danh tiếng IP (`src_ip`) đối soát với dữ liệu AbuseIPDB, VirusTotal, Tor Exit Nodes, C2 Botnet. Gắn nhãn độc hại (`is_malicious`, `reputation_score`) và tự động nâng Severity lên `CRITICAL`.

### 2. Multi-Stage Attack Chain Correlation Rules (Chuỗi tấn công liên hoàn)
- Phát hiện kịch bản tấn công theo chuỗi hành vi (Kill Chain):
  `SSH Brute Force` ➔ `SSH Login Success` ➔ `Sudo Privilege Escalation`.
- Tự động kích hoạt Alert tổng hợp `CRITICAL: Full System Compromise Detected`.

### 3. SOAR - Automated Response & Playbooks (Phản ứng sự cố tự động)
- **1-Click / Auto Block Attacker IP**: Nút bấm trên giao diện Alert để khóa IP qua `iptables`, `firewall` hoặc Cloudflare WAF API.
- **Auto Account Lockout**: Tự động vô hiệu hóa tài khoản khi phát hiện hành vi leo thang đặc quyền bất thường.

### 4. SOC Case Management & Audit Trail (Quản lý incident & Nhật ký thao tác)
- **Timeline & Case Notes**: Ghi chép tiến trình điều tra của Analyst, gắn nhãn `False Positive`, `True Positive`, `Resolved`.
- **Audit Logs**: Ghi nhật ký thao tác người dùng (ai sửa luật, ai vô hiệu hóa tài khoản, ai đóng alert).

### 5. High Throughput & Batch Pipeline Scaling (Tối ưu hóa hiệu năng)
- **Batch Bulk Indexing**: Gom log ghi theo lô sang Elasticsearch (`_bulk` API) tăng tốc độ ghi > 10.000 events/sec.
- **Go Worker Pool**: Mở rộng Parser Consumer xử lý đa luồng song song trên CPU multi-core.

### 6. Advanced Analytics & Chart Visualizations (Trực quan hóa chỉ số)
- Biểu đồ xu hướng sự kiện theo mốc thời gian (Event Trend Line Chart).
- Biểu đồ phân bố loại tấn công (SQLi vs SSH vs LFI vs Scanners).
- Biểu đồ Top 5 IP tấn công nhiều nhất (Top Attacking IPs).
- Bản đồ nguồn tấn công GeoIP và xuất báo cáo an ninh.

---
