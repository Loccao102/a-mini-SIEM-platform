import assert from "node:assert/strict";
import {test} from "node:test";
import {normalizeOrigin,verifyPublicHealth,smoke} from "./vps-smoke.mjs";

test("production origin requires strict HTTPS host only",()=>{
  assert.equal(normalizeOrigin("https://siem.example.org"),"https://siem.example.org");
  for(const bad of ["http://siem.example.org","https://siem.example.org/api",
    "https://siem.example.org:8080","https://user:pass@siem.example.org","not-a-url"]){
    assert.throws(()=>normalizeOrigin(bad));
  }
});

test("public health validation rejects secrets and unready status",()=>{
  assert.deepEqual(verifyPublicHealth(200,{status:"healthy",uptime:"2m"}),{ok:true,reason:"Ready."});
  assert.equal(verifyPublicHealth(503,{status:"unhealthy"}).ok,false);
  assert.equal(verifyPublicHealth(200,{status:"healthy",services:{database:"secret"}}).ok,false);
  assert.equal(verifyPublicHealth(200,{status:"healthy",message:"postgres password"}).ok,false);
});

test("smoke verifies frontend, private diagnostics auth, and sanitized health",async()=>{
  const visited=[];
  const mock=async url=>{
    const path=new URL(url).pathname;visited.push(path);
    if(path==="/")return new Response("<html></html>",{status:200,headers:{"Content-Type":"text/html"}});
    if(path==="/healthz")return Response.json({status:"healthy",uptime:"12m"});
    if(path==="/healthz/redis")return Response.json({status:"healthy"});
    return Response.json({error:"unauthorized"},{status:401});
  };
  const result=await smoke("https://siem.example.org",mock);
  assert.equal(result.ok,true);
  assert.ok(visited.includes("/api/v1/system/health"));
  assert.ok(visited.includes("/metrics"));
});

test("smoke rejects proxy misrouting admin route into Next HTML",async()=>{
  const mock=async url=>new Response("<html></html>",{status:200,headers:{"Content-Type":"text/html"}});
  await assert.rejects(()=>smoke("https://siem.example.org",mock),/phải trả 401/);
});
