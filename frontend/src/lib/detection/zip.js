import {validPath,BATCH_LIMIT,MAX_ENTRIES} from '../files/model.js';
const EOCD=0x06054b50, CENTRAL=0x02014b50, LOCAL=0x04034b50;
/** @param {Uint8Array} data */
export function crc32(data){let crc=0xffffffff;for(const b of data){crc^=b;for(let i=0;i<8;i++)crc=(crc>>>1)^((crc&1)?0xedb88320:0);}return (crc^0xffffffff)>>>0;}
/** @param {Uint8Array} raw @param {number} expected */
async function inflate(raw,expected){if(typeof DecompressionStream==='undefined')throw Error('ZIP decompression is not supported in this browser. Choose an extracted folder.');let stream;try{stream=new Blob([new Uint8Array(raw).buffer]).stream().pipeThrough(new DecompressionStream('deflate-raw'));}catch{throw Error('Deflate ZIP extraction is not supported in this browser. Choose an extracted folder.');}const reader=stream.getReader();const out=new Uint8Array(expected);let used=0;try{while(true){const {done,value}=await reader.read();if(done)break;if(used+value.length>expected)throw Error('ZIP expanded beyond declared size.');out.set(value,used);used+=value.length;}}finally{await reader.cancel().catch(()=>{});reader.releaseLock();}if(used!==expected)throw Error('ZIP size does not match its directory.');return out;}
/** @param {Uint8Array} bytes @param {(raw:Uint8Array,expected:number)=>Promise<Uint8Array>} [decompress] */
export async function unpackZip(bytes,decompress=inflate){
 if(bytes.length<22||bytes.length>BATCH_LIMIT)throw Error('Invalid ZIP or archive exceeds the 1 MiB demo limit.');
 const v=new DataView(bytes.buffer,bytes.byteOffset,bytes.byteLength),u16=(/** @type {number} */p)=>v.getUint16(p,true),u32=(/** @type {number} */p)=>v.getUint32(p,true);
 let end=-1;for(let i=bytes.length-22;i>=Math.max(0,bytes.length-65557);i--){if(u32(i)===EOCD&&i+22+u16(i+20)===bytes.length){end=i;break;}}
 if(end<0)throw Error('ZIP central directory not found.');
 const count=u16(end+10),length=u32(end+12),start=u32(end+16);
 if(u16(end+4)||u16(end+6)||u16(end+8)!==count||count===65535||count>MAX_ENTRIES||start+length!==end)throw Error('Multi-volume, ZIP64 or oversized archives are unsupported.');
 /** @type {{path:string,data:Uint8Array}[]} */const output=[];
 /** @type {Map<string,boolean>} */const names=new Map();
 /** @type {[number,number][]} */const ranges=[];
 let p=start,total=0;
 const decoder=new TextDecoder('utf-8',{fatal:true});
 for(let i=0;i<count;i++){
  if(p+46>end||u32(p)!==CENTRAL)throw Error('Corrupt ZIP directory.');
  const flags=u16(p+8),method=u16(p+10),crc=u32(p+16),compressed=u32(p+20),expanded=u32(p+24),nl=u16(p+28),extra=u16(p+30),comment=u16(p+32),external=u32(p+38),offset=u32(p+42),next=p+46+nl+extra+comment;
  if(next>end||u16(p+34)!==0||(flags&1)||(method!==0&&method!==8)||compressed===0xffffffff||expanded===0xffffffff)throw Error('Encrypted, split, ZIP64 or unsupported compression entry.');
  const rawName=bytes.subarray(p+46,p+46+nl);let name;try{name=decoder.decode(rawName);}catch{throw Error('ZIP filenames must be UTF-8 or ASCII.');}
  const folder=name.endsWith('/'),path=folder?name.slice(0,-1):name,type=(external>>>16)&0xf000;
  if(!validPath(path)||path.length>1024||names.has(path)||type===0xa000||(type!==0&&type!==0x8000&&type!==0x4000)||(type===0x4000&&!folder)||(type===0x8000&&folder))throw Error('Unsafe, duplicate or unsupported ZIP entry: '+path);
  for(const [prev,dir] of names){if((!dir&&path.startsWith(prev+'/'))||(!folder&&prev.startsWith(path+'/')))throw Error('ZIP file/folder conflict.');}
  names.set(path,folder);
  total+=expanded;if(total>BATCH_LIMIT)throw Error('Expanded ZIP exceeds the 1 MiB browser demo limit.');
  if(offset+30>start||u32(offset)!==LOCAL||u16(offset+6)!==flags||u16(offset+8)!==method)throw Error('Invalid ZIP local header.');
  const localNameLen=u16(offset+26),dataStart=offset+30+localNameLen+u16(offset+28),dataEnd=dataStart+compressed;
  if(dataStart>start||dataEnd>start||localNameLen!==nl||!rawName.every((b,j)=>bytes[offset+30+j]===b)||ranges.some(([a,b])=>offset<b&&dataEnd>a))throw Error('Conflicting ZIP local entry.');
  ranges.push([offset,dataEnd]);
  if(folder){if(expanded!==0)throw Error('Invalid directory entry.');}
  else{const raw=bytes.subarray(dataStart,dataEnd);const data=method===0?new Uint8Array(raw):await decompress(raw,expanded);if(data.length!==expanded||crc32(data)!==crc)throw Error('ZIP checksum or size mismatch: '+path);output.push({path,data});}
  p=next;
 }
 if(p!==end||!output.length)throw Error('ZIP has no files or an invalid directory.');
 return output;
}
