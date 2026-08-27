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
    try { setAlerts(await getAlerts()); setMessage(""); }
    catch (error) { setMessage(error instanceof ApiError && error.status === 401 ? "Sign in to view alerts." : "Alerts could not be loaded."); }
    finally { setLoading(false); }
  }

  useEffect(() => { const timer = window.setTimeout(() => void loadAlerts(), 0); const interval = window.setInterval(() => void loadAlerts(), 3000); return () => { window.clearTimeout(timer); window.clearInterval(interval); }; }, []);

  async function changeStatus(alert: Alert, status: string) {
    try { await updateAlert(alert.alert_id, status, alert.assigned_to ?? ""); await loadAlerts(); }
    catch { setMessage("This alert could not be updated."); }
  }

  const visibleAlerts = filter === "all" ? alerts : alerts.filter((alert) => alert.status === filter);

  return <main className="account-shell"><header className="account-header"><Link href="/">&lt;- Overview</Link><span className="eyebrow">Response queue</span><h1>Alerts</h1><p>Investigate signals, assign ownership, and close resolved incidents.</p></header><section className="data-page"><div className="toolbar"><label>Status<select value={filter} onChange={(event) => setFilter(event.target.value)}><option value="all">All statuses</option><option value="open">Open</option><option value="acknowledged">Acknowledged</option><option value="closed">Closed</option></select></label><button onClick={() => void loadAlerts}>Refresh</button></div>{message && <p className="notice">{message} {message.includes("Sign in") && <Link href="/accounts">Sign in</Link>}</p>}{loading && <p className="empty-state">Loading alerts...</p>}{!loading && !visibleAlerts.length && <p className="empty-state">No alerts match this filter.</p>}{visibleAlerts.map((alert) => <article className="data-row" key={alert.alert_id}><div><span className={`severity ${alert.severity}`}>{alert.severity}</span><h2>{alert.summary}</h2><small>{new Date(alert.triggered_at).toLocaleString()} · Alert #{alert.alert_id}</small></div><div className="row-meta"><span>{alert.status}</span><select aria-label={`Update alert ${alert.alert_id}`} value={alert.status} onChange={(event) => void changeStatus(alert, event.target.value)}><option value="open">Open</option><option value="acknowledged">Acknowledged</option><option value="closed">Closed</option></select></div></article>)}</section></main>;
}