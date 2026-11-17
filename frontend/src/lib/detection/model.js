import {decodeText,decodeBytes,encodeBytes,addUploads,emptyTree,validPath,treeBytes,BATCH_LIMIT,MAX_ENTRIES} from '../files/model.js';
import {unpackZip} from './zip.js';
/** @typedef {import('../files/model.js').Tree} Tree */
/** @typedef {{framework:string,root:string,reason:string,confidence:'Detected'|'Review required',manager:string,install:string,build:string,start:string,output:string,port:number,ram:number,cpu:number,databaseHint:boolean,environmentKeys:string[],candidates:string[]}} Detection */
export const frameworks=['Next.js','Node.js / Express','Node.js application','Frontend build','Static website','Python','PHP','Laravel','WordPress','Go','Rust','Custom Docker application','Docker Compose','Multiple technologies','Not detected'];
const signals=new Set(['package.json','composer.json','artisan','wp-config.php','wp-load.php','requirements.txt','pyproject.toml','manage.py','go.mod','Cargo.toml','Dockerfile','compose.yaml','compose.yml','docker-compose.yml','docker-compose.yaml','index.php','index.html']);
const ignored=new Set(['node_modules','vendor','dist','build','public','assets','tests','test','docs','.git','__MACOSX','.next']);
/** @param {Tree} tree */
export async function inspectSource(tree){const paths=Object.keys(tree).filter(p=>tree[p].kind==='file');if(!paths.length)throw Error('Source contains no files.');if(paths.length===1&&/\.zip$/i.test(paths[0])){const entry=tree[paths[0]];if(entry.kind!=='file')throw Error('Archive not found.');const items=await unpackZip(decodeBytes(entry.data));return addUploads(emptyTree('app'),'app',items.map(item=>({path:item.path,data:encodeBytes(item.data)})));}if(treeBytes(tree)>BATCH_LIMIT||paths.length>MAX_ENTRIES)throw Error('Source exceeds browser demo limits.');return tree;}
/** @param {Tree} tree @param {string} path */
function textAt(tree,path){const item=tree['app/'+path];if(item?.kind!=='file'||item.data.length>350000)return '';try{return decodeText(item.data);}catch{return '';}}
/** @param {string} value @returns {Record<string,any>} */
function json(value){try{const parsed=JSON.parse(value);return parsed&&typeof parsed==='object'&&!Array.isArray(parsed)?parsed:{};}catch{return {};}}
/** @param {Tree} tree @param {string} root @returns {Detection} */
function detectAt(tree,root){
 const prefix=root==='.'?'':root+'/';
 const has=(/** @type {string} */n)=>tree['app/'+prefix+n]?.kind==='file';
 const read=(/** @type {string} */n)=>textAt(tree,prefix+n);
 const pkg=json(read('package.json')),composer=json(read('composer.json')),deps={...pkg.dependencies,...pkg.devDependencies},scripts=pkg.scripts||{};
 /** @type {Detection} */const result={framework:'Not detected',root,reason:'No recognized application manifest or entry point.',confidence:'Review required',manager:'',install:'',build:'',start:'',output:'',port:8080,ram:128,cpu:25,databaseHint:false,environmentKeys:[],candidates:[]};
 const set=(/** @type {string} */f,/** @type {string} */reason,/** @type {boolean} */review=false)=>{result.framework=f;result.reason=reason;result.confidence=review?'Review required':'Detected';};
 if(['compose.yaml','compose.yml','docker-compose.yml','docker-compose.yaml'].some(has))set('Docker Compose','Compose configuration detected.',true);
 else if(has('Dockerfile'))set('Custom Docker application','Dockerfile detected. Review build context and exposed port.',true);
 else if(pkg.workspaces||has('pnpm-workspace.yaml'))set('Multiple technologies','Workspace / monorepo configuration detected.',true);
 else if(has('wp-load.php')||has('wp-config.php')||Object.keys(tree).some(p=>p.startsWith('app/'+prefix+'wp-content/')))set('WordPress','WordPress source structure detected.');
 else if(has('package.json')){
  if(deps.next)set('Next.js','Next.js dependency detected.');
  else if(deps.express)set('Node.js / Express','Express dependency detected.');
  else if(deps.vite||deps.react||deps.vue||deps.svelte||deps['@sveltejs/kit'])set('Frontend build','Frontend framework detected; confirm output or server adapter.',true);
  else set('Node.js application','package.json detected.',!Object.keys(pkg).length);
  const declared=typeof pkg.packageManager==='string'?pkg.packageManager.split('@')[0]:'';
  result.manager=['npm','pnpm','yarn','bun'].includes(declared)?declared:has('pnpm-lock.yaml')?'pnpm':has('yarn.lock')?'yarn':has('bun.lock')||has('bun.lockb')?'bun':'npm';
  result.install=result.manager==='npm'&&has('package-lock.json')?'npm ci':result.manager+' install';
  result.build=typeof scripts.build==='string'&&scripts.build.trim()?result.manager+' run build':'';
  result.start=typeof scripts.start==='string'&&scripts.start.trim()?result.manager+' run start':'';
  const entries=[...new Set([typeof pkg.main==='string'?pkg.main.replace(/^\.\//,''):'','server.js','index.js','app.js','server.mjs','index.mjs'])].filter(n=>n&&validPath(n)&&/^[A-Za-z0-9_./-]+$/.test(n)&&has(n));
  if(!result.start&&!['Next.js','Frontend build'].includes(result.framework)&&entries.length===1)result.start='node ./'+entries[0];
  if(result.framework==='Next.js'){result.port=3000;result.ram=512;result.cpu=50;}
  if((result.framework==='Next.js'&&!result.build)||(!result.start&&result.framework!=='Frontend build')){result.confidence='Review required';result.reason+=' Required build/start configuration could not be confirmed.';}
  if(result.framework==='Frontend build')result.output='dist';
  for(const filename of ['server.js','index.js','app.js','server.mjs','index.mjs','server.ts']){const m=read(filename).match(/(?:\.listen\(\s*|process\.env\.PORT\s*\|\|\s*)(\d{2,5})\b/);if(m&&Number(m[1])<=65535&&Number(m[1])>0){result.port=Number(m[1]);break;}}
 }else if(has('artisan')||composer.require?.['laravel/framework']){set('Laravel','Laravel application detected; confirm PHP runtime.',true);result.manager='composer';result.install='composer install --no-dev';result.output='public';result.port=80;result.ram=256;}
 else if(has('composer.json')||has('index.php')){set('PHP','PHP source detected.',has('composer.json'));result.manager=has('composer.json')?'composer':'';result.install=result.manager?'composer install --no-dev':'';result.output='.';result.port=80;}
 else if(has('requirements.txt')||has('pyproject.toml')||has('manage.py')){const t=read('requirements.txt')+' '+read('pyproject.toml');set('Python',(/fastapi/i.test(t)?'FastAPI':/django/i.test(t)||has('manage.py')?'Django':/flask/i.test(t)?'Flask':'Python')+' project detected; confirm production start command.',true);result.manager='pip';result.install=has('requirements.txt')?'pip install -r requirements.txt':'';}
 else if(has('go.mod')){set('Go','go.mod detected; confirm build package.',true);result.build='go build -o app .';result.start='./app';}
 else if(has('Cargo.toml')){set('Rust','Cargo.toml detected; confirm executable name.',true);result.build='cargo build --release';}
 else if(has('index.html')){set('Static website','HTML entry point detected.');result.output='.';result.port=80;result.ram=64;}
 if(result.framework==='WordPress'){result.port=80;result.ram=512;result.cpu=50;result.output='.';result.databaseHint=true;}
 const hint=['package.json','composer.json','requirements.txt','pyproject.toml','.env.example','.env.sample'].map(read).join('\n');result.databaseHint=result.databaseHint||/\b(pg|mysql2?|mariadb|prisma|sequelize|typeorm|psycopg2?|sqlalchemy|pymysql|DATABASE_URL|DB_HOST|DB_CONNECTION|MONGODB_URI)\b/i.test(hint);
 for(const n of ['.env.example','.env.sample'])for(const line of read(n).split(/\r?\n/)){const match=line.match(/^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=/);if(match&&!result.environmentKeys.includes(match[1])&&result.environmentKeys.length<50)result.environmentKeys.push(match[1]);}
 return result;
}
/** @param {Tree} tree @param {string} [root] */
export function detect(tree,root){if(root!==undefined){if(root!=='.'&&!validPath(root))throw Error('Invalid source root.');if(root!=='.'&&tree['app/'+root]?.kind!=='folder')throw Error('Source root does not exist.');return detectAt(tree,root);}
 const top=detectAt(tree,'.');if(top.framework!=='Not detected'&&top.framework!=='Static website')return top;
 const dirs=[...new Set(Object.keys(tree).filter(p=>tree[p].kind==='file'&&p.startsWith('app/')).map(p=>p.slice(4).split('/')).filter(parts=>parts.length>1&&parts.length<=6&&signals.has(parts.at(-1)||'')&&!parts.slice(0,-1).some(x=>ignored.has(x))).map(parts=>parts.slice(0,-1).join('/')))].sort((a,b)=>a.split('/').length-b.split('/').length||a.localeCompare(b));
 const roots=dirs.filter(d=>!dirs.some(x=>x!==d&&d.startsWith(x+'/')));
 if(top.framework==='Static website'&&roots.length)return {...top,framework:'Multiple technologies',reason:'HTML entry point and nested application directories found. Review the source root.',confidence:'Review required',candidates:['.',...roots]};
 if(roots.length===1){const found=detectAt(tree,roots[0]);found.reason+=' Found in '+roots[0]+'.';found.candidates=roots;return found;}
 if(roots.length>1)return {...top,framework:'Multiple technologies',reason:'Multiple application directories found. Select one source root for review.',candidates:roots};
 return top;
}
/** @param {Detection} config @param {Tree} tree */
export function validateConfig(config,tree){if(!frameworks.includes(config.framework))throw Error('Select a supported application type.');if(config.root!=='.'&&(!validPath(config.root)||tree['app/'+config.root]?.kind!=='folder'))throw Error('Select an existing source root.');if(!Number.isInteger(config.port)||config.port<1||config.port>65535||!Number.isInteger(config.ram)||config.ram<64||config.ram>65536||!Number.isInteger(config.cpu)||config.cpu<1||config.cpu>100)throw Error('Enter valid port, memory and CPU values.');if([config.install,config.build,config.start,config.output].some(x=>typeof x!=='string'||x.length>500||/[\x00\r\n]/.test(x)))throw Error('Use single-line deployment settings (up to 500 characters).');if(config.output&&config.output!=='.'&&!validPath(config.output))throw Error('Output directory must be a relative path.');if(config.environmentKeys.length>50||new Set(config.environmentKeys).size!==config.environmentKeys.length||config.environmentKeys.some(k=>!/^[A-Za-z_][A-Za-z0-9_]*$/.test(k)))throw Error('Environment keys must be unique valid identifiers.');return config;}
/** @param {Tree} tree @param {string} root @returns {Tree} */
export function projectTree(tree,root){if(root==='app')return tree;/** @type {Tree} */const out={};for(const [p,value] of Object.entries(tree))out[root+p.slice(3)]=structuredClone(value);return out;}
