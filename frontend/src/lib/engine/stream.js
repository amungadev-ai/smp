/** Decode SSE incrementally; ignores keepalive comments and unknown fields. */
/** @param {(value:string)=>void} onEvent */
export function consumeSSE(onEvent){let pending='';let data=/** @type {string[]} */ ([]);return {feed(/** @type {string} */chunk){pending+=chunk.replace(/\r\n?/g,'\n');if(pending.length>262144)throw Error('Log stream line is too long.');for(;;){const i=pending.indexOf('\n');if(i===-1)break;const line=pending.slice(0,i);pending=pending.slice(i+1);if(!line){if(data.length)onEvent(data.join('\n'));data=[];continue;}if(line.startsWith('data:'))data.push(line.slice(5).trimStart());}},finish(){if(data.length)onEvent(data.join('\n'));data=[];pending='';}};}
/** @param {number} n */
export function bytes(n){if(!Number.isFinite(n)||n<=0)return '0 B';let i=0;while(n>=1024&&i<4){n/=1024;i++;}return n.toFixed(i?1:0)+' '+['B','KB','MB','GB','TB'][i];}
/** @param {string} v */
export function validImage(v){return /^[a-z0-9][a-z0-9._/-]*(?::[a-zA-Z0-9._-]+)?$/.test(v.trim());}
/** @param {string} v */
export function validProjectName(v){return /^[a-z0-9][a-z0-9-]{0,48}$/.test(v.trim());}
/** @param {string} v */
export function validDomain(v){return v===''||/^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,24}$/.test(v.trim());}
