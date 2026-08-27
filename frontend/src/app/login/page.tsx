"use client";

import { FormEvent, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { login } from "@/lib/api";

export default function LoginPage() {
  const router = useRouter();
  const [form, setForm] = useState({ email: "admin@example.com", password: "" });
  const [message, setMessage] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    const timer = window.setTimeout(() => { if (localStorage.getItem("siem_token")) router.replace("/"); }, 0);
    return () => window.clearTimeout(timer);
  }, [router]);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setLoading(true);
    setMessage("");
    try {
      const result = await login(form.email, form.password);
      localStorage.setItem("siem_token", result.token);
      router.replace("/");
    } catch {
      setMessage("Email hoặc mật khẩu không đúng, hoặc API chưa sẵn sàng.");
    } finally {
      setLoading(false);
    }
  }

  return <main className="login-shell"><section className="login-panel"><div className="login-brand"><span className="mark">S</span><strong>Sentinel</strong></div><p className="eyebrow">Secure access</p><h1>Welcome back.</h1><p className="login-copy">Sign in to inspect signals, respond to alerts, and manage your detection system.</p><form className="account-form" onSubmit={submit}><label>Email<input required autoComplete="email" type="email" value={form.email} onChange={(event) => setForm({ ...form, email: event.target.value })} /></label><label>Password<input required autoComplete="current-password" type="password" value={form.password} onChange={(event) => setForm({ ...form, password: event.target.value })} /></label><button disabled={loading} type="submit">{loading ? "Signing in..." : "Sign in"}</button>{message && <small className="login-error">{message}</small>}</form><Link className="login-back" href="/">Back to overview</Link></section></main>;
}
