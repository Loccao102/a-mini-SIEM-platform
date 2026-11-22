"use client";

import { useEffect, useState } from "react";
import { Asset, ApiError, createAsset, getAssets } from "@/lib/api";
import { SeverityBadge } from "@/components/severity-badge";
import { MonitorIcon, PlusIcon, ServerIcon } from "@/components/ui/icons";
import {
  Button,
  EmptyState,
  KpiCard,
  Notice,
  PageHeader,
  PageShell,
  Panel,
  StatusPill,
} from "@/components/ui/primitives";

const PLATFORM_TABS = [
  { key: "all", label: "Tất cả nền tảng" },
  { key: "linux", label: "Linux" },
  { key: "windows", label: "Windows" },
] as const;

export default function AssetsPage() {
  const [assets, setAssets] = useState<Asset[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<"all" | "linux" | "windows">("all");
  const [form, setForm] = useState({
    hostname: "",
    ip_address: "",
    os_type: "linux",
    criticality: "medium",
    owner: "",
  });
  const [saving, setSaving] = useState(false);
  const [formStatus, setFormStatus] = useState<{ text: string; tone: "success" | "error" } | null>(null);

  useEffect(() => {
    getAssets()
      .then((data) => {
        setAssets(data);
        setLoading(false);
      })
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 401) {
          setError("Phiên đăng nhập đã hết hạn. Vui lòng đăng nhập lại.");
        } else {
          setError("Không thể tải danh sách tài sản. Kiểm tra kết nối Backend.");
        }
        setLoading(false);
      });
  }, []);

  async function handleCreateAsset(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setFormStatus(null);
    try {
      await createAsset(form);
      setForm({ hostname: "", ip_address: "", os_type: "linux", criticality: "medium", owner: "" });
      setFormStatus({ tone: "success", text: "Đã đăng ký tài sản thành công. Node có thể bắt đầu gửi log." });
      setAssets(await getAssets());
    } catch (err: unknown) {
      setFormStatus({
        tone: "error",
        text: err instanceof ApiError ? err.message : "Không thể đăng ký tài sản.",
      });
    } finally {
      setSaving(false);
    }
  }

  const filtered = assets.filter((a) => filter === "all" || a.os_type === filter);

  const critCounts = {
    critical: assets.filter((a) => a.criticality === "critical").length,
    high: assets.filter((a) => a.criticality === "high").length,
    medium: assets.filter((a) => a.criticality === "medium").length,
    low: assets.filter((a) => a.criticality === "low").length,
  };

  return (
    <PageShell>
      <PageHeader
        title="Asset Inventory"
        description="Quản lý toàn bộ máy chủ và thiết bị gửi log về SIEM, phân loại mức độ trọng yếu và chủ sở hữu."
        badge={<StatusPill tone="accent">{assets.length} thiết bị</StatusPill>}
      />

      {/* 4-column KPI strip (fixes old 3-column overflow bug) */}
      <section className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <KpiCard
          label="Tổng tài sản"
          value={loading ? "—" : assets.length}
          subtext="Node đã đăng ký"
          tone="accent"
          icon={<ServerIcon size={16} />}
        />
        <KpiCard
          label="Critical"
          value={loading ? "—" : critCounts.critical}
          subtext="Rủi ro cao nhất"
          tone="critical"
        />
        <KpiCard
          label="High"
          value={loading ? "—" : critCounts.high}
          subtext="Mức độ trọng yếu cao"
          tone="high"
        />
        <KpiCard
          label="Medium / Low"
          value={loading ? "—" : critCounts.medium + critCounts.low}
          subtext="Đang giám sát"
          tone="success"
        />
      </section>

      {/* Register New Asset */}
      <Panel title="Đăng ký máy chủ / thiết bị mới">
        <form onSubmit={handleCreateAsset} className="grid grid-cols-1 gap-3 md:grid-cols-12">
          <input
            className="soc-input md:col-span-3"
            required
            placeholder="Hostname (VD: web-prod-01)"
            value={form.hostname}
            onChange={(event) => setForm({ ...form, hostname: event.target.value })}
          />
          <input
            className="soc-input font-mono md:col-span-2"
            placeholder="Địa chỉ IP"
            value={form.ip_address}
            onChange={(event) => setForm({ ...form, ip_address: event.target.value })}
          />
          <select
            className="soc-select md:col-span-2"
            value={form.os_type}
            onChange={(event) => setForm({ ...form, os_type: event.target.value })}
          >
            <option value="linux">Linux</option>
            <option value="windows">Windows</option>
            <option value="generic">Generic</option>
          </select>
          <select
            className="soc-select md:col-span-2"
            value={form.criticality}
            onChange={(event) => setForm({ ...form, criticality: event.target.value })}
          >
            <option value="low">Low</option>
            <option value="medium">Medium</option>
            <option value="high">High</option>
            <option value="critical">Critical</option>
          </select>
          <input
            className="soc-input md:col-span-2"
            placeholder="Chủ sở hữu (tùy chọn)"
            value={form.owner}
            onChange={(event) => setForm({ ...form, owner: event.target.value })}
          />
          <Button disabled={saving} type="submit" variant="primary" className="md:col-span-1">
            <PlusIcon size={14} /> {saving ? "..." : "Thêm"}
          </Button>
        </form>
        {formStatus && (
          <div className="mt-3">
            <Notice tone={formStatus.tone} onDismiss={() => setFormStatus(null)}>
              {formStatus.text}
            </Notice>
          </div>
        )}
      </Panel>

      {error && <Notice tone="error">{error}</Notice>}

      {/* Asset List */}
      <Panel
        title={`Danh sách thiết bị (${filtered.length})`}
        actions={
          <div className="flex items-center gap-1 rounded-md border border-border bg-canvas p-0.5">
            {PLATFORM_TABS.map((tab) => (
              <button
                key={tab.key}
                type="button"
                onClick={() => setFilter(tab.key)}
                className={`cursor-pointer rounded px-2.5 py-1 text-xs font-medium transition-colors ${
                  filter === tab.key ? "bg-surface-raised text-ink" : "text-muted hover:text-ink"
                }`}
              >
                {tab.label}
              </button>
            ))}
          </div>
        }
        noPadding
      >
        {loading && !error ? (
          <div className="p-8 text-center font-mono text-xs text-faint">Đang tải danh sách tài sản...</div>
        ) : filtered.length === 0 ? (
          <EmptyState
            title="Chưa có tài sản nào phù hợp"
            description="Đăng ký máy chủ mới ở biểu mẫu phía trên trước khi gửi log."
            icon={<ServerIcon size={18} />}
          />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead>
                <tr className="border-b border-border bg-surface-raised/60 font-mono text-[11px] uppercase tracking-wider text-muted">
                  <th className="px-5 py-3">Hostname & IP</th>
                  <th className="px-4 py-3">Hệ điều hành</th>
                  <th className="px-4 py-3">Mức độ trọng yếu</th>
                  <th className="px-4 py-3">Chủ sở hữu</th>
                  <th className="px-5 py-3 text-right">Trạng thái</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {filtered.map((asset) => (
                  <tr key={asset.asset_id} className="transition-colors hover:bg-surface-hover/60">
                    <td className="px-5 py-3.5">
                      <div className="flex items-center gap-2.5">
                        <span className="flex h-7 w-7 items-center justify-center rounded-md border border-border bg-canvas text-muted">
                          {asset.os_type === "windows" ? <MonitorIcon size={14} /> : <ServerIcon size={14} />}
                        </span>
                        <div>
                          <p className="font-mono font-semibold text-ink">{asset.hostname}</p>
                          <p className="font-mono text-[11px] text-faint">{asset.ip_address || "—"}</p>
                        </div>
                      </div>
                    </td>
                    <td className="px-4 py-3.5 whitespace-nowrap">
                      <StatusPill tone="neutral">{asset.os_type.toUpperCase()}</StatusPill>
                    </td>
                    <td className="px-4 py-3.5 whitespace-nowrap">
                      <SeverityBadge severity={asset.criticality} />
                    </td>
                    <td className="px-4 py-3.5 text-ink-secondary">
                      <span>{asset.owner || "Chưa gán"}</span>
                      {asset.created_at && (
                        <span className="block font-mono text-[11px] text-faint">
                          Đăng ký: {new Date(asset.created_at).toLocaleDateString("vi-VN")}
                        </span>
                      )}
                    </td>
                    <td className="px-5 py-3.5 text-right whitespace-nowrap">
                      <StatusPill tone="success">
                        <span className="h-1.5 w-1.5 rounded-full bg-emerald-400" /> Active
                      </StatusPill>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Panel>
    </PageShell>
  );
}
