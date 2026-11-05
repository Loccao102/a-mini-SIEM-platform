import assert from "node:assert/strict";
import {test} from "node:test";
import {makeBackupName,snapshotEndpoint} from "./vps-backup.mjs";

test("VPS backup names are stable, timestamp based and safe for directories",()=>{
  assert.equal(makeBackupName(new Date("2026-11-05T14:05:06.000Z")),"siem-20261105-140506");
  assert.throws(()=>makeBackupName(new Date("not a date")));
});

test("snapshot endpoints are always localhost and validate ports",()=>{
  assert.equal(snapshotEndpoint(9200),"http://127.0.0.1:9200");
  assert.equal(snapshotEndpoint("19200"),"http://127.0.0.1:19200");
  for(const value of [0,-1,"bad",65536,NaN])assert.throws(()=>snapshotEndpoint(value));
});
