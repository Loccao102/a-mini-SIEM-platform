"use client";

import Link from "next/link";
import { FormEvent, useEffect, useState } from "react";
import { addCaseNote, ApiError, CaseRecord, CaseTimelineItem, createCase, getCaseTimeline, getCases, updateCase } from "@/lib/api";

export default function CasesPage() {
  const [cases, setCases] = useState<CaseRecord[]>([]);
  const [selected, setSelected] = useState<CaseRecord | null>(null);
  const [timeline, setTimeline] = useState<CaseTimelineItem[]>([]);
  const [title, setTitle] = useState("");
  const [note, setNote] = useState("");
  const [message, setMessage] = useState("");

  async function loadCases() {
    try {
      const nextCases = await getCases();
      setCases(nextCases);
      if (selected) setSelected(nextCases.find((item) => item.case_id === selected.case_id) ?? null);
    } catch (error) {
      setMessage(error instanceof ApiError && error.status === 401 ? "Sign in to view cases." : "Cases could not be loaded.");
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
        .catch((error: unknown) => setMessage(error instanceof ApiError && error.status === 401 ? "Sign in to view cases." : "Cases could not be loaded."));
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
    } catch { setMessage("Case could not be created. Analyst or Admin access required."); }
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

  return (
    <main className="account-shell">
      <header className="account-header">
        <Link href="/">← Overview</Link>
        <span className="eyebrow">SOC Case Management & Audit Trail</span>
        <h1>Cases</h1>
        <p>Gom cảnh báo thành incident, theo dõi điều tra và lưu lại mọi thao tác xử lý.</p>
      </header>
      <section className="data-page grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(320px,0.8fr)]">
        <div>
          <form onSubmit={handleCreate} className="mb-4 flex gap-3">
            <input className="flex-1" placeholder="Tên case mới" value={title} onChange={(event) => setTitle(event.target.value)} />
            <button type="submit">Tạo case</button>
          </form>
          {message && <p className="notice">{message}</p>}
          {!cases.length && <p className="empty-state">Chưa có case nào.</p>}
          {cases.map((item) => (
            <button key={item.case_id} type="button" onClick={() => void selectCase(item)} className="data-row mb-2 block w-full text-left">
              <strong>{item.title}</strong>
              <small>{item.status} · {item.priority} · {item.alert_count ?? 0} alerts · cập nhật {new Date(item.updated_at).toLocaleString()}</small>
            </button>
          ))}
        </div>
        <aside className="border border-(--line) bg-(--surface) p-4">
          {!selected ? <p className="empty-state">Chọn một case để xem timeline.</p> : <>
            <h2>{selected.title}</h2>
            <select value={selected.status} onChange={(event) => void handleStatus(event.target.value as CaseRecord["status"])}>
              <option value="open">Open</option><option value="investigating">Investigating</option><option value="resolved">Resolved</option><option value="closed">Closed</option>
            </select>
            <form onSubmit={handleNote} className="mt-4 grid gap-2">
              <textarea placeholder="Ghi chú điều tra" value={note} onChange={(event) => setNote(event.target.value)} />
              <button type="submit">Thêm ghi chú</button>
            </form>
            <div className="mt-4 grid gap-3">
              {timeline.map((item) => <div key={`${item.kind}-${item.id}`} className="border-t border-(--line) pt-2 text-xs"><strong>{item.kind}</strong> · {item.body}<br /><small>{new Date(item.created_at).toLocaleString()}</small></div>)}
            </div>
          </>}
        </aside>
      </section>
    </main>
  );
}