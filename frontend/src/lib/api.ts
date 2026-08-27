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
  severity: string;
  status: string;
  assigned_to: string | null;
  summary: string;
};

export type EventRecord = {
  event_id?: string;
  event_time?: string;
  message?: string;
  hostname?: string;
  event_type?: string;
  severity?: string;
  source_type?: string;
  [key: string]: unknown;
};

export type Rule = { rule_id: number; name: string; description: string | null; regex_pattern: string; target_field: string; severity: string; category: string; enabled: boolean; condition: Record<string, unknown> };
export type Asset = { asset_id: number; hostname: string; ip_address: string; os_type: string; criticality: string; owner: string | null; created_at: string };
export type User = { user_id: number; email: string; display_name: string; role: "admin" | "analyst" | "viewer"; enabled: boolean };

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
    const message = typeof payload === "object" && payload !== null && "error" in payload && typeof payload.error === "string"
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

export async function getEvents(limit = 100) {
  const payload = await request<{ hits?: { hits?: Array<{ _source?: EventRecord }> } }>(`/api/v1/events?limit=${limit}`);
  return payload.hits?.hits?.map((hit) => hit._source ?? {}) ?? [];
}

export function getRules() { return request<Rule[]>("/api/v1/rules"); }
export function getAssets() { return request<Asset[]>("/api/v1/assets"); }

export function createRule(rule: Omit<Rule, "rule_id" | "description" | "condition"> & { description?: string; condition?: Record<string, unknown> }) {
  return request<{ rule_id: number }>("/api/v1/rules", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(rule) });
}

export function deleteRule(id: number) { return request<void>(`/api/v1/rules/${id}`, { method: "DELETE" }); }

export function login(email: string, password: string) {
  return request<{ token: string; user: User }>("/api/v1/auth/login", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ email, password }) });
}

export function getUsers() { return request<User[]>("/api/v1/users"); }
export function createUser(user: { email: string; password: string; display_name: string; role: User["role"] }) {
  return request<User>("/api/v1/users", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(user) });
}
export function disableUser(id: number) { return request<void>(`/api/v1/users/${id}`, { method: "DELETE" }); }