"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Alert, ApiError, getAlerts, updateAlert } from "@/lib/api";

export default function AlertsPage() {
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [filter, setFilter] = useState("all");
  const [message, setMessage] = useState("");
  const [loading, setLoading] = useState(true);

  async function loadAlerts() {
    setLoading(true);
    try {
      setAlerts(await getAlerts());
      setMessage("");
    } catch (error) {
      setMessage(error instanceof ApiError && error.status === 401 ? "Sign in to view alerts." : "Alerts could not be loaded.");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    const timer = window.setTimeout(() => void loadAlerts(), 0);
    const interval = window.setInterval(() => void loadAlerts(), 3000);
    return () => {
      window.clearTimeout(timer);
      window.clearInterval(interval);
    };
  }, []);

  async function changeStatus(alert: Alert, status: string) {
    try {
      await updateAlert(alert.alert_id, status, alert.assigned_to ?? "");
      await loadAlerts();
    } catch {
      setMessage("This alert could not be updated. Analyst or Admin access required.");
    }
  }

  const visibleAlerts = filter === "all" ? alerts : alerts.filter((alert) => alert.status === filter);

  return (
    <main className="account-shell">
      <header className="account-header">
        <Link href="/">← Overview</Link>
        <span className="eyebrow">Response Queue & Alert Storm Control</span>
        <h1>Alerts</h1>
        <p>Theo dõi các cảnh báo an ninh, tự động gom cụm (Deduplication) theo Entity & Rule, và cập nhật trạng thái xử lý incident.</p>
      </header>

      <section className="data-page">
        <div className="toolbar">
          <label>
            Trạng thái (Status)
            <select value={filter} onChange={(event) => setFilter(event.target.value)}>
              <option value="all">Tất cả trạng thái</option>
              <option value="open">Open (Đang mở)</option>
              <option value="acknowledged">Acknowledged (Đã tiếp nhận)</option>
              <option value="closed">Closed (Đã đóng)</option>
            </select>
          </label>
          <button type="button" onClick={() => void loadAlerts()}>Làm mới</button>
        </div>

        {message && (
          <p className="notice">
            {message} {message.includes("Sign in") && <Link href="/login">Đăng nhập ngay</Link>}
          </p>
        )}
        {loading && <p className="empty-state">Đang tải cảnh báo...</p>}
        {!loading && !visibleAlerts.length && <p className="empty-state">Không có cảnh báo nào phù hợp với bộ lọc.</p>}

        {visibleAlerts.map((alert) => (
          <article className="data-row" key={alert.alert_id}>
            <div>
              <div className="flex items-center gap-2">
                <span className={`severity ${alert.severity}`}>{alert.severity}</span>
                {alert.occurrences && alert.occurrences > 1 ? (
                  <span className="dedup-tag">⚡ {alert.occurrences}x aggregated</span>
                ) : null}
              </div>
              <h2>{alert.summary}</h2>
              <small>
                Lần đầu: {new Date(alert.triggered_at).toLocaleString()} · Cập nhật cuối:{" "}
                {alert.last_seen ? new Date(alert.last_seen).toLocaleString() : new Date(alert.triggered_at).toLocaleString()} · ID #{alert.alert_id}{" "}
                {alert.entity_key ? `· Target Entity: ${alert.entity_key}` : ""}
              </small>
            </div>

            <div className="row-meta">
              <span>{alert.status}</span>
              <select
                aria-label={`Update alert ${alert.alert_id}`}
                value={alert.status}
                onChange={(event) => void changeStatus(alert, event.target.value)}
              >
                <option value="open">Open</option>
                <option value="acknowledged">Acknowledged</option>
                <option value="closed">Closed</option>
              </select>
            </div>
          </article>
        ))}
      </section>
    </main>
  );
}