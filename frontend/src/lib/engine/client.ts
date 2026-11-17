import {writable,get} from 'svelte/store';
import {consumeSSE} from './stream.js';
import type {Health,ServerInfo,ReadinessCheck,EngineProject,EngineRelease,EngineJob,EngineDomain,EngineLog} from './types';
const KEY='drevanta-engine-session-v1';
export const engineToken=writable('');
export function restoreEngineSession(){if(typeof sessionStorage!=='undefined'){try{engineToken.set(sessionStorage.getItem(KEY)||'');}catch{}}}
export function clearEngineSession(){engineToken.set('');try{sessionStorage.removeItem(KEY);}catch{}}
function saveEngineSession(v:string){engineToken.set(v);try{sessionStorage.setItem(KEY,v);}catch{}}
export class EngineAPIError extends Error{constructor(public status:number,message:string){super(message);}}
async function request<T>(method:string,path:string,body?:unknown):Promise<T>{
 const token=get(engineToken);const headers:Record<string,string>={'Accept':'application/json'};
 if(body!==undefined)headers['Content-Type']='application/json';
 if(token)headers.Authorization='Bearer '+token;
 let response:Response;
 try{response=await fetch('/api/v1'+path,{method,headers,body:body===undefined?undefined:JSON.stringify(body),credentials:'same-origin',cache:'no-store'});}
 catch{throw new EngineAPIError(0,'Drevanta engine cannot be reached at /api/v1. Start the engine or configure the reverse proxy.');}
 const data:unknown=await response.json().catch(()=>({}));
 if(!response.ok){if(response.status===401&&path!=='/auth/login')clearEngineSession();const msg=typeof data==='object'&&data!==null&&'error'in data?String((data as {error:unknown}).error):'HTTP '+response.status;throw new EngineAPIError(response.status,msg);}
 return data as T;
}
export const engineAPI={
 health:()=>request<Health>('GET','/health'),
 login:async(password:string)=>{const response=await request<{token:string;version:string}>('POST','/auth/login',{password});saveEngineSession(response.token);return response;},
 logout:async()=>{try{await request<{ok:boolean}>('POST','/auth/logout',{});}finally{clearEngineSession();}},
 server:()=>request<ServerInfo>('GET','/server'),
 readiness:()=>request<{checks:ReadinessCheck[];mock_mode:boolean}>('GET','/server/readiness'),
 projects:()=>request<{projects:EngineProject[]}>('GET','/projects'),
 deploy:(input:{name:string;image:string;port:number;domain?:string;https?:boolean})=>request<{project:EngineProject;job:EngineJob}>('POST','/projects/deploy',input),
 redeploy:(id:string,image:string)=>request<{job:EngineJob}>('POST','/projects/'+encodeURIComponent(id)+'/deploy',{image}),
 rollback:(id:string)=>request<{job:EngineJob}>('POST','/projects/'+encodeURIComponent(id)+'/rollback',{}),
 restart:(id:string)=>request<{job:EngineJob}>('POST','/projects/'+encodeURIComponent(id)+'/restart',{}),
 project:(id:string)=>request<EngineProject>('GET','/projects/'+encodeURIComponent(id)),
 releases:(id?:string)=>request<{releases:EngineRelease[]}>('GET','/releases'+(id?'?project='+encodeURIComponent(id):'')),
 jobs:(id?:string)=>request<{jobs:EngineJob[]}>('GET','/jobs'+(id?'?project='+encodeURIComponent(id):'')),
 job:(id:string)=>request<EngineJob>('GET','/jobs/'+encodeURIComponent(id)),
 domains:()=>request<{domains:EngineDomain[]}>('GET','/domains')
};
export async function followEngineLogs(projectId:string,onLine:(line:EngineLog)=>void,signal:AbortSignal){
 const token=get(engineToken);if(!token)throw new EngineAPIError(401,'Sign in to stream logs.');
 const res=await fetch('/api/v1/projects/'+encodeURIComponent(projectId)+'/logs/stream',{headers:{Authorization:'Bearer '+token,Accept:'text/event-stream'},signal,cache:'no-store'});
 if(!res.ok){if(res.status===401)clearEngineSession();throw new EngineAPIError(res.status,'Log stream refused: HTTP '+res.status);}
 if(!res.body)throw new Error('Streaming unsupported.');
 const reader=res.body.getReader(),decoder=new TextDecoder();
 const parser=consumeSSE((data:string)=>{try{const value=JSON.parse(data) as EngineLog;if(value&&typeof value.message==='string')onLine(value);}catch{}}); 
 try{for(;;){const {done,value}=await reader.read();if(done)break;parser.feed(decoder.decode(value,{stream:true}));}parser.feed(decoder.decode());parser.finish();}finally{await reader.cancel().catch(()=>{});reader.releaseLock();}
}
