#!/usr/bin/env node
/** On-host backup. Exports Postgres and a complete Elasticsearch snapshot repository. */
import {spawnSync} from "node:child_process";
import {createHash} from "node:crypto";
import {createReadStream} from "node:fs";
import {statSync,readFileSync,openSync,closeSync,mkdirSync,writeFileSync} from "node:fs";
import {homedir} from "node:os";
import {join,resolve} from "node:path";
import {pathToFileURL} from "node:url";
import {parseEnv,checkProductionConfig} from "./production-preflight.mjs";
import {projectName} from "./vps-release.mjs";

export function makeBackupName(date=new Date()){
  if(Number.isNaN(date.getTime()))throw new Error("Backup date không hợp lệ.");
  return "siem-"+date.toISOString().replace(/[-:]/g,"").replace(/\..*$/,"").replace("T","-");
}

export function snapshotEndpoint(port){
  const value=Number(port);
  if(!Number.isInteger(value)||value<1||value>65535)throw new Error("ELASTICSEARCH_PORT không hợp lệ.");
  return "http://127.0.0.1:"+value;
}

async function checksum(file){
  const hash=createHash("sha256");
  for await(const chunk of createReadStream(file))hash.update(chunk);
  return hash.digest("hex");
}

function command(args,stdio="inherit"){
  const result=spawnSync("docker",args,{stdio,shell:false,maxBuffer:1024*1024});
  if(result.error||result.status!==0)throw new Error("Docker backup failed: "+
    (result.error?.message||"exit "+result.status));
}

async function callES(base,method,path,body){
  const r=await fetch(base+path,{method,headers:{"Content-Type":"application/json"},
    body:body?JSON.stringify(body):undefined,signal:AbortSignal.timeout(600000)});
  let payload;
  try{payload=await r.json();}catch{throw new Error("Elasticsearch trả nội dung không hợp lệ.");}
  if(!r.ok||payload.error)throw new Error("Elasticsearch snapshot request thất bại (HTTP "+r.status+").");
  return payload;
}

async function main(){
  const args=process.argv.slice(2);
  const idx=args.indexOf("--env"),pidx=args.indexOf("--project"),oidx=args.indexOf("--output");
  const envFile=idx>=0?args[idx+1]:".env.production";
  const project=projectName(pidx>=0?args[pidx+1]:"siem-prod");
  const output=resolve(oidx>=0?args[oidx+1]:join(homedir(),"siem-backups"));
  if(!envFile||!project||args.some(a=>a===undefined))throw new Error("Tham số thiếu giá trị.");
  if(!args.includes("--apply")){
    console.log("DRY RUN: Chưa tạo bản sao lưu. Thêm --apply để xuất PostgreSQL và Elasticsearch.");
    console.log("Project: "+project+" | output: "+output);return;
  }
  if(process.platform!=="linux")throw new Error("Backup chỉ chạy trên VPS Linux.");
  const file=resolve(envFile);
  if((statSync(file).mode&0o077)!==0)throw new Error("File .env.production cần chmod 600.");
  const config=parseEnv(readFileSync(file,"utf8"));
  const validation=checkProductionConfig(config);
  if(validation.errors.length)throw new Error("Cấu hình VPS không qua production preflight.");
  const prefix=["compose","--env-file",envFile,"--profile","production","-p",project];
  mkdirSync(output,{recursive:true,mode:0o700});
  if((statSync(output).mode&0o077)!==0)throw new Error("Thư mục backup cần quyền chmod 700.");
  const name=makeBackupName();
  const target=join(output,name);
  mkdirSync(target,{mode:0o700});
  const dump=join(target,"postgres.dump");
  const postgresUser=config.POSTGRES_USER||"siem",postgresDb=config.POSTGRES_DB||"siem";
  console.log("==> Sao lưu PostgreSQL bằng pg_dump custom format");
  const fd=openSync(dump,"wx",0o600);
  try{command([...prefix,"exec","-T","postgres","pg_dump","-U",postgresUser,"-d",postgresDb,"-Fc"],["ignore",fd,"pipe"]);}
  finally{closeSync(fd);}
  if(statSync(dump).size===0)throw new Error("PostgreSQL dump rỗng.");
  const pgHash=await checksum(dump);
  writeFileSync(dump+".sha256",pgHash+"  postgres.dump\n",{mode:0o600,flag:"wx"});
  console.log("==> Snapshot Elasticsearch (sau đó xuất repository ra khỏi Docker volume)");
  const base=snapshotEndpoint(config.ELASTICSEARCH_PORT||9200);
  const repoName="siem_vps_local";
  const repo=await callES(base,"PUT","/_snapshot/"+repoName,{type:"fs",settings:{location:"siem-vps-local"}});
  if(repo.acknowledged!==true)throw new Error("Elasticsearch không xác nhận repository.");
  const snap=name.toLowerCase().replace(/[^a-z0-9-]/g,"-");
  const created=await callES(base,"PUT","/_snapshot/"+repoName+"/"+snap+"?wait_for_completion=true",{
    indices:"normalized_events,siem-events-*",ignore_unavailable:true,include_global_state:false
  });
  if(created.snapshot?.state!=="SUCCESS")throw new Error("Snapshot Elasticsearch chưa thành công.");
  const directory=join(target,"elasticsearch-repository");
  command([...prefix,"cp","elasticsearch:/snapshots/siem-vps-local",directory]);
  const report={version:1,time:new Date().toISOString(),project,postgres:{file:"postgres.dump",sha256:pgHash,bytes:statSync(dump).size},
    elasticsearch:{snapshotRepository:repoName,snapshot:snap,folder:"elasticsearch-repository",state:"SUCCESS"},
    note:"Keep the entire exported Elasticsearch repository directory for restore."};
  writeFileSync(join(target,"manifest.json"),JSON.stringify(report,null,2)+"\n",{mode:0o600,flag:"wx"});
  console.log("PASS: backup PostgreSQL và Elasticsearch đã xuất ra "+target);
  console.log("Hãy sao chép thư mục backup này ra nơi lưu trữ riêng, kiểm tra file dump và thử restore trên staging.");
}

if(process.argv[1]&&import.meta.url===pathToFileURL(resolve(process.argv[1])).href){
  main().catch(e=>{console.error("FAIL: "+(e instanceof Error?e.message:"Backup failed"));process.exitCode=1;});
}
