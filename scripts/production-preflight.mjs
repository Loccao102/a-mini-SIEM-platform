#!/usr/bin/env node
/**
 * Preflight for VPS deployment of the SIEM stack.
 * Reads .env without shell execution and never echoes secret values.
 * Run: node scripts/production-preflight.mjs --env .env.production
 */
import { readFileSync, statfsSync } from "node:fs";
import { totalmem } from "node:os";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

const GiB = 1024 ** 3;
const defaults = new Set([
  "change-me-now", "change-me-in-production", "admin", "password",
  "siem_dev_password", "replace_with_a_long_random_secret",
  "ci-only-admin-password", "ci-only-jwt-secret-never-use-in-production"
]);

export function parseEnv(text) {
  const values = {};
  for (const raw of text.split(/\r?\n/)) {
    const line = raw.trim();
    if (!line || line.startsWith("#")) continue;
    const match = line.match(/^(?:export\s+)?([A-Z][A-Z0-9_]*)\s*=\s*(.*)$/);
    if (!match) continue;
    let value = match[2].trim();
    if ((value.startsWith("'") && value.endsWith("'")) ||
        (value.startsWith('"') && value.endsWith('"'))) value = value.slice(1, -1);
    values[match[1]] = value;
  }
  return values;
}

export function checkProductionConfig(env) {
  const errors = [], warnings = [];
  for (const name of ["POSTGRES_PASSWORD", "ADMIN_PASSWORD", "JWT_SECRET"]) {
    const value = env[name] ?? "";
    const minimum = name === "JWT_SECRET" ? 32 : 16;
    if (value.length < minimum || defaults.has(value.toLowerCase())) {
      errors.push(name + ": yêu cầu secret riêng, dài tối thiểu " + minimum + " ký tự.");
    }
  }
  const adminEmail = env.ADMIN_EMAIL ?? "";
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(adminEmail)) errors.push("ADMIN_EMAIL: email admin chưa hợp lệ.");
  if ((env.MODE ?? "") !== "production") errors.push("MODE phải là production khi đưa lên VPS.");
  const domain=(env.SIEM_DOMAIN??"").toLowerCase();
  if(!/^(?=.{4,253}$)(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$/.test(domain) ||
    domain.endsWith(".local") || domain.endsWith(".localhost")){
    errors.push("SIEM_DOMAIN phải là hostname DNS public trỏ tới VPS (ví dụ siem.example.org).");
  }
  if(!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(env.ACME_EMAIL??"")){
    errors.push("ACME_EMAIL phải là email hợp lệ để Caddy đăng ký chứng chỉ TLS.");
  }
  const apiUrl=env.NEXT_PUBLIC_API_URL??"";
  try{
    const url=new URL(apiUrl);
    if(url.protocol!=="https:"||url.username||url.password||url.port||
       url.pathname!=="/"||url.search||url.hash||url.hostname!==domain||url.origin!==apiUrl){
      errors.push("NEXT_PUBLIC_API_URL phải là https://SIEM_DOMAIN, KHÔNG thêm /api, port hoặc dấu / cuối.");
    }
  }catch{
    errors.push("NEXT_PUBLIC_API_URL phải là URL HTTPS hợp lệ trùng SIEM_DOMAIN.");
  }
  // Docker Compose interpolates POSTGRES_PASSWORD inside a PostgreSQL URI.
  // Restrict to URL-unreserved characters rather than misreading '@', '/', '?'.
  if(env.POSTGRES_PASSWORD&&!/^[A-Za-z0-9._~-]+$/.test(env.POSTGRES_PASSWORD)){
    errors.push("POSTGRES_PASSWORD chỉ dùng ký tự URL-safe (A-Z,a-z,0-9,._~-) vì chuỗi kết nối Compose chưa URL-encode.");
  }
  const bindNames = ["POSTGRES_BIND_HOST","REDIS_BIND_HOST","ELASTICSEARCH_BIND_HOST"];
  for (const name of bindNames) {
    if (env[name] && !["127.0.0.1","::1"].includes(env[name])) {
      errors.push(name + " phải chỉ bind loopback; không đưa DB/Redis/Elasticsearch trực tiếp ra Internet.");
    }
  }
  for(const name of ["API_BIND_HOST","FRONTEND_BIND_HOST","BEATS_BIND_HOST","FLEET_BIND_HOST"]){
    if(env[name]&&!["127.0.0.1","::1"].includes(env[name])){
      errors.push(name+": dịch vụ phải chỉ bind loopback; traffic bên ngoài qua Caddy HTTPS.");
    }
  }
  if (!env.TELEGRAM_BOT_TOKEN) warnings.push("Không cấu hình Telegram; cảnh báo qua Telegram sẽ không được gửi.");
  return { errors, warnings };
}

export function checkHostResources(memoryBytes, freeDiskBytes) {
  const errors = [], warnings = [];
  if (memoryBytes < 3 * GiB) errors.push("Máy có dưới 3 GiB RAM; Elasticsearch và frontend dễ bị OOM.");
  else if (memoryBytes < 4 * GiB) warnings.push("RAM dưới 4 GiB: nên giám sát swap và JVM heap.");
  if (freeDiskBytes < 8 * GiB) errors.push("Ổ đĩa trống dưới 8 GiB; cần thêm chỗ cho index và Docker images.");
  else if (freeDiskBytes < 20 * GiB) warnings.push("Ổ đĩa trống dưới 20 GiB; nên tăng dung lượng cho logs/index.");
  return { errors, warnings };
}

function run() {
  const index = process.argv.indexOf("--env");
  const file = index >= 0 ? process.argv[index + 1] : ".env";
  if (!file || file.startsWith("--")) throw new Error("Dùng --env <path> để chọn file secrets.");
  let envContent;
  try { envContent = readFileSync(resolve(file), "utf8"); }
  catch { throw new Error("Không đọc được file cấu hình: " + file); }
  const env = { ...parseEnv(envContent) };
  const stats = statfsSync(process.cwd());
  const resources = checkHostResources(totalmem(), Number(stats.bavail) * Number(stats.bsize));
  const config = checkProductionConfig(env);
  console.log("SIEM production readiness preflight");
  for (const warning of [...config.warnings, ...resources.warnings]) console.warn("WARN: " + warning);
  for (const error of [...config.errors, ...resources.errors]) console.error("FAIL: " + error);
  const count = config.errors.length + resources.errors.length;
  if (count) {
    console.error("Chưa sẵn sàng triển khai (" + count + " vấn đề). Secret không được in ra màn hình.");
    process.exitCode = 1;
  } else console.log("PASS: cấu hình và tài nguyên cơ bản đạt ngưỡng. Vẫn phải kiểm tra TLS, firewall, backup và health thực tế.");
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  try { run(); } catch (error) { console.error(error instanceof Error ? error.message : error); process.exitCode = 1; }
}
