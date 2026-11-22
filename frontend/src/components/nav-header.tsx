"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import React, { useEffect, useState } from "react";
import { getMe, User } from "@/lib/api";
import {
  ActivityIcon,
  BellIcon,
  BriefcaseIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  DashboardIcon,
  LogInIcon,
  LogOutIcon,
  ServerIcon,
  ShieldAlertIcon,
  ShieldIcon,
  TerminalIcon,
  UsersIcon,
  ZapIcon,
} from "@/components/ui/icons";

const demoModeAvailable = process.env.NEXT_PUBLIC_MODE === "develop";

type NavGroup = {
  label: string;
  items: ReadonlyArray<{
    href: string;
    label: string;
    icon: React.ComponentType<{ size?: number; className?: string }>;
    adminOnly?: boolean;
  }>;
};

const NAV_GROUPS: NavGroup[] = [
  {
    label: "Operations",
    items: [
      { href: "/", label: "Overview", icon: DashboardIcon },
      { href: "/alerts", label: "Alerts", icon: BellIcon },
      { href: "/soar", label: "SOAR Actions", icon: ZapIcon },
    ],
  },
  {
    label: "Investigation",
    items: [
      { href: "/cases", label: "Cases", icon: BriefcaseIcon },
      { href: "/events", label: "Log Explorer", icon: TerminalIcon },
      { href: "/rules", label: "Detection Rules", icon: ShieldAlertIcon },
    ],
  },
  {
    label: "Administration",
    items: [
      { href: "/assets", label: "Assets", icon: ServerIcon },
      { href: "/accounts", label: "Accounts", icon: UsersIcon },
      { href: "/system", label: "System Health", icon: ActivityIcon, adminOnly: true },
    ],
  },
];

