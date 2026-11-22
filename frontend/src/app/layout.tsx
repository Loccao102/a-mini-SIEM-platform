import type { Metadata } from "next";
import "./globals.css";
import { RouteGuard } from "@/components/auth-gate";
import { NavHeader } from "@/components/nav-header";

export const metadata: Metadata = {
  title: "Sentinel SIEM | Security Command Center",
  description: "Security Operations Center command console for threat detection, triage, and automated response",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="vi" className="dark">
      <body className="min-h-screen bg-canvas text-ink antialiased">
        <RouteGuard>
          <NavHeader>{children}</NavHeader>
        </RouteGuard>
      </body>
    </html>
  );
}
