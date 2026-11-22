"use client";

import { FormEvent, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { login } from "@/lib/api";
import { EyeIcon, EyeOffIcon, ShieldIcon } from "@/components/ui/icons";
import { Button, Notice } from "@/components/ui/primitives";

const DEMO_ACCOUNTS = [
  {
    email: "admin@example.com",
    pass: "admin",
    role: "ADMIN",
    desc: "Toàn quyền quản trị hệ thống",
    tone: "border-red-500/30 text-red-300 hover:border-red-400/60",
  },
  {
    email: "analyst@example.com",
    pass: "analyst",
    role: "ANALYST",
    desc: "Điều tra cảnh báo & quản lý Case",
    tone: "border-amber-500/30 text-amber-300 hover:border-amber-400/60",
  },
  {
    email: "viewer@example.com",
    pass: "viewer",
    role: "VIEWER",
    desc: "Chỉ xem bảng điều khiển & báo cáo",
    tone: "border-cyan-500/30 text-cyan-300 hover:border-cyan-400/60",
  },
] as const;

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
    <main className="flex min-h-screen w-full items-center justify-center bg-canvas px-4 py-10">
      <div className="w-full max-w-md space-y-6">
        <div className="flex flex-col items-center text-center">
          <div className="flex h-11 w-11 items-center justify-center rounded-lg border border-accent/40 bg-accent/15 text-accent">
            <ShieldIcon size={22} />
          </div>
          <h1 className="mt-4 text-xl font-semibold tracking-tight text-ink">Sentinel SIEM</h1>
          <p className="mt-1 text-xs text-muted">Đăng nhập cổng điều hành an ninh SOC</p>
        </div>

        <section className="rounded-xl border border-border bg-surface p-6 shadow-2xl">
          {isDevelop && (
            <div className="mb-5 rounded-lg border border-border bg-canvas/80 p-3.5">
              <p className="mb-2.5 font-mono text-[11px] font-medium uppercase tracking-wider text-accent">
                Đăng nhập nhanh (Môi trường Demo)
              </p>
              <div className="space-y-2">
                {DEMO_ACCOUNTS.map((acc) => (
                  <button
                    key={acc.email}
                    type="button"
                    disabled={loading}
                    onClick={() => handleDemoLogin(acc.email, acc.pass)}
                    className={`flex w-full cursor-pointer items-center justify-between rounded-md border bg-surface px-3 py-2 text-left text-xs transition-colors disabled:opacity-50 ${acc.tone}`}
                  >
                    <span className="text-ink-secondary">{acc.desc}</span>
                    <span className="font-mono text-[10px] font-semibold">{acc.role}</span>
                  </button>
                ))}
              </div>
            </div>
          )}

          <form className="space-y-4" onSubmit={submit}>
            <div className="space-y-1.5">
              <label htmlFor="login-email" className="block text-xs font-medium text-ink-secondary">
                Email
              </label>
              <input
                id="login-email"
                className="soc-input"
                required
                autoComplete="email"
                type="email"
                value={form.email}
                onChange={(event) => setForm({ ...form, email: event.target.value })}
              />
            </div>

            <div className="space-y-1.5">
              <label htmlFor="login-password" className="block text-xs font-medium text-ink-secondary">
                Mật khẩu
              </label>
              <div className="relative">
                <input
                  id="login-password"
                  className="soc-input pr-10"
                  required
                  autoComplete="current-password"
                  type={showPassword ? "text" : "password"}
                  value={form.password}
                  onChange={(event) => setForm({ ...form, password: event.target.value })}
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  aria-label={showPassword ? "Ẩn mật khẩu" : "Hiện mật khẩu"}
                  tabIndex={-1}
                  className="absolute top-1/2 right-2.5 -translate-y-1/2 cursor-pointer rounded p-1 text-faint hover:text-ink"
                >
                  {showPassword ? <EyeOffIcon size={15} /> : <EyeIcon size={15} />}
                </button>
              </div>
            </div>

            <Button disabled={loading} type="submit" variant="primary" className="mt-2 w-full py-2.5">
              {loading ? "Đang xác thực..." : "Đăng nhập SOC"}
            </Button>

            {message && <Notice tone="error">{message}</Notice>}
          </form>
        </section>

        <div className="text-center">
          <Link className="font-mono text-xs text-muted transition-colors hover:text-accent" href="/">
            ← Quay lại Overview
          </Link>
        </div>
      </div>
    </main>
  );
}
