export interface Release{id:string;image:string;date:string;kind:'Deploy'|'Rollback'}
import type {Detection} from './detection/model.js';
import type {WebsiteSetup} from './websites/model.js';
export interface Project{id:string;name:string;framework:string;source:string;repo:string;branch:string;port:number;ram:number;cpu:number;status:'Live'|'Ready'|'Stopped';releases:Release[];activeRelease:string|null;deploymentConfig?:Detection;websiteSetup?:WebsiteSetup;websitePreview?:{directory:string;at:string};sourceRef?:{key:string;path:string;label:string}}
export interface Domain{id:string;name:string;project:string|null;status:'Active'|'Pending DNS'|'Available';ssl:boolean;system:boolean}
export interface Workspace{projects:Project[];domains:Domain[];events:{id:string;text:string;time:string}[]}
export const slugify=(s:string)=>s.toLowerCase().replace(/[^a-z0-9-]+/g,'-').replace(/^-+|-+$/g,'').slice(0,48);
export const primaryDomain=(domains:Domain[],id:string)=>domains.find(d=>d.project===id&&!d.system&&d.status==='Active')||domains.find(d=>d.project===id);
