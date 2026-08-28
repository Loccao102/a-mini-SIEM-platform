"use client";

import { FormEvent, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { login } from "@/lib/api";

export default function LoginPage() {
  const router = useRouter();
  const isDevelop = process.env.NEXT_PUBLIC_MODE === "develop";
  const [form, setForm] = useState({ email: "admin@example.com", password: isDevelop ? "admin" : "" });
  const [showPassword, setShowPassword] = useState(false);
  const [message, setMessage] = useState("");
  const [loading, setLoading] = useState(false);



  async function performLogin(email: string, pass: string) {
    setLoading(true);
    setMessage("");
    try {
      const result = await login(email, pass);
      localStorage.setItem("siem_token", result.token);
      router.replace("/");
    } catch {
      setMessage("Email hoặc mật khẩu không đúng, hoặc máy chủ API chưa sẵn sàng.");
    } finally {
      setLoading(false);
    }
  }

  async function submit(event: FormEvent) {
    event.preventDefault();
    await performLogin(form.email, form.password);
  }

  function handleDemoLogin(email: string, pass: string) {
    setForm({ email, password: pass });
    void performLogin(email, pass);
  }

  return (
    <main className="account-shell flex items-center justify-center min-h-screen">
      <section className="account-form max-w-md w-full p-8 border border-[var(--line)] bg-[var(--surface)] rounded">
        <div className="login-brand mb-4 flex items-center gap-3">
          <span className="mark">S</span>
          <strong className="text-xl">Sentinel SIEM</strong>
        </div>
        <p className="eyebrow">SOC Access Control</p>
        <h1 className="text-3xl font-medium mb-2">Sign in</h1>
        <p className="text-sm text-[var(--muted)] mb-6">
          Đăng nhập để điều tra sự cố, quản lý tập luật và phân quyền hệ thống.
        </p>

        {isDevelop && <div className="demo-logins-box mb-6 p-3 bg-[var(--canvas)] border border-[var(--line)] rounded">
          <span className="block text-xs font-mono text-[var(--aqua)] mb-2 uppercase tracking-wider">
            ⚡ 1-Click Demo Logins
          </span>
          <div className="flex flex-col gap-2">
            <button
              type="button"
              onClick={() => handleDemoLogin("admin@example.com", "admin")}
              className="btn-preset text-left flex justify-between items-center hover:border-[var(--coral)]"
            >
              <span>👑 Admin (Quản trị toàn quyền)</span>
              <code className="text-xs text-[var(--coral)]">admin</code>
            </button>
            <button
              type="button"
              onClick={() => handleDemoLogin("analyst@example.com", "analyst")}
              className="btn-preset text-left flex justify-between items-center hover:border-[var(--amber)]"
            >
              <span>🔍 Analyst (Điều tra Alert)</span>
              <code className="text-xs text-[var(--amber)]">analyst</code>
            </button>
            <button
              type="button"
              onClick={() => handleDemoLogin("viewer@example.com", "viewer")}
              className="btn-preset text-left flex justify-between items-center hover:border-[var(--acid)]"
            >
              <span>👁️ Viewer (Chỉ xem báo cáo)</span>
              <code className="text-xs text-[var(--acid)]">viewer</code>
            </button>
          </div>
        </div>}

        <form className="space-y-4" onSubmit={submit}>
          <label>
            Email
            <input
              required
              autoComplete="email"
              type="email"
              value={form.email}
              onChange={(event) => setForm({ ...form, email: event.target.value })}
            />
          </label>

          <label className="password-toggle">
            Mật khẩu
            <input
              required
              autoComplete="current-password"
              type={showPassword ? "text" : "password"}
              value={form.password}
              onChange={(event) => setForm({ ...form, password: event.target.value })}
            />
            <button
              type="button"
              className="toggle-eye"
              onClick={() => setShowPassword(!showPassword)}
              tabIndex={-1}
            >
              {showPassword ? "Ẩn" : "Hiện"}
            </button>
          </label>

          <button disabled={loading} type="submit" className="w-full mt-4">
            {loading ? "Đang đăng nhập..." : "Đăng nhập SOC"}
          </button>

          {message && <small className="notice block mt-4">{message}</small>}
        </form>

        <div className="mt-6 text-center">
          <Link className="text-xs text-[var(--acid)] uppercase font-mono hover:underline" href="/">
            ← Quay lại Dashboard Overview
          </Link>
        </div>
      </section>
    </main>
  );
}
