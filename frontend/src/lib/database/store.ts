import {writable,get} from 'svelte/store';
import {workspace} from '$lib/stores/workspace';
import {freshDatabases,reduceDatabase,type DatabaseState,type Command} from './model';
const KEY='drevanta-native-databases-v1';
export const databaseState=writable<DatabaseState>(freshDatabases());
export const newId=()=>globalThis.crypto.randomUUID();
export function initDatabases(){try{const raw=localStorage.getItem(KEY);if(raw){const saved=JSON.parse(raw);if(Array.isArray(saved.databases)&&Array.isArray(saved.users))databaseState.set(saved);}}catch{}return databaseState.subscribe(s=>{try{localStorage.setItem(KEY,JSON.stringify(s));}catch{}});}
export function command(c:Command){if((c.type==='create'||c.type==='link')&&c.project&&!get(workspace).projects.some(p=>p.id===c.project))throw Error('Project not found.');databaseState.update(s=>reduceDatabase(s,c,new Date().toISOString()));}
export function samplePassword(){const b=new Uint8Array(18);globalThis.crypto.getRandomValues(b);return 'Demo-'+Array.from(b,x=>x.toString(16).padStart(2,'0')).join('')+'!';}
export function validatePassword(p:string){if(p.length<8||p.length>128)throw Error('Enter a sample password of 8–128 characters.');}
