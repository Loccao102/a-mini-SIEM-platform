"use client";

import { FormEvent, useEffect, useState } from "react";
import Link from "next/link";

type User = { user_id: number; email: string; display_name: string; role: "admin" | "analyst" | "viewer"; enabled: boolean };

const apiURL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export default function AccountsPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [message, setMessage] = useState("");
  const [form, setForm] = useState({ email: "", password: "", displayName: "", role: "viewer" });
  const [loginForm, setLoginForm] = useState({ email: "admin@example.com", password: "" });
  const [loggedIn, setLoggedIn] = useState(() => typeof window !== "undefined" && Boolean(localStorage.getItem("siem_token")));

  async function loadUsers() {
    const token = localStorage.getItem("siem_token");
    const response = await fetch(`${apiURL}/api/v1/users`, { headers: { Authorization: `Bearer ${token}` } });
    if (response.ok) setUsers(await response.json());
    else setMessage("Bạn cần đăng nhập với quyền admin để quản lý tài khoản.");
  }

  useEffect(() => { if (!localStorage.getItem("siem_token")) return; const timer = window.setTimeout(() => void loadUsers(), 0); return () => window.clearTimeout(timer); }, []);

  async function login(event: FormEvent) {
    event.preventDefault();
    const response = await fetch(`${apiURL}/api/v1/auth/login`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(loginForm) });
    if (!response.ok) { setMessage("Email hoặc mật khẩu không đúng."); return; }
    const result = await response.json();
    localStorage.setItem("siem_token", result.token);
    setLoggedIn(true);
    await loadUsers();
  }

  async function createUser(event: FormEvent) {
    event.preventDefault();
    const token = localStorage.getItem("siem_token");
    const response = await fetch(`${apiURL}/api/v1/users`, { method: "POST", headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` }, body: JSON.stringify({ email: form.email, password: form.password, display_name: form.displayName, role: form.role }) });
    setMessage(response.ok ? "Đã tạo tài khoản." : "Không thể tạo tài khoản.");
    if (response.ok) { setForm({ email: "", password: "", displayName: "", role: "viewer" }); await loadUsers(); }
  }

  async function disableUser(id: number) {
    const token = localStorage.getItem("siem_token");
    await fetch(`${apiURL}/api/v1/users/${id}`, { method: "DELETE", headers: { Authorization: `Bearer ${token}` } });
    await loadUsers();
  }

  return <main className="account-shell"><header className="account-header"><Link href="/">← Overview</Link><span className="eyebrow">Administration</span><h1>Accounts & access</h1><p>Control who can see, investigate, and change the system.</p></header>{!loggedIn ? <form className="account-form login-form" onSubmit={login}><h2>Sign in</h2><label>Email<input required type="email" value={loginForm.email} onChange={(event) => setLoginForm({ ...loginForm, email: event.target.value })} /></label><label>Password<input required type="password" value={loginForm.password} onChange={(event) => setLoginForm({ ...loginForm, password: event.target.value })} /></label><button type="submit">Sign in</button>{message && <small>{message}</small>}</form> : <section className="account-grid"><form className="account-form" onSubmit={createUser}><h2>Create account</h2><label>Email<input required type="email" value={form.email} onChange={(event) => setForm({ ...form, email: event.target.value })} /></label><label>Display name<input value={form.displayName} onChange={(event) => setForm({ ...form, displayName: event.target.value })} /></label><label>Temporary password<input required minLength={8} type="password" value={form.password} onChange={(event) => setForm({ ...form, password: event.target.value })} /></label><label>Role<select value={form.role} onChange={(event) => setForm({ ...form, role: event.target.value })}><option value="viewer">Viewer · read only</option><option value="analyst">Analyst · investigate</option><option value="admin">Admin · manage access</option></select></label><button type="submit">Create account</button>{message && <small>{message}</small>}</form><section className="account-list"><div className="list-heading"><h2>Team</h2><span>{users.length} accounts</span></div>{users.map((user) => <div className="user-row" key={user.user_id}><div><strong>{user.display_name || user.email}</strong><small>{user.email}</small></div><span className={`role role-${user.role}`}>{user.role}</span><span>{user.enabled ? "Active" : "Disabled"}</span>{user.enabled && <button className="disable" onClick={() => void disableUser(user.user_id)} title="Disable account">Disable</button>}</div>)}</section></section>}</main>;
}
