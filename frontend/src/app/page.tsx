"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Alert, AnalyticsData, ApiError, DashboardSummary, getAlerts, getAnalytics, getSummary } from "@/lib/api";
import { AnalyticsCharts } from "@/components/analytics-charts";
import { SeverityBadge } from "@/components/severity-badge";

function formatDate(value: string) {
  return new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(new Date(value));
}

function formatNumber(value: number) {
  return new Intl.NumberFormat().format(value);
}

export default function Home() {
  const [summary, setSummary] = useState<DashboardSummary | null>(null);
  const [analytics, setAnalytics] = useState<AnalyticsData | null>(null);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const today = new Intl.DateTimeFormat(undefined, { dateStyle: "full" }).format(new Date());

  useEffect(() => {
    let active = true;
    const refresh = () =>
      Promise.all([getSummary(), getAlerts(), getAnalytics()])
        .then(([nextSummary, nextAlerts, nextAnalytics]) => {
          if (!active) return;
          setSummary(nextSummary);
          setAlerts(nextAlerts.slice(0, 4));
          setAnalytics(nextAnalytics);
        })
        .catch((reason: unknown) => {
          if (!active) return;
          if (reason instanceof ApiError && reason.status === 401) setError("Sign in to view live security data.");
          else setError("The security data could not be loaded.");
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
    <main className="shell">
      <section className="content">
        <p className="eyebrow" suppressHydrationWarning>
          {today}
        </p>
        <div className="dashboard-title-row"><h1>Security Command Center</h1><span className="live-readout"><span className="pulse-dot" /> Auto-refresh 5s</span></div>
        <p className="lede">Theo dõi liên tục các chỉ số an ninh, tín hiệu đe dọa GeoIP và bức tranh tổng quan sự cố SOC.</p>

        {error && (
          <div className="notice mb-6">
            {error} <Link href="/login">Đăng nhập ngay</Link>
          </div>
        )}

        <div className="metrics">
          <article>
            <span>Open Alerts</span>
            <strong>{loading ? "-" : formatNumber(summary?.open_alerts ?? 0)}</strong>
            <small className="red">Current Queue</small>
          </article>
          <article>
            <span>Events Processed</span>
            <strong>{loading ? "-" : formatNumber(summary?.events_processed ?? 0)}</strong>
            <small>Indexed Log Events</small>
          </article>
          <article>
            <span>Connected Assets</span>
            <strong>{loading ? "-" : formatNumber(summary?.connected_assets ?? 0)}</strong>
            <small className="green">Reporting Nodes</small>
          </article>
        </div>

        {/* Analytics Charts Component (Mục 6: Event Velocity, Severity & GeoIP Threat Matrix) */}
        <AnalyticsCharts data={analytics} />

        {/* Priority Alerts Queue */}
        <section className="panel mt-12">
          <div className="panel-heading">
            <div>
              <p className="eyebrow">Priority Queue</p>
              <h2>Recent Alerts</h2>
            </div>
            <Link href="/alerts">
              View all alerts <span>-&gt;</span>
            </Link>
          </div>

          {loading && <p className="empty-state">Loading alerts...</p>}
          {!loading && !alerts.length && <p className="empty-state">No alerts have been triggered.</p>}
          {alerts.map((alert) => (
            <div className="alert-row" key={alert.alert_id}>
              <SeverityBadge severity={alert.severity} />
              <div>
                <strong>{alert.summary}</strong>
                <small>
                  {formatDate(alert.triggered_at)} · Status: {alert.status}{" "}
                  {alert.occurrences && alert.occurrences > 1 ? `· ⚡ ${alert.occurrences}x aggregated` : ""}
                </small>
              </div>
              <span className="arrow">-&gt;</span>
            </div>
          ))}
        </section>
      </section>
    </main>
  );
}
