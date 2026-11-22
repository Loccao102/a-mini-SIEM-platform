import React from "react";
import { XIcon } from "./icons";

export function PageShell({ children, className = "" }: { children: React.ReactNode; className?: string }) {
  return (
    <main className={`min-h-screen w-full px-5 py-6 md:px-8 md:py-7 ${className}`}>
      <div className="mx-auto max-w-[1440px] space-y-6">{children}</div>
    </main>
  );
}

export function PageHeader({
  title,
  description,
  badge,
  actions,
}: {
  title: string;
  description?: string;
  badge?: React.ReactNode;
  actions?: React.ReactNode;
}) {
  return (
    <header className="flex flex-col gap-3 border-b border-border pb-5 sm:flex-row sm:items-end sm:justify-between">
      <div className="space-y-1">
        <div className="flex flex-wrap items-center gap-2.5">
          <h1 className="text-2xl font-semibold tracking-tight text-ink">{title}</h1>
          {badge}
        </div>
        {description && <p className="max-w-2xl text-sm text-muted">{description}</p>}
      </div>
      {actions && <div className="flex shrink-0 flex-wrap items-center gap-2.5">{actions}</div>}
    </header>
  );
}

export function Panel({
  title,
  subtitle,
  actions,
  children,
  className = "",
  noPadding = false,
}: {
  title?: string;
  subtitle?: string;
  actions?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
  noPadding?: boolean;
}) {
  return (
    <section className={`rounded-lg border border-border bg-surface ${className}`}>
      {(title || actions) && (
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-5 py-3.5">
          <div>
            {title && <h2 className="text-sm font-semibold text-ink">{title}</h2>}
            {subtitle && <p className="mt-0.5 text-xs text-muted">{subtitle}</p>}
          </div>
          {actions && <div className="flex items-center gap-2">{actions}</div>}
        </div>
      )}
      <div className={noPadding ? "" : "p-5"}>{children}</div>
    </section>
  );
}

type KpiTone = "default" | "critical" | "high" | "accent" | "success";

const kpiToneStyles: Record<KpiTone, string> = {
  default: "text-ink",
  critical: "text-critical",
  high: "text-high",
  accent: "text-accent",
  success: "text-success",
};

export function KpiCard({
  label,
  value,
  subtext,
  tone = "default",
  icon,
}: {
  label: string;
  value: React.ReactNode;
  subtext?: string;
  tone?: KpiTone;
  icon?: React.ReactNode;
}) {
  return (
    <article className="flex flex-col justify-between rounded-lg border border-border bg-surface p-4 transition-colors hover:border-border-strong">
      <div className="flex items-center justify-between gap-2">
        <span className="text-xs font-medium text-muted">{label}</span>
        {icon && <span className="text-faint">{icon}</span>}
      </div>
      <div className={`mt-3 font-mono text-3xl font-semibold tracking-tight tabular-nums ${kpiToneStyles[tone]}`}>
        {value}
      </div>
      {subtext && <p className="mt-1.5 text-xs text-faint">{subtext}</p>}
    </article>
  );
}

type ButtonVariant = "primary" | "secondary" | "ghost" | "danger" | "success";
type ButtonSize = "sm" | "md";

const buttonVariants: Record<ButtonVariant, string> = {
  primary: "bg-accent text-canvas font-semibold hover:bg-accent-hover border border-transparent",
  secondary: "bg-surface-raised text-ink border border-border-strong hover:bg-surface-hover",
  ghost: "bg-transparent text-muted border border-transparent hover:bg-surface-raised hover:text-ink",
  danger: "bg-critical/15 text-red-300 border border-critical/35 hover:bg-critical/25",
  success: "bg-success/15 text-emerald-300 border border-success/35 hover:bg-success/25",
};

const buttonSizes: Record<ButtonSize, string> = {
  sm: "px-2.5 py-1.5 text-xs",
  md: "px-3.5 py-2 text-xs",
};

export function Button({
  variant = "secondary",
  size = "md",
  className = "",
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant;
  size?: ButtonSize;
}) {
  return (
    <button
      {...props}
      className={`inline-flex cursor-pointer items-center justify-center gap-1.5 rounded-md font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${buttonVariants[variant]} ${buttonSizes[size]} ${className}`}
    />
  );
}

type PillTone = "neutral" | "accent" | "success" | "warning" | "danger" | "info";

const pillTones: Record<PillTone, string> = {
  neutral: "bg-slate-800/80 text-slate-300 border-slate-700",
  accent: "bg-cyan-500/15 text-cyan-300 border-cyan-500/30",
  success: "bg-emerald-500/15 text-emerald-300 border-emerald-500/30",
  warning: "bg-amber-500/15 text-amber-300 border-amber-500/30",
  danger: "bg-red-500/15 text-red-300 border-red-500/30",
  info: "bg-sky-500/15 text-sky-300 border-sky-500/30",
};

export function StatusPill({
  children,
  tone = "neutral",
  className = "",
}: {
  children: React.ReactNode;
  tone?: PillTone;
  className?: string;
}) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-md border px-2 py-0.5 font-mono text-[11px] font-medium ${pillTones[tone]} ${className}`}
    >
      {children}
    </span>
  );
}

export function Notice({
  tone = "error",
  children,
  onDismiss,
}: {
  tone?: "error" | "success" | "info";
  children: React.ReactNode;
  onDismiss?: () => void;
}) {
  const styles =
    tone === "success"
      ? "border-emerald-500/35 bg-emerald-950/40 text-emerald-200"
      : tone === "info"
      ? "border-cyan-500/35 bg-cyan-950/40 text-cyan-200"
      : "border-red-500/35 bg-red-950/40 text-red-200";
  return (
    <div role="alert" className={`flex items-center justify-between gap-3 rounded-lg border px-4 py-3 text-xs ${styles}`}>
      <div className="leading-relaxed">{children}</div>
      {onDismiss && (
        <button
          type="button"
          onClick={onDismiss}
          aria-label="Đóng thông báo"
          className="cursor-pointer rounded p-1 opacity-70 hover:opacity-100"
        >
          <XIcon size={14} />
        </button>
      )}
    </div>
  );
}

export function EmptyState({
  title,
  description,
  icon,
}: {
  title: string;
  description?: string;
  icon?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center px-6 py-12 text-center">
      {icon && <div className="mb-3 rounded-lg border border-border bg-surface-raised p-2.5 text-faint">{icon}</div>}
      <p className="text-sm font-medium text-ink-secondary">{title}</p>
      {description && <p className="mt-1 max-w-md text-xs text-faint">{description}</p>}
    </div>
  );
}

export function Drawer({
  open,
  onClose,
  title,
  subtitle,
  children,
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  subtitle?: string;
  children: React.ReactNode;
}) {
  if (!open) return null;
  return (
    <div
      className="fixed inset-0 z-50 flex justify-end bg-black/70 backdrop-blur-[2px]"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        className="flex h-full w-full max-w-2xl flex-col border-l border-border-strong bg-surface shadow-2xl"
      >
        <div className="flex items-start justify-between gap-4 border-b border-border px-6 py-4">
          <div>
            {subtitle && <p className="font-mono text-[11px] uppercase tracking-wider text-accent">{subtitle}</p>}
            <h2 className="mt-0.5 text-base font-semibold text-ink">{title}</h2>
          </div>
          <Button variant="ghost" size="sm" onClick={onClose} aria-label="Đóng chi tiết">
            <XIcon size={16} />
          </Button>
        </div>
        <div className="flex-1 overflow-y-auto p-6">{children}</div>
      </div>
    </div>
  );
}
