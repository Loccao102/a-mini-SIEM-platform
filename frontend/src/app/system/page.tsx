"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError, getSystemHealth, type SystemHealth, type SystemServiceHealth } from "@/lib/api";
import { ActivityIcon, RefreshIcon } from "@/components/ui/icons";
import {
  Button,
  EmptyState,
  KpiCard,
  Notice,
  PageHeader,
  PageShell,
  Panel,
  StatusPill,
} from "@/components/ui/primitives";

const SERVICE_LABELS: Record<string, string> = {
  postgres: "PostgreSQL",
  redis: "Redis",
  elasticsearch: "Elasticsearch",
  database: "Database schema",
  queues: "Ingest & retry queues",
  connectors: "Ingest / parser",
};

const PRIORITY = ["postgres", "redis", "elasticsearch", "database", "queues", "connectors"];

function displayService(name: string) {
  return SERVICE_LABELS[name] || name;
}

function formatChecked(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? "—"
    : new Intl.DateTimeFormat("vi-VN", { dateStyle: "short", timeStyle: "medium" }).format(date);
}

function statusTone(status?: string): "success" | "warning" | "danger" | "neutral" {
  if (status === "healthy") return "success";
  if (status === "degraded") return "warning";
  if (status === "unhealthy") return "danger";
  return "neutral";
}

function ServiceCard({ name, item }: { name: string; item: SystemServiceHealth }) {
  const checks = Object.entries(item.checks || {}).sort(([a], [b]) => a.localeCompare(b));
  return (
    <article className="flex flex-col justify-between rounded-lg border border-border bg-surface p-5 transition-colors hover:border-border-strong">
      <div>
        <div className="flex items-start justify-between gap-3">
          <div>
            <p className="font-mono text-[10px] uppercase tracking-wider text-faint">Subsystem</p>
            <h3 className="mt-0.5 text-base font-semibold text-ink">{displayService(name)}</h3>
          </div>
          <StatusPill tone={statusTone(item.status)}>{item.status}</StatusPill>
        </div>

        <p className="mt-3 min-h-[36px] text-xs leading-relaxed text-muted">{item.message}</p>
      </div>

      <div className="mt-4">
        <div className="flex items-center justify-between border-t border-border py-2.5 text-xs">
          <span className="text-muted">Độ trễ phản hồi</span>
          <strong className="font-mono text-ink tabular-nums">
            {Number.isFinite(item.response_time_ms) ? `${item.response_time_ms} ms` : "—"}
          </strong>
        </div>

        {checks.length > 0 && (
          <dl className="divide-y divide-border border-t border-border">
            {checks.map(([key, value]) => (
              <div key={key} className="flex items-start justify-between gap-4 py-2 text-[11px]">
                <dt className="text-faint break-words">{key.replace(/_/g, " ")}</dt>
                <dd className="text-right font-mono text-ink-secondary break-all whitespace-pre-wrap">
                  {value}
                </dd>
              </div>
            ))}
          </dl>
        )}
      </div>
    </article>
  );
}

export default function SystemPage() {
  const [snapshot, setSnapshot] = useState<SystemHealth | null>(null);
  const [busy, setBusy] = useState(true);
  const [error, setError] = useState("");
  const running = useRef(false);
  const active = useRef(false);

  const refresh = useCallback(async () => {
    if (running.current) return;
    running.current = true;
    if (active.current) setBusy(true);
    try {
      const result = await getSystemHealth();
      if (active.current) {
        setSnapshot(result);
        setError("");
      }
    } catch (reason) {
      if (active.current) {
        if (reason instanceof ApiError && (reason.status === 401 || reason.status === 403)) {
          setSnapshot(null);
        }
        setError(
          reason instanceof ApiError && reason.status === 403
            ? "Chỉ tài khoản SOC Admin được xem chẩn đoán hệ thống."
            : reason instanceof ApiError && reason.status === 401
            ? "Phiên đăng nhập đã hết hạn. Vui lòng đăng nhập lại."
            : "Không đọc được trạng thái các dịch vụ. Kiểm tra kết nối API và quyền tài khoản."
        );
      }
    } finally {
      running.current = false;
      if (active.current) setBusy(false);
    }
  }, []);

  useEffect(() => {
    active.current = true;
    const initial = window.setTimeout(() => void refresh(), 0);
    const interval = window.setInterval(() => {
      if (document.visibilityState === "visible") void refresh();
    }, 15000);
    return () => {
      active.current = false;
      window.clearTimeout(initial);
      window.clearInterval(interval);
    };
  }, [refresh]);

  const services = snapshot?.services || {};
  const ordered = [...new Set([...PRIORITY, ...Object.keys(services)])].filter((key) =>
    Boolean(services[key])
  );

  const overallTone =
    snapshot?.status === "healthy"
      ? "success"
      : snapshot?.status === "degraded"
      ? "high"
      : snapshot?.status === "unhealthy"
      ? "critical"
      : "default";

  return (
    <PageShell>
      <PageHeader
        title="System Health"
        description="Chẩn đoán trực tiếp các dịch vụ lõi, hàng đợi ingest và độ trễ kiểm tra (chỉ dành cho Admin)."
        badge={<StatusPill tone="neutral">Admin Only</StatusPill>}
        actions={
          <Button variant="secondary" onClick={() => void refresh()} disabled={busy}>
            <RefreshIcon size={14} className={busy ? "animate-spin" : ""} />
            {busy ? "Đang kiểm tra..." : "Kiểm tra lại"}
          </Button>
        }
      />

      <section className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <KpiCard
          label="Trạng thái tổng thể"
          value={snapshot?.status ? snapshot.status.toUpperCase() : "—"}
          tone={overallTone}
        />
        <KpiCard label="API Uptime" value={snapshot?.uptime || "—"} />
        <KpiCard
          label="Lần kiểm tra gần nhất"
          value={<span className="text-lg">{snapshot ? formatChecked(snapshot.checked_at) : "—"}</span>}
        />
        <KpiCard label="Dịch vụ báo cáo" value={ordered.length} tone="accent" icon={<ActivityIcon size={16} />} />
      </section>

      {error && <Notice tone="error">{error}</Notice>}

      {ordered.length > 0 ? (
        <section className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3" aria-label="Trạng thái chi tiết các dịch vụ">
          {ordered.map((name) => (
            <ServiceCard key={name} name={name} item={services[name]} />
          ))}
        </section>
      ) : (
        <Panel>
          <EmptyState
            title={busy ? "Đang kiểm tra trạng thái các thành phần..." : "Chưa có dữ liệu từ backend"}
            description="Chọn Kiểm tra lại để thử kết nối tới endpoint chẩn đoán."
            icon={<ActivityIcon size={18} />}
          />
        </Panel>
      )}

      <p className="text-xs text-faint">
        Tự động cập nhật mỗi 15 giây khi trang đang mở. Chỉ tài khoản Admin xem được thông số chi tiết từng thành phần; khi dịch vụ gặp sự cố, thông số hiển thị là kết quả kiểm tra gần nhất.
      </p>
    </PageShell>
  );
}
