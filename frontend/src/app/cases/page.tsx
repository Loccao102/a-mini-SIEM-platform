"use client";

import { FormEvent, useEffect, useState } from "react";
import {
  addCaseNote,
  ApiError,
  CaseRecord,
  CaseTimelineItem,
  createCase,
  getCaseReportHtml,
  getCaseTimeline,
  getCases,
  updateCase,
} from "@/lib/api";
import { SeverityBadge } from "@/components/severity-badge";
import { BriefcaseIcon, DownloadIcon, PlusIcon } from "@/components/ui/icons";
import { Button, EmptyState, Notice, PageHeader, PageShell, Panel, StatusPill } from "@/components/ui/primitives";

export default function CasesPage() {
  const [cases, setCases] = useState<CaseRecord[]>([]);
  const [selected, setSelected] = useState<CaseRecord | null>(null);
  const [timeline, setTimeline] = useState<CaseTimelineItem[]>([]);
  const [title, setTitle] = useState("");
  const [note, setNote] = useState("");
  const [message, setMessage] = useState<{ text: string; tone: "error" | "success" } | null>(null);
  const [exportingReport, setExportingReport] = useState(false);

  async function loadCases() {
    try {
      const nextCases = await getCases();
      setCases(nextCases);
      if (selected) setSelected(nextCases.find((item) => item.case_id === selected.case_id) ?? null);
    } catch (error) {
      setMessage({
        tone: "error",
        text:
          error instanceof ApiError && error.status === 401
            ? "Vui lòng đăng nhập để xem danh sách hồ sơ sự cố."
            : "Không thể tải danh sách hồ sơ sự cố.",
      });
    }
  }

  async function selectCase(item: CaseRecord) {
    setSelected(item);
    setTimeline(await getCaseTimeline(item.case_id));
  }

  useEffect(() => {
    const timer = window.setTimeout(() => {
      getCases()
        .then(setCases)
        .catch((error: unknown) =>
          setMessage({
            tone: "error",
            text:
              error instanceof ApiError && error.status === 401
                ? "Vui lòng đăng nhập để xem danh sách hồ sơ sự cố."
                : "Không thể tải danh sách hồ sơ sự cố.",
          })
        );
    }, 0);
    return () => window.clearTimeout(timer);
  }, []);

  async function handleCreate(event: FormEvent) {
    event.preventDefault();
    if (!title.trim()) return;
    try {
      const created = await createCase(title.trim());
      setTitle("");
      await loadCases();
      const item = await getCases().then((items) => items.find((entry) => entry.case_id === created.case_id));
      if (item) await selectCase(item);
      setMessage({ tone: "success", text: "Đã tạo hồ sơ sự cố mới." });
    } catch {
      setMessage({ tone: "error", text: "Không thể tạo case. Yêu cầu quyền Analyst hoặc Admin." });
    }
  }

  async function handleStatus(status: CaseRecord["status"]) {
    if (!selected) return;
    await updateCase(selected.case_id, { status });
    await loadCases();
    const refreshed = cases.find((item) => item.case_id === selected.case_id);
    if (refreshed) await selectCase({ ...refreshed, status });
  }

  async function handleNote(event: FormEvent) {
    event.preventDefault();
    if (!selected || !note.trim()) return;
    await addCaseNote(selected.case_id, note.trim());
    setNote("");
    setTimeline(await getCaseTimeline(selected.case_id));
  }

  async function handleExportReport() {
    if (!selected || exportingReport) return;
    setExportingReport(true);
    setMessage(null);
    try {
      const blob = await getCaseReportHtml(selected.case_id);
      const url = URL.createObjectURL(blob);
      try {
        const a = document.createElement("a");
        a.href = url;
        a.download = `sentinel-incident-${selected.case_id}.html`;
        document.body.appendChild(a);
        a.click();
        a.remove();
        setMessage({
          tone: "success",
          text: "Đã tải báo cáo HTML. Mở tệp trong trình duyệt rồi chọn Print / Save as PDF.",
        });
      } finally {
        window.setTimeout(() => URL.revokeObjectURL(url), 10000);
      }
    } catch (error) {
      setMessage({
        tone: "error",
        text:
          error instanceof ApiError && error.status === 401
            ? "Phiên đăng nhập đã hết hạn. Vui lòng đăng nhập lại."
            : "Không thể tải báo cáo sự cố. Kiểm tra quyền tài khoản và kết nối API.",
      });
    } finally {
      setExportingReport(false);
    }
  }

  return (
    <PageShell>
      <PageHeader
        title="Incident Cases"
        description="Gom cảnh báo thành hồ sơ điều tra, ghi nhận nhật ký xử lý và xuất báo cáo sự cố."
        badge={<StatusPill tone="neutral">{cases.length} hồ sơ</StatusPill>}
      />

      {message && (
        <Notice tone={message.tone} onDismiss={() => setMessage(null)}>
          {message.text}
        </Notice>
      )}

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-12">
        {/* Left: Case List */}
        <div className="space-y-4 lg:col-span-7">
          <form onSubmit={handleCreate} className="flex gap-2.5">
            <input
              className="soc-input flex-1"
              placeholder="Nhập tên hồ sơ sự cố mới..."
              value={title}
              onChange={(event) => setTitle(event.target.value)}
            />
            <Button type="submit" variant="primary">
              <PlusIcon size={14} /> Tạo case
            </Button>
          </form>

          <Panel title="Danh sách hồ sơ sự cố" noPadding>
            {cases.length === 0 ? (
              <EmptyState
                title="Chưa có hồ sơ sự cố nào"
                description="Tạo hồ sơ mới để bắt đầu theo dõi tiến trình điều tra."
                icon={<BriefcaseIcon size={18} />}
              />
            ) : (
              <div className="divide-y divide-border">
                {cases.map((item) => {
                  const isSelected = selected?.case_id === item.case_id;
                  return (
                    <button
                      key={item.case_id}
                      type="button"
                      onClick={() => void selectCase(item)}
                      className={`flex w-full cursor-pointer items-start justify-between gap-4 px-5 py-4 text-left transition-colors ${
                        isSelected
                          ? "border-l-2 border-l-accent bg-accent/10"
                          : "hover:bg-surface-hover/60"
                      }`}
                    >
                      <div className="min-w-0 space-y-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="font-mono text-xs text-faint">#{item.case_id}</span>
                          <strong className="truncate text-sm font-medium text-ink">{item.title}</strong>
                        </div>
                        <p className="font-mono text-[11px] text-muted">
                          {item.alert_count ?? 0} alerts · cập nhật{" "}
                          {new Date(item.updated_at).toLocaleString("vi-VN")}
                        </p>
                      </div>
                      <div className="flex shrink-0 items-center gap-2">
                        <SeverityBadge severity={item.priority} />
                        <StatusPill tone={item.status === "resolved" || item.status === "closed" ? "success" : "neutral"}>
                          {item.status}
                        </StatusPill>
                      </div>
                    </button>
                  );
                })}
              </div>
            )}
          </Panel>
        </div>

        {/* Right: Case Detail & Audit Timeline */}
        <aside className="lg:col-span-5">
          <Panel title="Chi tiết hồ sơ & Nhật ký điều tra">
            {!selected ? (
              <EmptyState
                title="Chưa chọn hồ sơ"
                description="Chọn một hồ sơ ở cột bên trái để xem timeline và cập nhật trạng thái."
                icon={<BriefcaseIcon size={18} />}
              />
            ) : (
              <div className="space-y-5">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <p className="font-mono text-[11px] uppercase tracking-wider text-accent">
                      Incident #{selected.case_id}
                    </p>
                    <h3 className="mt-1 text-base font-semibold text-ink">{selected.title}</h3>
                  </div>
                  <SeverityBadge severity={selected.priority} />
                </div>

                <div className="grid grid-cols-1 gap-2.5 sm:grid-cols-2">
                  <select
                    aria-label="Trạng thái hồ sơ"
                    className="soc-select font-mono text-xs"
                    value={selected.status}
                    onChange={(event) => void handleStatus(event.target.value as CaseRecord["status"])}
                  >
                    <option value="open">Open</option>
                    <option value="investigating">Investigating</option>
                    <option value="resolved">Resolved</option>
                    <option value="closed">Closed</option>
                  </select>

                  <Button
                    variant="secondary"
                    disabled={exportingReport}
                    onClick={() => void handleExportReport()}
                  >
                    <DownloadIcon size={14} />
                    {exportingReport ? "Đang xuất..." : "Xuất báo cáo HTML"}
                  </Button>
                </div>

                <form onSubmit={handleNote} className="space-y-2.5 border-t border-border pt-4">
                  <label className="block text-xs font-medium text-ink-secondary">Thêm ghi chú điều tra</label>
                  <textarea
                    className="soc-textarea"
                    placeholder="Ghi lại phát hiện, IOC hoặc hành động đã thực hiện..."
                    value={note}
                    onChange={(event) => setNote(event.target.value)}
                  />
                  <Button type="submit" variant="primary" size="sm">
                    Lưu ghi chú
                  </Button>
                </form>

                <div className="border-t border-border pt-4">
                  <h4 className="mb-3 font-mono text-[11px] uppercase tracking-wider text-muted">
                    Timeline ({timeline.length})
                  </h4>
                  {timeline.length === 0 ? (
                    <p className="text-xs text-faint">Chưa có ghi nhận nào trong timeline.</p>
                  ) : (
                    <ol className="space-y-3 border-l border-border pl-4">
                      {timeline.map((item) => (
                        <li key={`${item.kind}-${item.id}`} className="relative text-xs">
                          <span className="absolute -left-[21px] top-1.5 h-2 w-2 rounded-full bg-accent" />
                          <div className="flex items-center justify-between gap-2">
                            <span className="font-mono font-semibold uppercase text-ink-secondary">
                              {item.kind}
                            </span>
                            <span className="font-mono text-[11px] text-faint">
                              {new Date(item.created_at).toLocaleString("vi-VN")}
                            </span>
                          </div>
                          <p className="mt-1 text-muted">{item.body}</p>
                        </li>
                      ))}
                    </ol>
                  )}
                </div>
              </div>
            )}
          </Panel>
        </aside>
      </div>
    </PageShell>
  );
}