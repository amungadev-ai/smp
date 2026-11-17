/** @typedef {'Inbox'|'Sent'|'Drafts'|'Archive'|'Spam'|'Trash'} Folder */
/** @typedef {{id:string,local:string,domain:string,label:string,quota:number,suspended:boolean,replyEnabled:boolean,replyBody:string,rotated:string|null}} Mailbox */
/** @typedef {{id:string,folder:Folder,from:string,address:string,to:string,subject:string,body:string,date:string,read:boolean,star:boolean,previous?:Folder}} Message */
/** @typedef {{id:string,local:string,domain:string,to:string}} Forwarder */
/** @typedef {{mailboxes:Mailbox[],forwarders:Forwarder[],dns:Record<string,boolean>,messages:Record<string,Message[]>}} MailState */
export const quotas=[1,2,5,10,20,50];
/** @type {Folder[]} */export const folders=['Inbox','Sent','Drafts','Archive','Spam','Trash'];
/** @param {{local:string,domain:string}} m */export const address=m=>m.local+'@'+m.domain;
/** @param {string} s */export const validLocal=s=>s.length<=64&&/^[a-z0-9_+-]+(?:\.[a-z0-9_+-]+)*$/.test(s);
/** @param {string} s */export const validDomain=s=>/^(?=.{1,253}$)(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(s);
/** @param {string} s */export function validAddress(s){const p=s.toLowerCase().split('@');return p.length===2&&validLocal(p[0])&&validDomain(p[1]);}
/** @param {MailState} s @param {string} id */export const messagesFor=(s,id)=>s.messages[id]||[];
/** @param {Message[]} list */export const usage=list=>list.reduce((n,m)=>n+new TextEncoder().encode(m.subject+m.body+m.to+m.address+m.from).length,0);
/** @param {Message[]} list @param {Folder|'Starred'} folder @param {string} query */
export function visibleMessages(list,folder,query){const q=query.trim().toLowerCase();return list.filter(m=>(folder==='Starred'?m.star&&m.folder!=='Trash'&&m.folder!=='Spam':m.folder===folder)&&[m.from,m.address,m.to,m.subject,m.body].join(' ').toLowerCase().includes(q)).sort((a,b)=>b.date.localeCompare(a.date));}
/** @returns {MailState} */
export function freshMail(){const mailboxes=[{id:'mail-demo-info',local:'info',domain:'business.example.com',label:'',quota:5,suspended:false,replyEnabled:false,replyBody:'',rotated:null},{id:'mail-demo-support',local:'support',domain:'business.example.com',label:'',quota:2,suspended:false,replyEnabled:false,replyBody:'',rotated:null}];return {mailboxes,forwarders:[],dns:{},messages:{'mail-demo-info':[
{id:'welcome',folder:'Inbox',from:'Drevanta',address:'welcome@drevanta.example',to:address(mailboxes[0]),subject:'Welcome to your business inbox',body:'Your sample mailbox is ready to explore.\n\nOrganize messages, compose replies and manage folders. No mail server is connected.',date:'2026-11-15T15:40:00Z',read:false,star:true},
{id:'inquiry',folder:'Inbox',from:'Maya Chen',address:'maya@example.net',to:address(mailboxes[0]),subject:'Website inquiry',body:'Hello,\n\nI would love to learn more about your services. Could we arrange a short introduction this week?\n\nBest,\nMaya',date:'2026-11-16T09:20:00Z',read:false,star:false},
{id:'documents',folder:'Inbox',from:'Jordan Lee',address:'jordan@example.net',to:address(mailboxes[0]),subject:'Project documents',body:'Thanks for the update. I will share revised documents at our next meeting.',date:'2026-11-14T10:14:00Z',read:true,star:false},
{id:'sent-sample',folder:'Sent',from:address(mailboxes[0]),address:address(mailboxes[0]),to:'jordan@example.net',subject:'Re: Project timeline',body:'Thanks Jordan, the proposed timeline works for us.',date:'2026-11-13T10:00:00Z',read:true,star:false}
],'mail-demo-support':[]}};}
/** @typedef {{type:'create',id:string,local:string,domain:string,quota:number}|{type:'edit',id:string,label:string,quota:number}|{type:'suspend',id:string,value:boolean}|{type:'rotate',id:string}|{type:'autoreply',id:string,enabled:boolean,body:string}|{type:'delete-mailbox',id:string}|{type:'dns',domain:string}|{type:'forward-add',id:string,local:string,domain:string,to:string}|{type:'forward-remove',id:string}|{type:'compose',mailbox:string,id:string,draft?:string,to:string,subject:string,body:string,intent:'draft'|'send'}|{type:'read'|'star',mailbox:string,id:string,value:boolean}|{type:'move',mailbox:string,id:string,folder:Folder}|{type:'restore'|'delete-message',mailbox:string,id:string}} Command */
/** @param {MailState} state @param {Command} c @param {string} at @param {string[]} domains @returns {MailState} */
export function reduceMail(state,c,at,domains){
 const s=structuredClone(state);
 const mailbox=(/** @type {string} */id)=>{const m=s.mailboxes.find(x=>x.id===id);if(!m)throw Error('Mailbox not found.');return m;};
 const checkDomain=(/** @type {string} */d)=>{if(!validDomain(d)||!domains.includes(d))throw Error('Select an existing workspace domain.');};
 const quota=(/** @type {number} */q)=>{if(!quotas.includes(q))throw Error('Choose a mailbox quota.');};
 const unique=(/** @type {string} */local,/** @type {string} */domain)=>{if(!validLocal(local))throw Error('Enter a valid mailbox name.');checkDomain(domain);if([...s.mailboxes,...s.forwarders].some(x=>x.local===local&&x.domain===domain))throw Error('This email address is already in use.');};
 switch(c.type){
 case 'create':{const local=c.local.trim().toLowerCase();unique(local,c.domain);quota(c.quota);if(s.mailboxes.some(x=>x.id===c.id))throw Error('Duplicate mailbox ID.');s.mailboxes.push({id:c.id,local,domain:c.domain,label:'',quota:c.quota,suspended:false,replyEnabled:false,replyBody:'',rotated:null});s.messages[c.id]=[];break;}
 case 'edit':{const m=mailbox(c.id);quota(c.quota);if(c.label.length>100)throw Error('Label is too long.');m.label=c.label.trim();m.quota=c.quota;break;}
 case 'rotate':mailbox(c.id).rotated=at;break;
 case 'suspend':mailbox(c.id).suspended=c.value;break;
 case 'autoreply':{const m=mailbox(c.id);if(c.body.length>10000||c.enabled&&!c.body.trim())throw Error('Enter an automatic reply (up to 10,000 characters).');m.replyEnabled=c.enabled;m.replyBody=c.body;break;}
 case 'delete-mailbox':mailbox(c.id);s.mailboxes=s.mailboxes.filter(x=>x.id!==c.id);delete s.messages[c.id];break;
 case 'dns':checkDomain(c.domain);s.dns[c.domain]=true;break;
 case 'forward-add':{const local=c.local.trim().toLowerCase(),to=c.to.trim().toLowerCase();unique(local,c.domain);if(!validAddress(to)||to===local+'@'+c.domain)throw Error('Enter a valid, different destination address.');if(s.forwarders.some(f=>f.id===c.id))throw Error('Duplicate forwarder ID.');let next=to;const visited=new Set([local+'@'+c.domain]);while(next){if(visited.has(next))throw Error('Forwarding loop detected.');visited.add(next);next=s.forwarders.find(f=>address(f)===next)?.to||'';}s.forwarders.push({id:c.id,local,domain:c.domain,to});break;}
 case 'forward-remove':s.forwarders=s.forwarders.filter(f=>f.id!==c.id);break;
 default:{
 const m=mailbox(c.mailbox);if(m.suspended)throw Error('Mailbox suspended.');const list=s.messages[m.id]||(s.messages[m.id]=[]);
 if(c.type==='compose'){
 if(c.to.length>320||c.subject.length>300||c.body.length>50000)throw Error('Message exceeds the sample size limit.');
 const to=c.to.trim();if(c.intent==='send'&&!validAddress(to))throw Error('Enter a valid recipient email address.');if(c.intent==='send'&&!c.subject.trim()&&!c.body.trim())throw Error('Write a subject or message before sending.');
 const existing=c.draft?list.find(x=>x.id===c.draft&&x.folder==='Drafts'):undefined;
 if(c.draft&&!existing)throw Error('Draft is no longer available.');if(!existing&&list.some(x=>x.id===c.id))throw Error('Duplicate message ID.');
 /** @type {Message} */const message={id:existing?.id||c.id,folder:c.intent==='draft'?'Drafts':'Sent',from:address(m),address:address(m),to,subject:c.subject,body:c.body,date:at,read:true,star:existing?.star||false};
 s.messages[m.id]=[message,...list.filter(x=>x.id!==message.id)];
 }else{
 const msg=list.find(x=>x.id===c.id);if(!msg)throw Error('Message not found.');
 switch(c.type){
 case 'read':msg.read=c.value;break;
 case 'star':msg.star=c.value;break;
 case 'move':if(!folders.includes(c.folder)||c.folder==='Sent'||c.folder==='Drafts')throw Error('Invalid destination folder.');if(c.folder==='Trash'&&msg.folder!=='Trash')msg.previous=msg.folder;msg.folder=c.folder;break;
 case 'restore':if(msg.folder!=='Trash')throw Error('Only trashed messages can be restored.');msg.folder=msg.previous&&msg.previous!=='Trash'?msg.previous:'Inbox';delete msg.previous;break;
 case 'delete-message':if(msg.folder!=='Trash')throw Error('Move the message to Trash first.');s.messages[m.id]=list.filter(x=>x.id!==c.id);break;
 }
 }
 if(s.messages[m.id].length>300||usage(s.messages[m.id])>512*1024)throw Error('Sample mailbox limit: 300 messages and 512 KiB.');
 }
 }
 if(s.mailboxes.length>100||s.forwarders.length>100)throw Error('Sample limit: 100 mailboxes and forwarders.');return s;
}
/** @param {string} d */
export function dnsRecords(d){return [['MX','@','10 mail.'+d],['A','mail','203.0.113.10'],['TXT','@','v=spf1 mx -all'],['TXT','selector._domainkey','DKIM public key generated by mail service'],['TXT','_dmarc','v=DMARC1; p=none']];}
