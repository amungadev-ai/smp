import {writable,get} from 'svelte/store';
import {workspace} from '$lib/stores/workspace';
import {freshMail,reduceMail,type MailState,type Command} from './model.js';
const KEY='drevanta-native-mail-v1';
export const mailState=writable<MailState>(freshMail());
export const mailReady=writable(false);
export const mailStorageError=writable('');
export function mailDomains(state:MailState,domains:{name:string;system:boolean}[]){return [...new Set([...domains.filter(d=>!d.system&&d.name!=='panel.example.com').map(d=>d.name),...state.mailboxes.map(m=>m.domain),...state.forwarders.map(f=>f.domain)])].sort();}
function read():MailState{const raw=localStorage.getItem(KEY);if(!raw)return freshMail();const saved=JSON.parse(raw);if(!Array.isArray(saved.mailboxes)||!Array.isArray(saved.forwarders)||!saved.dns||!saved.messages)throw Error('Invalid sample mail state.');return saved;}
export function initMail(){function reload(){try{mailState.set(read());mailStorageError.set('');}catch{mailStorageError.set('Browser mail storage is unavailable or invalid. Changes are disabled.');}finally{mailReady.set(true);}}reload();const listener=(e:StorageEvent)=>{if(e.key===KEY||e.key===null)reload();};window.addEventListener('storage',listener);return ()=>window.removeEventListener('storage',listener);}
export function mailCommand(c:Command){const latest=read();const domains=mailDomains(latest,get(workspace).domains);const next=reduceMail(latest,c,new Date().toISOString(),domains);try{localStorage.setItem(KEY,JSON.stringify(next));}catch{throw Error('Browser storage is full or unavailable. No mail changes saved.');}mailState.set(next);}
export const mailId=()=>crypto.randomUUID();
