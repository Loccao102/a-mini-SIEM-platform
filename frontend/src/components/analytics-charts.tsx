"use client";

import { useState } from "react";
import { AnalyticsData } from "@/lib/api";
import { Panel, StatusPill } from "@/components/ui/primitives";

function formatTimelineLabel(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("vi-VN", {
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function formatFullTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("vi-VN", {
    day: "2-digit",
    month: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

export function AnalyticsCharts({ data }: { data: AnalyticsData | null }) {
  const [hoveredIdx, setHoveredIdx] = useState<number | null>(null);

  if (!data) return null;

  const timelinePoints = data.timeline ?? [];
  const maxCount = Math.max(...timelinePoints.map((p) => p.count), 1);
  // Nice rounded ceiling for Y-axis
  const yCeiling = maxCount <= 10 ? Math.max(4, Math.ceil(maxCount / 2) * 2) : Math.ceil(maxCount / 10) * 10;
  const yTicks = [0, 0.25, 0.5, 0.75, 1].map((r) => Math.round(yCeiling * r));

  const W = 680;
  const H = 220;
  const padL = 44;
  const padR = 16;
  const padT = 16;
  const padB = 30;
  const innerW = W - padL - padR;
  const innerH = H - padT - padB;

  const coords = timelinePoints.map((pt, idx) => {
    const x = padL + (idx / Math.max(timelinePoints.length - 1, 1)) * innerW;
    const y = padT + innerH - (pt.count / yCeiling) * innerH;
    return { x, y, pt };
  });

  const linePath = coords.map((c, i) => `${i === 0 ? "M" : "L"}${c.x.toFixed(1)},${c.y.toFixed(1)}`).join(" ");
  const areaPath = coords.length
    ? `${linePath} L${coords[coords.length - 1].x.toFixed(1)},${padT + innerH} L${coords[0].x.toFixed(1)},${padT + innerH} Z`
    : "";

  const labelStep = Math.max(1, Math.ceil(timelinePoints.length / 6));
  const totalEvents = Math.max(data.total_events, 1);

  const severities = [
    { key: "critical", label: "Critical", count: data.events_by_severity?.critical ?? 0, bar: "bg-red-500", text: "text-red-300" },
    { key: "high", label: "High", count: data.events_by_severity?.high ?? 0, bar: "bg-amber-500", text: "text-amber-300" },
    { key: "medium", label: "Medium", count: data.events_by_severity?.medium ?? 0, bar: "bg-sky-400", text: "text-sky-300" },
    { key: "info", label: "Low / Info", count: data.events_by_severity?.info ?? 0, bar: "bg-cyan-400", text: "text-cyan-300" },
  ];

  const activePoint = hoveredIdx !== null && coords[hoveredIdx] ? coords[hoveredIdx] : null;

  return (
    <div className="space-y-6">
      {/* Row 1: Velocity Chart + Severity Distribution */}
      <div className="grid grid-cols-1 gap-6 xl:grid-cols-3">
        <Panel
          title="Lưu lượng sự kiện theo thời gian"
          subtitle="Event velocity trong 24 giờ gần nhất"
          actions={<StatusPill tone="accent">24h Telemetry</StatusPill>}
          className="xl:col-span-2"
        >
          {timelinePoints.length === 0 ? (
            <p className="py-12 text-center font-mono text-xs text-faint">Chưa có dữ liệu chuỗi thời gian.</p>
          ) : (
            <div className="relative w-full">
              <svg
                viewBox={`0 0 ${W} ${H}`}
                className="h-56 w-full overflow-visible"
                onMouseLeave={() => setHoveredIdx(null)}
              >
                <defs>
                  <linearGradient id="socVelocityFill" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor="#06b6d4" stopOpacity="0.28" />
                    <stop offset="100%" stopColor="#06b6d4" stopOpacity="0.0" />
                  </linearGradient>
                </defs>

                {/* Horizontal Gridlines & Y-Axis Ticks */}
                {yTicks.map((tick) => {
                  const y = padT + innerH - (tick / yCeiling) * innerH;
                  return (
                    <g key={tick}>
                      <line x1={padL} y1={y} x2={W - padR} y2={y} stroke="#1f2937" strokeWidth="1" />
                      <text
                        x={padL - 8}
                        y={y + 4}
                        textAnchor="end"
                        fill="#64748b"
                        fontSize="11"
                        fontFamily="JetBrains Mono, monospace"
                      >
                        {tick}
                      </text>
                    </g>
                  );
                })}

                {/* Area & Line */}
                {areaPath && <path d={areaPath} fill="url(#socVelocityFill)" />}
                {linePath && (
                  <path
                    d={linePath}
                    fill="none"
                    stroke="#06b6d4"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                )}

                {/* Hover Crosshair */}
                {activePoint && (
                  <line
                    x1={activePoint.x}
                    y1={padT}
                    x2={activePoint.x}
                    y2={padT + innerH}
                    stroke="#334155"
                    strokeWidth="1"
                    strokeDasharray="3 3"
                  />
                )}

                {/* Interactive Points */}
                {coords.map((c, idx) => {
                  const isHovered = hoveredIdx === idx;
                  return (
                    <g key={idx}>
                      <circle
                        cx={c.x}
                        cy={c.y}
                        r={isHovered ? 5 : 3}
                        fill={isHovered ? "#22d3ee" : "#090d12"}
                        stroke="#06b6d4"
                        strokeWidth="2"
                      />
                      <rect
                        x={c.x - innerW / Math.max(coords.length * 2, 2)}
                        y={padT}
                        width={innerW / Math.max(coords.length, 1)}
                        height={innerH}
                        fill="transparent"
                        className="cursor-crosshair"
                        onMouseEnter={() => setHoveredIdx(idx)}
                      />
                    </g>
                  );
                })}

                {/* X-Axis Labels */}
                {coords.map((c, idx) => {
                  if (idx % labelStep !== 0 && idx !== coords.length - 1) return null;
                  return (
                    <text
                      key={`lbl-${idx}`}
                      x={c.x}
                      y={H - 6}
                      textAnchor={idx === 0 ? "start" : idx === coords.length - 1 ? "end" : "middle"}
                      fill="#64748b"
                      fontSize="11"
                      fontFamily="JetBrains Mono, monospace"
                    >
                      {formatTimelineLabel(c.pt.time)}
                    </text>
                  );
                })}
              </svg>

              {/* Floating Hover Tooltip */}
              {activePoint && (
                <div
                  className="pointer-events-none absolute top-2 right-2 rounded-md border border-border-strong bg-canvas/95 px-3 py-2 text-xs shadow-lg"
                >
                  <p className="font-mono text-[11px] text-muted">{formatFullTime(activePoint.pt.time)}</p>
                  <p className="mt-0.5 font-mono font-semibold text-accent tabular-nums">
                    {activePoint.pt.count.toLocaleString()} events
                  </p>
                </div>
              )}
            </div>
          )}
        </Panel>

        {/* Severity Distribution */}
        <Panel title="Phân bổ mức độ nghiêm trọng" subtitle={`Tổng ${data.total_events.toLocaleString()} sự kiện`}>
          <div className="space-y-4">
            {severities.map((sev) => {
              const pct = Math.min((sev.count / totalEvents) * 100, 100);
              return (
                <div key={sev.key}>
                  <div className="mb-1.5 flex items-baseline justify-between text-xs">
                    <span className={`font-medium ${sev.text}`}>{sev.label}</span>
                    <span className="font-mono text-ink-secondary tabular-nums">
                      {sev.count.toLocaleString()}{" "}
                      <span className="text-faint">({pct.toFixed(1)}%)</span>
                    </span>
                  </div>
                  <div className="h-2 w-full overflow-hidden rounded-full bg-canvas">
                    <div
                      className={`h-full rounded-full transition-all duration-300 ${sev.bar}`}
                      style={{ width: `${Math.max(pct, sev.count > 0 ? 2 : 0)}%` }}
                    />
                  </div>
                </div>
              );
            })}
          </div>
        </Panel>
      </div>

      {/* Row 2: GeoIP Threat Intelligence Table */}
      <Panel
        title="Nguồn tấn công hàng đầu & GeoIP"
        subtitle="Tổng hợp IP nguồn phát sinh nhiều sự kiện an ninh nhất"
        actions={<StatusPill tone="danger">{(data.top_attacking_ips ?? []).length} nguồn</StatusPill>}
        noPadding
      >
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead>
              <tr className="border-b border-border bg-surface-raised/60 font-mono text-[11px] uppercase tracking-wider text-muted">
                <th className="px-5 py-3">Attacker IP</th>
                <th className="px-4 py-3">Vị trí (GeoIP)</th>
                <th className="px-4 py-3">Phân loại mối đe dọa</th>
                <th className="px-4 py-3">Điểm rủi ro</th>
                <th className="px-5 py-3 text-right">Số sự kiện</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {(data.top_attacking_ips ?? []).length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-5 py-8 text-center text-faint">
                    Chưa ghi nhận IP tấn công nổi bật.
                  </td>
                </tr>
              ) : (
                (data.top_attacking_ips ?? []).map((ip, idx) => {
                  const score = ip.reputation_score ?? 0;
                  const scoreColor =
                    score >= 75 ? "bg-red-500" : score >= 40 ? "bg-amber-500" : "bg-cyan-500";
                  return (
                    <tr key={idx} className="transition-colors hover:bg-surface-hover/60">
                      <td className="px-5 py-3 font-mono font-semibold text-ink">{ip.ip}</td>
                      <td className="px-4 py-3 text-ink-secondary">
                        <span>{ip.country || "Local / Private"}</span>
                        <span className="ml-2 rounded border border-border bg-canvas px-1.5 py-0.5 font-mono text-[10px] text-muted">
                          {ip.country_code || "LAN"}
                        </span>
                      </td>
                      <td className="px-4 py-3">
                        <StatusPill tone="warning">{ip.threat_category ?? "Unknown"}</StatusPill>
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-2.5">
                          <div className="h-1.5 w-24 overflow-hidden rounded-full bg-canvas">
                            <div className={`h-full rounded-full ${scoreColor}`} style={{ width: `${score}%` }} />
                          </div>
                          <span className="font-mono text-ink-secondary tabular-nums">
                            {ip.reputation_score === undefined ? "—" : `${ip.reputation_score}/100`}
                          </span>
                        </div>
                      </td>
                      <td className="px-5 py-3 text-right font-mono font-semibold text-accent tabular-nums">
                        {ip.count.toLocaleString()}
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
      </Panel>
    </div>
  );
}
