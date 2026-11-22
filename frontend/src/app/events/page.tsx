"use client";

import { useEffect, useState } from "react";
import { EventRecord, getEvents, ingestLog } from "@/lib/api";
import { SeverityBadge } from "@/components/severity-badge";
import {
  CheckIcon,
  ChevronDownIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  CopyIcon,
  FlaskIcon,
  SearchIcon,
  TerminalIcon,
} from "@/components/ui/icons";
import {
  Button,
  Drawer,
  EmptyState,
  Notice,
  PageHeader,
  PageShell,
  Panel,
  StatusPill,
} from "@/components/ui/primitives";

function groupEvents(events: EventRecord[]) {
  const groups = new Map<string, EventRecord>();
  for (const event of events) {
    const key =
      event.fingerprint ?? [event.message, event.log_category, event.hostname, event.severity].join("|");
    const first = groups.get(key);
    if (!first) {
      groups.set(key, {
        ...event,
        first_seen: event.first_seen ?? event.event_time,
        last_seen: event.last_seen ?? event.event_time,
        duplicate_count: event.duplicate_count ?? 1,
      });
      continue;
    }
    const firstTime = new Date(first.last_seen ?? first.event_time ?? 0).getTime();
    const nextTime = new Date(event.event_time ?? 0).getTime();
    if (Math.abs(nextTime - firstTime) <= 180000) {
      first.duplicate_count = Math.max(
        first.duplicate_count ?? 1,
        event.duplicate_count ?? 1,
        (first.duplicate_count ?? 1) + 1
      );
      first.last_seen = event.event_time ?? first.last_seen;
    } else {
      groups.set(`${key}|${event.event_time}`, {
        ...event,
        first_seen: event.event_time,
        last_seen: event.event_time,
        duplicate_count: event.duplicate_count ?? 1,
      });
    }
  }
  return [...groups.values()];
}

