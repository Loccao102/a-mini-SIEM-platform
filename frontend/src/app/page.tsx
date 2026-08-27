"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Alert, ApiError, DashboardSummary, getAlerts, getSummary } from "@/lib/api";

function formatDate(value: string) {
  return new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(new Date(value));
}

function formatNumber(value: number) {
  return new Intl.NumberFormat().format(value);
}

export default function Home() {
  const [summary, setSummary] = useState<DashboardSummary | null>(null);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const today = new Intl.DateTimeFormat(undefined, { dateStyle: "full" }).format(new Date());

  useEffect(() => {
    let active = true;
    const refresh = () => Promise.all([getSummary(), getAlerts()])
      .then(([nextSummary, nextAlerts]) => { if (!active) return; setSummary(nextSummary); setAlerts(nextAlerts.slice(0, 3)); })
      .catch((reason: unknown) => {
        if (!active) return;
        if (reason instanceof ApiError && reason.status === 401) setError("Sign in to view live security data.");
        else setError("The security data could not be loaded.");
      })
      .finally(() => { if (active) setLoading(false); });
    const timer = window.setTimeout(refresh, 0);
    const interval = window.setInterval(refresh, 3000);
    return () => { active = false; window.clearTimeout(timer); window.clearInterval(interval); };
  }, []);

  return (
    <main className="shell">
      <header className="topbar"><div><span className="mark">S</span><strong>Sentinel</strong><span className="muted"> / command center</span></div><span className="status"><span className="dot" /> {error ? "Data unavailable" : "API connected"}</span></header>
      <section className="content"><p className="eyebrow" suppressHydrationWarning>{today}</p><h1>Security overview</h1><p className="lede">A quiet view of the signals that need your attention.</p>
        {error && <div className="notice">{error} <Link href="/accounts">Sign in</Link></div>}
        <div className="metrics"><article><span>Open alerts</span><strong>{loading ? "-" : formatNumber(summary?.open_alerts ?? 0)}</strong><small className="red">Current queue</small></article><article><span>Events processed</span><strong>{loading ? "-" : formatNumber(summary?.events_processed ?? 0)}</strong><small>Indexed events</small></article><article><span>Connected assets</span><strong>{loading ? "-" : formatNumber(summary?.connected_assets ?? 0)}</strong><small className="green">Reporting assets</small></article></div>
        <section className="panel"><div className="panel-heading"><div><p className="eyebrow">Priority queue</p><h2>Recent alerts</h2></div><Link href="/alerts">View all alerts <span>-&gt;</span></Link></div>{loading && <p className="empty-state">Loading alerts...</p>}{!loading && !alerts.length && <p className="empty-state">No alerts have been triggered.</p>}{alerts.map((alert) => <div className="alert-row" key={alert.alert_id}><span className={`severity ${alert.severity}`}>{alert.severity}</span><div><strong>{alert.summary}</strong><small>{formatDate(alert.triggered_at)} · {alert.status}</small></div><span className="arrow">-&gt;</span></div>)}</section>
      </section><nav className="rail"><Link className="rail-active" href="/">Overview</Link><Link href="/alerts">Alerts</Link><Link href="/events">Log explorer</Link><Link href="/rules">Rules</Link><Link href="/assets">Assets</Link><Link href="/accounts">Accounts</Link></nav>
    </main>
  );
}
