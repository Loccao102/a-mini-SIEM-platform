export default function Home() {
  return (
    <main className="shell">
      <header className="topbar"><div><span className="mark">S</span><strong>Sentinel</strong><span className="muted"> / command center</span></div><span className="status"><span className="dot" /> All systems operational</span></header>
      <section className="content"><p className="eyebrow">Thursday, August 27, 2026</p><h1>Security overview</h1><p className="lede">A quiet view of the signals that need your attention.</p>
        <div className="metrics"><article><span>Open alerts</span><strong>24</strong><small className="red">+6 since yesterday</small></article><article><span>Events processed</span><strong>18,492</strong><small>Last 24 hours</small></article><article><span>Connected assets</span><strong>38</strong><small className="green">All reporting</small></article></div>
        <section className="panel"><div className="panel-heading"><div><p className="eyebrow">Priority queue</p><h2>Recent alerts</h2></div><button>View all alerts <span>→</span></button></div><div className="alert-row"><span className="severity critical">Critical</span><div><strong>Repeated failed SSH logins</strong><small>web-prod-01 · 4 minutes ago</small></div><span className="arrow">→</span></div><div className="alert-row"><span className="severity high">High</span><div><strong>PowerShell encoded command</strong><small>finance-ws-14 · 18 minutes ago</small></div><span className="arrow">→</span></div><div className="alert-row"><span className="severity medium">Medium</span><div><strong>Unusual outbound connection</strong><small>db-staging-02 · 31 minutes ago</small></div><span className="arrow">→</span></div></section>
      </section><nav className="rail"><span className="rail-active">Overview</span><span>Alerts</span><span>Log explorer</span><span>Rules</span><span>Assets</span><a href="/accounts">Accounts</a></nav>
    </main>
  );
}
