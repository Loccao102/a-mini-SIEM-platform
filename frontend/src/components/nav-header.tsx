"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { getMe, User } from "@/lib/api";

const demoModeAvailable = process.env.NEXT_PUBLIC_MODE === "develop";

export function NavHeader() {
  const pathname = usePathname();
  const router = useRouter();
  const [currentUser, setCurrentUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(() =>
    typeof window !== "undefined" && Boolean(localStorage.getItem("siem_token"))
  );
  const [demoEnabled, setDemoEnabled] = useState(() =>
    demoModeAvailable && (typeof window === "undefined" || localStorage.getItem("siem_mode") !== "production")
  );

  useEffect(() => {
    let active = true;
    const token = typeof window !== "undefined" ? localStorage.getItem("siem_token") : null;
    if (!token) {
      return;
    }
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

  const roleLabel = currentUser?.role?.toUpperCase() ?? "GUEST";
  const badgeClass =
    currentUser?.role === "admin"
      ? "badge-admin"
      : currentUser?.role === "analyst"
      ? "badge-analyst"
      : currentUser?.role === "viewer"
      ? "badge-viewer"
      : "badge-guest";

  if (pathname === "/login") return null;

  const navigation = [
    ["/", "Overview", "overview"],
    ["/alerts", "Alerts", "alerts"],
    ["/cases", "Cases", "cases"],
    ["/events", "Log Explorer", "events"],
    ["/rules", "Rules", "rules"],
    ["/assets", "Assets", "assets"],
    ["/accounts", "Accounts", "accounts"],
  ] as const;

  return (
    <aside className="siem-nav-header">
      <div className="nav-brand">
        <Link href="/" className="logo-box">
          <span className="mark">S</span>
          <span><strong>Sentinel</strong><small>Security operations</small></span>
        </Link>
        <span className="version">SIEM v2.0</span>
      </div>

      <nav className="nav-links" aria-label="Primary navigation">
        <span className="nav-section-label">Operations</span>
        {navigation.map(([href, label, icon]) => (
          <Link className={pathname === href ? "active" : ""} href={href} key={href}>
            <span className={`nav-icon nav-icon-${icon}`} aria-hidden="true" />
            {label}
          </Link>
        ))}
      </nav>

      <div className="nav-user">
        <span className="status-pill"><span className="pulse-dot" /> Live link</span>
        <div className="mode-toggle" aria-label="Application mode">
          <button type="button" className={demoEnabled ? "active" : ""} disabled={!demoModeAvailable} onClick={() => setMode("demo")}>Demo</button>
          <button type="button" className={!demoEnabled ? "active" : ""} onClick={() => setMode("production")}>Production</button>
        </div>
        {!loading && currentUser ? (
          <>
            <div className="user-info">
              <span className="user-name">{currentUser.display_name || currentUser.email}</span>
              <span className={`role-badge ${badgeClass}`}>{roleLabel}</span>
            </div>
            <button onClick={signOut} className="btn-signout" title="Sign out">
              Sign out
            </button>
          </>
        ) : !loading ? (
          <Link href="/login" className="btn-signin">
            Sign in
          </Link>
        ) : (
          <span className="nav-user-loading">...</span>
        )}
      </div>
    </aside>
  );
}
