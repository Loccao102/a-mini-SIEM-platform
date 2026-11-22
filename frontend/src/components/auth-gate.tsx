"use client";

import { ReactNode, useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { ShieldIcon } from "@/components/ui/icons";

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

  if (!authorized) {
    return (
      <main className="flex min-h-screen flex-col items-center justify-center gap-3 bg-canvas text-muted">
        <div className="flex h-10 w-10 items-center justify-center rounded-lg border border-accent/30 bg-accent/10 text-accent">
          <ShieldIcon size={20} />
        </div>
        <p className="font-mono text-xs tracking-wide">Đang xác thực phiên SOC...</p>
      </main>
    );
  }
  return children;
}

export function RouteGuard({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  if (pathname === "/login" || pathname === "/accounts") return children;
  return <AuthGate>{children}</AuthGate>;
}
