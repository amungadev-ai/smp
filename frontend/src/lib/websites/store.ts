import {get} from 'svelte/store';
import {workspace,createProject,recordWebsitePreview} from '$lib/stores/workspace';
import {databaseState,stageDatabase,newId} from '$lib/database/store';
import {fileState,stageProject,getTree,storedRoot} from '$lib/files/store';
import {emptyTree,encodeText,type Tree} from '$lib/files/model.js';
import {chooseDatabaseName,validateWebsite,validateSetup,validatePublish,type WebsiteSetup} from './model.js';
import type {Detection} from '$lib/detection/model.js';
export function createWordPress(name:string,domain:string,input:WebsiteSetup){
 const slug=validateWebsite(name,domain,get(workspace)),setup=validateSetup(input),dbs=get(databaseState),dbName=chooseDatabaseName(slug,dbs.databases,dbs.users);
 const config:Detection={framework:'WordPress',root:'.',reason:'Guided WordPress setup; installation requires the server engine.',confidence:'Review required',manager:'',install:'',build:'',start:'',output:'.',port:80,ram:512,cpu:50,databaseHint:true,environmentKeys:[],candidates:[]};
 const tree:Tree={...emptyTree('public_html'),'public_html/wp-content':{kind:'folder'},'public_html/README.txt':{kind:'file',data:encodeText('Drevanta WordPress sample workspace. No WordPress files have been installed. The runtime and database are not live.')}};
 return createProject(name,'',domain,{key:'wordpress',path:'',label:setup.mode==='install'?'WordPress installer':'WordPress migration'},()=>{
   const undoFiles=stageProject(slug,tree);
   try{const undoDb=stageDatabase({type:'create',id:newId(),userId:newId(),engine:'MySQL',name:dbName,project:slug,first:{mode:'new',name:dbName,preset:'Full access'}});
     return ()=>{try{undoDb();}finally{undoFiles();}};
   }catch(e){undoFiles();throw e;}
 },config,setup);
}
export function previewWebsite(id:string){
 const p=get(workspace).projects.find(x=>x.id===id);if(!p)throw Error('Website not found.');
 const tree=getTree(get(fileState),'project:'+id,p.framework);if(!tree)throw Error('Website files are missing.');
 const root=storedRoot(tree,p.framework),relative=p.deploymentConfig?.root||'.';
 const dir=validatePublish(p.framework,relative==='.'?root:root+'/'+relative,tree,p.deploymentConfig?.output||'.');
 recordWebsitePreview(id,dir);
}
