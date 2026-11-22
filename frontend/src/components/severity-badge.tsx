type Severity = "critical" | "high" | "medium" | "low" | "info" | "warning" | "error";

const severityConfig: Record<string, { label: string; dot: string; pill: string }> = {
  critical: {
    label: "Critical",
    dot: "bg-red-400",
    pill: "border-red-500/35 bg-red-500/15 text-red-300",
  },
  error: {
    label: "Error",
    dot: "bg-red-400",
    pill: "border-red-500/35 bg-red-500/15 text-red-300",
  },
  high: {
    label: "High",
    dot: "bg-amber-400",
    pill: "border-amber-500/35 bg-amber-500/15 text-amber-300",
  },
  warning: {
    label: "Warning",
    dot: "bg-amber-400",
    pill: "border-amber-500/35 bg-amber-500/15 text-amber-300",
  },
  medium: {
    label: "Medium",
    dot: "bg-sky-400",
    pill: "border-sky-500/35 bg-sky-500/15 text-sky-300",
  },
  low: {
    label: "Low",
    dot: "bg-slate-400",
    pill: "border-slate-600/60 bg-slate-800/80 text-slate-300",
  },
  info: {
    label: "Info",
    dot: "bg-cyan-400",
    pill: "border-cyan-500/30 bg-cyan-500/10 text-cyan-300",
  },
};

export function SeverityBadge({ severity }: { severity?: string }) {
  const key = (severity ?? "low").toLowerCase() as Severity;
  const cfg = severityConfig[key] ?? severityConfig.low;

  return (
    <span
      className={`inline-flex w-fit items-center gap-1.5 rounded-md border px-2 py-0.5 font-mono text-[11px] font-medium uppercase tracking-wide ${cfg.pill}`}
    >
      <span className={`h-1.5 w-1.5 rounded-full ${cfg.dot}`} aria-hidden="true" />
      <span>{cfg.label}</span>
    </span>
  );
}