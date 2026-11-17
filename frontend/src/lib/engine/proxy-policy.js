// The SvelteKit production server may only proxy the Go engine on loopback.
// The browser never learns this origin; all API requests stay same-origin.
const requestHeaderNames=['authorization','accept','content-type','last-event-id'];
const responseHeaderNames=['content-type','retry-after','x-accel-buffering'];
export const MAX_PROXY_BODY=1048576;
/** @param {string} raw */
export function engineOrigin(raw){
 try{
  const u=new URL(raw);
  if(u.protocol!=='http:'||u.hostname!=='127.0.0.1'||!u.port||!Number.isInteger(Number(u.port))||u.username||u.password||u.pathname!=='/'||u.search||u.hash)throw Error('Invalid backend origin');
  return u.origin;
 }catch{throw Error('DRVT_ENGINE_URL must be an explicit loopback HTTP URL (for example http://127.0.0.1:8080).');}
}
/** @param {string} pathname @param {string} search */
export function enginePath(pathname,search=''){
 if(!pathname.startsWith('/api/v1/')||pathname.length>2048||search.length>2048||/[\\\x00-\x1f]/.test(pathname+search))throw Error('Invalid engine API path.');
 return pathname+search;
}
/** @param {Headers} headers */
export function forwardRequestHeaders(headers){
 const out=new Headers();
 for(const name of requestHeaderNames){const v=headers.get(name);if(v)out.set(name,v);}
 return out;
}
/** @param {Headers} headers */
export function forwardResponseHeaders(headers){
 const out=new Headers({'Cache-Control':'no-store','X-Content-Type-Options':'nosniff','Referrer-Policy':'no-referrer','X-Frame-Options':'DENY'});
 for(const name of responseHeaderNames){const v=headers.get(name);if(v)out.set(name,v);}
 return out;
}
