import type { Metadata } from "next";
import "./globals.css";
import { RouteGuard } from "@/components/auth-gate";
import { NavHeader } from "@/components/nav-header";

export const metadata: Metadata = {
  title: "Sentinel | Security Command Center",
  description: "A mini SIEM command center",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="vi">
      <body className="min-h-full flex flex-col">
        <RouteGuard>
          <NavHeader />
          {children}
        </RouteGuard>
      </body>
    </html>
  );
}
