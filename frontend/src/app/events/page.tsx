"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { EventRecord, getEvents } from "@/lib/api";

export default function EventsPage() {
  const [events, setEvents] = useState<EventRecord[]>([]);
  const [query, setQuery] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    const refresh = () => getEvents().then((nextEvents) => { if (active) { setEvents(nextEvents); setError(""); } }).catch(() => { if (active) setError("Events could not be loaded. Sign in and try again."); });
    const timer = window.setTimeout(refresh, 0);
    const interval = window.setInterval(refresh, 3000);
    return () => { active = false; window.clearTimeout(timer); window.clearInterval(interval); };
  }, []);
  const visibleEvents = events.filter((event) => JSON.stringify(event).toLowerCase().includes(query.toLowerCase()));

  return <main className="account-shell"><header className="account-header"><Link href="/">&lt;- Overview</Link><span className="eyebrow">Signal history</span><h1>Log explorer</h1><p>Search normalized events across every connected source.</p></header><section className="data-page"><input className="search-input" aria-label="Search events" placeholder="Search message, hostname, source..." value={query} onChange={(event) => setQuery(event.target.value)} />{error && <p className="notice">{error}</p>}{!error && !visibleEvents.length && <p className="empty-state">No events found.</p>}{visibleEvents.map((event, index) => <article className="data-row event-row" key={event.event_id ?? index}><div><span className="eyebrow">{event.event_type ?? event.source_type ?? "event"}</span><h2>{event.message ?? "Unlabeled event"}</h2><small>{event.hostname ?? "Unknown host"} · {event.event_time ? new Date(event.event_time).toLocaleString() : "Unknown time"}</small></div><code>{event.severity ?? "-"}</code></article>)}</section></main>;
}