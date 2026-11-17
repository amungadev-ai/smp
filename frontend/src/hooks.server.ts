import type {Handle} from '@sveltejs/kit';
import {engineOrigin,enginePath,forwardRequestHeaders,forwardResponseHeaders,MAX_PROXY_BODY} from '$lib/engine/proxy-policy.js';

async function requestBody(request:Request):Promise<ArrayBuffer|undefined>{
 if(request.method==='GET'||request.method==='HEAD'||!request.body)return;
 const sizeHint=Number(request.headers.get('content-length')||0);
 if(sizeHint>MAX_PROXY_BODY)throw new Error('Request body exceeds the 1 MiB engine API limit.');
 const reader=request.body.getReader();
 const chunks:Uint8Array[]=[];let length=0;
 try{
  for(;;){
   const {done,value}=await reader.read();if(done)break;
   length+=value.byteLength;
   if(length>MAX_PROXY_BODY)throw new Error('Request body exceeds the 1 MiB engine API limit.');
   chunks.push(value);
  }
 }finally{reader.releaseLock();}
 const out=new Uint8Array(length);let offset=0;
 for(const b of chunks){out.set(b,offset);offset+=b.byteLength;}
 return out.buffer;
}
const jsonError=(status:number,message:string)=>new Response(JSON.stringify({error:message}),{status,headers:{'Content-Type':'application/json','Cache-Control':'no-store','X-Content-Type-Options':'nosniff'}});
export const handle:Handle=async({event,resolve})=>{
 if(!event.url.pathname.startsWith('/api/v1/'))return resolve(event);
 let base:string,path:string;
 try{base=engineOrigin(process.env.DRVT_ENGINE_URL||'http://127.0.0.1:8080');path=enginePath(event.url.pathname,event.url.search);}catch(e){return jsonError(503,e instanceof Error?e.message:'Engine proxy not configured.');}
 if(!['GET','HEAD','POST','PUT','PATCH','DELETE'].includes(event.request.method))return jsonError(405,'HTTP method not allowed.');
 let body:ArrayBuffer|undefined;
 try{body=await requestBody(event.request);}catch(e){return jsonError(413,e instanceof Error?e.message:'Body too large.');}
 try{
  // Streaming GET response bodies are passed through to the browser (including SSE).
  // Do not send cookies, forwarding host headers or untrusted upstream URLs.
  const upstream=await fetch(base+path,{method:event.request.method,headers:forwardRequestHeaders(event.request.headers),body,redirect:'manual',cache:'no-store',signal:event.request.signal});
  if(upstream.status>=300&&upstream.status<400){await upstream.body?.cancel();return jsonError(502,'Engine returned an unexpected redirect.');}
  return new Response(upstream.body,{status:upstream.status,headers:forwardResponseHeaders(upstream.headers)});
 }catch{return jsonError(502,'Engine API unavailable on this host.');}
};
