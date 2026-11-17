/** @typedef {{id:string;at:string;label:string}} History */
/** @typedef {{id:string;name:string;schedule:string;command:string;enabled:boolean;runs:History[]}} CronJob */
/** @typedef {{id:string;name:string;project:string;kind:'Worker command'|'HTTP request';target:string;enabled:boolean;runs:History[]}} AppJob */
/** @typedef {{id:string;target:string;label:string;at:string;restores:History[]}} Backup */
/** @typedef {{domain:string;at:string;kind:'check'|'renew';result:string}} CertificateCheck */
/** @typedef {{cron:CronJob[];jobs:AppJob[];backups:Backup[];certificates:CertificateCheck[];serviceEvents:History[];runtimeAssignments:Record<string,string>;autoDeploy:Record<string,boolean>}} InfraState */
/** @typedef {{type:'cron-create';id:string;name:string;schedule:string;command:string}|{type:'cron-edit';id:string;name:string;schedule:string;command:string}|{type:'cron-toggle';id:string;enabled:boolean}|{type:'cron-delete';id:string}|{type:'cron-run';id:string}|{type:'job-create';id:string;name:string;project:string;kind:'Worker command'|'HTTP request';target:string}|{type:'job-edit';id:string;name:string;kind:'Worker command'|'HTTP request';target:string}|{type:'job-toggle';id:string;enabled:boolean}|{type:'job-delete';id:string}|{type:'job-run';id:string}|{type:'backup-create';id:string;target:string;label:string}|{type:'backup-restore';id:string}|{type:'backup-delete';id:string}|{type:'certificate-check';domain:string;kind:'check'|'renew'}|{type:'service-check';service:string}|{type:'runtime-assign';project:string;profile:string}|{type:'auto-deploy';project:string;enabled:boolean}} Command */
/** @typedef {{id:string;name:string;framework:string;source:string}} App */
/** @typedef {{projects:App[];domains:{id:string;name:string;project:string|null;status:string;ssl:boolean}[];databases:string[];spaces:string[]}} Context */
export const services=[['Docker','Application containers'],['Caddy','HTTPS and reverse proxy'],['PostgreSQL','Drevanta platform state'],['Mail server','Business email delivery'],['Background workers','Deployment and scheduled jobs']];
export const runtimeProfiles=[
 {id:'node-22',name:'Node.js 22',for:'Node.js / Express, Next.js and JavaScript',runtime:'Node.js',port:3000},
 {id:'python-3',name:'Python 3',for:'Django, Flask and FastAPI',runtime:'Python',port:8000},
 {id:'php-8',name:'PHP 8',for:'PHP and Laravel',runtime:'PHP',port:80},
 {id:'go',name:'Go',for:'Go applications',runtime:'Go',port:8080},
 {id:'static',name:'Static website',for:'HTML, CSS and JavaScript',runtime:'Static',port:80},
 {id:'docker',name:'Docker image',for:'Prebuilt containers and custom Dockerfile',runtime:'Docker',port:8080}
];
const runtimeIds=new Set(runtimeProfiles.map(x=>x.id));
const serviceNames=new Set(services.map(x=>x[0]));
/** @returns {InfraState} */
export const freshInfra=()=>({cron:[],jobs:[],backups:[],certificates:[],serviceEvents:[],runtimeAssignments:{},autoDeploy:{}});
/** @param {string} value */
export function cronExpression(value){
 const parts=value.trim().split(/\s+/);
 if(parts.length!==5)throw Error('Use five cron fields: minute hour day month weekday.');
 const bounds=[[0,59],[0,23],[1,31],[1,12],[0,7]];
 parts.forEach((field,i)=>{const [min,max]=bounds[i];if(!field||field.length>120)throw Error('Invalid cron field.');
  for(const item of field.split(',')){const m=/^(\*|\d+)(?:-(\d+))?(?:\/(\d+))?$/.exec(item);if(!m)throw Error('Invalid cron expression. Use *, numbers, ranges, steps or lists.');
    const first=m[1]==='*'?min:Number(m[1]),last=m[2]?Number(m[2]):m[1]==='*'?max:first,step=m[3]?Number(m[3]):null;
    if(first<min||first>max||last<first||last>max||step!==null&&(step<1||step>max-min+1)||m[1]==='*'&&m[2])throw Error('Cron field is out of range.');
  }
 });
 return parts.join(' ');
}
/** @param {string} input */
export function githubRepository(input){const v=input.trim();const m=/^https:\/\/github\.com\/([A-Za-z0-9-]{1,39})\/([A-Za-z0-9._-]{1,100}?)(?:\.git)?\/?$/.exec(v);if(!m||m[1].startsWith('-')||m[1].endsWith('-')||['.','..'].includes(m[2]))throw Error('Use a GitHub repository URL, such as https://github.com/owner/repo.');return m[1]+'/'+m[2];}
/** @param {string} branch */
export function gitBranch(branch){if(!/^[A-Za-z0-9_][A-Za-z0-9._\/-]{0,99}$/.test(branch)||branch.includes('..')||branch.includes('//')||branch.endsWith('/')||branch.endsWith('.lock')||branch.includes('@{'))throw Error('Enter a valid Git branch name.');return branch;}
/** @param {string} value @param {string} label */
const safeText=(value,label)=>{const v=value.trim();if(!v||v.length>300||/[\x00-\x1f\x7f]/.test(v))throw Error(label+' is required (one line, 300 characters or fewer).');return v;};
/** @param {string} value */
export function validateTarget(value){const s=value.trim();if(s.length>500||!s||/[\x00-\x1f\x7f]/.test(s))throw Error('Enter a single-line command or HTTPS URL of 500 characters or fewer.');return s;}
/** @param {string} target @param {Context} ctx */
export function backupLabel(target,ctx){
 if(target==='workspace')return 'Workspace';
 if(target.startsWith('project:')){const id=target.slice(8),p=ctx.projects.find(x=>x.id===id);if(p)return p.name;throw Error('Project not found.');}
 if(target.startsWith('database:')){const id=target.slice(9);if(ctx.databases.includes(id))return 'Database '+id;throw Error('Database not found.');}
 if(target.startsWith('space:')){const id=target.slice(6);if(ctx.spaces.includes(id))return 'Storage '+id;throw Error('Storage space not found.');}
 throw Error('Choose an allowed backup target.');
}
/** @param {InfraState} state @param {Command} c @param {string} at @param {Context} context @returns {InfraState} */
export function reduceInfra(state,c,at,context){
 const s=structuredClone(state);
 const run=(/** @type {History[]} */events,/** @type {string} */label,/** @type {string} */id)=>{events.unshift({id,at,label});events.splice(40);};
 const cron=(/** @type {string} */id)=>{const x=s.cron.find(x=>x.id===id);if(!x)throw Error('Scheduled job not found.');return x;};
 const job=(/** @type {string} */id)=>{const x=s.jobs.find(x=>x.id===id);if(!x)throw Error('Application job not found.');return x;};
 const project=(/** @type {string} */id)=>{const p=context.projects.find(x=>x.id===id);if(!p)throw Error('Project not found.');return p;};
 const name=(/** @type {string} */s)=>safeText(s,'Name');
 switch(c.type){
 case 'cron-create':{if(s.cron.length>=100||s.cron.some(x=>x.id===c.id))throw Error('Cron job limit reached or duplicate ID.');s.cron.push({id:c.id,name:name(c.name),schedule:cronExpression(c.schedule),command:validateTarget(c.command),enabled:false,runs:[]});break;}
 case 'cron-edit':{const x=cron(c.id);x.name=name(c.name);x.schedule=cronExpression(c.schedule);x.command=validateTarget(c.command);break;}
 case 'cron-toggle':cron(c.id).enabled=c.enabled;break;
 case 'cron-delete':cron(c.id);s.cron=s.cron.filter(x=>x.id!==c.id);break;
 case 'cron-run':{const x=cron(c.id);run(x.runs,'Simulated run only — command was not executed',at+'-'+x.runs.length);break;}
 case 'job-create':{project(c.project);if(s.jobs.length>=100||s.jobs.some(x=>x.id===c.id))throw Error('Application job limit or duplicate ID.');if(!['Worker command','HTTP request'].includes(c.kind))throw Error('Choose a job type.');const target=validateTarget(c.target);if(c.kind==='HTTP request'&&!/^https:\/\/[a-z0-9.-]+(?::\d{2,5})?(?:\/[^\s]*)?$/i.test(target))throw Error('HTTP jobs require an HTTPS URL.');s.jobs.push({id:c.id,project:c.project,name:name(c.name),kind:c.kind,target,enabled:false,runs:[]});break;}
 case 'job-edit':{const x=job(c.id);if(!['Worker command','HTTP request'].includes(c.kind))throw Error('Choose a job type.');const target=validateTarget(c.target);if(c.kind==='HTTP request'&&!/^https:\/\/[a-z0-9.-]+(?::\d{2,5})?(?:\/[^\s]*)?$/i.test(target))throw Error('HTTP jobs require an HTTPS URL.');x.name=name(c.name);x.kind=c.kind;x.target=target;break;}
 case 'job-toggle':job(c.id).enabled=c.enabled;break;
 case 'job-delete':job(c.id);s.jobs=s.jobs.filter(x=>x.id!==c.id);break;
 case 'job-run':{const x=job(c.id);run(x.runs,'Simulated '+x.kind+' run — no requests or processes started',at+'-'+x.runs.length);break;}
 case 'backup-create':{if(s.backups.length>=100||s.backups.some(x=>x.id===c.id))throw Error('Sample backup limit or duplicate ID.');const label=backupLabel(c.target,context);s.backups.unshift({id:c.id,target:c.target,label,at,restores:[]});break;}
 case 'backup-restore':{const x=s.backups.find(x=>x.id===c.id);if(!x)throw Error('Backup entry not found.');backupLabel(x.target,context);run(x.restores,'Restore preview — no data changed',at+'-'+x.restores.length);break;}
 case 'backup-delete':{if(!s.backups.some(x=>x.id===c.id))throw Error('Backup entry not found.');s.backups=s.backups.filter(x=>x.id!==c.id);break;}
 case 'certificate-check':{const d=context.domains.find(x=>x.id===c.domain);if(!d)throw Error('Domain not found.');if(!d.project)throw Error('Connect the domain to a project before checking certificates.');if(c.kind==='renew'&&!d.ssl)throw Error('A sample existing certificate is required to preview renewal.');s.certificates.unshift({domain:d.name,at,kind:c.kind,result:'Example only: No DNS, ACME, Caddy or TLS request was made.'});s.certificates=s.certificates.slice(0,100);break;}
 case 'service-check':{if(!serviceNames.has(c.service))throw Error('Service not found.');run(s.serviceEvents,'Example service check: '+c.service+' — no server connection',at+'-'+s.serviceEvents.length);break;}
 case 'runtime-assign':{project(c.project);if(c.profile&&!runtimeIds.has(c.profile))throw Error('Unknown runtime profile.');if(c.profile)s.runtimeAssignments[c.project]=c.profile;else delete s.runtimeAssignments[c.project];break;}
 case 'auto-deploy':{const p=project(c.project);if(p.source!=='GitHub')throw Error('Auto-deploy requires a GitHub source.');s.autoDeploy[c.project]=c.enabled;break;}
 }
 return s;
}
