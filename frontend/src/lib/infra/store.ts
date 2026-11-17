import {writable,get} from 'svelte/store';
import {workspace} from '$lib/stores/workspace';import {databaseState} from '$lib/database/store';import {fileState} from '$lib/files/store';
import {freshInfra,reduceInfra,type Command,type InfraState,type Context} from './model.js';
const KEY='drevanta-native-infra-v1';
export const infraState=writable<InfraState>(freshInfra()),infraError=writable('');
export const infraId=()=>crypto.randomUUID();
function context():Context{const w=get(workspace);return {projects:w.projects,domains:w.domains,databases:get(databaseState).databases.map(d=>d.id),spaces:get(fileState).spaces.map(s=>s.id)};}
export function initInfra(){try{const raw=localStorage.getItem(KEY);if(raw){const parsed=JSON.parse(raw);if(!Array.isArray(parsed.cron)||!Array.isArray(parsed.jobs)||!Array.isArray(parsed.backups)||!Array.isArray(parsed.certificates)||!Array.isArray(parsed.serviceEvents)||!parsed.runtimeAssignments||!parsed.autoDeploy)throw Error('Invalid state');infraState.set(parsed);}infraError.set('');}catch{infraError.set('Browser infrastructure state is unreadable; edits are disabled.');}}
export function infraCommand(c:Command){if(get(infraError))throw Error(get(infraError));const next=reduceInfra(get(infraState),c,new Date().toISOString(),context());try{localStorage.setItem(KEY,JSON.stringify(next));}catch{throw Error('Unable to store simulated infrastructure data.');}infraState.set(next);}
