export const engines=['MySQL','MariaDB','PostgreSQL'] as const;
export type Engine=typeof engines[number];
export const presets=['Full access','Read / write','Read only'] as const;
export type Preset=typeof presets[number];
export type HostMode='Private remote network'|'Restricted public TLS';
export type InitialUser={mode:'new';name:string;preset:Preset}|{mode:'existing';id:string;preset:Preset}|{mode:'none'};
export interface Database {id:string;name:string;engine:Engine;project:string|null;tables:string[];bytes:number|null;operations:{text:string;at:string}[]}
export interface DbUser {id:string;name:string;engine:Engine;grants:Record<string,string[]>;hosts:{mode:HostMode;address:string}[];rotated:string|null}
export interface DatabaseState {databases:Database[];users:DbUser[]}
export const permissions=(engine:Engine):string[]=>engine==='PostgreSQL'?['CONNECT','USAGE','SELECT','INSERT','UPDATE','DELETE','TRUNCATE','CREATE','REFERENCES','TRIGGER']:['ALTER','ALTER ROUTINE','CREATE','CREATE ROUTINE','CREATE TEMPORARY TABLES','CREATE VIEW','DELETE','DROP','EVENT','EXECUTE','INDEX','INSERT','LOCK TABLES','REFERENCES','SELECT','SHOW VIEW','TRIGGER','UPDATE'];
export const preset=(engine:Engine,value:Preset)=>permissions(engine).filter(p=>value==='Full access'||(value==='Read only'?['CONNECT','USAGE','SELECT']:['CONNECT','USAGE','SELECT','INSERT','UPDATE','DELETE']).includes(p));
export function privilegeLabel(engine:Engine,values:string[]){for(const p of presets){const expected=preset(engine,p);if(values.length===expected.length&&expected.every(x=>values.includes(x)))return p;}return 'Custom · '+values.length+' privileges';}
export const validName=(name:string)=>/^[a-z][a-z0-9_]{0,39}$/.test(name);
export function validHost(address:string){const m=/^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})(?:\/(\d{1,2}))?$/.exec(address);return !!m&&m.slice(1,5).every(x=>Number(x)<=255)&&Number(m[1])>0&&Number(m[1])<224&&(!m[5]||(Number(m[5])>=24&&Number(m[5])<=32));}
export const port=(engine:Engine)=>engine==='PostgreSQL'?5432:3306;
export const host=(engine:Engine)=>'db-'+engine.toLowerCase()+'.internal';
export const adminTool=(engine:Engine)=>engine==='PostgreSQL'?'pgAdmin':'phpMyAdmin';
export function sizeLabel(bytes:number|null){if(bytes===null)return 'Not measured';if(bytes<1024)return bytes+' B';if(bytes<1048576)return Math.round(bytes/1024)+' KB';return (bytes/1048576).toFixed(1)+' MB';}
export const usersFor=(s:DatabaseState,id:string)=>s.users.filter(u=>Object.prototype.hasOwnProperty.call(u.grants,id));
export const freshDatabases=():DatabaseState=>({databases:[{id:'db-main-website',name:'main_website',engine:'MySQL',project:'main-website',tables:['posts','users','settings'],bytes:262144,operations:[]},{id:'db-admin-dashboard',name:'admin_dashboard',engine:'PostgreSQL',project:'admin-dashboard',tables:['customers','orders','users'],bytes:524288,operations:[]}],users:[{id:'user-main-website',name:'main_website',engine:'MySQL',grants:{'db-main-website':permissions('MySQL')},hosts:[],rotated:null},{id:'user-admin-dashboard',name:'admin_dashboard',engine:'PostgreSQL',grants:{'db-admin-dashboard':permissions('PostgreSQL')},hosts:[],rotated:null}]});
export type Command=
 | {type:'create';id:string;userId:string;name:string;engine:Engine;project:string|null;first:InitialUser}
 | {type:'user';id:string;name:string;engine:Engine;database?:string;preset:Preset}
 | {type:'grant';database:string;user:string;privileges:string[]}
 | {type:'revoke';database:string;user:string}
 | {type:'host-add';user:string;mode:HostMode;address:string}
 | {type:'host-remove';user:string;mode:HostMode;address:string}
 | {type:'rotate';user:string}
 | {type:'delete-user';user:string}
 | {type:'delete-database';database:string}
 | {type:'link';database:string;project:string|null}
 | {type:'operation';database:string;label:string};
