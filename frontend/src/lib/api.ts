export type DashboardSummary = {
  open_alerts: number;
  events_processed: number;
  connected_assets: number;
};

export type Alert = {
  alert_id: number;
  rule_id: number;
  asset_id: number | null;
  triggered_at: string;
  last_seen?: string;
  occurrences?: number;
  entity_key?: string;
  severity: string;
  status: string;
  assigned_to: string | null;
  summary: string;
};

export type CaseRecord = {
  case_id: number;
  title: string;
  status: "open" | "investigating" | "resolved" | "closed";
  classification: "true_positive" | "false_positive" | null;
  priority: string;
  assigned_to: number | null;
  created_by: number;
  resolution: string | null;
  alert_count?: number;
  created_at: string;
  updated_at: string;
};

export type CaseTimelineItem = {
  kind: "note" | "audit";
  id: number;
  actor_user_id: number;
  body: string;
  created_at: string;
};

export type EventRecord = {
  event_id?: string;
  event_time?: string;
  message?: string;
  hostname?: string;
  event_type?: string;
  severity?: string;
  source_type?: string;
  log_category?: string;
  src_ip?: string;
  username?: string;
  fingerprint?: string;
  duplicate_count?: number;
  first_seen?: string;
  last_seen?: string;
  raw?: string;
  extra_fields?: Record<string, string>;
  [key: string]: unknown;
};

export type Rule = {
  rule_id: number;
  name: string;
  description: string | null;
  regex_pattern: string;
  target_field: string;
  severity: string;
  category: string;
  enabled: boolean;
  condition: Record<string, unknown>;
};

export type Asset = {
  asset_id: number;
  hostname: string;
  ip_address: string;
  os_type: string;
  criticality: string;
  owner: string | null;
  created_at: string;
};

export type User = {
  user_id: number;
  email: string;
  display_name: string;
  role: "admin" | "analyst" | "viewer";
  enabled: boolean;
};

export class ApiError extends Error {
  constructor(public readonly status: number, message: string) {
    super(message);
  }
}

const apiURL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  const token = typeof window !== "undefined" ? localStorage.getItem("siem_token") : null;
  if (token) headers.set("Authorization", `Bearer ${token}`);

  const response = await fetch(`${apiURL}${path}`, { ...init, headers });
  const body = await response.text();
  let payload: unknown;
  try {
    payload = body ? JSON.parse(body) : undefined;
  } catch {
    payload = undefined;
  }
  if (!response.ok) {
    const message =
      typeof payload === "object" && payload !== null && "error" in payload && typeof payload.error === "string"
        ? payload.error
        : `Request failed with status ${response.status}`;
    throw new ApiError(response.status, message);
  }
  return payload as T;
}

export function getSummary() {
  return request<DashboardSummary>("/api/v1/summary");
}

export function getAlerts() {
  return request<Alert[]>("/api/v1/alerts");
}

export function updateAlert(id: number, status: string, assignedTo = "") {
  return request<{ alert_id: number; status: string }>(`/api/v1/alerts/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ status, assigned_to: assignedTo }),
  });
}

export function getCases() {
  return request<CaseRecord[]>("/api/v1/cases");
}

export function createCase(title: string, priority = "medium", alertId?: number) {
  return request<{ case_id: number }>("/api/v1/cases", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ title, priority, alert_id: alertId }),
  });
}

export function updateCase(id: number, update: Partial<Pick<CaseRecord, "title" | "status" | "classification" | "priority" | "resolution">> & { assigned_to?: number | null }) {
  return request<{ case_id: number }>(`/api/v1/cases/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(update),
  });
}

export function addCaseNote(id: number, body: string) {
  return request<{ note_id: number }>(`/api/v1/cases/${id}/notes`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ body }),
  });
}

export function getCaseTimeline(id: number) {
  return request<CaseTimelineItem[]>(`/api/v1/cases/${id}/timeline`);
}

export async function getEvents(options: { page?: number; pageSize?: number; q?: string; severity?: string; category?: string; host?: string; from?: string; to?: string } = {}) {
  const params = new URLSearchParams({ page: String(options.page ?? 1), page_size: String(options.pageSize ?? 25) });
  for (const [key, value] of Object.entries({ q: options.q, severity: options.severity, category: options.category, host: options.host, from: options.from, to: options.to })) {
    if (value) params.set(key, value);
  }
  return request<{ items: EventRecord[]; total: number; page: number; page_size: number }>(`/api/v1/events?${params}`);
}

export function getRules() {
  return request<Rule[]>("/api/v1/rules");
}

export function getAssets() {
  return request<Asset[]>("/api/v1/assets");
}

export function createAsset(asset: {
  hostname: string;
  ip_address?: string;
  os_type: string;
  criticality?: string;
  owner?: string;
}) {
  return request<{ asset_id: number; hostname: string }>("/api/v1/assets", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(asset),
  });
}

export function createRule(
  rule: Omit<Rule, "rule_id" | "description" | "condition"> & {
    description?: string;
    condition?: Record<string, unknown>;
  }
) {
  return request<{ rule_id: number }>("/api/v1/rules", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(rule),
  });
}

export function deleteRule(id: number) {
  return request<void>(`/api/v1/rules/${id}`, { method: "DELETE" });
}

export function login(email: string, password: string) {
  return request<{ token: string; user: User }>("/api/v1/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
}

export function getMe() {
  return request<User>("/api/v1/auth/me");
}

export function getUsers() {
  return request<User[]>("/api/v1/users");
}

export function createUser(user: { email: string; password: string; display_name: string; role: User["role"] }) {
  return request<User>("/api/v1/users", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(user),
  });
}

export function disableUser(id: number) {
  return request<void>(`/api/v1/users/${id}`, { method: "DELETE" });
}

export function testRegex(pattern: string, log: string, target_field = "message") {
  return request<{ matched: boolean; groups: string[]; pattern: string }>("/api/v1/rules/test-regex", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ pattern, log, target_field }),
  });
}

export function ingestLog(message: string, sourceType = "generic", hostname = "web-01") {
  return request<{ stream_id: string }>("/api/v1/ingest", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ message, source_type: sourceType, hostname }),
  });
}

export type AnalyticsData = {
  total_events: number;
  events_by_severity: Record<string, number>;
  events_by_category: Record<string, number>;
  top_attacking_ips: Array<{
    ip: string;
    count: number;
    country: string;
    country_code: string;
    threat_level: string;
    threat_category?: string;
    reputation_score?: number;
  }>;
  top_targeted_users: Array<{ username: string; count: number }>;
  timeline: Array<{ time: string; count: number }>;
};

export function getAnalytics() {
  return request<AnalyticsData>("/api/v1/analytics");
}