import type {Workspace} from './model';
export const freshWorkspace=():Workspace=>({
projects:[
{id:'main-website',name:'Main website',framework:'Next.js',source:'GitHub',repo:'demo/main-website',branch:'main',port:8080,ram:512,cpu:50,status:'Live',releases:[],activeRelease:null},
{id:'admin-dashboard',name:'Admin dashboard',framework:'Next.js',source:'GitHub',repo:'demo/admin-dashboard',branch:'main',port:8080,ram:512,cpu:50,status:'Live',releases:[],activeRelease:null},
{id:'mobile-api',name:'Mobile API',framework:'Node.js / Express',source:'GitHub',repo:'demo/mobile-api',branch:'main',port:8080,ram:128,cpu:25,status:'Ready',releases:[],activeRelease:null}],
domains:[
{id:'example.com',name:'example.com',project:'main-website',status:'Active',ssl:true,system:false},
{id:'app.example.com',name:'app.example.com',project:'admin-dashboard',status:'Active',ssl:true,system:false},
{id:'api.example.com',name:'api.example.com',project:'mobile-api',status:'Pending DNS',ssl:false,system:false},
{id:'business.example.com',name:'business.example.com',project:null,status:'Available',ssl:false,system:false},
{id:'shop.business.example.com',name:'shop.business.example.com',project:null,status:'Available',ssl:false,system:false}],events:[]});
