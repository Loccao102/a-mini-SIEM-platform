"use client";

import { FormEvent, useEffect, useState } from "react";
import Link from "next/link";
import { ApiError, createUser as createUserRequest, disableUser as disableUserRequest, getUsers, login as loginRequest, User } from "@/lib/api";

export default function AccountsPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [message, setMessage] = useState("");
  const [form, setForm] = useState({ email: "", password: "", displayName: "", role: "viewer" });
  const [loginForm, setLoginForm] = useState({ email: "admin@example.com", password: "" });
  const [loggedIn, setLoggedIn] = useState(() => typeof window !== "undefined" && Boolean(localStorage.getItem("siem_token")));

  async function loadUsers() {
    try { setUsers(await getUsers()); }
    catch (error) { setMessage(error instanceof ApiError && error.status === 401 ? "Bạn cần đăng nhập với quyền admin để quản lý tài khoản." : "Không thể tải danh sách tài khoản."); }
  }

  useEffect(() => { if (!localStorage.getItem("siem_token")) return; const timer = window.setTimeout(() => void loadUsers(), 0); return () => window.clearTimeout(timer); }, []);

  async function login(event: FormEvent) {
    event.preventDefault();
    try { const result = await loginRequest(loginForm.email, loginForm.password); localStorage.setItem("siem_token", result.token); setLoggedIn(true); await loadUsers(); }
    catch { setMessage("Email hoặc mật khẩu không đúng hoặc API không khả dụng."); }
  }

  async function createUser(event: FormEvent) {
    event.preventDefault();
    try { await createUserRequest({ email: form.email, password: form.password, display_name: form.displayName, role: form.role as User["role"] }); setMessage("Đã tạo tài khoản."); setForm({ email: "", password: "", displayName: "", role: "viewer" }); await loadUsers(); }
    catch { setMessage("Không thể tạo tài khoản."); }
  }

  async function disableUser(id: number) {
    try { await disableUserRequest(id); await loadUsers(); } catch { setMessage("Không thể vô hiệu hóa tài khoản."); }
  }

  return <main className="account-shell"><header className="account-header"><Link href="/">← Overview</Link><span className="eyebrow">Administration</span><h1>Accounts & access</h1><p>Control who can see, investigate, and change the system.</p></header>{!loggedIn ? <form className="account-form login-form" onSubmit={login}><h2>Sign in</h2><label>Email<input required type="email" value={loginForm.email} onChange={(event) => setLoginForm({ ...loginForm, email: event.target.value })} /></label><label>Password<input required type="password" value={loginForm.password} onChange={(event) => setLoginForm({ ...loginForm, password: event.target.value })} /></label><button type="submit">Sign in</button>{message && <small>{message}</small>}</form> : <section className="account-grid"><form className="account-form" onSubmit={createUser}><h2>Create account</h2><label>Email<input required type="email" value={form.email} onChange={(event) => setForm({ ...form, email: event.target.value })} /></label><label>Display name<input value={form.displayName} onChange={(event) => setForm({ ...form, displayName: event.target.value })} /></label><label>Temporary password<input required minLength={8} type="password" value={form.password} onChange={(event) => setForm({ ...form, password: event.target.value })} /></label><label>Role<select value={form.role} onChange={(event) => setForm({ ...form, role: event.target.value })}><option value="viewer">Viewer · read only</option><option value="analyst">Analyst · investigate</option><option value="admin">Admin · manage access</option></select></label><button type="submit">Create account</button>{message && <small>{message}</small>}</form><section className="account-list"><div className="list-heading"><h2>Team</h2><span>{users.length} accounts</span></div>{users.map((user) => <div className="user-row" key={user.user_id}><div><strong>{user.display_name || user.email}</strong><small>{user.email}</small></div><span className={`role role-${user.role}`}>{user.role}</span><span>{user.enabled ? "Active" : "Disabled"}</span>{user.enabled && <button className="disable" onClick={() => void disableUser(user.user_id)} title="Disable account">Disable</button>}</div>)}</section></section>}</main>;
}
