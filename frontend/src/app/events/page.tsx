"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { EventRecord, getEvents, ingestLog } from "@/lib/api";

export default function EventsPage() {
  const isDevelop = process.env.NEXT_PUBLIC_MODE === "develop";
  const [demoEnabled, setDemoEnabled] = useState(() =>
    isDevelop && (typeof window === "undefined" || localStorage.getItem("siem_mode") !== "production")
  );
  const [events, setEvents] = useState<EventRecord[]>([]);
  const [query, setQuery] = useState("");
  const [error, setError] = useState("");

  // Simulator state
  const [simOpen, setSimOpen] = useState(true);
  const [customMsg, setCustomMsg] = useState("");
  const [customSource, setCustomSource] = useState("nginx");
  const [customHost, setCustomHost] = useState("");
  const [simStatus, setSimStatus] = useState("");
  const [sending, setSending] = useState(false);

  async function fetchEvents() {
    try {
      const nextEvents = await getEvents();
      setEvents(nextEvents);
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
    const refresh = () =>
      getEvents()
        .then((nextEvents) => {
          if (active) {
            setEvents(nextEvents);
            setError("");
          }
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
  }, []);

  async function handleSendLog(msg: string, sourceType: string, hostname: string) {
    setSending(true);
    setSimStatus("");
    try {
      const res = await ingestLog(msg, sourceType, hostname);
      setSimStatus(`✔ Sent log successfully (Stream ID: ${res.stream_id})`);
      await fetchEvents();
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
      await fetchEvents();
    } catch {
      setSimStatus("✖ Batch ingestion failed.");
    } finally {
      setSending(false);
    }
  }

  const visibleEvents = events.filter((event) =>
    JSON.stringify(event).toLowerCase().includes(query.toLowerCase())
  );

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
        <input
          className="search-input"
          aria-label="Search events"
          placeholder="Search message, hostname, source, username, IP..."
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />

        {error && <p className="notice">{error}</p>}
        {!error && !visibleEvents.length && <p className="empty-state">No events found.</p>}

        {visibleEvents.map((event, index) => (
          <article className="data-row event-row" key={event.event_id ?? index}>
            <div>
              <div className="flex items-center gap-2">
                <span className="eyebrow">{event.event_type ?? event.source_type ?? "event"}</span>
                {event.duplicate_count && event.duplicate_count > 1 ? (
                  <span className="dedup-tag">⚡ {event.duplicate_count}x deduplicated</span>
                ) : null}
              </div>
              <h2>{event.message ?? "Unlabeled event"}</h2>
              <small>
                Host: {event.hostname ?? "Unknown"} · Cat: {event.log_category ?? "generic"}{" "}
                {event.src_ip ? `· IP: ${event.src_ip}` : ""}{" "}
                {event.username ? `· User: ${event.username}` : ""}{" "}
                · {event.event_time ? new Date(event.event_time).toLocaleString() : "Unknown time"}
              </small>
            </div>
            <code>{event.severity ?? "info"}</code>
          </article>
        ))}
      </section>
    </main>
  );
}