"use client";

import Link from "next/link";
import { FormEvent, useEffect, useState } from "react";
import { createRule, deleteRule, getRules, Rule } from "@/lib/api";

const initialForm = { name: "", regex_pattern: "", target_field: "message", severity: "medium", category: "authentication" };

export default function RulesPage() {
  const [rules, setRules] = useState<Rule[]>([]);
  const [form, setForm] = useState(initialForm);
  const [message, setMessage] = useState("");
  async function loadRules() { try { setRules(await getRules()); } catch { setMessage("Rules could not be loaded. Sign in with admin access."); } }
  useEffect(() => { const timer = window.setTimeout(() => void loadRules(), 0); return () => window.clearTimeout(timer); }, []);
  async function submit(event: FormEvent) { event.preventDefault(); try { await createRule({ ...form, enabled: true }); setForm(initialForm); setMessage("Rule created."); await loadRules(); } catch { setMessage("Rule could not be created."); } }
  async function remove(id: number) { if (!window.confirm("Delete this rule?")) return; try { await deleteRule(id); await loadRules(); } catch { setMessage("Rule could not be deleted."); } }
  return <main className="account-shell"><header className="account-header"><Link href="/">&lt;- Overview</Link><span className="eyebrow">Detection logic</span><h1>Rules</h1><p>Define the patterns that turn normalized events into actionable alerts.</p></header><section className="account-grid"><form className="account-form" onSubmit={submit}><h2>New rule</h2><label>Name<input required value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} /></label><label>Regex pattern<input required value={form.regex_pattern} onChange={(event) => setForm({ ...form, regex_pattern: event.target.value })} /></label><label>Target field<select value={form.target_field} onChange={(event) => setForm({ ...form, target_field: event.target.value })}><option value="message">Message</option><option value="hostname">Hostname</option><option value="username">Username</option><option value="src_ip">Source IP</option></select></label><label>Severity<select value={form.severity} onChange={(event) => setForm({ ...form, severity: event.target.value })}><option value="low">Low</option><option value="medium">Medium</option><option value="high">High</option><option value="critical">Critical</option></select></label><label>Category<input required value={form.category} onChange={(event) => setForm({ ...form, category: event.target.value })} /></label><button type="submit">Create rule</button>{message && <small>{message}</small>}</form><section className="account-list"><div className="list-heading"><h2>Active rules</h2><span>{rules.length} rules</span></div>{!rules.length && <p className="empty-state">No rules found.</p>}{rules.map((rule) => <div className="data-row" key={rule.rule_id}><div><strong>{rule.name}</strong><small>{rule.target_field} matches /{rule.regex_pattern}/ · {rule.category}</small></div><button className="disable" onClick={() => void remove(rule.rule_id)}>Delete</button></div>)}</section></section></main>;
}
