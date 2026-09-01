type Severity = "critical" | "high" | "medium" | "low" | "info";

const labels: Record<Severity, string> = {
  critical: "Critical",
  high: "High",
  medium: "Medium",
  low: "Low",
  info: "Low",
};

export function SeverityBadge({ severity }: { severity: string }) {
  const normalized = (severity.toLowerCase() in labels ? severity.toLowerCase() : "low") as Severity;

  return (
    <span className={`severity-badge severity-${normalized}`}>
      <svg aria-hidden="true" viewBox="0 0 16 16" focusable="false">
        {normalized === "critical" ? <path d="M8 1.5 14.5 14h-13L8 1.5Zm0 4v4m0 2.1v.1" /> : null}
        {normalized === "high" ? <path d="M8 2v8m0 3v.1M3 5l5-3 5 3M3 11l5 3 5-3" /> : null}
        {normalized === "medium" ? <path d="M3 8h10M8 3v10" /> : null}
        {normalized === "low" ? <path d="M3 3v10h10M5 10l2-2 2 1 3-4" /> : null}
      </svg>
      <span>{labels[normalized]}</span>
    </span>
  );
}