"use client";

import { FormEvent, useEffect, useState } from "react";
import {
  ApiError,
  createUser as createUserRequest,
  disableUser as disableUserRequest,
  getUsers,
  login as loginRequest,
  User,
} from "@/lib/api";
import { LockIcon, UsersIcon } from "@/components/ui/icons";
import { Button, EmptyState, Notice, PageHeader, PageShell, Panel, StatusPill } from "@/components/ui/primitives";

export default function AccountsPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [message, setMessage] = useState<{ text: string; tone: "error" | "success" } | null>(null);
  const [form, setForm] = useState({ email: "", password: "", displayName: "", role: "viewer" });
  const [loginForm, setLoginForm] = useState({ email: "admin@example.com", password: "admin" });
  const [loggedIn, setLoggedIn] = useState(
    () => typeof window !== "undefined" && Boolean(localStorage.getItem("siem_token"))
  );

  async function loadUsers() {
    try {
      setUsers(await getUsers());
      setMessage(null);
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        setMessage({ tone: "error", text: "Bạn cần đăng nhập quyền Admin để xem danh sách tài khoản." });
      } else if (error instanceof ApiError && error.status === 403) {
        setMessage({ tone: "error", text: "Chỉ tài khoản Admin mới có quyền quản lý người dùng." });
      } else {
        setMessage({ tone: "error", text: "Không thể tải danh sách tài khoản." });
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
      setMessage({ tone: "error", text: "Email hoặc mật khẩu không đúng hoặc API chưa sẵn sàng." });
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
      setMessage({ tone: "success", text: "Đã tạo tài khoản SOC mới thành công." });
      setForm({ email: "", password: "", displayName: "", role: "viewer" });
      await loadUsers();
    } catch (err: unknown) {
      const errMsg = err instanceof ApiError ? err.message : "Không thể tạo tài khoản.";
      setMessage({ tone: "error", text: `Tạo tài khoản thất bại: ${errMsg}` });
    }
  }

  async function disableUser(id: number) {
    if (!window.confirm("Vô hiệu hóa tài khoản này?")) return;
    try {
      await disableUserRequest(id);
      await loadUsers();
    } catch {
      setMessage({ tone: "error", text: "Không thể vô hiệu hóa tài khoản. Yêu cầu quyền Admin." });
    }
  }

  return (
    <PageShell>
      <PageHeader
        title="SOC Accounts & RBAC"
        description="Quản lý thành viên đội ngũ SOC, phân quyền vai trò Admin / Analyst / Viewer và kiểm soát truy cập."
        badge={loggedIn ? <StatusPill tone="neutral">{users.length} tài khoản</StatusPill> : undefined}
      />

      {message && (
        <Notice tone={message.tone} onDismiss={() => setMessage(null)}>
          {message.text}
        </Notice>
      )}

      {!loggedIn ? (
        <div className="max-w-md">
          <Panel title="Đăng nhập quyền quản trị">
            <form className="space-y-4" onSubmit={login}>
              <div>
                <label className="mb-1 block text-xs font-medium text-ink-secondary">Email</label>
                <input
                  className="soc-input"
                  required
                  type="email"
                  value={loginForm.email}
                  onChange={(event) => setLoginForm({ ...loginForm, email: event.target.value })}
                />
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium text-ink-secondary">Mật khẩu</label>
                <input
                  className="soc-input"
                  required
                  type="password"
                  value={loginForm.password}
                  onChange={(event) => setLoginForm({ ...loginForm, password: event.target.value })}
                />
              </div>
              <Button type="submit" variant="primary" className="w-full">
                Đăng nhập
              </Button>
            </form>
          </Panel>
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-12">
          {/* Left: Create User Form */}
          <div className="lg:col-span-4">
            <Panel title="Tạo tài khoản mới">
              <form className="space-y-3.5" onSubmit={createUser}>
                <div>
                  <label className="mb-1 block text-xs font-medium text-ink-secondary">Email</label>
                  <input
                    className="soc-input"
                    required
                    type="email"
                    placeholder="user@example.com"
                    value={form.email}
                    onChange={(event) => setForm({ ...form, email: event.target.value })}
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-medium text-ink-secondary">Tên hiển thị</label>
                  <input
                    className="soc-input"
                    placeholder="Nguyễn Văn A"
                    value={form.displayName}
                    onChange={(event) => setForm({ ...form, displayName: event.target.value })}
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-medium text-ink-secondary">Mật khẩu tạm thời</label>
                  <input
                    className="soc-input"
                    required
                    minLength={4}
                    type="password"
                    value={form.password}
                    onChange={(event) => setForm({ ...form, password: event.target.value })}
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-medium text-ink-secondary">Vai trò RBAC</label>
                  <select
                    className="soc-select"
                    value={form.role}
                    onChange={(event) => setForm({ ...form, role: event.target.value })}
                  >
                    <option value="viewer">Viewer · Chỉ xem log & báo cáo</option>
                    <option value="analyst">Analyst · Điều tra & xử lý Alert</option>
                    <option value="admin">Admin · Toàn quyền quản trị</option>
                  </select>
                </div>
                <Button type="submit" variant="primary" className="w-full">
                  Thêm người dùng
                </Button>
              </form>
            </Panel>
          </div>

          {/* Right: User Table */}
          <div className="lg:col-span-8">
            <Panel title={`Đội ngũ SOC (${users.length})`} noPadding>
              {users.length === 0 ? (
                <EmptyState
                  title="Chưa có dữ liệu tài khoản"
                  description="Tài khoản hiện tại có thể chưa có quyền Admin hoặc danh sách đang trống."
                  icon={<UsersIcon size={18} />}
                />
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full text-left text-xs">
                    <thead>
                      <tr className="border-b border-border bg-surface-raised/60 font-mono text-[11px] uppercase tracking-wider text-muted">
                        <th className="px-5 py-3">Thành viên</th>
                        <th className="px-4 py-3">Vai trò</th>
                        <th className="px-4 py-3">Trạng thái</th>
                        <th className="px-5 py-3 text-right">Thao tác</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-border">
                      {users.map((user) => {
                        const roleTone =
                          user.role === "admin"
                            ? "danger"
                            : user.role === "analyst"
                            ? "warning"
                            : "accent";
                        return (
                          <tr key={user.user_id} className="transition-colors hover:bg-surface-hover/60">
                            <td className="px-5 py-3.5">
                              <p className="font-medium text-ink">{user.display_name || user.email}</p>
                              <p className="font-mono text-[11px] text-faint">{user.email}</p>
                            </td>
                            <td className="px-4 py-3.5 whitespace-nowrap">
                              <StatusPill tone={roleTone}>{user.role.toUpperCase()}</StatusPill>
                            </td>
                            <td className="px-4 py-3.5 whitespace-nowrap">
                              <StatusPill tone={user.enabled ? "success" : "neutral"}>
                                {user.enabled ? "Hoạt động" : "Đã khóa"}
                              </StatusPill>
                            </td>
                            <td className="px-5 py-3.5 text-right whitespace-nowrap">
                              {user.enabled && (
                                <Button
                                  variant="danger"
                                  size="sm"
                                  onClick={() => void disableUser(user.user_id)}
                                  title="Vô hiệu hóa tài khoản"
                                >
                                  <LockIcon size={12} /> Khóa
                                </Button>
                              )}
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              )}
            </Panel>
          </div>
        </div>
      )}
    </PageShell>
  );
}
