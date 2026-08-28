"use client";

import { useEffect, useState } from "react";
import { Asset, ApiError, getAssets } from "@/lib/api";

function criticality_color(c: string) {
  if (c === "critical") return "var(--coral)";
  if (c === "high") return "var(--amber)";
  if (c === "medium") return "var(--aqua)";
  return "var(--acid)";
}

function osIcon(os: string) {
  if (os === "windows") return "🪟";
  if (os === "linux") return "🐧";
  return "💻";
}

export default function AssetsPage() {
  const [assets, setAssets] = useState<Asset[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<"all" | "linux" | "windows">("all");

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
          setError("Không thể tải danh sách assets. Kiểm tra kết nối Backend.");
        }
        setLoading(false);
      });
  }, []);

  const filtered = assets.filter((a) => filter === "all" || a.os_type === filter);

  const critCounts = {
    critical: assets.filter((a) => a.criticality === "critical").length,
    high: assets.filter((a) => a.criticality === "high").length,
    medium: assets.filter((a) => a.criticality === "medium").length,
    low: assets.filter((a) => a.criticality === "low").length,
  };

  return (
    <main className="account-shell">
      <header className="account-header">
        <span className="eyebrow">Infrastructure Inventory</span>
        <h1>Asset Management</h1>
        <p>
          Theo dõi toàn bộ máy chủ và thiết bị đang gửi log về SIEM, phân loại mức độ nghiêm trọng và trạng thái hoạt động.
        </p>
      </header>

      <section className="data-page">
        {/* Summary Cards */}
        <div className="metrics" style={{ marginBottom: "2rem" }}>
          <article>
            <span>Total Assets</span>
            <strong style={{ color: "var(--acid)" }}>{loading ? "-" : assets.length}</strong>
            <small>Registered Nodes</small>
          </article>
          <article>
            <span>Critical</span>
            <strong style={{ color: "var(--coral)" }}>{loading ? "-" : critCounts.critical}</strong>
            <small className="red">Highest Risk</small>
          </article>
          <article>
            <span>High</span>
            <strong style={{ color: "var(--amber)" }}>{loading ? "-" : critCounts.high}</strong>
            <small>Elevated Risk</small>
          </article>
          <article>
            <span>Medium / Low</span>
            <strong style={{ color: "var(--aqua)" }}>{loading ? "-" : critCounts.medium + critCounts.low}</strong>
            <small className="green">Monitored</small>
          </article>
        </div>

        {/* Filter tabs */}
        <div style={{ display: "flex", gap: "0.75rem", marginBottom: "1.5rem" }}>
          {(["all", "linux", "windows"] as const).map((f) => (
            <button
              key={f}
              onClick={() => setFilter(f)}
              style={{
                padding: "0.4rem 1rem",
                borderRadius: "4px",
                border: `1px solid ${filter === f ? "var(--acid)" : "var(--line)"}`,
                background: filter === f ? "var(--acid)" : "transparent",
                color: filter === f ? "var(--canvas)" : "var(--muted)",
                fontFamily: "var(--font-mono)",
                fontSize: "0.75rem",
                cursor: "pointer",
                textTransform: "uppercase",
                letterSpacing: "0.05em",
              }}
            >
              {f === "all" ? "All Platforms" : f === "linux" ? "🐧 Linux" : "🪟 Windows"}
            </button>
          ))}
        </div>

        {/* Error state */}
        {error && (
          <div
            style={{
              padding: "1rem 1.25rem",
              background: "rgba(255,80,80,0.08)",
              border: "1px solid var(--coral)",
              borderRadius: "6px",
              color: "var(--coral)",
              fontFamily: "var(--font-mono)",
              fontSize: "0.85rem",
              marginBottom: "1.5rem",
            }}
          >
            ⚠️ {error}
          </div>
        )}

        {/* Loading state */}
        {loading && !error && (
          <p className="empty-state">Loading assets...</p>
        )}

        {/* Empty state */}
        {!loading && !error && filtered.length === 0 && (
          <p className="empty-state">No assets found. Assets register automatically when agents send their first log.</p>
        )}

        {/* Asset Grid */}
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "repeat(auto-fill, minmax(320px, 1fr))",
            gap: "1rem",
          }}
        >
          {filtered.map((asset) => (
            <article
              key={asset.asset_id}
              style={{
                padding: "1.25rem",
                border: "1px solid var(--line)",
                borderLeft: `3px solid ${criticality_color(asset.criticality)}`,
                background: "var(--surface)",
                borderRadius: "6px",
                display: "flex",
                flexDirection: "column",
                gap: "0.5rem",
              }}
            >
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
                <div style={{ display: "flex", alignItems: "center", gap: "0.6rem" }}>
                  <span style={{ fontSize: "1.4rem" }}>{osIcon(asset.os_type)}</span>
                  <div>
                    <div
                      style={{
                        fontFamily: "var(--font-mono)",
                        fontWeight: 700,
                        color: "var(--ink)",
                        fontSize: "0.95rem",
                      }}
                    >
                      {asset.hostname}
                    </div>
                    <div
                      style={{
                        fontFamily: "var(--font-mono)",
                        fontSize: "0.75rem",
                        color: "var(--muted)",
                      }}
                    >
                      {asset.ip_address}
                    </div>
                  </div>
                </div>
                <span
                  className={`severity ${asset.criticality}`}
                  style={{ flexShrink: 0 }}
                >
                  {asset.criticality}
                </span>
              </div>

              <div
                style={{
                  display: "flex",
                  gap: "0.5rem",
                  flexWrap: "wrap",
                  marginTop: "0.25rem",
                }}
              >
                <span
                  style={{
                    padding: "0.2rem 0.6rem",
                    background: "var(--canvas)",
                    border: "1px solid var(--line)",
                    borderRadius: "4px",
                    fontFamily: "var(--font-mono)",
                    fontSize: "0.7rem",
                    color: "var(--muted)",
                    textTransform: "uppercase",
                  }}
                >
                  {asset.os_type}
                </span>
                <span
                  style={{
                    padding: "0.2rem 0.6rem",
                    background: "var(--canvas)",
                    border: "1px solid var(--line)",
                    borderRadius: "4px",
                    fontFamily: "var(--font-mono)",
                    fontSize: "0.7rem",
                    color: "var(--acid)",
                  }}
                >
                  ● Active
                </span>
              </div>

              <div
                style={{
                  fontFamily: "var(--font-mono)",
                  fontSize: "0.75rem",
                  color: "var(--muted)",
                  borderTop: "1px solid var(--line)",
                  paddingTop: "0.5rem",
                  marginTop: "0.25rem",
                }}
              >
                Owner: <span style={{ color: "var(--ink)" }}>{asset.owner ?? "Unassigned"}</span>
                {asset.created_at && (
                  <> · Registered: {new Date(asset.created_at).toLocaleDateString()}</>
                )}
              </div>
            </article>
          ))}
        </div>
      </section>
    </main>
  );
}
