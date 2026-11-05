#!/usr/bin/env node
/** Explicit, non-destructive VPS release helper: --apply is required. */
import {existsSync,readFileSync,statSync} from "node:fs";
import {spawnSync} from "node:child_process";
import {resolve} from "node:path";
import {pathToFileURL} from "node:url";
import {parseEnv,checkProductionConfig} from "./production-preflight.mjs";

export function projectName(raw="siem-prod"){
  if(!/^siem-[a-z0-9][a-z0-9-]{1,38}$/.test(raw))
    throw new Error("Tên Docker project phải bắt đầu siem- và chỉ gồm a-z, số hoặc dấu gạch nối.");
  return raw;
}

export function releasePlan(envFile,project){
  const prefix=["compose","--env-file",envFile,"--profile","production","-p",project];
  return [
    {title:"Kiểm tra preflight",command:"node",args:["scripts/production-preflight.mjs","--env",envFile]},
    {title:"Kiểm tra cấu hình Compose",command:"docker",args:[...prefix,"config","--quiet"]},
    {title:"Kiểm tra Caddyfile",command:"docker",args:[...prefix,"run","--rm","--no-deps","caddy","caddy","validate","--config","/etc/caddy/Caddyfile","--adapter","caddyfile"]},
    {title:"Dựng và khởi động dịch vụ",command:"docker",args:[...prefix,"up","-d","--build","--wait","--wait-timeout","360","postgres","redis","elasticsearch","backend","ingest","frontend","caddy"]},
  ];
}

function command(step){
  const result=spawnSync(step.command,step.args,{stdio:"inherit",shell:false});
  if(result.error)throw result.error;
  if(result.status!==0)throw new Error(step.title+" thất bại (exit "+result.status+").");
}

async function main(){
  const args=process.argv.slice(2);
  const envIndex=args.indexOf("--env");
  const projectIndex=args.indexOf("--project");
  const envFile=envIndex>=0?args[envIndex+1]:".env.production";
  const project=projectName(projectIndex>=0?args[projectIndex+1]:"siem-prod");
  if(!envFile||envFile.startsWith("--"))throw new Error("Thiếu đường dẫn --env.");
  const apply=args.includes("--apply");
  const plan=releasePlan(envFile,project);
  if(!apply){
    console.log("DRY RUN: Chưa sửa VPS hay khởi động container.");
    console.log("Project: "+project+" | file: "+envFile);
    plan.forEach(step=>console.log(step.title+": "+step.command+" "+step.args.join(" ")));
    console.log("Sau khi kiểm tra DNS/firewall và backup, chạy lại kèm --apply.");
    return;
  }
  if(process.platform!=="linux")throw new Error("Chỉ dùng --apply trên VPS Linux.");
  const file=resolve(envFile);
  if(!existsSync(file))throw new Error("Không thấy file secrets .env.production.");
  if((statSync(file).mode&0o077)!==0)
    throw new Error("File secrets đang đọc được bởi group/others. Chạy chmod 600 "+envFile);
  const config=checkProductionConfig(parseEnv(readFileSync(file,"utf8")));
  if(config.errors.length){
    config.errors.forEach(error=>console.error("FAIL: "+error));
    throw new Error("Chưa đủ điều kiện cấu hình production.");
  }
  console.log("Dự án Compose: "+project);
  console.log("Các volume có prefix của project này. Nếu từng chạy với project khác, phải di chuyển dữ liệu trước.");
  for(const step of plan){console.log("==> "+step.title);command(step);}
  const env=parseEnv(readFileSync(file,"utf8"));
  console.log("==> Kiểm tra HTTPS public và phân quyền");
  command({title:"HTTPS smoke test",command:"node",args:["scripts/vps-smoke.mjs","--origin",env.NEXT_PUBLIC_API_URL]});
  console.log("PASS: triển khai SIEM hoàn tất. Không chạy docker compose down -v trên production.");
}

if(process.argv[1]&&import.meta.url===pathToFileURL(resolve(process.argv[1])).href){
  main().catch(error=>{console.error("FAIL: "+(error instanceof Error?error.message:"Release failed"));process.exitCode=1;});
}
