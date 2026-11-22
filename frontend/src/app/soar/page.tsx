"use client";

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
import { CheckCircleIcon, CheckIcon, ShieldAlertIcon, UnlockIcon, XIcon } from "@/components/ui/icons";
import { Button, EmptyState, Notice, PageHeader, PageShell, Panel, StatusPill } from "@/components/ui/primitives";

export default function SoarPage() {
  const [playbooks, setPlaybooks] = useState<Playbook[]>([]);
  const [executions, setExecutions] = useState<Execution[]>([]);
  const [blocked, setBlocked] = useState<BlockedEntity[]>([]);
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
    }
  }

  useEffect(() => {
    const timer = window.setTimeout(() => void loadData(), 0);
    const interval = window.setInterval(() => void loadData(), 5000);
    return () => {
      window.clearTimeout(timer);
      window.clearInterval(interval);
    };
  }, []);

  async function handleApprove(id: number) {
    setActionInProgress(id);
    try {
      await approveExecution(id);
      setMessage({ text: `Đã phê duyệt và thực thi hành động ngăn chặn #${id}.`, type: "success" });
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
      setMessage({ text: `Đã gỡ bỏ chặn cho ${val}.`, type: "success" });
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
    <PageShell>
      <PageHeader
        title="SOAR Automated Response"
        description="Cô lập IP hoặc tài khoản nghi vấn với cổng phê duyệt Human-in-the-loop và tự động hoàn tác theo TTL."
        actions={
          <StatusPill tone="success">
            <span className="soc-pulse h-1.5 w-1.5 rounded-full bg-emerald-400" />
            Auto-sync 5s
          </StatusPill>
        }
      />

      {message && (
        <Notice tone={message.type} onDismiss={() => setMessage(null)}>
          {message.text}
        </Notice>
      )}

      {/* Pending Approval Gate */}
      <Panel
        title="Cổng phê duyệt ngăn chặn (Human-in-the-Loop)"
        subtitle="Các hành động cô lập cần chuyên viên SOC xác nhận trước khi thực thi"
        actions={
          <StatusPill tone={pendingExecutions.length > 0 ? "warning" : "success"}>
            {pendingExecutions.length} chờ duyệt
          </StatusPill>
        }
      >
        {pendingExecutions.length === 0 ? (
          <EmptyState
            title="Không có hành động nào đang chờ phê duyệt"
            description="Hệ thống phòng thủ đang hoạt động bình thường."
            icon={<CheckCircleIcon size={18} />}
          />
        ) : (
          <div className="space-y-3">
            {pendingExecutions.map((exec) => (
              <div
                key={exec.execution_id}
                className="flex flex-col justify-between gap-4 rounded-lg border border-amber-500/35 bg-amber-950/15 p-4 md:flex-row md:items-center"
              >
                <div className="space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <StatusPill tone="warning">#{exec.execution_id}</StatusPill>
                    <strong className="text-sm font-semibold text-ink">
                      {exec.action_type?.toUpperCase() ?? "ACTION"}
                    </strong>
                    <span className="font-mono text-xs text-muted">Alert #{exec.alert_id ?? "N/A"}</span>
                  </div>
                  <p className="text-xs text-ink-secondary">
                    Mục tiêu:{" "}
                    <code className="rounded bg-canvas px-1.5 py-0.5 font-mono text-amber-300">
                      {exec.target}
                    </code>
                  </p>
                  <p className="font-mono text-[11px] text-faint">
                    Yêu cầu lúc {new Date(exec.created_at).toLocaleTimeString("vi-VN")} · Tự động giải tỏa sau 1 giờ
                  </p>
                </div>

                <div className="flex shrink-0 items-center gap-2">
                  <Button
                    variant="success"
                    disabled={actionInProgress === exec.execution_id}
                    onClick={() => handleApprove(exec.execution_id)}
                  >
                    <CheckIcon size={14} /> Phê duyệt chặn
                  </Button>
                  <Button
                    variant="danger"
                    disabled={actionInProgress === exec.execution_id}
                    onClick={() => handleReject(exec.execution_id)}
                  >
                    <XIcon size={14} /> Từ chối
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </Panel>

      {/* Active Blocked Entities */}
      <Panel
        title="Danh sách thực thể đang bị cô lập"
        actions={<StatusPill tone="danger">{activeBlocked.length} đang chặn</StatusPill>}
        noPadding
      >
        {activeBlocked.length === 0 ? (
          <EmptyState
            title="Chưa có IP hay tài khoản nào bị cô lập"
            icon={<ShieldAlertIcon size={18} />}
          />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead>
                <tr className="border-b border-border bg-surface-raised/60 font-mono text-[11px] uppercase tracking-wider text-muted">
                  <th className="px-5 py-3">Loại</th>
                  <th className="px-4 py-3">Thực thể</th>
                  <th className="px-4 py-3">Lý do ngăn chặn</th>
                  <th className="px-4 py-3">Bắt đầu</th>
                  <th className="px-4 py-3">Hết hạn (TTL)</th>
                  <th className="px-5 py-3 text-right">Thao tác</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {activeBlocked.map((item) => (
                  <tr key={item.blocked_id} className="transition-colors hover:bg-surface-hover/60">
                    <td className="px-5 py-3 whitespace-nowrap">
                      <StatusPill tone="danger">{item.entity_type?.toUpperCase() ?? "ENTITY"}</StatusPill>
                    </td>
                    <td className="px-4 py-3 font-mono font-semibold text-amber-300">{item.entity_value}</td>
                    <td className="px-4 py-3 text-ink-secondary">{item.reason}</td>
                    <td className="px-4 py-3 font-mono text-muted whitespace-nowrap">
                      {new Date(item.blocked_at).toLocaleTimeString("vi-VN")}
                    </td>
                    <td className="px-4 py-3 font-mono text-emerald-400 whitespace-nowrap">
                      {item.expires_at ? new Date(item.expires_at).toLocaleTimeString("vi-VN") : "Vĩnh viễn"}
                    </td>
                    <td className="px-5 py-3 text-right whitespace-nowrap">
                      <Button
                        size="sm"
                        disabled={actionInProgress === item.blocked_id}
                        onClick={() => handleRelease(item.blocked_id, item.entity_value)}
                      >
                        <UnlockIcon size={13} /> Gỡ chặn
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Panel>

      {/* Playbooks & Recent Executions */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <Panel title={`Kịch bản phản ứng (${playbooks.length})`}>
          {playbooks.length === 0 ? (
            <p className="text-xs text-faint">Chưa có playbook nào được cấu hình.</p>
          ) : (
            <div className="space-y-3">
              {playbooks.map((p) => (
                <div key={p.playbook_id} className="rounded-lg border border-border bg-canvas/70 p-3.5 text-xs">
                  <div className="flex items-center justify-between gap-2">
                    <strong className="text-sm font-medium text-ink">{p.name}</strong>
                    <StatusPill tone="info">{p.action_type}</StatusPill>
                  </div>
                  <p className="mt-1.5 text-muted">{p.description}</p>
                  <div className="mt-2.5 flex flex-wrap gap-3 font-mono text-[11px] text-faint">
                    <span>Trigger: {p.trigger_type}</span>
                    <span>Cổng duyệt: {p.requires_approval ? "Thủ công (Gate)" : "Tự động"}</span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </Panel>

        <Panel title={`Nhật ký thực thi gần đây (${historyExecutions.length})`} noPadding>
          {historyExecutions.length === 0 ? (
            <div className="p-6 text-xs text-faint">Chưa có lịch sử thực thi.</div>
          ) : (
            <div className="max-h-96 divide-y divide-border overflow-y-auto">
              {historyExecutions.slice(0, 15).map((ex) => {
                const tone =
                  ex.status === "executed" || ex.status === "approved"
                    ? "success"
                    : ex.status === "rolled_back"
                    ? "info"
                    : ex.status === "rejected"
                    ? "neutral"
                    : "danger";
                return (
                  <div key={ex.execution_id} className="flex items-center justify-between gap-3 px-5 py-3 text-xs">
                    <div className="min-w-0">
                      <span className="mr-2 font-mono text-faint">#{ex.execution_id}</span>
                      <strong className="mr-2 text-ink-secondary">{ex.action_type}</strong>
                      <code className="font-mono text-amber-300">{ex.target}</code>
                    </div>
                    <StatusPill tone={tone}>{ex.status}</StatusPill>
                  </div>
                );
              })}
            </div>
          )}
        </Panel>
      </div>
    </PageShell>
  );
}
