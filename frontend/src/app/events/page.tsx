"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { EventRecord, getEvents, ingestLog } from "@/lib/api";

function groupEvents(events: EventRecord[]) {
  const groups = new Map<string, EventRecord>();
  for (const event of events) {
    const key = event.fingerprint ?? [event.message, event.log_category, event.hostname, event.severity].join("|");
    const first = groups.get(key);
    if (!first) { groups.set(key, { ...event, first_seen: event.first_seen ?? event.event_time, last_seen: event.last_seen ?? event.event_time, duplicate_count: event.duplicate_count ?? 1 }); continue; }
    const firstTime = new Date(first.last_seen ?? first.event_time ?? 0).getTime();
    const nextTime = new Date(event.event_time ?? 0).getTime();
    if (Math.abs(nextTime - firstTime) <= 180000) {
      first.duplicate_count = Math.max(first.duplicate_count ?? 1, event.duplicate_count ?? 1, (first.duplicate_count ?? 1) + 1);
      first.last_seen = event.event_time ?? first.last_seen;
    } else groups.set(`${key}|${event.event_time}`, { ...event, first_seen: event.event_time, last_seen: event.event_time, duplicate_count: event.duplicate_count ?? 1 });
  }
  return [...groups.values()];
}

export default function EventsPage() {
  const isDevelop = process.env.NEXT_PUBLIC_MODE === "develop";
  const [demoEnabled, setDemoEnabled] = useState(() =>
    isDevelop && (typeof window === "undefined" || localStorage.getItem("siem_mode") !== "production")
  );
  const [events, setEvents] = useState<EventRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(25);
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const [severity, setSeverity] = useState("");
  const [category, setCategory] = useState("");
  const [host, setHost] = useState("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [selectedEvent, setSelectedEvent] = useState<EventRecord | null>(null);
  const [error, setError] = useState("");

  // Simulator state
  const [simOpen, setSimOpen] = useState(true);
  const [customMsg, setCustomMsg] = useState("");
  const [customSource, setCustomSource] = useState("nginx");
  const [customHost, setCustomHost] = useState("");
  const [simStatus, setSimStatus] = useState("");
  const [sending, setSending] = useState(false);

  async function fetchEvents(nextPage = page) {
    try {
      const result = await getEvents({ page: nextPage, pageSize, q: debouncedQuery, severity, category, host, from, to });
      setEvents(result.items);
      setTotal(result.total);
      setError("");
    } catch {
      setError("Events could not be loaded. Sign in and try again.");
    }
  }

  useEffect(() => {
    const handleModeChange = (event: Event) => setDemoEnabled((event as CustomEvent<boolean>).detail);
    window.addEventListener("siem-mode-change", handleModeChange);
    return () => window.removeEventListener("siem-mode-change", handleModeChange);
  }, []);

  useEffect(() => {
    let active = true;
    const refresh = () => getEvents({ page, pageSize, q: debouncedQuery, severity, category, host, from, to })
        .then((result) => {
          if (active) { setEvents(result.items); setTotal(result.total); setError(""); }
        })
        .catch(() => {
          if (active) setError("Events could not be loaded. Sign in and try again.");
        });

    const timer = window.setTimeout(refresh, 0);
    const interval = window.setInterval(refresh, 3000);
    return () => {
      active = false;
      window.clearTimeout(timer);
      window.clearInterval(interval);
    };
  }, [page, pageSize, debouncedQuery, severity, category, host, from, to]);

  useEffect(() => {
    const timer = window.setTimeout(() => { setDebouncedQuery(query.trim()); setPage(1); }, 300);
    return () => window.clearTimeout(timer);
  }, [query]);

  async function handleSendLog(msg: string, sourceType: string, hostname: string) {
    setSending(true);
    setSimStatus("");
    try {
      const res = await ingestLog(msg, sourceType, hostname);
      setSimStatus(`✔ Sent log successfully (Stream ID: ${res.stream_id})`);
      await fetchEvents(1);
    } catch {
      setSimStatus("✖ Ingestion failed. Check API status.");
    } finally {
      setSending(false);
    }
  }

  async function handleSendBatch(logs: Array<{ msg: string; source: string; host: string }>) {
    setSending(true);
    setSimStatus("");
    try {
      let count = 0;
      for (const item of logs) {
        await ingestLog(item.msg, item.source, item.host);
        count++;
      }
      setSimStatus(`✔ Sent batch of ${count} logs successfully! Real-time alerts & dedup triggered.`);
      await fetchEvents(1);
    } catch {
      setSimStatus("✖ Batch ingestion failed.");
    } finally {
      setSending(false);
    }
  }

  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const clearFilters = () => { setQuery(""); setSeverity(""); setCategory(""); setHost(""); setFrom(""); setTo(""); setPage(1); };
  const highlight = (value: string) => {
    if (!debouncedQuery) return value;
    const parts = value.split(new RegExp(`(${debouncedQuery.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")})`, "ig"));
    return parts.map((part, index) => part.toLowerCase() === debouncedQuery.toLowerCase() ? <mark key={index}>{part}</mark> : part);
  };
  const fieldEntries = (event: EventRecord) => Object.entries({
    event_id: event.event_id, event_time: event.event_time, hostname: event.hostname, category: event.log_category,
    event_type: event.event_type, severity: event.severity, source_ip: event.src_ip, username: event.username, ...event.extra_fields,
  }).filter(([, value]) => value !== undefined && value !== "");

  return (
    <main className="account-shell">
      <header className="account-header">
        <Link href="/">← Overview</Link>
        <span className="eyebrow">Signal History & Simulator</span>
        <h1>Log Explorer</h1>
        <p>Tìm kiếm sự kiện chuẩn hóa và thử nghiệm bộ mô phỏng Log Ingestion theo thời gian thực.</p>
      </header>

      {/* Log Ingest Simulator Section */}
      <section className="simulator-box">
        <div className="flex justify-between items-center mb-2">
          <h2 className="text-lg font-medium text-(--acid)">
            🧪 Simulate Client Log Ingestion (Bộ mô phỏng gửi Log)
          </h2>
          <button
            type="button"
            onClick={() => setSimOpen(!simOpen)}
            className="btn-preset text-xs"
          >
            {simOpen ? "Thu gọn ▲" : "Mở rộng ▼"}
          </button>
        </div>

        {simOpen && (
          <div className="mt-4">
            <p className="text-xs text-(--muted) mb-3">
              Bấm 1-Click để gửi các mẫu kịch bản tấn công thực tế hoặc kiểm thử tính năng gom cụm dữ liệu trùng (Deduplication):
            </p>

            {isDevelop && demoEnabled && <div className="sim-grid">
              <button
                disabled={sending}
                type="button"
                className="btn-sim"
                onClick={() =>
                  handleSendBatch([
                    { msg: "Failed password for invalid user admin from 192.168.1.105 port 55101 ssh2", source: "linux_sshd", host: "ssh-gateway" },
                    { msg: "Failed password for invalid user admin from 192.168.1.105 port 55102 ssh2", source: "linux_sshd", host: "ssh-gateway" },
                    { msg: "Failed password for root from 192.168.1.105 port 55103 ssh2", source: "linux_sshd", host: "ssh-gateway" },
                    { msg: "Failed password for root from 192.168.1.105 port 55104 ssh2", source: "linux_sshd", host: "ssh-gateway" },
                    { msg: "Failed password for root from 192.168.1.105 port 55105 ssh2", source: "linux_sshd", host: "ssh-gateway" },
                  ])
                }
              >
                🚀 SSH Brute Force (5x Failed)
              </button>

              <button
                disabled={sending}
                type="button"
                className="btn-sim"
                onClick={() =>
                  handleSendLog(
                    "192.168.1.120 - - [28/Aug/2026:10:00:00 +0000] \"GET /api/user?id=1 UNION SELECT 1,username,password FROM users -- HTTP/1.1\" 200 450",
                    "nginx",
                    "web-prod-01"
                  )
                }
              >
                💉 Web SQL Injection
              </button>

              <button
                disabled={sending}
                type="button"
                className="btn-sim"
                onClick={() =>
                  handleSendLog(
                    "192.168.1.130 - - [28/Aug/2026:10:00:00 +0000] \"GET /download?file=../../../../etc/passwd HTTP/1.1\" 403 120",
                    "nginx",
                    "web-prod-01"
                  )
                }
              >
                📂 Path Traversal / LFI
              </button>

              <button
                disabled={sending}
                type="button"
                className="btn-sim"
                onClick={() =>
                  handleSendLog(
                    "An account failed to log on. Event ID: 4625. Account Name: Administrator. Source Network Address: 192.168.1.50",
                    "windows_eventlog",
                    "win-dc-01"
                  )
                }
              >
                🪟 Windows 4625 Failed Logon
              </button>

              <button
                disabled={sending}
                type="button"
                className="btn-sim hover:border-(--acid)"
                onClick={() =>
                  handleSendBatch([
                    { msg: "Repeat exact duplicate raw log line test for SHA-256 fingerprint deduplication", source: "generic", host: "test-node" },
                    { msg: "Repeat exact duplicate raw log line test for SHA-256 fingerprint deduplication", source: "generic", host: "test-node" },
                  ])
                }
              >
                ⚡ Test Duplicate Raw Log (SHA-256 Dedup)
              </button>
            </div>}

            {/* Custom Log Ingestion */}
            <div className="mt-4 pt-4 border-t border-(--line) grid grid-cols-1 md:grid-cols-4 gap-3">
              <input
                className="md:col-span-2 text-xs"
                placeholder="Nhập nội dung log tùy ý (Custom Log Message)..."
                value={customMsg}
                onChange={(e) => setCustomMsg(e.target.value)}
              />
              <input
                className="text-xs"
                placeholder="Hostname đã đăng ký"
                value={customHost}
                onChange={(e) => setCustomHost(e.target.value)}
              />
              <select className="text-xs" value={customSource} onChange={(e) => setCustomSource(e.target.value)}>
                <option value="nginx">Nginx Web</option>
                <option value="linux_sshd">Linux SSH</option>
                <option value="linux_audit">Linux Audit/Sudo</option>
                <option value="windows_security">Windows Security</option>
                <option value="generic">Generic</option>
              </select>
              <button
                disabled={sending || !customMsg.trim() || !customHost.trim()}
                type="button"
                className="btn-preset bg-(--acid) text-(--canvas) font-bold"
                onClick={() => handleSendLog(customMsg, customSource, customHost)}
              >
                {sending ? "Sending..." : "Gửi Log"}
              </button>
            </div>

            {simStatus && <div className="mt-3 text-xs font-mono text-(--acid)">{simStatus}</div>}
          </div>
        )}
      </section>

      <section className="data-page">
        <div className="event-controls">
          <input className="search-input" aria-label="Search events" placeholder="Search message, hostname, category, IP..." value={query} onChange={(event) => setQuery(event.target.value)} />
          <div className="filter-grid">
            <select aria-label="Filter severity" value={severity} onChange={(event) => { setSeverity(event.target.value); setPage(1); }}><option value="">All severity</option><option value="info">Info</option><option value="warning">Warning</option><option value="error">Error</option><option value="critical">Critical</option></select>
            <input aria-label="Filter category" placeholder="Category" value={category} onChange={(event) => { setCategory(event.target.value); setPage(1); }} />
            <input aria-label="Filter host" placeholder="Host" value={host} onChange={(event) => { setHost(event.target.value); setPage(1); }} />
            <input aria-label="Filter from date" type="datetime-local" value={from} onChange={(event) => { setFrom(event.target.value); setPage(1); }} />
            <input aria-label="Filter to date" type="datetime-local" value={to} onChange={(event) => { setTo(event.target.value); setPage(1); }} />
            <button type="button" className="filter-clear" onClick={clearFilters}>Clear filters</button>
          </div>
          <div className="results-line"><span>{total.toLocaleString()} results</span><label>Rows <select value={pageSize} onChange={(event) => { setPageSize(Number(event.target.value)); setPage(1); }}><option value="25">25</option><option value="50">50</option><option value="100">100</option></select></label></div>
        </div>

        {error && <p className="notice">{error}</p>}
        {!error && !events.length && <p className="empty-state">No events found.</p>}

        {groupEvents(events).map((event, index) => (
          <button className="data-row event-row event-button" key={event.event_id ?? index} type="button" onClick={() => setSelectedEvent(event)}>
            <div>
              <div className="flex items-center gap-2">
                  <span className="event-category">{event.log_category ?? event.source_type ?? "event"}</span>
                {event.duplicate_count && event.duplicate_count > 1 ? (
                  <span className="dedup-tag">{event.duplicate_count}x deduplicated</span>
                ) : null}
              </div>
              <h2>{highlight(event.message ?? "Unlabeled event")}</h2>
              <small>
                Host: {event.hostname ?? "Unknown"} · Cat: {event.log_category ?? "generic"}{" "}
                {event.src_ip ? `· IP: ${event.src_ip}` : ""}{" "}
                {event.username ? `· User: ${event.username}` : ""}{" "}
                · {event.first_seen && event.last_seen && event.first_seen !== event.last_seen ? `First seen: ${new Date(event.first_seen).toLocaleString()} · Last seen: ${new Date(event.last_seen).toLocaleString()}` : event.event_time ? new Date(event.event_time).toLocaleString() : "Unknown time"}
              </small>
            </div>
            <code className={`event-severity severity-${event.severity ?? "info"}`}>{event.severity ?? "info"}</code>
          </button>
        ))}
        <div className="pagination"><button type="button" disabled={page === 1} onClick={() => setPage(page - 1)}>Prev</button><span>Page <strong>{page}</strong> of {totalPages}</span><button type="button" disabled={page >= totalPages} onClick={() => setPage(page + 1)}>Next</button></div>
      </section>
      {selectedEvent && <dialog open className="event-dialog" onClick={(event) => { if (event.target === event.currentTarget) setSelectedEvent(null); }}><div className="event-dialog-panel"><div className="dialog-heading"><div><span className="eyebrow">Event detail</span><h2>{selectedEvent.event_type ?? "Normalized event"}</h2></div><button type="button" onClick={() => setSelectedEvent(null)} aria-label="Close details">Close</button></div><div className="event-fields">{fieldEntries(selectedEvent).map(([key, value]) => <div key={key}><dt>{key.replaceAll("_", " ")}</dt><dd>{String(value)}</dd></div>)}</div><h3>Raw log</h3><pre>{selectedEvent.raw ?? selectedEvent.message ?? "No raw payload available"}</pre><button type="button" className="copy-raw" onClick={() => void navigator.clipboard?.writeText(selectedEvent.raw ?? selectedEvent.message ?? "")}>Copy raw log</button></div></dialog>}
    </main>
  );
}