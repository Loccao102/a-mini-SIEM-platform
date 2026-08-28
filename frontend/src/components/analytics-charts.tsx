"use client";

import { AnalyticsData } from "@/lib/api";

function countryFlag(code: string) {
  switch (code.toUpperCase()) {
    case "DE":
      return "🇩🇪";
    case "US":
      return "🇺🇸";
    case "RU":
      return "🇷🇺";
    case "CN":
      return "🇨🇳";
    case "VN":
      return "🇻🇳";
    case "LAN":
      return "🏠";
    default:
      return "🌐";
  }
}

function formatTimelineLabel(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat(undefined, {
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

export function AnalyticsCharts({ data }: { data: AnalyticsData | null }) {
  if (!data) return null;

  const timelinePoints = data.timeline ?? [];
  const maxTimelineCount = Math.max(...timelinePoints.map((p) => p.count), 1);
  const totalEvents = Math.max(data.total_events, 1);
  const chartHeight = 120;
  const chartWidth = 640;

  // Build SVG path for Timeline
  const pointsString = timelinePoints
    .map((pt, idx) => {
      const x = (idx / Math.max(timelinePoints.length - 1, 1)) * chartWidth;
      const y = chartHeight - (pt.count / maxTimelineCount) * (chartHeight - 20);
      return `${x},${y}`;
    })
    .join(" ");

  const areaString = timelinePoints.length
    ? `0,${chartHeight} ${pointsString} ${chartWidth},${chartHeight}`
    : "";

  return (
    <section className="analytics-section mt-12 space-y-8">
      {/* 1. Timeline Velocity Chart & Severity Breakdown */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        {/* Timeline Chart */}
        <div className="md:col-span-2 p-6 border border-(--line) bg-(--surface) rounded">
          <div className="flex justify-between items-center mb-4">
            <div>
              <span className="eyebrow block">Real-time Telemetry</span>
              <h3 className="text-xl font-medium text-(--ink)">Event Velocity Timeline</h3>
            </div>
            <span className="text-xs font-mono text-(--acid)">24h Velocity</span>
          </div>

          <div className="relative w-full overflow-hidden">
            <svg viewBox={`0 0 ${chartWidth} ${chartHeight}`} className="w-full h-32 overflow-visible">
              <defs>
                <linearGradient id="chartGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="var(--acid)" stopOpacity="0.35" />
                  <stop offset="100%" stopColor="var(--acid)" stopOpacity="0.0" />
                </linearGradient>
              </defs>

              {/* Area Fill */}
              {areaString && <polygon points={areaString} fill="url(#chartGrad)" />}

              {/* Line */}
              {pointsString && (
                <polyline
                  fill="none"
                  stroke="var(--acid)"
                  strokeWidth="2.5"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  points={pointsString}
                />
              )}

              {/* Dots */}
              {timelinePoints.map((pt, idx) => {
                const x = (idx / Math.max(timelinePoints.length - 1, 1)) * chartWidth;
                const y = chartHeight - (pt.count / maxTimelineCount) * (chartHeight - 20);
                return (
                  <circle
                    key={idx}
                    cx={x}
                    cy={y}
                    r="4"
                    fill="var(--canvas)"
                    stroke="var(--acid)"
                    strokeWidth="2"
                  />
                );
              })}
            </svg>

            <div className="flex justify-between text-xs font-mono text-(--muted) mt-2">
              {timelinePoints.map((pt, idx) => (
                <span key={idx}>{formatTimelineLabel(pt.time)}</span>
              ))}
            </div>
          </div>
        </div>

        {/* Severity Distribution */}
        <div className="p-6 border border-(--line) bg-(--surface) rounded flex flex-col justify-between">
          <div>
            <span className="eyebrow block">Risk Matrix</span>
            <h3 className="text-xl font-medium text-(--ink) mb-4">Severity Breakdown</h3>

            <div className="space-y-3">
              <div>
                <div className="flex justify-between text-xs font-mono mb-1">
                  <span className="text-(--coral) uppercase font-bold">Critical</span>
                  <span>{data.events_by_severity?.critical ?? 0}</span>
                </div>
                <div className="w-full bg-(--canvas) h-2 rounded overflow-hidden">
                  <div
                    className="bg-(--coral) h-full"
                    style={{ width: `${Math.min(((data.events_by_severity?.critical ?? 0) / totalEvents) * 100, 100)}%` }}
                  />
                </div>
              </div>

              <div>
                <div className="flex justify-between text-xs font-mono mb-1">
                  <span className="text-(--amber) uppercase font-bold">High</span>
                  <span>{data.events_by_severity?.high ?? 0}</span>
                </div>
                <div className="w-full bg-(--canvas) h-2 rounded overflow-hidden">
                  <div
                    className="bg-(--amber) h-full"
                    style={{ width: `${Math.min(((data.events_by_severity?.high ?? 0) / totalEvents) * 100, 100)}%` }}
                  />
                </div>
              </div>

              <div>
                <div className="flex justify-between text-xs font-mono mb-1">
                  <span className="text-(--aqua) uppercase font-bold">Medium</span>
                  <span>{data.events_by_severity?.medium ?? 0}</span>
                </div>
                <div className="w-full bg-(--canvas) h-2 rounded overflow-hidden">
                  <div
                    className="bg-(--aqua) h-full"
                    style={{ width: `${Math.min(((data.events_by_severity?.medium ?? 0) / totalEvents) * 100, 100)}%` }}
                  />
                </div>
              </div>

              <div>
                <div className="flex justify-between text-xs font-mono mb-1">
                  <span className="text-(--acid) uppercase font-bold">Info / Low</span>
                  <span>{data.events_by_severity?.info ?? 0}</span>
                </div>
                <div className="w-full bg-(--canvas) h-2 rounded overflow-hidden">
                  <div
                    className="bg-(--acid) h-full"
                    style={{ width: `${Math.min(((data.events_by_severity?.info ?? 0) / totalEvents) * 100, 100)}%` }}
                  />
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* 2. GeoIP Threat Matrix & Top Attacking IPs */}
      <div className="p-6 border border-(--line) bg-(--surface) rounded">
        <div className="flex justify-between items-center mb-4">
          <div>
            <span className="eyebrow block">Threat Intelligence & GeoIP</span>
            <h3 className="text-xl font-medium text-(--ink)">🌐 Top Attacking IPs & GeoIP Origin</h3>
          </div>
          <span className="text-xs font-mono text-(--coral)">Active Threats</span>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs font-mono">
            <thead>
              <tr className="border-b border-(--line) text-(--muted) uppercase">
                <th className="pb-3">Attacker IP</th>
                <th className="pb-3">Location (GeoIP)</th>
                <th className="pb-3">Threat Category</th>
                <th className="pb-3">Reputation Score</th>
                <th className="pb-3 text-right">Event Count</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-(--line)">
              {(data.top_attacking_ips ?? []).map((ip, idx) => (
                <tr key={idx} className="hover:bg-(--canvas)">
                  <td className="py-3 font-bold text-(--ink)">{ip.ip}</td>
                  <td className="py-3">
                    <span className="mr-2 text-base">{countryFlag(ip.country_code)}</span>
                    <span>
                      {ip.country} ({ip.country_code})
                    </span>
                  </td>
                  <td className="py-3 text-(--amber)">{ip.threat_category ?? "Unknown"}</td>
                  <td className="py-3">
                    <div className="flex items-center gap-2">
                      <div className="w-24 bg-(--canvas) h-2 rounded overflow-hidden">
                        <div
                          className="bg-(--coral) h-full"
                          style={{ width: `${ip.reputation_score ?? 0}%` }}
                        />
                      </div>
                      <span className="text-(--coral)">{ip.reputation_score ?? "Unknown"}{ip.reputation_score === undefined ? "" : "/100"}</span>
                    </div>
                  </td>
                  <td className="py-3 text-right font-bold text-(--acid)">{ip.count}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}
