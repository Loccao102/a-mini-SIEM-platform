"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Alert, ApiError, getAlerts, updateAlert } from "@/lib/api";
import { SeverityBadge } from "@/components/severity-badge";
import { BellIcon, RefreshIcon } from "@/components/ui/icons";
import { Button, EmptyState, Notice, PageHeader, PageShell, Panel, StatusPill } from "@/components/ui/primitives";

const STATUS_TABS = [
  { key: "all", label: "Tất cả" },
  { key: "open", label: "Open" },
  { key: "acknowledged", label: "Acknowledged" },
  { key: "closed", label: "Closed" },
] as const;

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
      setMessage(
        error instanceof ApiError && error.status === 401
          ? "Vui lòng đăng nhập để xem danh sách cảnh báo."
          : "Không thể tải danh sách cảnh báo."
      );
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
      setMessage("Không thể cập nhật trạng thái. Yêu cầu quyền Analyst hoặc Admin.");
    }
  }

  const visibleAlerts = filter === "all" ? alerts : alerts.filter((alert) => alert.status === filter);
  const openCount = alerts.filter((a) => a.status === "open").length;

  return (
    <PageShell>
      <PageHeader
        title="Alerts Queue"
        description="Theo dõi cảnh báo an ninh, gom cụm trùng lặp theo thực thể & quy tắc, và cập nhật trạng thái xử lý."
        badge={<StatusPill tone={openCount > 0 ? "danger" : "success"}>{openCount} đang mở</StatusPill>}
        actions={
          <Button variant="secondary" onClick={() => void loadAlerts()} disabled={loading}>
            <RefreshIcon size={14} className={loading ? "animate-spin" : ""} />
            Làm mới
          </Button>
        }
      />

      {message && (
        <Notice tone="error">
          {message}{" "}
          {message.includes("đăng nhập") && (
            <Link href="/login" className="font-semibold underline underline-offset-2">
              Đăng nhập ngay
            </Link>
          )}
        </Notice>
      )}

      <Panel
        noPadding
        actions={
          <div className="flex flex-wrap items-center gap-1 rounded-md border border-border bg-canvas p-0.5">
            {STATUS_TABS.map((tab) => {
              const active = filter === tab.key;
              return (
                <button
                  key={tab.key}
                  type="button"
                  onClick={() => setFilter(tab.key)}
                  className={`cursor-pointer rounded px-2.5 py-1 text-xs font-medium transition-colors ${
                    active ? "bg-surface-raised text-ink" : "text-muted hover:text-ink"
                  }`}
                >
                  {tab.label}
                </button>
              );
            })}
          </div>
        }
        title={`Danh sách cảnh báo (${visibleAlerts.length})`}
      >
        {loading && alerts.length === 0 ? (
          <div className="p-8 text-center font-mono text-xs text-faint">Đang tải cảnh báo...</div>
        ) : visibleAlerts.length === 0 ? (
          <EmptyState
            title="Không có cảnh báo nào phù hợp"
            description="Thử chuyển bộ lọc trạng thái hoặc đợi hệ thống ghi nhận sự kiện mới."
            icon={<BellIcon size={18} />}
          />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead>
                <tr className="border-b border-border bg-surface-raised/60 font-mono text-[11px] uppercase tracking-wider text-muted">
                  <th className="px-5 py-3">ID</th>
                  <th className="px-4 py-3">Mức độ</th>
                  <th className="px-4 py-3">Cảnh báo & Thực thể</th>
                  <th className="px-4 py-3">Cập nhật cuối</th>
                  <th className="px-5 py-3 text-right">Trạng thái</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {visibleAlerts.map((alert) => (
                  <tr key={alert.alert_id} className="transition-colors hover:bg-surface-hover/60">
                    <td className="px-5 py-3.5 font-mono text-muted whitespace-nowrap">#{alert.alert_id}</td>
                    <td className="px-4 py-3.5 whitespace-nowrap">
                      <SeverityBadge severity={alert.severity} />
                    </td>
                    <td className="px-4 py-3.5">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-medium text-ink">{alert.summary}</span>
                        {alert.occurrences && alert.occurrences > 1 ? (
                          <StatusPill tone="accent">{alert.occurrences}x gom cụm</StatusPill>
                        ) : null}
                      </div>
                      <div className="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 font-mono text-[11px] text-faint">
                        <span>Lần đầu: {new Date(alert.triggered_at).toLocaleString("vi-VN")}</span>
                        {alert.entity_key && <span>Entity: {alert.entity_key}</span>}
                      </div>
                    </td>
                    <td className="px-4 py-3.5 font-mono text-muted whitespace-nowrap tabular-nums">
                      {new Date(alert.last_seen ?? alert.triggered_at).toLocaleString("vi-VN")}
                    </td>
                    <td className="px-5 py-3.5 text-right whitespace-nowrap">
                      <select
                        aria-label={`Update alert ${alert.alert_id}`}
                        value={alert.status}
                        onChange={(event) => void changeStatus(alert, event.target.value)}
                        className="soc-select !w-auto min-w-[136px] !py-1.5 font-mono text-xs"
                      >
                        <option value="open">Open</option>
                        <option value="acknowledged">Acknowledged</option>
                        <option value="closed">Closed</option>
                      </select>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Panel>
    </PageShell>
  );
}