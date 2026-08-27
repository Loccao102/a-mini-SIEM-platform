import type { Metadata } from "next";
import "./globals.css";
import { RouteGuard } from "@/components/auth-gate";

export const metadata: Metadata = {
  title: "Sentinel | Security overview",
  description: "A mini SIEM command center",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en">
      <body className="min-h-full flex flex-col"><RouteGuard>{children}</RouteGuard></body>
    </html>
  );
}
