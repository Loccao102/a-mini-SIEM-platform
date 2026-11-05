"use client";

import {useCallback,useEffect,useRef,useState} from "react";
import {ApiError,getSystemHealth,type SystemHealth,type SystemServiceHealth} from "@/lib/api";

const SERVICE_LABELS:Record<string,string>={
  postgres:"PostgreSQL",redis:"Redis",elasticsearch:"Elasticsearch",
  database:"Database schema",queues:"Ingest & retry queues",connectors:"Ingest / parser"
};
const PRIORITY=["postgres","redis","elasticsearch","database","queues","connectors"];
function displayService(name:string){return SERVICE_LABELS[name]||name;}
function formatChecked(value:string){
  const date=new Date(value);
  return Number.isNaN(date.getTime())?"—":
    new Intl.DateTimeFormat("vi-VN",{dateStyle:"short",timeStyle:"medium"}).format(date);
}
function ServiceCard({name,item}:{name:string;item:SystemServiceHealth}){
  return <article className="system-service-card">
    <div className="system-service-heading">
      <div><small>SUBSYSTEM</small><h3>{displayService(name)}</h3></div>
      <span className={"system-status system-status-"+item.status}>{item.status}</span>
    </div>
    <p>{item.message}</p>
    <div className="system-response">Response <strong>{Number.isFinite(item.response_time_ms)?item.response_time_ms+" ms":"—"}</strong></div>
    {Object.keys(item.checks||{}).length>0&&<dl className="system-checks">
      {Object.entries(item.checks).sort(([a],[b])=>a.localeCompare(b)).map(([key,value])=>
        <div key={key}><dt>{key.replace(/_/g," ")}</dt><dd>{value}</dd></div>)}
    </dl>}
  </article>;
}

export default function SystemPage(){
  const [snapshot,setSnapshot]=useState<SystemHealth|null>(null);
  const [busy,setBusy]=useState(true);
  const [error,setError]=useState("");
  const running=useRef(false);
  const active=useRef(false);

  const refresh=useCallback(async()=>{
    if(running.current)return;
    running.current=true;
    if(active.current)setBusy(true);
    try{
      const result=await getSystemHealth();
      if(active.current){setSnapshot(result);setError("");}
    }catch(reason){
      if(active.current){
        setError(reason instanceof ApiError&&reason.status===403?
          "Chỉ tài khoản SOC Admin được xem chẩn đoán hệ thống.":
          reason instanceof ApiError&&reason.status===401?
          "Phiên đăng nhập đã hết hạn. Vui lòng đăng nhập lại.":
          "Không đọc được trạng thái các dịch vụ. Kiểm tra kết nối API và quyền tài khoản.");
      }
    }finally{
      running.current=false;
      if(active.current)setBusy(false);
    }
  },[]);

  useEffect(()=>{
    active.current=true;
    void refresh();
    const interval=window.setInterval(()=>{
      if(document.visibilityState==="visible")void refresh();
    },15000);
    return ()=>{active.current=false;window.clearInterval(interval);};
  },[refresh]);

  const services=snapshot?.services||{};
  const ordered=[...new Set([...PRIORITY,...Object.keys(services)])].filter(key=>Boolean(services[key]));
  return <main className="shell system-shell">
    <section className="content">
      <p className="eyebrow">PLATFORM OPERATIONS / ADMIN ONLY</p>
      <div className="system-page-heading">
        <div><h1>System Health</h1>
          <p>Chẩn đoán trực tiếp các dịch vụ cốt lõi, luồng ingest và độ trễ kiểm tra.</p></div>
        <button type="button" className="system-refresh" onClick={()=>void refresh()} disabled={busy}>
          {busy?"Đang kiểm tra…":"Kiểm tra lại"}
        </button>
      </div>
      <section className="system-summary">
        <div><span>Overall state</span><strong className={"system-overall-"+(snapshot?.status||"unknown")}>{snapshot?.status||"Chưa có dữ liệu"}</strong></div>
        <div><span>API uptime</span><strong>{snapshot?.uptime||"—"}</strong></div>
        <div><span>Checked at</span><strong>{snapshot?formatChecked(snapshot.checked_at):"—"}</strong></div>
        <div><span>Systems reported</span><strong>{ordered.length}</strong></div>
      </section>
      {error&&<div className="system-error" role="alert">{error}</div>}
      {ordered.length?<section className="system-cards" aria-label="Trạng thái chi tiết của các dịch vụ">
        {ordered.map(name=><ServiceCard key={name} name={name} item={services[name]}/>)}
      </section>:<div className="system-empty">
        {busy?"Đang kiểm tra trạng thái các thành phần…":"Chưa có dữ liệu từ backend. Chọn Kiểm tra lại để thử lần nữa."}
      </div>}
      <p className="system-footnote">Tự cập nhật mỗi 15 giây khi trang đang mở. Chỉ admin xem được thông tin chi tiết; các endpoint health public chỉ trả trạng thái tổng quát. Khi dịch vụ gặp sự cố, thông số hiển thị là kết quả kiểm tra gần nhất.</p>
    </section>
  </main>;
}
