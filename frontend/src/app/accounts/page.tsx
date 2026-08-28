"use client";

import { FormEvent, useEffect, useState } from "react";
import Link from "next/link";
import {
  ApiError,
  createUser as createUserRequest,
  disableUser as disableUserRequest,
  getUsers,
  login as loginRequest,
  User,
} from "@/lib/api";

export default function AccountsPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [message, setMessage] = useState("");
  const [form, setForm] = useState({ email: "", password: "", displayName: "", role: "viewer" });
  const [loginForm, setLoginForm] = useState({ email: "admin@example.com", password: "admin" });
  const [loggedIn, setLoggedIn] = useState(
    () => typeof window !== "undefined" && Boolean(localStorage.getItem("siem_token"))
  );

  async function loadUsers() {
    try {
      setUsers(await getUsers());
      setMessage("");
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        setMessage("Bạn cần đăng nhập với quyền Admin để truy cập danh sách tài khoản.");
      } else if (error instanceof ApiError && error.status === 403) {
        setMessage("Chỉ tài khoản Admin mới có quyền xem và quản lý danh sách người dùng.");
      } else {
        setMessage("Không thể tải danh sách tài khoản.");
      }
    }
  }

  useEffect(() => {
    if (!localStorage.getItem("siem_token")) return;
    const timer = window.setTimeout(() => void loadUsers(), 0);
    return () => window.clearTimeout(timer);
  }, []);

  async function login(event: FormEvent) {
    event.preventDefault();
    try {
      const result = await loginRequest(loginForm.email, loginForm.password);
      localStorage.setItem("siem_token", result.token);
      setLoggedIn(true);
      await loadUsers();
    } catch {
      setMessage("Email hoặc mật khẩu không đúng hoặc API không khả dụng.");
    }
  }

  async function createUser(event: FormEvent) {
    event.preventDefault();
    try {
      await createUserRequest({
        email: form.email,
        password: form.password,
        display_name: form.displayName,
        role: form.role as User["role"],
      });
      setMessage("Đã tạo tài khoản thành công.");
      setForm({ email: "", password: "", displayName: "", role: "viewer" });
      await loadUsers();
    } catch (err: unknown) {
      const errMsg = err instanceof ApiError ? err.message : "Không thể tạo tài khoản.";
      setMessage(`Tạo tài khoản thất bại: ${errMsg}`);
    }
  }

  async function disableUser(id: number) {
    if (!window.confirm("Vô hiệu hóa tài khoản này?")) return;
    try {
      await disableUserRequest(id);
      await loadUsers();
    } catch {
      setMessage("Không thể vô hiệu hóa tài khoản. Yêu cầu quyền Admin.");
    }
  }

  return (
    <main className="account-shell">
      <header className="account-header">
        <Link href="/">← Overview</Link>
        <span className="eyebrow">Administration & RBAC</span>
        <h1>SOC Accounts & Access</h1>
        <p>Quản lý danh sách đội ngũ SOC, phân quyền vai trò (Admin, Analyst, Viewer) và kiểm soát truy cập hệ thống.</p>
      </header>

      {!loggedIn ? (
        <form className="account-form login-form max-w-md" onSubmit={login}>
          <h2>Sign in for Management</h2>
          <label>
            Email
            <input
              required
              type="email"
              value={loginForm.email}
              onChange={(event) => setLoginForm({ ...loginForm, email: event.target.value })}
            />
          </label>
          <label>
            Password
            <input
              required
              type="password"
              value={loginForm.password}
              onChange={(event) => setLoginForm({ ...loginForm, password: event.target.value })}
            />
          </label>
          <button type="submit">Sign in</button>
          {message && <small className="notice block">{message}</small>}
        </form>
      ) : (
        <section className="account-grid">
          <form className="account-form" onSubmit={createUser}>
            <h2>Tạo tài khoản mới</h2>
            <label>
              Email
              <input
                required
                type="email"
                placeholder="user@example.com"
                value={form.email}
                onChange={(event) => setForm({ ...form, email: event.target.value })}
              />
            </label>
            <label>
              Tên hiển thị (Display Name)
              <input
                placeholder="Nguyễn Văn A"
                value={form.displayName}
                onChange={(event) => setForm({ ...form, displayName: event.target.value })}
              />
            </label>
            <label>
              Mật khẩu tạm thời
              <input
                required
                minLength={4}
                type="password"
                value={form.password}
                onChange={(event) => setForm({ ...form, password: event.target.value })}
              />
            </label>
            <label>
              Vai trò SOC (RBAC Role)
              <select value={form.role} onChange={(event) => setForm({ ...form, role: event.target.value })}>
                <option value="viewer">👁️ Viewer · Chỉ xem log & báo cáo</option>
                <option value="analyst">🔍 Analyst · Điều tra & Cập nhật Alert</option>
                <option value="admin">👑 Admin · Toàn quyền quản trị</option>
              </select>
            </label>
            <button type="submit">Thêm người dùng</button>
            {message && <small className="notice block">{message}</small>}
          </form>

          <section className="account-list">
            <div className="list-heading">
              <h2>Đội ngũ SOC ({users.length})</h2>
            </div>
            {!users.length && <p className="empty-state">Chưa có thông tin hoặc không có quyền xem.</p>}
            {users.map((user) => (
              <div className="user-row" key={user.user_id}>
                <div>
                  <strong>{user.display_name || user.email}</strong>
                  <small>{user.email}</small>
                </div>
                <span
                  className={`role-badge ${
                    user.role === "admin"
                      ? "badge-admin"
                      : user.role === "analyst"
                      ? "badge-analyst"
                      : "badge-viewer"
                  }`}
                >
                  {user.role === "admin" ? "👑 ADMIN" : user.role === "analyst" ? "🔍 ANALYST" : "👁️ VIEWER"}
                </span>
                <span>{user.enabled ? "Hoạt động" : "Đã khóa"}</span>
                {user.enabled && (
                  <button className="disable" onClick={() => void disableUser(user.user_id)} title="Disable account">
                    Khóa
                  </button>
                )}
              </div>
            ))}
          </section>
        </section>
      )}
    </main>
  );
}