export function reduceDatabase(state:DatabaseState,cmd:Command,at:string):DatabaseState{
 const s=structuredClone(state);
 const db=(id:string)=>{const d=s.databases.find(d=>d.id===id);if(!d)throw Error('Database not found.');return d;};
 const user=(id:string)=>{const u=s.users.find(u=>u.id===id);if(!u)throw Error('User not found.');return u;};
 const unique=(name:string,engine:Engine)=>{if(!validName(name))throw Error('Use 1–40 lowercase letters, numbers or underscores, starting with a letter.');if(s.users.some(u=>u.name===name&&u.engine===engine))throw Error('This username already exists on the selected engine.');};
 const addUser=(id:string,name:string,engine:Engine)=>{unique(name,engine);if(s.users.some(u=>u.id===id))throw Error('Duplicate user ID.');const u:DbUser={id,name,engine,grants:{},hosts:[],rotated:null};s.users.push(u);return u;};
 const link=(d:Database,project:string|null)=>{if(project&&s.databases.some(x=>x.id!==d.id&&x.project===project))throw Error('This project already has a linked database.');d.project=project;};
 const event=(d:Database,text:string)=>{d.operations.unshift({text,at});d.operations=d.operations.slice(0,60);};
 switch(cmd.type){
 case 'create':{if(!engines.includes(cmd.engine)||!validName(cmd.name))throw Error('Enter a valid database name and engine.');if(s.databases.some(d=>d.id===cmd.id||(d.name===cmd.name&&d.engine===cmd.engine)))throw Error('Database already exists on this engine.');const d:Database={id:cmd.id,name:cmd.name,engine:cmd.engine,project:null,tables:[],bytes:0,operations:[]};link(d,cmd.project);s.databases.push(d);if(cmd.first.mode!=='none'){const u=cmd.first.mode==='new'?addUser(cmd.userId,cmd.first.name,cmd.engine):user(cmd.first.id);if(u.engine!==d.engine)throw Error('Database and user must use the same engine.');u.grants[d.id]=preset(d.engine,cmd.first.preset);}event(d,'Database created (demo)');break;}
 case 'user':{const u=addUser(cmd.id,cmd.name,cmd.engine);if(cmd.database){const d=db(cmd.database);if(u.engine!==d.engine)throw Error('Database and user must use the same engine.');u.grants[d.id]=preset(d.engine,cmd.preset);event(d,'Created user '+u.name+' (demo)');}break;}
 case 'grant':{const d=db(cmd.database),u=user(cmd.user);if(u.engine!==d.engine)throw Error('Database and user must use the same engine.');if(!cmd.privileges.length||cmd.privileges.some(p=>!permissions(d.engine).includes(p)))throw Error('Select at least one supported privilege.');u.grants[d.id]=[...new Set(cmd.privileges)];event(d,'Saved privileges for '+u.name+' (demo)');break;}
 case 'revoke':{const d=db(cmd.database),u=user(cmd.user);delete u.grants[d.id];event(d,'Revoked access for '+u.name+' (demo)');break;}
 case 'host-add':{const u=user(cmd.user),address=cmd.address.trim();if(!['Private remote network','Restricted public TLS'].includes(cmd.mode)||!validHost(address))throw Error('Enter a specific IPv4 address or /24–/32 CIDR; unrestricted hosts are not supported.');if(u.hosts.some(h=>h.address===address&&h.mode===cmd.mode))throw Error('Host already exists.');u.hosts.push({mode:cmd.mode,address});break;}
 case 'host-remove':{const u=user(cmd.user);u.hosts=u.hosts.filter(h=>h.address!==cmd.address||h.mode!==cmd.mode);break;}
 case 'rotate':user(cmd.user).rotated=at;break;
 case 'delete-user':{const u=user(cmd.user);for(const d of s.databases.filter(d=>Object.prototype.hasOwnProperty.call(u.grants,d.id)))event(d,'Removed user '+u.name+' (demo)');s.users=s.users.filter(x=>x.id!==u.id);break;}
 case 'delete-database':{db(cmd.database);s.databases=s.databases.filter(d=>d.id!==cmd.database);for(const u of s.users)delete u.grants[cmd.database];break;}
 case 'link':{const d=db(cmd.database);link(d,cmd.project);event(d,cmd.project?'Project linked (demo)':'Project detached (demo)');break;}
 case 'operation':{const d=db(cmd.database);event(d,cmd.label);break;}
 }
 return s;
}