export function NavHeader({ children }: { children?: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const [collapsed, setCollapsed] = useState(false);
  const [currentUser, setCurrentUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(
    () => typeof window !== "undefined" && Boolean(localStorage.getItem("siem_token"))
  );
  const [demoEnabled, setDemoEnabled] = useState(
    () =>
      demoModeAvailable &&
      (typeof window === "undefined" || localStorage.getItem("siem_mode") !== "production")
  );

  useEffect(() => {
    let active = true;
    const token = typeof window !== "undefined" ? localStorage.getItem("siem_token") : null;
    if (!token) return;
    getMe()
      .then((user) => {
        if (active) setCurrentUser(user);
      })
      .catch(() => {
        if (active) setCurrentUser(null);
      })
      .finally(() => {
        if (active) setLoading(false);
      });

    return () => {
      active = false;
    };
  }, [pathname]);

  function signOut() {
    localStorage.removeItem("siem_token");
    setCurrentUser(null);
    router.push("/login");
  }

  function setMode(mode: "demo" | "production") {
    const enabled = mode === "demo" && demoModeAvailable;
    localStorage.setItem("siem_mode", enabled ? "demo" : "production");
    setDemoEnabled(enabled);
    window.dispatchEvent(new CustomEvent("siem-mode-change", { detail: enabled }));
  }

  // Standalone layout on /login (no sidebar, no left offset)
  if (pathname === "/login") {
    return <>{children}</>;
  }

  const roleLabel = currentUser?.role?.toUpperCase() ?? "GUEST";
  const roleBadgeClass =
    currentUser?.role === "admin"
      ? "border-red-500/35 bg-red-500/15 text-red-300"
      : currentUser?.role === "analyst"
      ? "border-amber-500/35 bg-amber-500/15 text-amber-300"
      : "border-cyan-500/35 bg-cyan-500/15 text-cyan-300";

  return (
    <div className="flex min-h-screen w-full flex-col bg-canvas lg:flex-row">
      {/* Desktop Sidebar & Mobile Header */}
      <aside
        className={`sticky top-0 z-40 flex w-full shrink-0 flex-col border-b border-border bg-surface/95 backdrop-blur-md transition-[width] duration-200 lg:h-screen lg:border-r lg:border-b-0 ${
          collapsed ? "lg:w-[68px]" : "lg:w-[248px]"
        }`}
      >
        {/* Brand Row */}
        <div className="flex items-center justify-between gap-2 border-b border-border px-4 py-3.5">
          <Link href="/" className="flex min-w-0 items-center gap-2.5">
            <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md border border-accent/40 bg-accent/15 text-accent">
              <ShieldIcon size={17} />
            </span>
            {!collapsed && (
              <span className="min-w-0">
                <strong className="block truncate text-sm font-semibold tracking-tight text-ink">
                  Sentinel SIEM
                </strong>
                <span className="block font-mono text-[10px] uppercase tracking-wider text-faint">
                  SOC Console v2.0
                </span>
              </span>
            )}
          </Link>

          <button
            type="button"
            onClick={() => setCollapsed((c) => !c)}
            title={collapsed ? "Mở rộng menu" : "Thu gọn menu"}
            aria-label={collapsed ? "Mở rộng menu" : "Thu gọn menu"}
            className="hidden h-7 w-7 cursor-pointer items-center justify-center rounded-md border border-border bg-canvas text-muted transition-colors hover:border-border-strong hover:text-ink lg:inline-flex"
          >
            {collapsed ? <ChevronRightIcon size={14} /> : <ChevronLeftIcon size={14} />}
          </button>
        </div>

        {/* Navigation Links */}
        <nav
          aria-label="Primary navigation"
          className="flex gap-1 overflow-x-auto px-3 py-2.5 lg:flex-1 lg:flex-col lg:gap-5 lg:overflow-y-auto lg:py-4"
        >
          {NAV_GROUPS.map((group) => {
            const visibleItems = group.items.filter(
              (item) => !item.adminOnly || currentUser?.role === "admin"
            );
            if (!visibleItems.length) return null;

            return (
              <div key={group.label} className="flex shrink-0 items-center gap-1 lg:block">
                {!collapsed && (
                  <p className="hidden px-2.5 pb-1.5 font-mono text-[10px] font-medium uppercase tracking-widest text-faint lg:block">
                    {group.label}
                  </p>
                )}
                <div className="flex gap-1 lg:flex-col lg:gap-0.5">
                  {visibleItems.map((item) => {
                    const active = pathname === item.href;
                    const Icon = item.icon;
                    return (
                      <Link
                        key={item.href}
                        href={item.href}
                        title={collapsed ? item.label : undefined}
                        className={`group relative flex shrink-0 items-center gap-2.5 rounded-md px-2.5 py-2 text-[13px] font-medium transition-colors ${
                          collapsed ? "lg:justify-center lg:px-0" : ""
                        } ${
                          active
                            ? "bg-accent/12 text-accent"
                            : "text-muted hover:bg-surface-raised hover:text-ink"
                        }`}
                      >
                        {active && (
                          <span
                            aria-hidden="true"
                            className="hidden h-4 w-0.5 rounded-r bg-accent lg:absolute lg:left-0 lg:block"
                          />
                        )}
                        <Icon
                          size={16}
                          className={active ? "text-accent" : "text-faint group-hover:text-ink-secondary"}
                        />
                        {!collapsed && <span className="truncate">{item.label}</span>}
                      </Link>
                    );
                  })}
                </div>
              </div>
            );
          })}
        </nav>

        {/* Bottom Status & User Controls */}
        <div className="hidden border-t border-border p-3 lg:block">
          {!collapsed && (
            <div className="mb-3 space-y-2.5">
              <div className="flex items-center justify-between px-1">
                <span className="inline-flex items-center gap-2 font-mono text-[11px] text-emerald-400">
                  <span className="soc-pulse h-2 w-2 rounded-full bg-emerald-400" />
                  Live link
                </span>
                <span className="font-mono text-[10px] text-faint">TLS 1.3</span>
              </div>

              <div
                className="grid grid-cols-2 rounded-md border border-border bg-canvas p-0.5"
                aria-label="Application mode"
              >
                <button
                  type="button"
                  disabled={!demoModeAvailable}
                  onClick={() => setMode("demo")}
                  className={`cursor-pointer rounded py-1 font-mono text-[10px] font-medium uppercase transition-colors disabled:cursor-not-allowed disabled:opacity-40 ${
                    demoEnabled ? "bg-accent text-canvas font-semibold" : "text-muted hover:text-ink"
                  }`}
                >
                  Demo
                </button>
                <button
                  type="button"
                  onClick={() => setMode("production")}
                  className={`cursor-pointer rounded py-1 font-mono text-[10px] font-medium uppercase transition-colors ${
                    !demoEnabled ? "bg-accent text-canvas font-semibold" : "text-muted hover:text-ink"
                  }`}
                >
                  Prod
                </button>
              </div>
            </div>
          )}

          {!loading && currentUser ? (
            <div className={`flex items-center gap-2.5 ${collapsed ? "flex-col" : "justify-between"}`}>
              {!collapsed && (
                <div className="min-w-0 flex-1">
                  <p className="truncate text-xs font-medium text-ink">
                    {currentUser.display_name || currentUser.email}
                  </p>
                  <span
                    className={`mt-1 inline-block rounded border px-1.5 py-0.5 font-mono text-[10px] font-semibold ${roleBadgeClass}`}
                  >
                    {roleLabel}
                  </span>
                </div>
              )}
              <button
                type="button"
                onClick={signOut}
                title="Đăng xuất"
                aria-label="Đăng xuất"
                className="inline-flex h-8 w-8 shrink-0 cursor-pointer items-center justify-center rounded-md border border-border bg-canvas text-muted transition-colors hover:border-red-500/40 hover:text-red-300"
              >
                <LogOutIcon size={14} />
              </button>
            </div>
          ) : !loading ? (
            <Link
              href="/login"
              className="flex w-full items-center justify-center gap-1.5 rounded-md bg-accent px-3 py-2 text-xs font-semibold text-canvas transition-colors hover:bg-accent-hover"
            >
              <LogInIcon size={14} />
              {!collapsed && <span>Đăng nhập</span>}
            </Link>
          ) : (
            <div className="h-8 animate-pulse rounded bg-surface-raised" />
          )}
        </div>
      </aside>

      {/* Main Content Container */}
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  );
}
