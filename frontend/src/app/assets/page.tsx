"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Asset, getAssets } from "@/lib/api";

export default function AssetsPage() {
  const [assets, setAssets] = useState<Asset[]>([]);
  const [error, setError] = useState("");
  useEffect(() => { getAssets().then(setAssets).catch(() => setError("Assets could not be loaded. Sign in and try again.")); }, []);
  return <main className="account-shell"><header className="account-header"><Link href="/">&lt;- Overview</Link><span className="eyebrow">Infrastructure</span><h1>Assets</h1><p>See which hosts are reporting and how critical they are to the environment.</p></header><section className="data-page">{error && <p className="notice">{error}</p>}{!error && !assets.length && <p className="empty-state">No assets have reported yet.</p>}{assets.map((asset) => <article className="data-row" key={asset.asset_id}><div><h2>{asset.hostname}</h2><small>{asset.ip_address} · {asset.os_type} · {asset.owner ?? "Unassigned"}</small></div><span className={`severity ${asset.criticality}`}>{asset.criticality}</span></article>)}</section></main>;
}
