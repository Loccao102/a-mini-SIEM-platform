"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import {
  approveExecution,
  BlockedEntity,
  Execution,
  getBlockedEntities,
  getExecutions,
  getPlaybooks,
  Playbook,
  rejectExecution,
  releaseBlockedEntity,
  ApiError,
} from "@/lib/api";

export default function SoarPage() {
  const [playbooks, setPlaybooks] = useState<Playbook[]>([]);
  const [executions, setExecutions] = useState<Execution[]>([]);
  const [blocked, setBlocked] = useState<BlockedEntity[]>([]);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState<{ text: string; type: "success" | "error" } | null>(null);
  const [actionInProgress, setActionInProgress] = useState<number | null>(null);

  async function loadData() {
    try {
      const [pb, ex, blk] = await Promise.all([
        getPlaybooks().catch(() => []),
        getExecutions(50).catch(() => []),
        getBlockedEntities().catch(() => []),
      ]);
      setPlaybooks(pb);
      setExecutions(ex);
      setBlocked(blk);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setMessage({ text: "Đăng nhập để xem trạng thái SOAR.", type: "error" });
      }
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    loadData();
    const interval = window.setInterval(loadData, 5000);
    return () => window.clearInterval(interval);
  }, []);

  async function handleApprove(id: number) {
    setActionInProgress(id);
    try {
      await approveExecution(id);
      setMessage({ text: `Đã phê duyệt và thực thi hành động ngăn chặn #${id}!`, type: "success" });
      await loadData();
    } catch (err) {
      setMessage({ text: `Lỗi phê duyệt: ${err instanceof Error ? err.message : "Thất bại"}`, type: "error" });
    } finally {
      setActionInProgress(null);
    }
  }

  async function handleReject(id: number) {
    setActionInProgress(id);
    try {
      await rejectExecution(id);
      setMessage({ text: `Đã từ chối thực thi hành động #${id}.`, type: "success" });
      await loadData();
    } catch (err) {
      setMessage({ text: `Lỗi: ${err instanceof Error ? err.message : "Thất bại"}`, type: "error" });
    } finally {
      setActionInProgress(null);
    }
  }

  async function handleRelease(id: number, val: string) {
    setActionInProgress(id);
    try {
      await releaseBlockedEntity(id);
      setMessage({ text: `Đã gỡ bỏ chặn thành công cho ${val}!`, type: "success" });
      await loadData();
    } catch (err) {
      setMessage({ text: `Lỗi gỡ chặn: ${err instanceof Error ? err.message : "Thất bại"}`, type: "error" });
    } finally {
      setActionInProgress(null);
    }
  }

  const pendingExecutions = executions.filter((e) => e.status === "pending_approval");
  const historyExecutions = executions.filter((e) => e.status !== "pending_approval");
  const activeBlocked = blocked.filter((b) => b.status === "active");

  return (
    <main className="account-shell">
      <header className="account-header">
        <Link href="/">← Overview</Link>
        <span className="eyebrow">Closed-Loop Incident Containment</span>
        <div className="flex justify-between items-baseline">
          <h1>SOAR Automated Response</h1>
          <span className="live-readout text-xs text-gray-400">
            <span className="pulse-dot inline-block w-2 h-2 rounded-full bg-green-400 mr-1.5" /> Auto-sync 5s
          </span>
        </div>
        <p>
          Tự động cô lập máy chủ tấn công, tài khoản bị xâm nhập với cổng phê duyệt Human-in-the-loop và tự động hoàn tác TTL.
        </p>
      </header>

      {message && (
        <div
          className={`p-3 rounded mb-6 text-sm flex justify-between items-center ${
            message.type === "success"
              ? "bg-emerald-950/60 border border-emerald-500/40 text-emerald-300"
              : "bg-red-950/60 border border-red-500/40 text-red-300"
          }`}
        >
          <span>{message.text}</span>
          <button onClick={() => setMessage(null)} className="text-xs opacity-70 hover:opacity-100">✕</button>
        </div>
      )}

      {/* SECTION 1: PENDING APPROVAL GATE */}
      <section className="mb-8">
        <div className="flex items-center justify-between mb-3 border-b border-(--line) pb-2">
          <div className="flex items-center gap-2">
            <h2 className="text-base font-semibold text-amber-400 uppercase tracking-wider">
              ⏳ Cổng Phê Duyệt Ngăn Chặn (Human-in-the-Loop)
            </h2>
            <span className="bg-amber-500/20 text-amber-300 border border-amber-500/40 text-xs px-2 py-0.5 rounded-full font-mono">
              {pendingExecutions.length} Chờ duyệt
            </span>
          </div>
        </div>

        {pendingExecutions.length === 0 ? (
          <div className="p-6 border border-dashed border-(--line) rounded text-center text-sm text-gray-400">
            ✓ Không có hành động nào đang chờ phê duyệt. Hệ thống phòng thủ đang ở trạng thái an toàn.
          </div>
        ) : (
          <div className="grid gap-4">
            {pendingExecutions.map((exec) => (
              <div
                key={exec.execution_id}
                className="p-4 rounded border border-amber-500/40 bg-amber-950/10 flex flex-col md:flex-row md:items-center justify-between gap-4"
              >
                <div>
                  <div className="flex items-center gap-2 mb-1">
                    <span className="text-xs font-mono bg-amber-500/20 text-amber-300 px-2 py-0.5 rounded">
                      #{exec.execution_id}
                    </span>
                    <strong className="text-sm font-semibold">{exec.action_type.toUpperCase()}</strong>
                    <span className="text-xs text-gray-400">từ Alert #{exec.alert_id ?? "N/A"}</span>
                  </div>
                  <div className="text-xs text-gray-300">
                    Đối tượng mục tiêu: <code className="text-amber-300 font-bold bg-black/40 px-1.5 py-0.5 rounded">{exec.target}</code>
                  </div>
                  <div className="text-xs text-gray-400 mt-1">
                    Yêu cầu lúc: {new Date(exec.created_at).toLocaleTimeString()} · Tự động giải tỏa: 1 giờ
                  </div>
                </div>

                <div className="flex gap-2 shrink-0">
                  <button
                    type="button"
                    disabled={actionInProgress === exec.execution_id}
                    onClick={() => handleApprove(exec.execution_id)}
                    className="px-4 py-2 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white text-xs font-semibold rounded shadow transition-colors"
                  >
                    ✓ Phê duyệt Chặn
                  </button>
                  <button
                    type="button"
                    disabled={actionInProgress === exec.execution_id}
                    onClick={() => handleReject(exec.execution_id)}
                    className="px-4 py-2 bg-zinc-800 hover:bg-zinc-700 disabled:opacity-50 text-red-400 text-xs font-semibold rounded border border-red-500/30 transition-colors"
                  >
                    ✗ Bỏ qua / Từ chối
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>

      {/* SECTION 2: ACTIVE BLOCKED ENTITIES */}
      <section className="mb-8">
        <div className="flex items-center justify-between mb-3 border-b border-(--line) pb-2">
          <div className="flex items-center gap-2">
            <h2 className="text-base font-semibold text-rose-400 uppercase tracking-wider">
              🛡️ Danh Sách Đang Bị Cô Lập (Active Blocked Entities)
            </h2>
            <span className="bg-rose-500/20 text-rose-300 border border-rose-500/40 text-xs px-2 py-0.5 rounded-full font-mono">
              {activeBlocked.length} Đang chặn
            </span>
          </div>
        </div>

        {activeBlocked.length === 0 ? (
          <div className="p-4 border border-(--line) rounded text-center text-xs text-gray-400">
            Hiện chưa có IP hay tài khoản nào bị cô lập.
          </div>
        ) : (
          <div className="overflow-x-auto border border-(--line) rounded">
            <table className="w-full text-left text-xs">
              <thead className="bg-zinc-900/80 border-b border-(--line) text-gray-400">
                <tr>
                  <th className="p-3">Loại</th>
                  <th className="p-3">Giá trị Thực thể</th>
                  <th className="p-3">Lý do Ngăn chặn</th>
                  <th className="p-3">Thời gian chặn</th>
                  <th className="p-3">Hết hạn (TTL)</th>
                  <th className="p-3 text-right">Thao tác</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-(--line)">
                {activeBlocked.map((item) => (
                  <tr key={item.blocked_id} className="hover:bg-zinc-800/30">
                    <td className="p-3">
                      <span className="px-2 py-0.5 text-[11px] font-mono rounded bg-rose-500/20 text-rose-300 border border-rose-500/40">
                        {item.entity_type.toUpperCase()}
                      </span>
                    </td>
                    <td className="p-3 font-mono font-bold text-amber-300">{item.entity_value}</td>
                    <td className="p-3 text-gray-300">{item.reason}</td>
                    <td className="p-3 text-gray-400">{new Date(item.blocked_at).toLocaleTimeString()}</td>
                    <td className="p-3 text-emerald-400">
                      {item.expires_at ? new Date(item.expires_at).toLocaleTimeString() : "Vĩnh viễn"}
                    </td>
                    <td className="p-3 text-right">
                      <button
                        type="button"
                        disabled={actionInProgress === item.blocked_id}
                        onClick={() => handleRelease(item.blocked_id, item.entity_value)}
                        className="px-2.5 py-1 bg-zinc-800 hover:bg-zinc-700 text-xs text-gray-300 hover:text-white rounded border border-(--line)"
                      >
                        Gỡ chặn ngay
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {/* SECTION 3: PLAYBOOK REGISTRY & RECENT EXECUTIONS */}
      <section className="grid gap-6 md:grid-cols-2">
        <div>
          <h2 className="text-sm font-semibold text-gray-300 uppercase tracking-wider mb-3 border-b border-(--line) pb-2">
            📖 Kịch Bản Phản Ứng Đã Cấu Hình ({playbooks.length})
          </h2>
          <div className="grid gap-3">
            {playbooks.map((p) => (
              <div key={p.playbook_id} className="p-3 rounded border border-(--line) bg-zinc-900/40 text-xs">
                <div className="flex justify-between items-baseline mb-1">
                  <strong className="text-gray-200 text-sm">{p.name}</strong>
                  <span className="text-[10px] px-1.5 py-0.5 rounded bg-blue-500/20 text-blue-300 border border-blue-500/30">
                    {p.action_type}
                  </span>
                </div>
                <p className="text-gray-400 mb-2">{p.description}</p>
                <div className="flex gap-3 text-[11px] text-gray-400">
                  <span>Loại kích hoạt: <code>{p.trigger_type}</code></span>
                  <span>Cần phê duyệt: {p.requires_approval ? "Có (Gate)" : "Tự động"}</span>
                </div>
              </div>
            ))}
          </div>
        </div>

        <div>
          <h2 className="text-sm font-semibold text-gray-300 uppercase tracking-wider mb-3 border-b border-(--line) pb-2">
            📜 Nhật Ký Thực Thi Gần Đây ({historyExecutions.length})
          </h2>
          {historyExecutions.length === 0 ? (
            <p className="text-xs text-gray-500 italic">Chưa có lịch sử thực thi.</p>
          ) : (
            <div className="grid gap-2 max-h-96 overflow-y-auto">
              {historyExecutions.slice(0, 10).map((ex) => (
                <div key={ex.execution_id} className="p-2.5 rounded border border-(--line) bg-zinc-900/20 text-xs flex justify-between items-center">
                  <div>
                    <span className="font-mono text-gray-400 mr-2">#{ex.execution_id}</span>
                    <strong className="text-gray-300 mr-2">{ex.action_type}</strong>
                    <code className="text-amber-300 mr-2">{ex.target}</code>
                  </div>
                  <span
                    className={`text-[10px] font-semibold px-2 py-0.5 rounded uppercase ${
                      ex.status === "executed" || ex.status === "approved"
                        ? "bg-emerald-500/20 text-emerald-300"
                        : ex.status === "rejected"
                        ? "bg-zinc-800 text-gray-400"
                        : ex.status === "rolled_back"
                        ? "bg-blue-500/20 text-blue-300"
                        : "bg-red-500/20 text-red-300"
                    }`}
                  >
                    {ex.status}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      </section>
    </main>
  );
}
