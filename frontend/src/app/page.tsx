"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Alert, AnalyticsData, ApiError, DashboardSummary, getAlerts, getAnalytics, getSummary } from "@/lib/api";
import { AnalyticsCharts } from "@/components/analytics-charts";
import { SeverityBadge } from "@/components/severity-badge";
import { ActivityIcon, ArrowUpRightIcon, BellIcon, ServerIcon } from "@/components/ui/icons";
import { EmptyState, KpiCard, Notice, PageHeader, PageShell, Panel, StatusPill } from "@/components/ui/primitives";

function formatDate(value: string) {
  return new Intl.DateTimeFormat("vi-VN", { dateStyle: "short", timeStyle: "short" }).format(new Date(value));
}

function formatNumber(value: number) {
  return new Intl.NumberFormat("vi-VN").format(value);
}

export default function Home() {
  const [summary, setSummary] = useState<DashboardSummary | null>(null);
  const [analytics, setAnalytics] = useState<AnalyticsData | null>(null);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let active = true;
    const refresh = () =>
      Promise.all([getSummary(), getAlerts(), getAnalytics()])
        .then(([nextSummary, nextAlerts, nextAnalytics]) => {
          if (!active) return;
          setSummary(nextSummary);
          setAlerts(nextAlerts.slice(0, 6));
          setAnalytics(nextAnalytics);
          setError("");
        })
        .catch((reason: unknown) => {
          if (!active) return;
          if (reason instanceof ApiError && reason.status === 401) setError("Vui lòng đăng nhập để xem dữ liệu an ninh trực tiếp.");
          else setError("Không thể tải dữ liệu an ninh từ máy chủ API.");
        })
        .finally(() => {
          if (active) setLoading(false);
        });

    const timer = window.setTimeout(refresh, 0);
    const interval = window.setInterval(refresh, 5000);
    return () => {
      active = false;
      window.clearTimeout(timer);
      window.clearInterval(interval);
    };
  }, []);

  return (
    <PageShell>
      <PageHeader
        title="Security Command Center"
        description="Giám sát chỉ số an ninh thời gian thực, tín hiệu đe dọa GeoIP và hàng đợi cảnh báo SOC."
        actions={
          <StatusPill tone="success">
            <span className="soc-pulse h-1.5 w-1.5 rounded-full bg-emerald-400" />
            Auto-refresh 5s
          </StatusPill>
        }
      />

      {error && (
        <Notice tone="error">
          {error}{" "}
          <Link href="/login" className="font-semibold underline underline-offset-2">
            Đăng nhập ngay
          </Link>
        </Notice>
      )}

      {/* KPI Strip */}
      <section className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <KpiCard
          label="Cảnh báo đang mở"
          value={loading ? "—" : formatNumber(summary?.open_alerts ?? 0)}
          subtext="Hàng đợi cần xử lý"
          tone={(summary?.open_alerts ?? 0) > 0 ? "critical" : "default"}
          icon={<BellIcon size={16} />}
        />
        <KpiCard
          label="Sự kiện đã xử lý"
          value={loading ? "—" : formatNumber(summary?.events_processed ?? 0)}
          subtext="Log đã chuẩn hóa & lập chỉ mục"
          tone="accent"
          icon={<ActivityIcon size={16} />}
        />
        <KpiCard
          label="Tài sản đang kết nối"
          value={loading ? "—" : formatNumber(summary?.connected_assets ?? 0)}
          subtext="Node đang gửi telemetry"
          tone="success"
          icon={<ServerIcon size={16} />}
        />
      </section>

      {/* Analytics Charts */}
      <AnalyticsCharts data={analytics} />

      {/* Priority Alerts Queue */}
      <Panel
        title="Hàng đợi cảnh báo ưu tiên"
        subtitle="Các cảnh báo mới nhất cần chuyên viên SOC kiểm tra"
        actions={
          <Link
            href="/alerts"
            className="inline-flex items-center gap-1 font-mono text-xs font-medium text-accent transition-colors hover:text-accent-hover"
          >
            Xem tất cả <ArrowUpRightIcon size={14} />
          </Link>
        }
        noPadding
      >
        {loading ? (
          <div className="p-8 text-center font-mono text-xs text-faint">Đang tải cảnh báo...</div>
        ) : alerts.length === 0 ? (
          <EmptyState
            title="Chưa có cảnh báo nào được kích hoạt"
            description="Hệ thống sẽ tự động hiển thị cảnh báo ngay khi quy tắc phát hiện khớp với log."
            icon={<BellIcon size={18} />}
          />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead>
                <tr className="border-b border-border bg-surface-raised/60 font-mono text-[11px] uppercase tracking-wider text-muted">
                  <th className="px-5 py-3">Mức độ</th>
                  <th className="px-4 py-3">Nội dung cảnh báo</th>
                  <th className="px-4 py-3">Trạng thái</th>
                  <th className="px-5 py-3 text-right">Thời gian</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {alerts.map((alert) => (
                  <tr key={alert.alert_id} className="transition-colors hover:bg-surface-hover/60">
                    <td className="px-5 py-3 whitespace-nowrap">
                      <SeverityBadge severity={alert.severity} />
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-medium text-ink">{alert.summary}</span>
                        {alert.occurrences && alert.occurrences > 1 ? (
                          <StatusPill tone="accent">{alert.occurrences}x gom cụm</StatusPill>
                        ) : null}
                      </div>
                    </td>
                    <td className="px-4 py-3 whitespace-nowrap">
                      <StatusPill
                        tone={
                          alert.status === "closed"
                            ? "success"
                            : alert.status === "acknowledged"
                            ? "warning"
                            : "neutral"
                        }
                      >
                        {alert.status}
                      </StatusPill>
                    </td>
                    <td className="px-5 py-3 text-right font-mono text-muted whitespace-nowrap tabular-nums">
                      {formatDate(alert.triggered_at)}
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
