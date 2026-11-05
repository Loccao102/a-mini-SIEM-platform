#!/usr/bin/env node
/** Read-only public smoke test for a deployed SIEM origin. Never logs credentials. */
import {pathToFileURL} from "node:url";
import {resolve} from "node:path";

export function normalizeOrigin(value){
  let url;
  try{url=new URL(value);}catch{throw new Error("Cần URL HTTPS hợp lệ.");}
  if(url.protocol!=="https:"||url.username||url.password||url.port||
     url.pathname!=="/"||url.search||url.hash)
    throw new Error("Origin phải là https://hostname, không kèm path, port hoặc credentials.");
  return url.origin;
}

export function verifyPublicHealth(status,payload){
  if(!payload||typeof payload!=="object"||typeof payload.status!=="string")
    return {ok:false,reason:"Health response không có status."};
  const exposed=["services","checks","message","postgres_url"].filter(key=>key in payload);
  if(exposed.length)return {ok:false,reason:"Public health lộ trường nội bộ."};
  return status===200&&payload.status==="healthy"
    ?{ok:true,reason:"Ready."}
    :{ok:false,reason:"Health chưa healthy (HTTP "+status+", trạng thái "+payload.status+")."};
}

export async function smoke(origin,fetcher=fetch){
  const root=normalizeOrigin(origin);
  const timed=(path,options={})=>fetcher(root+path,{redirect:"follow",
    signal:AbortSignal.timeout(12000),cache:"no-store",...options});
  const frontend=await timed("/");
  if(!frontend.ok||!(frontend.headers.get("content-type")||"").includes("text/html"))
    throw new Error("Frontend không trả HTML thành công (HTTP "+frontend.status+").");
  const protectedPaths=["/api/v1/system/health","/metrics"];
  for(const path of protectedPaths){
    const response=await timed(path);
    if(response.status!==401)throw new Error(path+" phải trả 401 khi không đăng nhập, nhận HTTP "+response.status);
  }
  const response=await timed("/healthz");
  let body;
  try{body=await response.json();}catch{throw new Error("API /healthz không trả JSON.");}
  const health=verifyPublicHealth(response.status,body);
  if(!health.ok)throw new Error(health.reason);
  const redis=await timed("/healthz/redis");
  let part;
  try{part=await redis.json();}catch{throw new Error("Redis health không trả JSON.");}
  if(part?.status!=="healthy"||Object.keys(part).some(key=>key!=="status"))
    throw new Error("Redis health không healthy hoặc đang lộ chi tiết.");
  return {ok:true,status:body.status};
}

async function main(){
  const index=process.argv.indexOf("--origin");
  if(index<0||!process.argv[index+1])throw new Error("Dùng --origin https://siem.example.org");
  const origin=normalizeOrigin(process.argv[index+1]);
  const deadline=Date.now()+120000;
  while(true){
    try{const result=await smoke(origin);console.log("PASS: SIEM HTTPS, frontend, auth và public health đều đạt.",result.status);return;}
    catch(error){
      if(Date.now()>deadline)throw error;
      console.warn("Chưa ready, thử lại: "+(error instanceof Error?error.message:"Không rõ lỗi"));
      await new Promise(done=>setTimeout(done,4000));
    }
  }
}

if(process.argv[1]&&import.meta.url===pathToFileURL(resolve(process.argv[1])).href){
  main().catch(error=>{console.error("FAIL: "+(error instanceof Error?error.message:"Smoke test failed"));process.exitCode=1;});
}
