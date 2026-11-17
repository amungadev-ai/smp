/** @typedef {'install'|'migrate'} WebsiteMode */
/** @typedef {{name:string,size:number}} Backup */
/** @typedef {{mode:WebsiteMode,sample:boolean,archive:Backup|null,sql:Backup|null}} WebsiteSetup */
export const websiteFrameworks=['WordPress','PHP','Static website','Laravel'];
/** @param {string} framework */
export const isWebsite=framework=>websiteFrameworks.includes(framework);
/** @param {string} name */
export const websiteSlug=name=>name.trim().toLowerCase().replace(/[^a-z0-9-]+/g,'-').replace(/^-+|-+$/g,'').slice(0,48);
/** @param {WebsiteSetup} setup @returns {WebsiteSetup} */
export function validateSetup(setup){
 if(!['install','migrate'].includes(setup.mode))throw Error('Choose a WordPress operation.');
 if(setup.mode==='install')return {mode:setup.mode,sample:false,archive:null,sql:null};
 if(setup.sample)return {mode:setup.mode,sample:true,archive:null,sql:null};
 const check=(/** @type {Backup|null} */file,/** @type {RegExp} */ext)=>!!file&&typeof file.name==='string'&&file.name.length<=255&&ext.test(file.name)&&Number.isSafeInteger(file.size)&&file.size>0;
 if(!setup.archive||!setup.sql||!check(setup.archive,/\.zip$/i)||!check(setup.sql,/\.sql(?:\.gz)?$/i))throw Error('Choose a website ZIP and a nonempty SQL or SQL.GZ export.');
 return {mode:setup.mode,sample:false,archive:{name:setup.archive.name,size:setup.archive.size},sql:{name:setup.sql.name,size:setup.sql.size}};
}
/** @param {string} slug @param {{name:string,engine:string}[]} databases @param {{name:string,engine:string}[]} users */
export function chooseDatabaseName(slug,databases,users){
 const base=('wp_'+slug.replace(/[^a-z0-9_]/g,'_')).slice(0,34);if(!slug||!/^wp_[a-z0-9_]+$/.test(base))throw Error('Invalid WordPress project name.');
 const taken=new Set([...databases,...users].filter(x=>x.engine==='MySQL').map(x=>x.name));
 for(let i=0;i<1000;i++){const name=base+(i?'_'+i:'');if(!taken.has(name))return name;}throw Error('Could not allocate a unique database name.');
}
/** @param {string} name @param {string} domain @param {{projects:{id:string}[],domains:{id:string,project:string|null,system:boolean}[]}} workspace */
export function validateWebsite(name,domain,workspace){const slug=websiteSlug(name);if(!slug||name.trim().length>80)throw Error('Enter a website name of 1–80 characters.');if(workspace.projects.some(p=>p.id===slug))throw Error('A project with this name already exists.');if(domain&&!workspace.domains.some(d=>d.id===domain&&!d.project&&!d.system))throw Error('Choose an available custom domain.');if(!domain&&workspace.domains.some(d=>d.id===slug+'.drevanta.example'))throw Error('The automatic website address is already in use.');return slug;}
/** @param {string} framework @param {string} root @param {Record<string,{kind:string}>} tree @param {string} [output] */
export function validatePublish(framework,root,tree,output='.'){
 if(!['Static website','PHP'].includes(framework))throw Error('This website requires a server-side installation or build.');
 const dir=output&&output!=='.'?root+'/'+output:root;
 if(!dir||dir.split('/').some(p=>!p||p==='.'||p==='..'||p.includes('\\')))throw Error('Invalid website publishing root.');
 if(!['index.html',...(framework==='PHP'?['index.php']:[])].some(n=>tree[dir+'/'+n]?.kind==='file'))throw Error('Add an index.html or supported PHP entry point to the publishing directory first.');
 return dir;
}