export default function EventsPage() {
  const isDevelop = process.env.NEXT_PUBLIC_MODE === "develop";
  const [demoEnabled, setDemoEnabled] = useState(
    () =>
      isDevelop &&
      (typeof window === "undefined" || localStorage.getItem("siem_mode") !== "production")
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
  const [copied, setCopied] = useState(false);
  const [error, setError] = useState("");

  // Simulator state
  const [simOpen, setSimOpen] = useState(false);
  const [customMsg, setCustomMsg] = useState("");
  const [customSource, setCustomSource] = useState("nginx");
  const [customHost, setCustomHost] = useState("");
  const [simStatus, setSimStatus] = useState("");
  const [sending, setSending] = useState(false);

  async function fetchEvents(nextPage = page) {
    try {
      const result = await getEvents({
        page: nextPage,
        pageSize,
        q: debouncedQuery,
        severity,
        category,
        host,
        from,
        to,
      });
      setEvents(result.items);
      setTotal(result.total);
      setError("");
    } catch {
      setError("Không thể tải danh sách sự kiện. Vui lòng đăng nhập và thử lại.");
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
      getEvents({ page, pageSize, q: debouncedQuery, severity, category, host, from, to })
        .then((result) => {
          if (active) {
            setEvents(result.items);
            setTotal(result.total);
            setError("");
          }
        })
        .catch(() => {
          if (active) setError("Không thể tải danh sách sự kiện. Vui lòng đăng nhập và thử lại.");
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
    const timer = window.setTimeout(() => {
      setDebouncedQuery(query.trim());
      setPage(1);
    }, 300);
    return () => window.clearTimeout(timer);
  }, [query]);

  async function handleSendLog(msg: string, sourceType: string, hostname: string) {
    setSending(true);
    setSimStatus("");
    try {
      const res = await ingestLog(msg, sourceType, hostname);
      setSimStatus(`Đã gửi log thành công (Stream ID: ${res.stream_id})`);
      await fetchEvents(1);
    } catch {
      setSimStatus("Gửi log thất bại. Kiểm tra kết nối API.");
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
      setSimStatus(`Đã gửi thành công lô ${count} log mẫu.`);
      await fetchEvents(1);
    } catch {
      setSimStatus("Gửi lô log thất bại.");
    } finally {
      setSending(false);
    }
  }

  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const clearFilters = () => {
    setQuery("");
    setSeverity("");
    setCategory("");
    setHost("");
    setFrom("");
    setTo("");
    setPage(1);
  };

  const highlight = (value: string) => {
    if (!debouncedQuery) return value;
    const parts = value.split(new RegExp(`(${debouncedQuery.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")})`, "ig"));
    return parts.map((part, index) =>
      part.toLowerCase() === debouncedQuery.toLowerCase() ? <mark key={index}>{part}</mark> : part
    );
  };

  const fieldEntries = (event: EventRecord) =>
    Object.entries({
      event_id: event.event_id,
      event_time: event.event_time,
      hostname: event.hostname,
      category: event.log_category,
      event_type: event.event_type,
      severity: event.severity,
      source_ip: event.src_ip,
      username: event.username,
      ...event.extra_fields,
    }).filter(([, value]) => value !== undefined && value !== "");

  function handleCopyRaw(text: string) {
    void navigator.clipboard?.writeText(text);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1800);
  }

  const grouped = groupEvents(events);

  return (
    <PageShell>
      <PageHeader
        title="Log Explorer"
        description="Truy vấn sự kiện đã chuẩn hóa, lọc theo trường ngữ nghĩa và kiểm tra payload gốc."
        badge={<StatusPill tone="accent">{total.toLocaleString()} sự kiện</StatusPill>}
        actions={
          <Button variant={simOpen ? "primary" : "secondary"} onClick={() => setSimOpen((o) => !o)}>
            <FlaskIcon size={14} />
            Log Simulator
            <ChevronDownIcon size={13} className={`transition-transform ${simOpen ? "rotate-180" : ""}`} />
          </Button>
        }
      />

      {/* Collapsible Simulator Panel */}
      {simOpen && (
        <Panel
          title="Bộ mô phỏng gửi Log (Ingestion Simulator)"
          subtitle="Kích hoạt nhanh kịch bản tấn công mẫu hoặc gửi dòng log tùy chỉnh vào pipeline"
        >
          <div className="space-y-4">
            {isDevelop && demoEnabled && (
              <div className="flex flex-wrap gap-2">
                <Button
                  size="sm"
                  disabled={sending}
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
                  SSH Brute Force (5x)
                </Button>
                <Button
                  size="sm"
                  disabled={sending}
                  onClick={() =>
                    handleSendLog(
                      '192.168.1.120 - - [28/Aug/2026:10:00:00 +0000] "GET /api/user?id=1 UNION SELECT 1,username,password FROM users -- HTTP/1.1" 200 450',
                      "nginx",
                      "web-prod-01"
                    )
                  }
                >
                  Web SQL Injection
                </Button>
                <Button
                  size="sm"
                  disabled={sending}
                  onClick={() =>
                    handleSendLog(
                      '192.168.1.130 - - [28/Aug/2026:10:00:00 +0000] "GET /download?file=../../../../etc/passwd HTTP/1.1" 403 120',
                      "nginx",
                      "web-prod-01"
                    )
                  }
                >
                  Path Traversal / LFI
                </Button>
                <Button
                  size="sm"
                  disabled={sending}
                  onClick={() =>
                    handleSendLog(
                      "An account failed to log on. Event ID: 4625. Account Name: Administrator. Source Network Address: 192.168.1.50",
                      "windows_eventlog",
                      "win-dc-01"
                    )
                  }
                >
                  Windows 4625 Failed Logon
                </Button>
                <Button
                  size="sm"
                  disabled={sending}
                  onClick={() =>
                    handleSendBatch([
                      { msg: "Repeat exact duplicate raw log line test for SHA-256 fingerprint deduplication", source: "generic", host: "test-node" },
                      { msg: "Repeat exact duplicate raw log line test for SHA-256 fingerprint deduplication", source: "generic", host: "test-node" },
                    ])
                  }
                >
                  Test SHA-256 Dedup (2x)
                </Button>
              </div>
            )}

            <div className="grid grid-cols-1 gap-2.5 border-t border-border pt-4 md:grid-cols-12">
              <input
                className="soc-input md:col-span-5"
                placeholder="Nhập nội dung log tùy ý..."
                value={customMsg}
                onChange={(e) => setCustomMsg(e.target.value)}
              />
              <input
                className="soc-input md:col-span-3"
                placeholder="Hostname đã đăng ký"
                value={customHost}
                onChange={(e) => setCustomHost(e.target.value)}
              />
              <select
                className="soc-select md:col-span-2"
                value={customSource}
                onChange={(e) => setCustomSource(e.target.value)}
              >
                <option value="nginx">Nginx Web</option>
                <option value="linux_sshd">Linux SSH</option>
                <option value="linux_audit">Linux Audit</option>
                <option value="windows_security">Windows Security</option>
                <option value="generic">Generic</option>
              </select>
              <Button
                variant="primary"
                disabled={sending || !customMsg.trim() || !customHost.trim()}
                className="md:col-span-2"
                onClick={() => handleSendLog(customMsg, customSource, customHost)}
              >
                {sending ? "Đang gửi..." : "Gửi Log"}
              </Button>
            </div>

            {simStatus && <p className="font-mono text-xs text-accent">{simStatus}</p>}
          </div>
        </Panel>
      )}

      {/* Search & Filter Bar */}
      <Panel>
        <div className="space-y-3">
          <div className="relative">
            <SearchIcon size={15} className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-faint" />
            <input
              className="soc-input !pl-9"
              aria-label="Search events"
              placeholder="Tìm theo nội dung log, hostname, phân loại hoặc địa chỉ IP..."
              value={query}
              onChange={(event) => setQuery(event.target.value)}
            />
          </div>

          <div className="grid grid-cols-2 gap-2.5 md:grid-cols-6">
            <select
              className="soc-select"
              aria-label="Filter severity"
              value={severity}
              onChange={(event) => {
                setSeverity(event.target.value);
                setPage(1);
              }}
            >
              <option value="">Tất cả mức độ</option>
              <option value="info">Info</option>
              <option value="warning">Warning</option>
              <option value="error">Error</option>
              <option value="critical">Critical</option>
            </select>
            <input
              className="soc-input"
              aria-label="Filter category"
              placeholder="Category"
              value={category}
              onChange={(event) => {
                setCategory(event.target.value);
                setPage(1);
              }}
            />
            <input
              className="soc-input"
              aria-label="Filter host"
              placeholder="Hostname"
              value={host}
              onChange={(event) => {
                setHost(event.target.value);
                setPage(1);
              }}
            />
            <input
              className="soc-input font-mono text-xs"
              aria-label="Filter from date"
              type="datetime-local"
              value={from}
              onChange={(event) => {
                setFrom(event.target.value);
                setPage(1);
              }}
            />
            <input
              className="soc-input font-mono text-xs"
              aria-label="Filter to date"
              type="datetime-local"
              value={to}
              onChange={(event) => {
                setTo(event.target.value);
                setPage(1);
              }}
            />
            <Button variant="ghost" onClick={clearFilters} className="border border-border">
              Xóa bộ lọc
            </Button>
          </div>
        </div>
      </Panel>

      {error && <Notice tone="error">{error}</Notice>}

      {/* Events Table */}
      <Panel
        title="Luồng sự kiện chuẩn hóa"
        actions={
          <div className="flex items-center gap-2 text-xs text-muted">
            <span>Hiển thị</span>
            <select
              className="soc-select !w-auto !py-1 font-mono text-xs"
              value={pageSize}
              onChange={(event) => {
                setPageSize(Number(event.target.value));
                setPage(1);
              }}
            >
              <option value="25">25</option>
              <option value="50">50</option>
              <option value="100">100</option>
            </select>
          </div>
        }
        noPadding
      >
        {grouped.length === 0 ? (
          <EmptyState
            title="Không tìm thấy sự kiện phù hợp"
            description="Thử mở rộng bộ lọc thời gian hoặc gửi log mẫu bằng Log Simulator."
            icon={<TerminalIcon size={18} />}
          />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead>
                <tr className="border-b border-border bg-surface-raised/60 font-mono text-[11px] uppercase tracking-wider text-muted">
                  <th className="px-5 py-3">Thời gian</th>
                  <th className="px-4 py-3">Mức độ</th>
                  <th className="px-4 py-3">Host / Nguồn</th>
                  <th className="px-5 py-3">Thông điệp sự kiện</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {grouped.map((event, index) => (
                  <tr
                    key={event.event_id ?? index}
                    onClick={() => setSelectedEvent(event)}
                    className="cursor-pointer transition-colors hover:bg-surface-hover/70"
                  >
                    <td className="px-5 py-3 font-mono text-[11px] text-muted whitespace-nowrap tabular-nums">
                      {event.event_time ? new Date(event.event_time).toLocaleString("vi-VN") : "—"}
                    </td>
                    <td className="px-4 py-3 whitespace-nowrap">
                      <SeverityBadge severity={event.severity ?? "info"} />
                    </td>
                    <td className="px-4 py-3 whitespace-nowrap">
                      <div className="font-mono font-medium text-ink-secondary">{event.hostname ?? "Unknown"}</div>
                      <div className="font-mono text-[11px] text-faint">
                        {event.log_category ?? event.source_type ?? "event"}
                        {event.src_ip ? ` · ${event.src_ip}` : ""}
                      </div>
                    </td>
                    <td className="px-5 py-3">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-mono text-ink break-all">
                          {highlight(event.message ?? "Unlabeled event")}
                        </span>
                        {event.duplicate_count && event.duplicate_count > 1 ? (
                          <StatusPill tone="accent">{event.duplicate_count}x dedup</StatusPill>
                        ) : null}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination */}
        <div className="flex items-center justify-between border-t border-border px-5 py-3 text-xs text-muted">
          <span className="font-mono">
            Trang <strong className="text-ink">{page}</strong> / {totalPages}
          </span>
          <div className="flex items-center gap-2">
            <Button size="sm" disabled={page === 1} onClick={() => setPage(page - 1)}>
              <ChevronLeftIcon size={13} /> Trước
            </Button>
            <Button size="sm" disabled={page >= totalPages} onClick={() => setPage(page + 1)}>
              Sau <ChevronRightIcon size={13} />
            </Button>
          </div>
        </div>
      </Panel>

      {/* Slide-over Event Inspector Drawer */}
      <Drawer
        open={Boolean(selectedEvent)}
        onClose={() => setSelectedEvent(null)}
        subtitle="Event Inspector"
        title={selectedEvent?.event_type ?? "Normalized Security Event"}
      >
        {selectedEvent && (
          <div className="space-y-6">
            <div>
              <h3 className="mb-3 font-mono text-[11px] uppercase tracking-wider text-muted">
                Trường chuẩn hóa
              </h3>
              <dl className="grid grid-cols-1 gap-px overflow-hidden rounded-lg border border-border bg-border sm:grid-cols-2">
                {fieldEntries(selectedEvent).map(([key, value]) => (
                  <div key={key} className="bg-canvas px-3.5 py-2.5">
                    <dt className="font-mono text-[10px] uppercase tracking-wider text-faint">
                      {key.replaceAll("_", " ")}
                    </dt>
                    <dd className="mt-1 font-mono text-xs text-ink break-all">{String(value)}</dd>
                  </div>
                ))}
              </dl>
            </div>

            <div>
              <div className="mb-2 flex items-center justify-between">
                <h3 className="font-mono text-[11px] uppercase tracking-wider text-muted">Raw Payload</h3>
                <Button
                  size="sm"
                  onClick={() => handleCopyRaw(selectedEvent.raw ?? selectedEvent.message ?? "")}
                >
                  {copied ? <CheckIcon size={13} /> : <CopyIcon size={13} />}
                  {copied ? "Đã sao chép" : "Sao chép raw log"}
                </Button>
              </div>
              <pre className="overflow-x-auto rounded-lg border border-border bg-canvas p-4 font-mono text-xs leading-relaxed text-cyan-300 whitespace-pre-wrap break-all">
                {selectedEvent.raw ?? selectedEvent.message ?? "No raw payload available"}
              </pre>
            </div>
          </div>
        )}
      </Drawer>
    </PageShell>
  );
}