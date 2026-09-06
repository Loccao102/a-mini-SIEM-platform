// Run only on an isolated stack: node scripts/restore-drill.mjs siem-integration DIR
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { mkdirSync, openSync, closeSync, readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
const [project,dir='work/restore-drill']=process.argv.slice(2);
if(!/^siem-(ci|integration)[a-z0-9-]*$/.test(project??''))throw Error('Requires an isolated siem-ci* or siem-integration* project');
const target=resolve(dir);mkdirSync(target,{recursive:true});const file=resolve(target,'postgres.dump');
const base=['compose','-p',project,'exec','-T','postgres'];
function run(args,options={}){const r=spawnSync('docker',[...base,...args],{encoding:'utf8',...options});if(r.error||r.status!==0)throw Error(r.error?.message||String(r.stderr)||`docker exit ${r.status}`);return r.stdout?.trim()}
const dump=openSync(file,'w');try{run(['pg_dump','-U','siem','-d','siem','-Fc'],{stdio:['ignore',dump,'pipe']})}finally{closeSync(dump)}
const checksum=createHash('sha256').update(readFileSync(file)).digest('hex');writeFileSync(file+'.sha256',checksum+'\n');
if(createHash('sha256').update(readFileSync(file)).digest('hex')!==readFileSync(file+'.sha256','utf8').trim())throw Error('Checksum mismatch');
const id='restore_drill_'+Date.now();run(['createdb','-U','siem',id]);
const source=run(['psql','-U','siem','-d','siem','-Atc','SELECT count(*) FROM assets']);
const input=openSync(file,'r');try{run(['pg_restore','-U','siem','-d',id,'--exit-on-error','--no-owner'],{stdio:[input,'pipe','pipe']})}finally{closeSync(input)}
const restored=run(['psql','-U','siem','-d',id,'-Atc','SELECT count(*) FROM assets']);if(source!==restored)throw Error('Restored asset count differs; pause producers during drill');
const url=process.env.INTEGRATION_ELASTICSEARCH_URL??'http://localhost:9200';
async function es(method,path,body){const r=await fetch(url+path,{method,headers:{'Content-Type':'application/json'},body:body===undefined?undefined:JSON.stringify(body),signal:AbortSignal.timeout(120000)});if(!r.ok)throw Error(`${method} ${path}: ${r.status} ${await r.text()}`);return r.json()}
await es('PUT','/_snapshot/siem-drill',{type:'fs',settings:{location:'siem-drill'}});
const before=await es('GET','/normalized_events,siem-events-*/_count?ignore_unavailable=true');
const snapshot=await es('PUT',`/_snapshot/siem-drill/${id}?wait_for_completion=true`,{indices:'normalized_events,siem-events-*',ignore_unavailable:true,include_global_state:false});
if(snapshot.snapshot.state!=='SUCCESS')throw Error('Incomplete snapshot');
const restore=await es('POST',`/_snapshot/siem-drill/${id}/_restore?wait_for_completion=true`,{indices:'*',include_global_state:false,include_aliases:false,rename_pattern:'(.+)',rename_replacement:`${id}-$1`,ignore_index_settings:['index.lifecycle.name']});
if(restore.snapshot.shards.failed!==0)throw Error('Restore shards failed');
await es('POST',`/${id}-*/_refresh`);const after=await es('GET',`/${id}-*/_count`);if(before.count!==after.count)throw Error('Restored event count differs; pause producers during drill');
writeFileSync(resolve(target,'report.json'),JSON.stringify({timestamp:new Date().toISOString(),project,postgres:{sha256:checksum,restoredDatabase:id,assetCount:Number(restored)},elasticsearch:{snapshot:id,eventCount:after.count},status:'passed'},null,2));
console.log(`Restore drill passed: ${restored} assets, ${after.count} events. Report: ${target}`);
