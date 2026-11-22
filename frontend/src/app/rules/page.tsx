"use client";

import { FormEvent, useEffect, useState } from "react";
import { createRule, deleteRule, getRules, Rule, testRegex } from "@/lib/api";
import { SeverityBadge } from "@/components/severity-badge";
import { FlaskIcon, PlusIcon, ShieldAlertIcon, TrashIcon } from "@/components/ui/icons";
import { Button, EmptyState, Notice, PageHeader, PageShell, Panel, StatusPill } from "@/components/ui/primitives";

const initialForm = {
  name: "",
  regex_pattern: "",
  target_field: "message",
  severity: "medium",
  category: "authentication",
  conditionCount: 1,
  conditionWindow: 60,
  conditionCooldown: 180,
};

const RULE_PRESETS = [
  {
    name: "SSH Brute Force Attack",
    pattern: "(?i)(Failed password|authentication failure)",
    target: "message",
    severity: "high",
    category: "authentication",
    count: 5,
    window: 60,
    cooldown: 180,
    sampleLog: "Failed password for root from 192.168.1.105 port 54322 ssh2",
  },
  {
    name: "Web SQL Injection Attempt",
    pattern: "(?i)(UNION\\s+SELECT|OR\\s+['\"]?1['\"]?\\s*=\\s*['\"]?1|information_schema|sleep\\(\\d+\\))",
    target: "message",
    severity: "critical",
    category: "web_attack",
    count: 1,
    window: 0,
    cooldown: 180,
    sampleLog: "GET /api/products?id=1 UNION SELECT 1,username,password FROM users -- HTTP/1.1",
  },
  {
    name: "Web Directory Traversal / LFI",
    pattern: "(?i)(\\.\\./|\\.\\.\\\\|/etc/passwd|/proc/self|\\.env|web\\.config)",
    target: "message",
    severity: "high",
    category: "web_attack",
    count: 1,
    window: 0,
    cooldown: 180,
    sampleLog: "GET /download?file=../../../../etc/passwd HTTP/1.1",
  },
  {
    name: "Automated Security Scanner Probing",
    pattern: "(?i)(sqlmap|nikto|acunetix|nuclei|nmap|masscan)",
    target: "message",
    severity: "medium",
    category: "web_attack",
    count: 1,
    window: 0,
    cooldown: 180,
    sampleLog: "GET /admin HTTP/1.1 User-Agent: sqlmap/1.6.12#stable (https://sqlmap.org)",
  },
  {
    name: "Windows Logon Failure Spike (Event 4625)",
    pattern: "(?i)(4625|An account failed to log on|logon failure)",
    target: "message",
    severity: "high",
    category: "authentication",
    count: 5,
    window: 120,
    cooldown: 180,
    sampleLog: "An account failed to log on. Event ID: 4625. Account Name: Administrator",
  },
  {
    name: "Suspicious Privilege Escalation (Sudo Shell)",
    pattern: "(?i)(COMMAND=.*(/bin/bash|-i|/bin/sh)|curl\\s+|wget\\s+|nc\\s+)",
    target: "message",
    severity: "high",
    category: "privilege_escalation",
    count: 1,
    window: 0,
    cooldown: 180,
    sampleLog: "sudo: alice : TTY=pts/0 ; PWD=/home/alice ; USER=root ; COMMAND=/bin/bash -i",
  },
  {
    name: "Windows Security Audit Log Cleared (Event 1102)",
    pattern: "(?i)(1102|104|audit log was cleared)",
    target: "message",
    severity: "critical",
    category: "defense_evasion",
    count: 1,
    window: 0,
    cooldown: 180,
    sampleLog: "Event ID: 1102 The audit log was cleared by Administrator",
  },
];

