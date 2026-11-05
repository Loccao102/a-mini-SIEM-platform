import assert from "node:assert/strict";
import {test} from "node:test";
import {projectName,releasePlan} from "./vps-release.mjs";

test("release project namespace must be explicit and safe",()=>{
  assert.equal(projectName(),"siem-prod");
  assert.equal(projectName("siem-stage"),"siem-stage");
  for(const value of ["default","siem-prod;rm -rf /","SIEM-PROD","siem-/tmp","siem-"]){
    assert.throws(()=>projectName(value));
  }
});

test("release plan is confined to a stable Compose project, never deletes volumes",()=>{
  const steps=releasePlan(".env.production","siem-prod");
  assert.equal(steps.length,4);
  for(const step of steps){
    const all=step.args.join(" ");
    assert.ok(!all.includes("down"));
    assert.ok(!all.includes("-v "));
    assert.ok(!all.includes("docker system prune"));
  }
  const last=steps[3];
  assert.ok(last.args.includes("--profile"));
  assert.ok(last.args.includes("production"));
  assert.ok(last.args.includes("--wait"));
  assert.ok(last.args.includes("caddy"));
});
