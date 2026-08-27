"use client";

import { ReactNode, useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";

export function AuthGate({ children }: { children: ReactNode }) {
  const router = useRouter();
  const [authorized, setAuthorized] = useState(false);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      if (localStorage.getItem("siem_token")) setAuthorized(true);
      else router.replace("/login");
    }, 0);
    return () => window.clearTimeout(timer);
  }, [router]);

  if (!authorized) return <main className="auth-loading">Checking access...</main>;
  return children;
}

export function RouteGuard({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  if (pathname === "/login" || pathname === "/accounts") return children;
  return <AuthGate>{children}</AuthGate>;
}