export default function RulesPage() {
  const [rules, setRules] = useState<Rule[]>([]);
  const [form, setForm] = useState(initialForm);
  const [message, setMessage] = useState<{ text: string; tone: "error" | "success" } | null>(null);

  // Sandbox state
  const [sandboxLog, setSandboxLog] = useState("Failed password for root from 192.168.1.105 port 54322 ssh2");
  const [sandboxPattern, setSandboxPattern] = useState(
    "Failed password for (?:invalid user )?(\\S+) from (\\S+) port (\\d+)"
  );
  const [sandboxTarget, setSandboxTarget] = useState("message");
  const [sandboxResult, setSandboxResult] = useState<{ matched: boolean; groups: string[]; error?: string } | null>(
    null
  );
  const [testingSandbox, setTestingSandbox] = useState(false);

  async function loadRules() {
    try {
      setRules(await getRules());
    } catch {
      setMessage({ tone: "error", text: "Không thể tải danh sách luật. Kiểm tra quyền truy cập." });
    }
  }

  useEffect(() => {
    const timer = window.setTimeout(() => void loadRules(), 0);
    return () => window.clearTimeout(timer);
  }, []);

  async function submit(event: FormEvent) {
    event.preventDefault();
    try {
      const condition: Record<string, unknown> = {};
      if (form.conditionCount > 1) {
        condition.count = Number(form.conditionCount);
        condition.window_seconds = Number(form.conditionWindow);
      }
      condition.cooldown_seconds = Number(form.conditionCooldown);

      await createRule({
        name: form.name,
        regex_pattern: form.regex_pattern,
        target_field: form.target_field,
        severity: form.severity,
        category: form.category,
        enabled: true,
        condition,
      });
      setForm(initialForm);
      setMessage({ tone: "success", text: "Đã lưu quy tắc phát hiện mới vào hệ thống." });
      await loadRules();
    } catch {
      setMessage({ tone: "error", text: "Không thể tạo luật. Yêu cầu quyền Admin." });
    }
  }

  async function remove(id: number) {
    if (!window.confirm("Xóa quy tắc phát hiện này?")) return;
    try {
      await deleteRule(id);
      await loadRules();
    } catch {
      setMessage({ tone: "error", text: "Không thể xóa luật. Yêu cầu quyền Admin." });
    }
  }

  function applyPreset(preset: (typeof RULE_PRESETS)[0]) {
    setForm({
      name: preset.name,
      regex_pattern: preset.pattern,
      target_field: preset.target,
      severity: preset.severity,
      category: preset.category,
      conditionCount: preset.count,
      conditionWindow: preset.window,
      conditionCooldown: preset.cooldown,
    });
    setSandboxPattern(preset.pattern);
    setSandboxLog(preset.sampleLog);
    setSandboxTarget(preset.target);
  }

  async function handleTestRegex() {
    setTestingSandbox(true);
    setSandboxResult(null);
    try {
      const res = await testRegex(sandboxPattern, sandboxLog, sandboxTarget);
      setSandboxResult({ matched: res.matched, groups: res.groups });
    } catch (err: unknown) {
      const errMsg = err instanceof Error ? err.message : "Regex test failed";
      setSandboxResult({ matched: false, groups: [], error: errMsg });
    } finally {
      setTestingSandbox(false);
    }
  }

  return (
    <PageShell>
      <PageHeader
        title="Detection Rules"
        description="Quản lý tập luật phát hiện theo MITRE ATT&CK và kiểm thử biểu thức Regex trực tiếp trên Sandbox."
        badge={<StatusPill tone="accent">{rules.length} luật hoạt động</StatusPill>}
      />

      {message && (
        <Notice tone={message.tone} onDismiss={() => setMessage(null)}>
          {message.text}
        </Notice>
      )}

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-12">
        {/* Left Column: Presets + Create Form + Sandbox */}
        <div className="space-y-6 xl:col-span-5">
          <Panel title="Mẫu quy tắc nhanh (MITRE ATT&CK)" subtitle="Nạp sẵn mẫu vào biểu mẫu và Regex Sandbox">
            <div className="flex flex-wrap gap-2">
              {RULE_PRESETS.map((preset) => (
                <Button key={preset.name} size="sm" onClick={() => applyPreset(preset)}>
                  <PlusIcon size={12} /> {preset.name}
                </Button>
              ))}
            </div>
          </Panel>

          <Panel title="Tạo quy tắc phát hiện mới">
            <form className="space-y-3.5" onSubmit={submit}>
              <div>
                <label className="mb-1 block text-xs font-medium text-ink-secondary">Tên quy tắc</label>
                <input
                  className="soc-input"
                  required
                  placeholder="VD: SSH Brute Force Attack"
                  value={form.name}
                  onChange={(event) => setForm({ ...form, name: event.target.value })}
                />
              </div>

              <div>
                <label className="mb-1 block text-xs font-medium text-ink-secondary">Biểu thức Regex</label>
                <input
                  className="soc-input font-mono text-xs"
                  required
                  placeholder="(?i)(Failed password|authentication failure)"
                  value={form.regex_pattern}
                  onChange={(event) => setForm({ ...form, regex_pattern: event.target.value })}
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="mb-1 block text-xs font-medium text-ink-secondary">Trường kiểm tra</label>
                  <select
                    className="soc-select"
                    value={form.target_field}
                    onChange={(event) => setForm({ ...form, target_field: event.target.value })}
                  >
                    <option value="message">message</option>
                    <option value="hostname">hostname</option>
                    <option value="username">username</option>
                    <option value="src_ip">src_ip</option>
                    <option value="event_type">event_type</option>
                    <option value="log_category">log_category</option>
                  </select>
                </div>

                <div>
                  <label className="mb-1 block text-xs font-medium text-ink-secondary">Mức độ nghiêm trọng</label>
                  <select
                    className="soc-select"
                    value={form.severity}
                    onChange={(event) => setForm({ ...form, severity: event.target.value })}
                  >
                    <option value="info">Info</option>
                    <option value="low">Low</option>
                    <option value="medium">Medium</option>
                    <option value="high">High</option>
                    <option value="critical">Critical</option>
                  </select>
                </div>
              </div>

              <div>
                <label className="mb-1 block text-xs font-medium text-ink-secondary">Phân loại (Category)</label>
                <input
                  className="soc-input font-mono text-xs"
                  required
                  value={form.category}
                  onChange={(event) => setForm({ ...form, category: event.target.value })}
                />
              </div>

              <div className="grid grid-cols-3 gap-3">
                <div>
                  <label className="mb-1 block text-xs font-medium text-ink-secondary">Ngưỡng (Count)</label>
                  <input
                    className="soc-input font-mono"
                    type="number"
                    min={1}
                    value={form.conditionCount}
                    onChange={(event) => setForm({ ...form, conditionCount: Number(event.target.value) })}
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-medium text-ink-secondary">Window (s)</label>
                  <input
                    className="soc-input font-mono"
                    type="number"
                    min={0}
                    value={form.conditionWindow}
                    onChange={(event) => setForm({ ...form, conditionWindow: Number(event.target.value) })}
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-medium text-ink-secondary">Cooldown (s)</label>
                  <input
                    className="soc-input font-mono"
                    type="number"
                    min={0}
                    value={form.conditionCooldown}
                    onChange={(event) => setForm({ ...form, conditionCooldown: Number(event.target.value) })}
                  />
                </div>
              </div>

              <Button type="submit" variant="primary" className="w-full">
                Lưu quy tắc vào hệ thống
              </Button>
            </form>
          </Panel>

          <Panel
            title="Regex Sandbox"
            subtitle="Kiểm thử khớp mẫu và bóc tách Capture Groups trước khi kích hoạt"
            actions={<FlaskIcon size={15} className="text-accent" />}
          >
            <div className="space-y-3">
              <div>
                <label className="mb-1 block text-xs font-medium text-ink-secondary">Dòng log mẫu</label>
                <input
                  className="soc-input font-mono text-xs"
                  value={sandboxLog}
                  onChange={(e) => setSandboxLog(e.target.value)}
                  placeholder="Dán dòng log kiểm thử..."
                />
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium text-ink-secondary">Biểu thức kiểm thử</label>
                <input
                  className="soc-input font-mono text-xs"
                  value={sandboxPattern}
                  onChange={(e) => setSandboxPattern(e.target.value)}
                  placeholder="Nhập Regex..."
                />
              </div>
              <Button variant="secondary" onClick={handleTestRegex} disabled={testingSandbox}>
                {testingSandbox ? "Đang kiểm thử..." : "Chạy kiểm thử Regex"}
              </Button>

              {sandboxResult && (
                <div className="rounded-lg border border-border bg-canvas p-3 text-xs">
                  {sandboxResult.error ? (
                    <StatusPill tone="danger">Lỗi: {sandboxResult.error}</StatusPill>
                  ) : sandboxResult.matched ? (
                    <div className="space-y-2">
                      <StatusPill tone="success">Khớp mẫu thành công</StatusPill>
                      <div className="space-y-1 font-mono text-[11px]">
                        <p className="text-muted">Capture Groups ({sandboxResult.groups.length}):</p>
                        {sandboxResult.groups.map((group, idx) => (
                          <div key={idx} className="text-ink-secondary">
                            <span className="text-accent">Group {idx}:</span>{" "}
                            <code>{group || "(empty)"}</code>
                          </div>
                        ))}
                      </div>
                    </div>
                  ) : (
                    <StatusPill tone="danger">Không khớp mẫu</StatusPill>
                  )}
                </div>
              )}
            </div>
          </Panel>
        </div>

        {/* Right Column: Active Rules Table */}
        <div className="xl:col-span-7">
          <Panel title={`Tập luật đang kích hoạt (${rules.length})`} noPadding>
            {rules.length === 0 ? (
              <EmptyState
                title="Chưa có quy tắc nào được lưu"
                description="Chọn một mẫu MITRE ATT&CK ở cột trái hoặc tự định nghĩa quy tắc mới."
                icon={<ShieldAlertIcon size={18} />}
              />
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs">
                  <thead>
                    <tr className="border-b border-border bg-surface-raised/60 font-mono text-[11px] uppercase tracking-wider text-muted">
                      <th className="px-5 py-3">Mức độ</th>
                      <th className="px-4 py-3">Quy tắc & Biểu thức</th>
                      <th className="px-4 py-3">Phân loại</th>
                      <th className="px-5 py-3 text-right">Thao tác</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border">
                    {rules.map((rule) => (
                      <tr key={rule.rule_id} className="transition-colors hover:bg-surface-hover/60">
                        <td className="px-5 py-3.5 whitespace-nowrap">
                          <SeverityBadge severity={rule.severity} />
                        </td>
                        <td className="px-4 py-3.5">
                          <p className="font-medium text-ink">{rule.name}</p>
                          <p className="mt-1 font-mono text-[11px] text-muted break-all">
                            <span className="text-accent">{rule.target_field}</span> =~{" "}
                            <span className="text-ink-secondary">/{rule.regex_pattern}/</span>
                          </p>
                        </td>
                        <td className="px-4 py-3.5 whitespace-nowrap">
                          <StatusPill tone="neutral">{rule.category}</StatusPill>
                        </td>
                        <td className="px-5 py-3.5 text-right whitespace-nowrap">
                          <Button variant="danger" size="sm" onClick={() => void remove(rule.rule_id)}>
                            <TrashIcon size={13} /> Xóa
                          </Button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Panel>
        </div>
      </div>
    </PageShell>
  );
}
