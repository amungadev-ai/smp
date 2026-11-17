/** @typedef {{kind:'folder'}|{kind:'file',data:string}} Entry */
/** @typedef {Record<string,Entry>} Tree */
/** @typedef {{id:string,name:string,quota:number,access:'Private'|'Public read',files:Tree}} Space */
/** @typedef {{spaces:Space[],projects:Record<string,Tree>}} FileState */
/** @typedef {{path:string,data:string}} Upload */
export const BATCH_LIMIT=1048576;
export const DEMO_LIMIT=3*1048576;
export const MAX_ENTRIES=500;
/** @param {string} value */
export function validPart(value){return typeof value==='string'&&value.length>0&&value.length<=160&&value===value.trim()&&!['.','..','__proto__','constructor','prototype'].includes(value)&&!/[\\/:*?"<>|\x00-\x1f\x7f]/.test(value);}
/** @param {string} path */
export const validPath=path=>typeof path==='string'&&path.split('/').every(validPart);
/** @param {string} path */
export const basename=path=>path.split('/').at(-1)||'';
/** @param {string} path */
export const dirname=path=>path.split('/').slice(0,-1).join('/');
/** @param {string} data */
export function byteCount(data){return data.length*3/4-(data.endsWith('==')?2:data.endsWith('=')?1:0);}
/** @param {Tree} tree */
export function treeBytes(tree){return Object.values(tree).reduce((n,e)=>n+(e.kind==='file'?byteCount(e.data):0),0);}
/** @param {number} bytes */
export function sizeLabel(bytes){return bytes>=1048576?(bytes/1048576).toFixed(2)+' MB':bytes>=1024?(bytes/1024).toFixed(1)+' KB':bytes+' B';}
/** @param {string} root @returns {Tree} */
export const emptyTree=root=>({[root]:{kind:'folder'}});
/** @returns {FileState} */
export function freshFiles(){return {spaces:[{id:'media-library',name:'Media library',quota:1024,access:'Private',files:{files:{kind:'folder'},'files/images':{kind:'folder'},'files/documents':{kind:'folder'},'files/README.txt':{kind:'file',data:'U2FtcGxlIGluZGVwZW5kZW50IHN0b3JhZ2Uu'}}}],projects:{}};}
/** @param {Tree} tree @param {string} folder */
export function children(tree,folder){return Object.keys(tree).filter(p=>dirname(p)===folder).sort((a,b)=> Number(tree[b].kind==='folder')-Number(tree[a].kind==='folder')||a.localeCompare(b));}
/** @param {Tree} tree @param {string} folder */
function requireFolder(tree,folder){if(!validPath(folder)||!Object.hasOwn(tree,folder)||tree[folder].kind!=='folder')throw Error('Folder not found.');}
/** @param {Tree} tree @param {string} path */
function requireEntry(tree,path){if(!validPath(path)||!Object.hasOwn(tree,path))throw Error('File or folder not found.');}
/** @param {string} data */
function validateData(data){if(typeof data!=='string'||data.length%4!==0||/[^A-Za-z0-9+/=]/.test(data)||!(/^[A-Za-z0-9+/]*={0,2}$/).test(data))throw Error('Invalid file data.');}
/** @param {Tree} tree */
function limitEntries(tree){if(Object.keys(tree).length>MAX_ENTRIES)throw Error('Sample workspace limit: 500 entries per file tree.');}
/** @param {Tree} tree @param {string} folder @param {string} name @param {'folder'|'file'} kind @returns {Tree} */
export function createEntry(tree,folder,name,kind){requireFolder(tree,folder);if(!validPart(name))throw Error('Enter a valid file or folder name.');const dest=folder+'/'+name;if(Object.hasOwn(tree,dest))throw Error('Name already exists.');/** @type {Entry} */const entry=kind==='folder'?{kind:'folder'}:{kind:'file',data:''};/** @type {Tree} */const next={...tree,[dest]:entry};limitEntries(next);return next;}
/** @param {Tree} tree @param {string} folder @param {Upload[]} uploads @returns {Tree} */
export function addUploads(tree,folder,uploads){requireFolder(tree,folder);if(!uploads.length)throw Error('Choose files first.');if(uploads.length>MAX_ENTRIES)throw Error('Too many files.');let bytes=0;const next={...tree};for(const upload of uploads){if(!validPath(upload.path))throw Error('Invalid upload path.');validateData(upload.data);bytes+=byteCount(upload.data);if(bytes>BATCH_LIMIT)throw Error('Choose a batch of 1 MiB or less.');const parts=upload.path.split('/');let path=folder;for(const part of parts.slice(0,-1)){path+='/'+part;if(Object.hasOwn(next,path)&&next[path].kind!=='folder')throw Error('A file conflicts with a folder.');next[path]={kind:'folder'};}path+='/'+parts.at(-1);if(Object.hasOwn(next,path))throw Error('Name already exists: '+basename(path));next[path]={kind:'file',data:upload.data};limitEntries(next);}return next;}
/** @param {Tree} tree @param {string} path @param {string} name @returns {Tree} */
export function renameEntry(tree,path,name){requireEntry(tree,path);const parent=dirname(path);if(!parent)throw Error('Root folders cannot be renamed.');if(!validPart(name))throw Error('Invalid name.');const dest=parent+'/'+name;if(path===dest)return tree;if(Object.hasOwn(tree,dest))throw Error('Name already exists.');const next={...tree};for(const key of Object.keys(tree).filter(p=>p===path||p.startsWith(path+'/'))){next[dest+key.slice(path.length)]=tree[key];delete next[key];}return next;}
/** @param {Tree} tree @param {string} path @returns {Tree} */
export function deleteEntry(tree,path){requireEntry(tree,path);if(!dirname(path))throw Error('Root folders cannot be deleted.');const next={...tree};for(const p of Object.keys(tree))if(p===path||p.startsWith(path+'/'))delete next[p];return next;}
/** @param {Tree} tree @param {string} path @param {string} data @returns {Tree} */
export function editEntry(tree,path,data){requireEntry(tree,path);if(tree[path].kind!=='file')throw Error('Select a file.');validateData(data);if(byteCount(data)>BATCH_LIMIT)throw Error('File exceeds sample limit.');return {...tree,[path]:{kind:'file',data}};}
/** @param {Tree} tree @param {string} path @param {string} allowedRoot @param {string} [newRoot] @returns {Tree} */
export function sourceCopy(tree,path,allowedRoot,newRoot='app'){requireEntry(tree,path);if(path!==allowedRoot&&!path.startsWith(allowedRoot+'/'))throw Error('Only project source folders or storage files can be selected.');const next=emptyTree(newRoot);if(tree[path].kind==='file'){if(!/\.zip$/i.test(path))throw Error('Select a ZIP archive or a folder.');next[newRoot+'/'+basename(path)]=structuredClone(tree[path]);}else{for(const p of Object.keys(tree).filter(p=>p.startsWith(path+'/')))next[newRoot+p.slice(path.length)]=structuredClone(tree[p]);}if(!Object.values(next).some(e=>e.kind==='file'))throw Error('Source folder contains no files.');limitEntries(next);if(treeBytes(next)>BATCH_LIMIT)throw Error('Source copy exceeds the 1 MiB demo limit.');return next;}
/** @param {FileState} state @param {{id:string,name:string,quota:number,access:Space['access']}} input @param {boolean} editing @returns {FileState} */
export function saveSpace(state,input,editing=false){const name=input.name.trim();if(!name||name.length>80||!validPart(input.id)||!Number.isInteger(input.quota)||input.quota<1||input.quota>1048576||!['Private','Public read'].includes(input.access))throw Error('Enter a valid name, quota and access setting.');const existing=state.spaces.find(s=>s.id===input.id);if(editing&&!existing)throw Error('Storage space not found.');if(!editing&&existing||state.spaces.some(s=>s.id!==input.id&&s.name.toLowerCase()===name.toLowerCase()))throw Error('Choose a unique storage name.');const files=existing?.files||emptyTree('files');if(treeBytes(files)>input.quota*1048576)throw Error('Quota cannot be below current usage.');const space={...input,name,files};return {...state,spaces:editing?state.spaces.map(s=>s.id===input.id?space:s):[...state.spaces,space]};}
/** @param {FileState} state @param {string} key @param {Tree} next @returns {FileState} */
export function replaceTree(state,key,next){const id=key.slice(key.indexOf(':')+1);let result;if(key.startsWith('space:')){const space=state.spaces.find(s=>s.id===id);if(!space)throw Error('Storage space not found.');if(treeBytes(next)>space.quota*1048576)throw Error('Storage quota exceeded.');result={...state,spaces:state.spaces.map(s=>s.id===id?{...s,files:next}:s)};}else if(key.startsWith('project:')){if(!validPart(id))throw Error('Invalid project.');result={...state,projects:{...state.projects,[id]:next}};}else throw Error('Unknown file target.');limitEntries(next);const total=result.spaces.reduce((n,s)=>n+treeBytes(s.files),0)+Object.values(result.projects).reduce((n,t)=>n+treeBytes(t),0);if(total>DEMO_LIMIT)throw Error('Browser demo limit: 3 MiB of file contents.');return result;}
/** @param {string} text */
export function encodeText(text){return encodeBytes(new TextEncoder().encode(text));}
/** @param {Uint8Array} bytes */
export function encodeBytes(bytes){let binary='';for(let i=0;i<bytes.length;i+=8192)binary+=String.fromCharCode(...bytes.subarray(i,i+8192));return btoa(binary);}
/** @param {string} data */
export function decodeBytes(data){return Uint8Array.from(atob(data),c=>c.charCodeAt(0));}
/** @param {string} data */
export function decodeText(data){return new TextDecoder('utf-8',{fatal:true}).decode(decodeBytes(data));}
