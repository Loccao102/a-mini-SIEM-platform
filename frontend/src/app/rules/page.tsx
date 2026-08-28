"use client";

import Link from "next/link";
import { FormEvent, useEffect, useState } from "react";
import { createRule, deleteRule, getRules, Rule, testRegex } from "@/lib/api";

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
    pattern: "(?i)(\\.\\./|\\.\\.\\\|/etc/passwd|/proc/self|\\.env|web\\.config)",
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
  const [message, setMessage] = useState("");

  // Sandbox state
  const [sandboxLog, setSandboxLog] = useState("Failed password for root from 192.168.1.105 port 54322 ssh2");
  const [sandboxPattern, setSandboxPattern] = useState("Failed password for (?:invalid user )?(\\S+) from (\\S+) port (\\d+)");
  const [sandboxTarget, setSandboxTarget] = useState("message");
  const [sandboxResult, setSandboxResult] = useState<{ matched: boolean; groups: string[]; error?: string } | null>(null);
  const [testingSandbox, setTestingSandbox] = useState(false);

  async function loadRules() {
    try {
      setRules(await getRules());
    } catch {
      setMessage("Không thể tải danh sách luật. Hãy đăng nhập tài khoản có quyền xem/admin.");
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
      setMessage("Đã tạo luật thành công.");
      await loadRules();
    } catch {
      setMessage("Không thể tạo luật. Kiểm tra quyền Admin.");
    }
  }

  async function remove(id: number) {
    if (!window.confirm("Xóa luật này?")) return;
    try {
      await deleteRule(id);
      await loadRules();
    } catch {
      setMessage("Không thể xóa luật. Yêu cầu quyền Admin.");
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
    <main className="account-shell">
      <header className="account-header">
        <Link href="/">← Overview</Link>
        <span className="eyebrow">Detection logic & Sandbox</span>
        <h1>Detection Rules</h1>
        <p>Định nghĩa các mẫu nhận diện MITRE ATT&CK và thử nghiệm Regular Expression trước khi áp dụng.</p>
      </header>

      <section className="account-grid">
        <div>
          {/* 1-Click Presets */}
          <div className="mb-6 p-4 border border-[var(--line)] bg-[var(--surface)] rounded">
            <span className="block text-xs font-mono text-[var(--acid)] uppercase tracking-wider mb-2">
              ⚡ 1-Click Rule Presets (MITRE ATT&CK)
            </span>
            <div className="presets-group">
              {RULE_PRESETS.map((preset) => (
                <button
                  key={preset.name}
                  type="button"
                  onClick={() => applyPreset(preset)}
                  className="btn-preset"
                  title={`Nạp preset: ${preset.name}`}
                >
                  + {preset.name}
                </button>
              ))}
            </div>
          </div>

          {/* Rule Creation Form */}
          <form className="account-form" onSubmit={submit}>
            <h2>Thêm luật mới</h2>
            <label>
              Tên luật (Name)
              <input required value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} />
            </label>

            <label>
              Biểu thức Regex (Regex Pattern)
              <input
                required
                value={form.regex_pattern}
                onChange={(event) => setForm({ ...form, regex_pattern: event.target.value })}
              />
            </label>

            <label>
              Trường kiểm tra (Target field)
              <select
                value={form.target_field}
                onChange={(event) => setForm({ ...form, target_field: event.target.value })}
              >
                <option value="message">Message (Nội dung log thô)</option>
                <option value="hostname">Hostname</option>
                <option value="username">Username</option>
                <option value="src_ip">Source IP</option>
                <option value="event_type">Event Type</option>
                <option value="log_category">Log Category</option>
              </select>
            </label>

            <label>
              Độ nghiêm trọng (Severity)
              <select value={form.severity} onChange={(event) => setForm({ ...form, severity: event.target.value })}>
                <option value="info">Info</option>
                <option value="low">Low</option>
                <option value="medium">Medium</option>
                <option value="high">High</option>
                <option value="critical">Critical</option>
              </select>
            </label>

            <label>
              Phân loại (Category)
              <input required value={form.category} onChange={(event) => setForm({ ...form, category: event.target.value })} />
            </label>

            <div className="grid grid-cols-2 gap-4 mt-2">
              <label>
                Ngưỡng lặp (Count)
                <input
                  type="number"
                  min={1}
                  value={form.conditionCount}
                  onChange={(event) => setForm({ ...form, conditionCount: Number(event.target.value) })}
                />
              </label>
              <label>
                Window (Giây)
                <input
                  type="number"
                  min={0}
                  value={form.conditionWindow}
                  onChange={(event) => setForm({ ...form, conditionWindow: Number(event.target.value) })}
                />
              </label>
            </div>

            <label>
              Cooldown Period (Giây)
              <input
                type="number"
                min={0}
                value={form.conditionCooldown}
                onChange={(event) => setForm({ ...form, conditionCooldown: Number(event.target.value) })}
              />
            </label>

            <button type="submit">Lưu Rule vào Hệ thống</button>
            {message && <small className="notice block">{message}</small>}
          </form>

          {/* Interactive Regex Sandbox */}
          <div className="sandbox-box">
            <h3 className="sandbox-title">🧪 Interactive Regex Sandbox</h3>
            <p className="text-xs text-[var(--muted)] mb-4">
              Dán chuỗi log mẫu để kiểm tra khả năng khớp và bóc tách Capture Groups trước khi lưu.
            </p>

            <label className="block text-xs font-mono text-[var(--muted)] uppercase mb-2">Log Mẫu (Sample Log)</label>
            <input
              className="w-full mb-3"
              value={sandboxLog}
              onChange={(e) => setSandboxLog(e.target.value)}
              placeholder="Dán dòng log kiểm thử vào đây..."
            />

            <label className="block text-xs font-mono text-[var(--muted)] uppercase mb-2">Regex Test Pattern</label>
            <input
              className="w-full mb-3"
              value={sandboxPattern}
              onChange={(e) => setSandboxPattern(e.target.value)}
              placeholder="Nhập biểu thức regex..."
            />

            <button type="button" onClick={handleTestRegex} disabled={testingSandbox} className="btn-preset bg-[var(--acid)] text-[var(--canvas)] font-bold">
              {testingSandbox ? "Testing..." : "🔍 Test Match Now"}
            </button>

            {sandboxResult && (
              <div className="mt-4">
                {sandboxResult.error ? (
                  <span className="match-badge match-fail">Error: {sandboxResult.error}</span>
                ) : sandboxResult.matched ? (
                  <div>
                    <span className="match-badge match-success">✔ MATCH FOUND (MATCH SUCCESS)</span>
                    <div className="groups-list">
                      <strong>Captured Groups ({sandboxResult.groups.length}):</strong>
                      {sandboxResult.groups.map((group, idx) => (
                        <div key={idx} className="mt-1">
                          <span className="text-[var(--aqua)] font-bold">Group {idx}:</span>{" "}
                          <code>{group || "(empty)"}</code>
                        </div>
                      ))}
                    </div>
                  </div>
                ) : (
                  <span className="match-badge match-fail">✖ NO MATCH FOUND</span>
                )}
              </div>
            )}
          </div>
        </div>

        {/* Active Rules List */}
        <section className="account-list">
          <div className="list-heading">
            <h2>Tập luật kích hoạt ({rules.length})</h2>
          </div>
          {!rules.length && <p className="empty-state">Chưa có luật nào được lưu.</p>}
          {rules.map((rule) => (
            <div className="data-row" key={rule.rule_id}>
              <div>
                <strong>{rule.name}</strong>
                <small className="block mt-1">
                  Trường <code>{rule.target_field}</code> matches <code>/{rule.regex_pattern}/</code> · {rule.category} ·{" "}
                  <span className={`severity ${rule.severity}`}>{rule.severity}</span>
                </small>
              </div>
              <button className="disable" onClick={() => void remove(rule.rule_id)}>
                Xóa
              </button>
            </div>
          ))}
        </section>
      </section>
    </main>
  );
}
