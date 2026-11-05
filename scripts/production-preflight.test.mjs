import assert from "node:assert/strict";
import { test } from "node:test";
import { parseEnv, checkProductionConfig, checkHostResources } from "./production-preflight.mjs";

const valid = {
  MODE: "production", POSTGRES_PASSWORD: "very-long-unique-database-secret",
  ADMIN_EMAIL: "security@example.org", ADMIN_PASSWORD: "nondefault-admin-passphrase",
  JWT_SECRET: "64-character-random-token-do-not-reuse-this-value-in-production",
  NEXT_PUBLIC_API_URL: "https://siem.example.org",
  SIEM_DOMAIN: "siem.example.org", ACME_EMAIL: "ops@example.org",
  POSTGRES_BIND_HOST: "127.0.0.1",
  REDIS_BIND_HOST: "127.0.0.1",
  ELASTICSEARCH_BIND_HOST: "127.0.0.1",
};

test("parseEnv ignores comments and does not evaluate shell code", () => {
  const parsed = parseEnv('# comment\nexport MODE="production"\nJWT_SECRET=$(rm -rf /)\n');
  assert.equal(parsed.MODE, "production");
  assert.equal(parsed.JWT_SECRET, "$(rm -rf /)");
});

test("strong production config passes without mandatory integrations", () => {
  const result = checkProductionConfig(valid);
  assert.deepEqual(result.errors, []);
  assert.ok(result.warnings.some(w => w.includes("Telegram")));
});

test("rejects defaults, insecure bind addresses and HTTP API origins", () => {
  const result = checkProductionConfig({
    ...valid, ADMIN_PASSWORD: "admin", JWT_SECRET: "change-me-in-production",
    POSTGRES_PASSWORD: "siem_dev_password",
    REDIS_BIND_HOST: "0.0.0.0",
    NEXT_PUBLIC_API_URL: "http://127.0.0.1:8080"
  });
  assert.ok(result.errors.length >= 5);
  assert.ok(result.errors.some(e => e.includes("REDIS_BIND_HOST")));
});

test("resource thresholds are explicit and deterministic", () => {
  assert.equal(checkHostResources(2 * 1024**3, 5 * 1024**3).errors.length,2);
  assert.deepEqual(checkHostResources(8 * 1024**3, 40 * 1024**3).errors,[]);
});


test("rejects duplicated /api routing, domain mismatch and bad TLS setup",()=>{
  for(const bad of [
    {NEXT_PUBLIC_API_URL:"https://siem.example.org/api"},
    {NEXT_PUBLIC_API_URL:"https://other.example.org"},
    {NEXT_PUBLIC_API_URL:"https://siem.example.org:8080"},
    {SIEM_DOMAIN:"localhost"},
    {ACME_EMAIL:"not-an-email"}
  ]){
    const result=checkProductionConfig({...valid,...bad});
    assert.ok(result.errors.length>0,JSON.stringify(bad));
  }
});

test("rejects characters requiring percent encoding inside PostgreSQL URL",()=>{
  const bad=checkProductionConfig({...valid,POSTGRES_PASSWORD:"abcd1234567890@/secret"});
  assert.ok(bad.errors.some(line=>line.includes("POSTGRES_PASSWORD")&&line.includes("URL-safe")));
  const okay=checkProductionConfig({...valid,POSTGRES_PASSWORD:"A".repeat(20)+"-._~"});
  assert.deepEqual(okay.errors,[]);
});

test("rejects exposing API/frontend/agents on non-loopback addresses",()=>{
  const bad=checkProductionConfig({
    ...valid,API_BIND_HOST:"0.0.0.0",FRONTEND_BIND_HOST:"::",
    BEATS_BIND_HOST:"10.1.2.3",FLEET_BIND_HOST:"192.168.0.1"
  });
  for(const field of ["API_BIND_HOST","FRONTEND_BIND_HOST","BEATS_BIND_HOST","FLEET_BIND_HOST"]){
    assert.ok(bad.errors.some(e=>e.includes(field)));
  }
});
