import './style.css'

interface Callsign { value:string; start:number; end:number; confidence:number }
interface DecodeMessage { sequence:number; signalUtc:string; receivedUtc:string; snr:number; dt:number; frequencyHz:number; message:string; callsigns?:Callsign[]; confidence:number; jttyMessageId?:number; jttyTerminal?:string }
interface WaterfallFrame { timestamp:string; startHz:number; binHz:number; power:number[] }
interface AudioDevice { id:string; name:string; isInput:boolean; isOutput:boolean; sampleRates?:number[] }
interface RadioMeterCapabilities { strength:boolean; alc:boolean; powerWatts:boolean; powerPercent:boolean; swr:boolean }
interface RadioMeterValues { rxDb:number|null; alc:number|null; powerWatts:number|null; powerPercent:number|null; swr:number|null; transmitting:boolean }
interface HamlibCapabilities { modelId:number; portType:string; pttType:string; hasCATPTT:boolean; hasCATPTTMicData:boolean; hasCATIndirectSerialPTT:boolean; asynchronous:boolean; hasGetFreq:boolean; hasSetFreq:boolean; hasGetMode:boolean; hasSetMode:boolean; hasGetPTT:boolean; hasSetPTT:boolean; hasGetSplitVFO:boolean; hasSetSplitVFO:boolean; hasGetSplitFreq:boolean; hasSetSplitFreq:boolean; hasGetSplitMode:boolean; hasSetSplitMode:boolean; supportedModes?:string[] }
interface HamlibModel { modelId:number; label:string; manufacturer?:string; model?:string; version?:string; status?:string }
interface QSOListItem { key:string; utc:string; frequencyHz:number; message:string; callsigns?:Callsign[]; tx?:boolean; sequence:number }
interface MacroConfig { id:string; name:string; shortcut:string; template:string; enabled:boolean; global:boolean }
interface FrequencyBandSetting { band:string; frequencies:number[]; defaultHz:number }
type RadioProfile = Omit<RadioSettings, "backend"|"enabled"|"configVersion"|"local"|"net">
interface RadioSettings {
 configVersion?:number; local?:RadioProfile; net?:RadioProfile;
  backend:string; enabled:boolean; rigName:string; rigModelId:number; rigctldPath:string; host:string; port:number; serialPort:string;
  baud:number; dataBits:number; stopBits:number; handshake:string; pttMethod:string; pttSerialPort:string;
  splitMode:string; txAudioSource:string; forceDTR:string; forceRTS:string; pollIntervalSec:number; mode:string; passbandHz:number; readPowerSWR:boolean; haltOnSWR:boolean
}
interface Settings {
  myCall:string; myGrid:string
  audio:{inputDeviceId:string;outputDeviceId:string;sampleRate:number;inputChannel:string;outputChannel:string;bufferMs:number;txAudioLevel:number;recordDirectory:string}
  radio:RadioSettings
  duplexMode:boolean
  waterfall:{minHz:number;maxHz:number;fft:number;hop:number}
  frequencies:FrequencyBandSetting[]
  decoder:{threads:number;mode:string;thresholdDb:number;frequencyToleranceHz:number;trackerToleranceHz:number;trackerTtlMs:number}
  layout:{waterfallHeight:number;operationHeight:number;windowWidth:number;windowHeight:number}
  macros:MacroConfig[]; logbookPath:string; decodeLogMode:string; decodeLogEnabled:boolean; recordEnabled:boolean; decodeWindowLimit:number; autoFollowTail:boolean; autoStartMonitor:boolean; exchangeSerialNumber:number; rxFrequencyHz:number; txFrequencyHz:number
}


const DEFAULT_FREQUENCY_BANDS:FrequencyBandSetting[]=[
  {band:'160m',frequencies:[1838000],defaultHz:1838000},
  {band:'80m',frequencies:[3575000],defaultHz:3575000},
  {band:'60m',frequencies:[5357000],defaultHz:5357000},
  {band:'40m',frequencies:[7090000],defaultHz:7090000},
  {band:'30m',frequencies:[10140000],defaultHz:10140000},
  {band:'20m',frequencies:[14090000],defaultHz:14090000},
  {band:'17m',frequencies:[18100000],defaultHz:18100000},
  {band:'15m',frequencies:[21090000],defaultHz:21090000},
  {band:'12m',frequencies:[24920000],defaultHz:24920000},
  {band:'10m',frequencies:[28090000],defaultHz:28090000},
  {band:'6m',frequencies:[50316000],defaultHz:50316000},
  {band:'2m',frequencies:[144077000],defaultHz:144077000},
  {band:'70cm',frequencies:[432077000],defaultHz:432077000}
]
function cloneDefaultFrequencyBands(){return DEFAULT_FREQUENCY_BANDS.map(x=>({...x,frequencies:[...x.frequencies]}))}
function normalizeFrequencyBandsFrontend(v:any):FrequencyBandSetting[]{
  const raw=Array.isArray(v)?v:[];const byBand=new Map<string,FrequencyBandSetting>();const custom:FrequencyBandSetting[]=[]
  for(const item of raw){if(!item||typeof item.band!=='string')continue;const band=item.band.trim();if(!band)continue;const key=band.toLowerCase();if(byBand.has(key))continue;let values=Array.isArray(item.frequencies)?item.frequencies.map((n:number)=>Number(n)).filter((n:number)=>Number.isFinite(n)&&n>0&&n<=2e10):[];values=[...new Set(values.map((n:number)=>Math.round(n)))];let def=Number(item.defaultHz);if(!Number.isFinite(def)||def<=0||!values.includes(Math.round(def)))def=values[0]||0;if(!values.length)def=0;const normalized={band,frequencies:values,defaultHz:Math.round(def)};byBand.set(key,normalized)}
  const out:FrequencyBandSetting[]=[]
  for(const d of cloneDefaultFrequencyBands()){const b=byBand.get(d.band.toLowerCase());if(!b||!b.frequencies.length)out.push(d);else out.push(b);byBand.delete(d.band.toLowerCase())}
  for(const item of raw){const key=String(item?.band||'').trim().toLowerCase();if(key&&byBand.has(key)){custom.push(byBand.get(key)!);byBand.delete(key)}}
  return out.concat(custom)
}
function formatFrequencyListMHz(values:number[]){return values.map(hz=>(hz/1e6).toFixed(6)).join(', ')}
function parseFrequencyListMHz(text:string){const out:number[]=[];for(const token of text.split(/[,;\s]+/)){const mhz=Number(token.trim());if(!Number.isFinite(mhz)||mhz<=0)continue;const hz=Math.round(mhz*1e6);if(hz>0&&!out.includes(hz))out.push(hz)}return out}

const rows=new Map<string,DecodeMessage>()
const decodeRowElements=new Map<string,HTMLDivElement>()
const qsoRows:QSOListItem[]=[]
let qsoSequence=0
let followTail=true
let newSinceScroll=0
let waterfallFrame:WaterfallFrame|null=null
let rxRunning=false
let receiverBusy=false
let radioConnected=false
let rxFrequency=1500
let txFrequency=1500
let duplexMode=false
let renderWaterfall:(()=>void)|null=null
let settingsState:Settings=defaultSettings()
let settingsDraft:Settings=settingsState
let settingsTab='general'
let closeAfterSettingsSave=false
let audioDevices:{inputs:AudioDevice[];outputs:AudioDevice[]}={inputs:[],outputs:[]}
let hamlibModels:HamlibModel[]=[]
let hamlibCapabilities:HamlibCapabilities|null=null
let hamlibCapabilitiesModelId=0
let hamlibCapabilitiesPendingModelId=0
let serialPorts:string[]=[]
let dialFrequency=0
let dialMode=''
let dxGrid=''
let activeBand='20'
let recordState={enabled:false,active:false,directory:'',path:''}
let qsoStarts=new Map<string,string>()
let radioMeterCapabilities:RadioMeterCapabilities={strength:false,alc:false,powerWatts:false,powerPercent:false,swr:false}
let radioMeterValues:RadioMeterValues={rxDb:null,alc:null,powerWatts:null,powerPercent:null,swr:null,transmitting:false}
let audioInputDbfs=-120


// Temporarily disabled while the Hamlib radio path is being redesigned. Keep the implementation for later re-enablement.
const RADIO_SETTINGS_UI_ENABLED = true

const LAYOUT_LIMITS = {
  waterfallMin: 150, waterfallMax: 340,
  operationHeight: 270,
  decodeMin: 190,
  splitter: 6
}
let layoutSaveTimer:number|undefined
let windowSizeSaveTimer:number|undefined
let txLevelSaveTimer:number|undefined
let pttTestCountdownTimer:number|undefined
let pttTestCountdownDeadline=0

function clampLayoutValues(wf:number, _op:number){
  const viewport=document.querySelector<HTMLElement>('.shell')
  const menu=document.querySelector<HTMLElement>('.menu')
  const footer=document.querySelector<HTMLElement>('.shell>footer')
  const available=Math.max(0,(viewport?.clientHeight||820)-(menu?.clientHeight||34)-(footer?.clientHeight||25)-LAYOUT_LIMITS.splitter)
  const adaptiveMax=Math.min(LAYOUT_LIMITS.waterfallMax,Math.max(LAYOUT_LIMITS.waterfallMin,available-LAYOUT_LIMITS.decodeMin-LAYOUT_LIMITS.operationHeight))
  const waterfall=Math.max(LAYOUT_LIMITS.waterfallMin,Math.min(adaptiveMax,Math.round(wf)))
  return {waterfallHeight:waterfall,operationHeight:LAYOUT_LIMITS.operationHeight}
}

function applyLayout(){
  const shell=document.querySelector<HTMLElement>('.shell'); if(!shell)return
  const c=clampLayoutValues(settingsState.layout.waterfallHeight,settingsState.layout.operationHeight)
  settingsState.layout={...settingsState.layout,...c}
  shell.style.setProperty('--waterfall-height',`${c.waterfallHeight}px`)
  shell.style.setProperty('--operation-height',`${c.operationHeight}px`)
}

function scheduleLayoutSave(){
  window.clearTimeout(layoutSaveTimer)
  layoutSaveTimer=window.setTimeout(()=>{
    settingsState.layout={...settingsState.layout,...clampLayoutValues(settingsState.layout.waterfallHeight,settingsState.layout.operationHeight)}
    emitBackendEvent('settings:save',settingsState)
  },400)
}


function $(sel:string){return document.querySelector(sel)!}
function esc(s:string){return String(s??'').replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;')}
function num(v:string|number,fallback:number){const n=Number(v);return Number.isFinite(n)?n:fallback}
function cloneSettings(v:Settings):Settings{return structuredClone(v)}
function runtimeApi(){return (window as any).runtime ?? (window as any).go?.runtime}
function emitBackendEvent(name:string,payload:any){const runtime=runtimeApi();if(runtime?.EventsEmit){runtime.EventsEmit(name,payload);return true}return false}

function defaultSettings():Settings{
  return {
    myCall:'',myGrid:'',
    audio:{inputDeviceId:'',outputDeviceId:'',sampleRate:48000,inputChannel:'Mono',outputChannel:'Mono',bufferMs:20,txAudioLevel:65,recordDirectory:''},
    radio:{backend:'none',enabled:false,rigName:'None',rigModelId:0,rigctldPath:'',host:'127.0.0.1',port:4532,serialPort:'',baud:9600,dataBits:8,stopBits:1,handshake:'None',pttMethod:'VOX',pttSerialPort:'',splitMode:'Rig',txAudioSource:'Front',forceDTR:'none',forceRTS:'none',pollIntervalSec:1,mode:'PKTUSB',passbandHz:3000,readPowerSWR:false,haltOnSWR:false},
    duplexMode:false,
    waterfall:{minHz:0,maxHz:2700,fft:4096,hop:1024},
    frequencies:cloneDefaultFrequencyBands(),
    decoder:{threads:0,mode:'realtime',thresholdDb:8,frequencyToleranceHz:20,trackerToleranceHz:20,trackerTtlMs:600},
    layout:{waterfallHeight:210,operationHeight:270,windowWidth:1360,windowHeight:820},
    macros:[
      {id:'F1',name:'CQ',shortcut:'F1',template:'CQ %M CQ',enabled:true,global:true},
      {id:'F2',name:'Exchange',shortcut:'F2',template:'%H %E',enabled:true,global:true},
      {id:'F3',name:'TU + CQ',shortcut:'F3',template:'%H TU CQ %M CQ',enabled:true,global:true},
      {id:'F4',name:'My Call',shortcut:'F4',template:'%M',enabled:true,global:true},
      {id:'F5',name:'His Call',shortcut:'F5',template:'%H',enabled:true,global:true},
      {id:'F6',name:'TU Now',shortcut:'F6',template:'TU NOW %Q %E',enabled:true,global:true},
      {id:'F7',name:'Again',shortcut:'F7',template:'%H AGN?',enabled:true,global:true},
      {id:'F8',name:'Exchange',shortcut:'F8',template:'%E',enabled:true,global:true}
    ],
    logbookPath:'JTTY.adi',decodeLogMode:'single',decodeLogEnabled:true,recordEnabled:false,decodeWindowLimit:2000,autoFollowTail:true,autoStartMonitor:false,exchangeSerialNumber:1,rxFrequencyHz:1500,txFrequencyHz:1500
  }
}

function normalizePTTMethod(value:any){const method=String(value||'VOX').trim().toUpperCase();if(method==='CAT'||method==='RIGCTL')return 'Rigctl';if(method==='DTR'||method==='RTS')return method;return 'VOX'}

function mergeSettings(v:any):Settings{
  const d=defaultSettings();if(!v||typeof v!=='object')return d
  return {...d,...v,
    audio:{...d.audio,...(v.audio||{})},radio:{...d.radio,...(v.radio||{}),enabled:(typeof v.radio?.enabled==='boolean'?v.radio.enabled:['rigctld-net','hamlib-net-rigctl','hamlib-rigctld'].includes(String(v.radio?.backend||'').toLowerCase())),pttMethod:normalizePTTMethod(v.radio?.pttMethod??d.radio.pttMethod)},waterfall:{...d.waterfall,...(v.waterfall||{}),minHz:0,maxHz:[2500,2700,3000,3500,4000].includes(Number(v.waterfall?.maxHz))?Number(v.waterfall.maxHz):2700,fft:4096,hop:1024},frequencies:normalizeFrequencyBandsFrontend(v.frequencies),decoder:{...d.decoder,...(v.decoder||{})},layout:{...d.layout,...(v.layout||{}),operationHeight:LAYOUT_LIMITS.operationHeight,windowWidth:Number(v.layout?.windowWidth)>0?Math.round(Number(v.layout.windowWidth)):d.layout.windowWidth,windowHeight:Number(v.layout?.windowHeight)>0?Math.round(Number(v.layout.windowHeight)):d.layout.windowHeight},
    macros:Array.isArray(v.macros)?v.macros:d.macros,decodeLogMode:typeof v.decodeLogMode==='string'?v.decodeLogMode:d.decodeLogMode,decodeLogEnabled:typeof v.decodeLogEnabled==='boolean'?v.decodeLogEnabled:d.decodeLogEnabled,recordEnabled:typeof v.recordEnabled==='boolean'?v.recordEnabled:d.recordEnabled,exchangeSerialNumber:Number(v.exchangeSerialNumber)>0?Math.floor(Number(v.exchangeSerialNumber)):d.exchangeSerialNumber,rxFrequencyHz:Number(v.rxFrequencyHz)>=200&&Number(v.rxFrequencyHz)<=5000?Math.round(Number(v.rxFrequencyHz)):d.rxFrequencyHz,txFrequencyHz:Number(v.txFrequencyHz)>=200&&Number(v.txFrequencyHz)<=5000?Math.round(Number(v.txFrequencyHz)):d.txFrequencyHz} as Settings
}

function setDX(call:string,notifyBackend=true){const normalized=String(call||'').trim().toUpperCase();const dx=document.querySelector<HTMLInputElement>('#dx');if(dx)dx.value=normalized;if(notifyBackend)emitBackendEvent('qso:dxcall',normalized);return normalized}
function setTXAudioLevel(level:number, notifyBackend=true){
  let n=Math.round(level)
  if(!Number.isFinite(n)) n=65
  n=Math.max(0,Math.min(100,n))
  settingsState.audio.txAudioLevel=n
  const slider=document.querySelector<HTMLInputElement>('#tx-level'); if(slider) slider.value=String(n)
  const label=document.querySelector('#tx-level-value'); if(label) label.textContent=`${n}%`
  const settingsInput=document.querySelector<HTMLInputElement>('#set-tx-level'); if(settingsInput) settingsInput.value=String(n)
  if(notifyBackend){
    window.clearTimeout(txLevelSaveTimer)
    txLevelSaveTimer=window.setTimeout(()=>emitBackendEvent('audio:set-tx-level',n),200)
  }
}

function formatDialFrequency(hz:number){return hz>0?(hz/1e6).toFixed(6):''}
function formatDialDisplay(hz:number){if(hz<=0)return '—';const parts=(hz/1e6).toFixed(6).split('.');const frac=parts[1]||'000000';return `${parts[0]}.${frac.slice(0,3)} ${frac.slice(3)}`}
function setDialFrequency(hz:number){dialFrequency=Math.max(0,Math.round(hz));const e=document.querySelector<HTMLInputElement>('#dial-freq');if(e)e.value=dialFrequency>0?formatDialDisplay(dialFrequency):'';const out=document.querySelector('#dial-freq-display');if(out)out.textContent=dialFrequency>0?formatDialDisplay(dialFrequency):'—';const mode=document.querySelector('#dial-mode');if(mode)mode.textContent=dialMode||'—'}
function applyRXFrequency(hz:number, notifyBackend=true){
  rxFrequency=Math.round(Math.max(200,Math.min(5000,hz)));
  settingsState.rxFrequencyHz=rxFrequency
  const main=document.querySelector('#rx-freq'); if(main) main.textContent=String(rxFrequency);
  const bottom=document.querySelector('#rx-freq-bottom'); if(bottom) bottom.textContent=`RX ${rxFrequency}`;
  const rb=document.querySelector<HTMLInputElement>('#op-rx-freq'); if(rb) rb.value=String(rxFrequency);
  updateWaterfallMarkers();
  renderQSOList();
  if(notifyBackend) emitBackendEvent('frequency:set-rx',rxFrequency);
}
function applyTXFrequency(hz:number, notifyBackend=true){
  txFrequency=Math.round(Math.max(200,Math.min(5000,hz)));
  settingsState.txFrequencyHz=txFrequency
  const main=document.querySelector('#tx-freq'); if(main) main.textContent=String(txFrequency);
  const bottom=document.querySelector('#tx-freq-bottom'); if(bottom) bottom.textContent=`TX ${txFrequency}`;
  const rb=document.querySelector<HTMLInputElement>('#op-tx-freq'); if(rb) rb.value=String(txFrequency);
  updateWaterfallMarkers();
  if(notifyBackend) emitBackendEvent('frequency:set-tx',txFrequency);
}
function normalizeRangeMax(v:number){const n=Math.round(v);return [2500,2700,3000,3500,4000].includes(n)?n:2700}
function updateFrequencyRangeMenu(){const popup=document.querySelector('#range-popup');if(!popup)return;popup.querySelectorAll<HTMLButtonElement>('[data-range-max]').forEach(b=>b.classList.toggle('active',Number(b.dataset.rangeMax)===settingsState.waterfall.maxHz))}
function setFrequencyRange(maxHz:number){maxHz=normalizeRangeMax(maxHz);settingsState.waterfall.minHz=0;settingsState.waterfall.maxHz=maxHz;emitBackendEvent('frequency-range:set-max',maxHz);renderWaterfall?.();updateFrequencyRangeMenu();setToast(`频率范围 0–${maxHz} Hz`);document.querySelector('#range-popup')?.classList.remove('open')}
function applyTolerance(hz:number, notifyBackend=true){
  const values=[2,5,10,20,50,100,150,200,250,300,350,400,450,500]
  let n=Math.round(hz); if(!Number.isFinite(n))n=20
  n=Math.max(1,Math.min(500,n))
  const input=document.querySelector<HTMLInputElement>('#tolerance'); if(input) input.value=String(n)
  const settingsInput=document.querySelector<HTMLInputElement>('#set-ftol'); if(settingsInput) settingsInput.value=String(n)
  settingsState.decoder.frequencyToleranceHz=n
  if(notifyBackend) emitBackendEvent('jtty:set-tolerance',n)
  return values
}
function setRXFrequency(hz:number){applyRXFrequency(hz,true)}
function setTXFrequency(hz:number){applyTXFrequency(hz,true)}

function clamp01(v:number){return Math.max(0,Math.min(1,v))}
function finiteOrNull(v:any){const n=Number(v);return v===null||v===undefined||!Number.isFinite(n)?null:n}
function setMeter(id:string, percent:number, value:string, unavailable=false){const root=document.querySelector<HTMLElement>(`#${id}`);if(!root)return;const fill=root.querySelector<HTMLElement>('.meter-fill');const text=root.querySelector<HTMLElement>('.meter-value');if(fill)fill.style.width=`${Math.round(Math.max(0,Math.min(100,percent)))}%`;if(text)text.textContent=value;root.classList.toggle('meter-unavailable',unavailable)}
function audioDbfsPercent(db:number){return clamp01((db+60)/60)*100}
function updateMeters(){const rv=radioMeterValues;let p=audioDbfsPercent(audioInputDbfs),t=`${audioInputDbfs.toFixed(1)} dBFS`;if(rv.transmitting&&rv.alc!==null){p=clamp01(rv.alc)*100;t=`${(clamp01(rv.alc)*100).toFixed(0)} % ALC`}else if(!rv.transmitting&&rv.rxDb!==null){p=clamp01((rv.rxDb+60)/120)*100;t=`${rv.rxDb.toFixed(0)} dB`};setMeter('meter-rx-alc',p,t,false);const ps=radioMeterCapabilities.powerWatts||radioMeterCapabilities.powerPercent;if(rv.powerWatts!==null&&radioMeterCapabilities.powerWatts)setMeter('meter-power',Math.min(100,rv.powerWatts),`${rv.powerWatts.toFixed(rv.powerWatts>=100?0:1)} W`,false);else if(rv.powerPercent!==null&&radioMeterCapabilities.powerPercent)setMeter('meter-power',clamp01(rv.powerPercent)*100,`${(clamp01(rv.powerPercent)*100).toFixed(0)} %`,false);else setMeter('meter-power',0,'—',!ps);if(rv.swr!==null&&radioMeterCapabilities.swr){const swr=Math.max(1,rv.swr);setMeter('meter-swr',clamp01((swr-1)/2)*100,swr.toFixed(2),false)}else setMeter('meter-swr',0,'—',!radioMeterCapabilities.swr)}
function resetRadioMeters(){radioMeterCapabilities={strength:false,alc:false,powerWatts:false,powerPercent:false,swr:false};radioMeterValues={rxDb:null,alc:null,powerWatts:null,powerPercent:null,swr:null,transmitting:false};updateMeters()}

function decodeSortTime(m:DecodeMessage){
  const t=Date.parse(m.signalUtc||m.receivedUtc||'')
  return Number.isFinite(t)?t:Number.MAX_SAFE_INTEGER
}
function decodeRowKey(m:DecodeMessage){
  return m.jttyMessageId!==undefined?`jtty:${m.jttyMessageId}`:`seq:${m.sequence}`
}
function insertDecodeRowSorted(list:HTMLElement,div:HTMLElement,time:number,sequence:number){
  const oldTime=Number(div.dataset.sortTime),oldSequence=Number(div.dataset.sortSequence)
  div.dataset.sortTime=String(time)
  div.dataset.sortSequence=String(sequence)
  if(div.parentElement===list && oldTime===time && oldSequence===sequence)return
  const children=list.children
  let before:Element|null=null
  for(let i=0;i<children.length;i++){
    const node=children[i] as HTMLElement
    if(node===div)continue
    const otherTime=Number(node.dataset.sortTime)
    const otherSequence=Number(node.dataset.sortSequence)
    if(!Number.isFinite(otherTime))continue
    if(otherTime>time||(otherTime===time&&otherSequence>sequence)){before=node;break}
  }
  if(before)list.insertBefore(div,before)
  else list.appendChild(div)
}
function renderMessage(m:DecodeMessage,root:Element,targetId='decodes'){
  const list=root.querySelector<HTMLDivElement>(`#${targetId}`)!
  const key=decodeRowKey(m)
  let div=decodeRowElements.get(key)
  const isNew=!div
  if(!div){div=document.createElement('div');div.className='decode-row';div.dataset.decodeKey=key;decodeRowElements.set(key,div)}
  if(m.jttyMessageId!==undefined)div.dataset.jttyMessageId=String(m.jttyMessageId)
  div.dataset.sequence=String(m.sequence)
  div.innerHTML='<span class="utc"></span><span class="snr-value"></span><span class="df"></span><span class="message"></span>'
  const dt=new Date(m.signalUtc||m.receivedUtc||Date.now());div.querySelector('.utc')!.textContent=dt.toISOString().slice(11,19);div.querySelector('.snr-value')!.textContent=Number.isFinite(m.snr)?`${m.snr} dB`:'—';div.querySelector('.df')!.textContent=String(m.frequencyHz)
  const message=div.querySelector('.message')!;const calls=m.callsigns||[];let cursor=0
  for(const c of calls){if(c.start>cursor)message.append(document.createTextNode(m.message.slice(cursor,c.start)));const b=document.createElement('button');b.className='call-token';b.textContent=c.value;b.title='设置 DX 呼号';b.addEventListener('click',e=>{e.stopPropagation();setDX(c.value);const g=extractGrid(m.message);if(g){dxGrid=g;const gi=document.querySelector<HTMLInputElement>('#dx-grid');if(gi)gi.value=g;emitBackendEvent('qso:dxgrid',g)}});message.append(b);cursor=c.end}
  if(cursor<m.message.length)message.append(document.createTextNode(m.message.slice(cursor)))
  const states:Record<string,string>={growing:'接收中',complete:'完整',expired:'超时', 'reception-ended':'接收结束'}
  if(m.jttyTerminal&&states[m.jttyTerminal]){const status=document.createElement('small');status.className='jtty-reception-state';status.textContent=states[m.jttyTerminal];status.title=m.jttyTerminal==='expired'?'未收到消息结束标记，保留已接收内容':status.textContent;message.append(status)}
  insertDecodeRowSorted(list,div,decodeSortTime(m),m.sequence)
  if(followTail&&isNew&&targetId==='decodes')scheduleDecodeTail(list)
}
let decodeTailScheduled=false
function scheduleDecodeTail(list:HTMLDivElement){
  if(decodeTailScheduled)return
  decodeTailScheduled=true
  requestAnimationFrame(()=>{decodeTailScheduled=false;list.scrollTop=list.scrollHeight})
}
function trimDecodeRows(root:Element){
  const list=root.querySelector<HTMLDivElement>('#decodes')
  const limit=Math.max(100,Math.round(settingsState.decodeWindowLimit||2000))
  while(rows.size>limit){
    const first=list?.firstElementChild as HTMLElement|null
    if(first){const key=first.dataset.decodeKey;if(key){rows.delete(key);decodeRowElements.delete(key)}first.remove()}
    else {const oldest=rows.keys().next();if(oldest.done)break;rows.delete(oldest.value)}
  }
}
function addRow(m:DecodeMessage,root:Element){
  const key=decodeRowKey(m)
  if(rows.has(key)){updateRow(m,root);return}
  rows.set(key,m);renderMessage(m,root);trimDecodeRows(root)
  if(!followTail){newSinceScroll++;const i=root.querySelector('#new-indicator');if(i)i.textContent=`↓ ${newSinceScroll} 个新解码`}
  const count=root.querySelector('#count');if(count)count.textContent=String(rows.size)
}
function updateRow(m:DecodeMessage,root:Element){
  rows.set(decodeRowKey(m),m)
  renderMessage(m,root)
}
function addQSOTx(message:string,utc:string,frequencyHz:number){
  const hz=Math.round(Number(frequencyHz)||txFrequency);qsoSequence++
  qsoRows.push({key:`tx:${qsoSequence}`,utc,frequencyHz:hz,message,callsigns:[],tx:true,sequence:qsoSequence})
  while(qsoRows.length>64)qsoRows.shift()
  renderQSOList()
}
function renderQSOMessage(m:QSOListItem,msg:HTMLElement){const calls=m.callsigns||[];let cursor=0;for(const c of calls){if(c.start>cursor)msg.append(document.createTextNode(m.message.slice(cursor,c.start)));const b=document.createElement('button');b.className='call-token';b.textContent=c.value;b.title='设置 DX 呼号';b.addEventListener('click',e=>{e.stopPropagation();setDX(c.value);const g=extractGrid(m.message);if(g){dxGrid=g;const gi=document.querySelector<HTMLInputElement>('#dx-grid');if(gi)gi.value=g;emitBackendEvent('qso:dxgrid',g)}});msg.append(b);cursor=c.end}if(cursor<m.message.length)msg.append(document.createTextNode(m.message.slice(cursor)))}
function renderQSOList(){const list=document.querySelector<HTMLDivElement>('#qso-decodes');if(!list)return;list.innerHTML='';const source=qsoRows.filter(r=>r.tx).sort((a,b)=>{const ta=Date.parse(a.utc),tb=Date.parse(b.utc);const va=Number.isFinite(ta)?ta:Number.POSITIVE_INFINITY;const vb=Number.isFinite(tb)?tb:Number.POSITIVE_INFINITY;if(va!==vb)return va-vb;return a.sequence-b.sequence}).slice(-24);for(const m of source){const div=document.createElement('div');div.className=`qso-decode-row${m.tx?' qso-tx-row':''}`;div.dataset.key=m.key;div.innerHTML='<span class="qso-utc"></span><span class="qso-df"></span><span class="qso-msg"></span>';const utc=div.querySelector('.qso-utc')!;const d=new Date(m.utc);utc.textContent=Number.isNaN(d.getTime())?'--:--:--':d.toISOString().slice(11,19);div.querySelector('.qso-df')!.textContent=String(m.frequencyHz);renderQSOMessage(m,div.querySelector('.qso-msg')!);div.addEventListener('click',()=>setTXFrequency(m.frequencyHz));list.append(div)}if(!source.length){const e=document.createElement('div');e.className='qso-empty';e.textContent='—';list.append(e)}}

function updateWaterfallMarkers(){
  renderWaterfall?.()
}

function drawWaterfall(canvas:HTMLCanvasElement, surface:HTMLElement){
  const ctx=canvas.getContext('2d')!;
  let lastW=0,lastH=0;
  const BANDWIDTH_HZ=125;
  let raf=0;
  let resizeObserver:ResizeObserver|null=null;
  let hoverHz:number|null=null;
  let lastRangeKey='';
  let bitmap=document.createElement('canvas');
  let bitmapCtx=bitmap.getContext('2d')!;
  let bitmapRows=0;
  let bitmapRangeKey='';
  let rowImage:ImageData|null=null;
  const maxRows=1024;

  const overlay=document.createElementNS('http://www.w3.org/2000/svg','svg');
  overlay.classList.add('waterfall-overlay');
  overlay.setAttribute('aria-hidden','true');
  overlay.setAttribute('preserveAspectRatio','none');
  overlay.innerHTML=`
    <rect class="wf-axis-bg" x="0" y="0" width="100%" height="24"></rect>
    <g class="wf-axis-ticks"></g>
    <g class="wf-hover">
      <rect class="wf-hover-band" x="0" y="24" width="0" height="0"></rect>
      <line class="wf-hover-line" x1="0" y1="0" x2="0" y2="0"></line>
      <rect class="wf-hover-label-bg" x="0" y="0" width="92" height="20"></rect>
      <text class="wf-hover-label" x="8" y="14">— Hz</text>
    </g>
    <g class="wf-rx">
      <rect class="wf-band wf-band-rx" x="0" y="24" width="0" height="0"></rect>
      <line class="wf-center wf-center-rx" x1="0" y1="0" x2="0" y2="0"></line>
      <text class="wf-marker-label wf-marker-label-rx" x="0" y="57">RX 1500 Hz</text>
    </g>
    <g class="wf-tx">
      <rect class="wf-band wf-band-tx" x="0" y="24" width="0" height="0"></rect>
      <line class="wf-center wf-center-tx" x1="0" y1="0" x2="0" y2="0"></line>
      <text class="wf-marker-label wf-marker-label-tx" x="0" y="74">TX 1500 Hz</text>
    </g>`;
  surface.append(overlay);

  const axisTicks=overlay.querySelector<SVGGElement>('.wf-axis-ticks')!;
  const hover=overlay.querySelector<SVGGElement>('.wf-hover')!;
  const hoverBand=hover.querySelector<SVGRectElement>('.wf-hover-band')!;
  const hoverLine=hover.querySelector<SVGLineElement>('.wf-hover-line')!;
  const hoverLabelBg=hover.querySelector<SVGRectElement>('.wf-hover-label-bg')!;
  const hoverLabel=hover.querySelector<SVGTextElement>('.wf-hover-label')!;
  const rxBand=overlay.querySelector<SVGRectElement>('.wf-band-rx')!;
  const txBand=overlay.querySelector<SVGRectElement>('.wf-band-tx')!;
  const rxLine=overlay.querySelector<SVGLineElement>('.wf-center-rx')!;
  const txLine=overlay.querySelector<SVGLineElement>('.wf-center-tx')!;
  const rxLabel=overlay.querySelector<SVGTextElement>('.wf-marker-label-rx')!;
  const txLabel=overlay.querySelector<SVGTextElement>('.wf-marker-label-tx')!;

  const sizeCanvas=(w:number,h:number)=>{
    const dpr=window.devicePixelRatio||1;
    const pxW=Math.max(1,Math.floor(w*dpr));
    const pxH=Math.max(1,Math.floor(h*dpr));
    if(canvas.width!==pxW||canvas.height!==pxH){canvas.width=pxW;canvas.height=pxH;}
    ctx.setTransform(dpr,0,0,dpr,0,0);
  };

  const hzToX=(hz:number,w:number)=>{
    const start=settingsState.waterfall.minHz,end=settingsState.waterfall.maxHz;
    return Math.max(0,Math.min(w,(hz-start)/Math.max(1,end-start)*w));
  };

  const updateAxis=()=>{
    const w=Math.max(1,surface.clientWidth),h=Math.max(1,surface.clientHeight);
    const start=settingsState.waterfall.minHz,end=settingsState.waterfall.maxHz;
    const rangeKey=`${start}:${end}:${Math.round(w)}:${Math.round(h)}`;
    if(rangeKey===lastRangeKey)return;
    lastRangeKey=rangeKey;
    axisTicks.innerHTML='';
    const firstMinor=Math.ceil(start/100)*100;
    for(let hz=firstMinor;hz<=end;hz+=100){
      const x=hzToX(hz,w);
      const major=hz%500===0;
      const line=document.createElementNS('http://www.w3.org/2000/svg','line');
      line.setAttribute('x1',String(x));line.setAttribute('x2',String(x));line.setAttribute('y1','0');line.setAttribute('y2',major?'17':'9');line.setAttribute('class',major?'wf-major-tick':'wf-minor-tick');axisTicks.append(line);
      if(major){const text=document.createElementNS('http://www.w3.org/2000/svg','text');text.setAttribute('x',String(x));text.setAttribute('y','17');text.setAttribute('class','wf-major-label');text.textContent=String(hz);axisTicks.append(text)}
    }
    const leftText=document.createElementNS('http://www.w3.org/2000/svg','text');leftText.setAttribute('x','5');leftText.setAttribute('y','22');leftText.setAttribute('class','wf-edge-label');leftText.textContent=String(start);axisTicks.append(leftText);
    const rightText=document.createElementNS('http://www.w3.org/2000/svg','text');rightText.setAttribute('x',String(w-5));rightText.setAttribute('y','22');rightText.setAttribute('text-anchor','end');rightText.setAttribute('class','wf-edge-label');rightText.textContent=String(end);axisTicks.append(rightText);
  };

  const setLine=(line:SVGLineElement,x:number,h:number)=>{line.setAttribute('x1',String(x));line.setAttribute('x2',String(x));line.setAttribute('y1','0');line.setAttribute('y2',String(h));};
  const setBand=(rect:SVGRectElement,center:number,w:number,h:number)=>{const pxPerHz=w/Math.max(1,settingsState.waterfall.maxHz-settingsState.waterfall.minHz);const bandPx=Math.max(2,BANDWIDTH_HZ*pxPerHz);const x=hzToX(center,w)-bandPx/2;rect.setAttribute('x',String(x));rect.setAttribute('y','24');rect.setAttribute('width',String(bandPx));rect.setAttribute('height',String(Math.max(0,h-30)));};
  const setLabel=(text:SVGTextElement,x:number,label:string,w:number,y:number)=>{const edgeRight=x>w-150;text.setAttribute('x',String(edgeRight?x-7:x+7));text.setAttribute('y',String(y));text.setAttribute('text-anchor',edgeRight?'end':'start');text.textContent=label;};

  const renderOverlay=()=>{
    const w=Math.max(1,surface.clientWidth),h=Math.max(1,surface.clientHeight);overlay.setAttribute('viewBox',`0 0 ${w} ${h}`);overlay.setAttribute('width',String(w));overlay.setAttribute('height',String(h));updateAxis();
    const rxX=hzToX(rxFrequency,w),txX=hzToX(txFrequency,w);setBand(rxBand,rxFrequency,w,h);setBand(txBand,txFrequency,w,h);setLine(rxLine,rxX,h);setLine(txLine,txX,h);setLabel(rxLabel,rxX,`RX ${rxFrequency} Hz`,w,37);setLabel(txLabel,txX,`TX ${txFrequency} Hz`,w,53);
    const same=Math.abs(rxX-txX)<3;rxBand.style.opacity=same?'0.22':'0.30';txBand.style.opacity=same?'0.28':'0.30';rxLine.style.strokeWidth=same?'3':'2';txLine.style.strokeWidth='2';
    if(hoverHz!==null){const hx=hzToX(hoverHz,w);setBand(hoverBand,hoverHz,w,h);setLine(hoverLine,hx,h);const labelW=92;const lx=Math.max(2,Math.min(w-labelW-2,hx+8));hoverLabelBg.setAttribute('x',String(lx));hoverLabel.setAttribute('x',String(lx+8));hoverLabel.textContent=`${Math.round(hoverHz)} Hz`;hover.style.display='block';hoverLabelBg.setAttribute('y',String(Math.max(25,Math.min(h-22,32))));hoverLabel.setAttribute('y',String(Math.max(36,Math.min(h-8,44))));}else hover.style.display='none';
  };

  const writeColorForDb=(data:Uint8ClampedArray,off:number,db:number)=>{
    const stops=[[0,3,8,28],[0.18,4,21,92],[0.38,4,73,175],[0.58,0,165,220],[0.75,27,215,142],[0.88,220,220,62],[1,255,250,220]]
    const t=Math.max(0,Math.min(1,(db+105)/75))
    for(let i=1;i<stops.length;i++){
      if(t<=stops[i][0]){
        const a=stops[i-1],b=stops[i],u=(t-a[0])/(b[0]-a[0]);
        data[off]=Math.round(a[1]+(b[1]-a[1])*u);data[off+1]=Math.round(a[2]+(b[2]-a[2])*u);data[off+2]=Math.round(a[3]+(b[3]-a[3])*u);data[off+3]=255;return
      }
    }
    data[off]=255;data[off+1]=250;data[off+2]=220;data[off+3]=255
  }

  const rowFromFrame=(f:WaterfallFrame,width:number)=>{
    if(!rowImage||rowImage.width!==width){
      rowImage=bitmapCtx.createImageData(width,1)
    }
    const data=rowImage.data;const bins=f.power.length;const start=f.startHz;const bin=Math.max(0.000001,f.binHz);const end=start+bin*Math.max(0,bins-1);const viewStart=settingsState.waterfall.minHz;const viewEnd=settingsState.waterfall.maxHz;const span=Math.max(1,viewEnd-viewStart)
    for(let x=0;x<width;x++){
      const hz=viewStart+((x+0.5)/width)*span;let db=-110
      if(bins>0&&hz>=start&&hz<=end){const pos=(hz-start)/bin;const i=Math.floor(pos);const frac=pos-i;if(i<=0)db=f.power[0];else if(i>=bins-1)db=f.power[bins-1];else db=f.power[i]*(1-frac)+f.power[i+1]*frac}
      writeColorForDb(data,x*4,Number(db))
    }
    return rowImage
  }

  const rangeKey=()=>`${settingsState.waterfall.minHz}:${settingsState.waterfall.maxHz}`
  const resetBitmap=(width:number)=>{
    bitmap.width=Math.max(1,Math.floor(width));bitmap.height=maxRows;bitmapCtx=bitmap.getContext('2d')!;rowImage=bitmapCtx.createImageData(bitmap.width,1);bitmapCtx.clearRect(0,0,bitmap.width,maxRows);bitmapRows=0;bitmapRangeKey=rangeKey()
  }
  const appendFrame=(f:WaterfallFrame)=>{
    const key=rangeKey();const plotW=Math.max(1,Math.floor(canvas.clientWidth))
    if(bitmap.width!==plotW||bitmap.height!==maxRows||bitmapRangeKey!==key)resetBitmap(plotW)

    // WSJT-X-style waterfall motion: insert the newest spectrum row at the
    // top and move the existing history down by exactly one pixel.  The
    // previous circular-buffer renderer drew from the newest slot forward,
    // which placed the new row at the top but left blank rows underneath until
    // the ring wrapped, making the waterfall appear to blink without scrolling.
    const row=rowFromFrame(f,bitmap.width)
    const visibleCapacity=Math.min(bitmap.height-1,Math.max(1,Math.floor(canvas.clientHeight-24)))
    const rowsToMove=Math.min(bitmapRows,visibleCapacity)
    if(rowsToMove>0){
      bitmapCtx.drawImage(bitmap,0,0,bitmap.width,rowsToMove,0,1,bitmap.width,rowsToMove)
    }
    bitmapCtx.putImageData(row,0,0)
    bitmapRows=Math.min(bitmap.height,bitmapRows+1)
  }

  const render=()=>{
    const w=Math.max(1,canvas.clientWidth),h=Math.max(1,canvas.clientHeight),dpr=window.devicePixelRatio||1;
    if(w!==lastW||h!==lastH){lastW=w;lastH=h;sizeCanvas(w,h);resetBitmap(w);}else ctx.setTransform(dpr,0,0,dpr,0,0);
    ctx.globalCompositeOperation='source-over';ctx.clearRect(0,0,w,h);ctx.fillStyle='#06131c';ctx.fillRect(0,0,w,h);
    if(bitmapRows>0){
      const plotTop=24;
      const plotH=Math.max(1,h-plotTop);
      const visibleRows=Math.min(bitmapRows,Math.floor(plotH));
      ctx.imageSmoothingEnabled=false;
      ctx.drawImage(bitmap,0,0,bitmap.width,visibleRows,0,plotTop,w,visibleRows);
    }
    renderOverlay();
  };
  const schedule=()=>{if(raf)return;raf=requestAnimationFrame(()=>{raf=0;render()})};
  const freqFromEvent=(e:MouseEvent)=>{const r=surface.getBoundingClientRect();const x=Math.max(0,Math.min(r.width,e.clientX-r.left));return settingsState.waterfall.minHz+(x/Math.max(1,r.width))*(settingsState.waterfall.maxHz-settingsState.waterfall.minHz)};
  surface.addEventListener('mousemove',e=>{hoverHz=freqFromEvent(e);schedule()});surface.addEventListener('mouseleave',()=>{hoverHz=null;schedule()});surface.addEventListener('mousedown',e=>{if(e.button===0){e.preventDefault();setRXFrequency(freqFromEvent(e))}else if(e.button===2){e.preventDefault();setTXFrequency(freqFromEvent(e))}});surface.addEventListener('contextmenu',e=>e.preventDefault());
  window.addEventListener('jtty:waterfall',(e:any)=>{const f=e.detail as WaterfallFrame;if(f?.power?.length)waterfallFrame=f;appendFrame(f);schedule()});
  window.addEventListener('jtty:range-reset',()=>{bitmapRangeKey='';bitmapRows=0;schedule()});
  window.addEventListener('resize',()=>{lastRangeKey='';bitmapRangeKey='';bitmapRows=0;schedule()});
  if('ResizeObserver' in window){resizeObserver=new ResizeObserver(()=>{lastRangeKey='';bitmapRangeKey='';bitmapRows=0;schedule()});resizeObserver.observe(surface)}
  renderWaterfall=()=>schedule();render();return schedule;
}

function setupLayoutSplitters(root:Element){
  const wfHandle=root.querySelector<HTMLElement>('#splitter-waterfall')
  if(!wfHandle)return
  wfHandle.addEventListener('pointerdown',(ev:PointerEvent)=>{
    ev.preventDefault();wfHandle.setPointerCapture(ev.pointerId)
    const startY=ev.clientY
    const start=settingsState.layout.waterfallHeight
    const move=(e:PointerEvent)=>{
      const next=clampLayoutValues(start+(e.clientY-startY),LAYOUT_LIMITS.operationHeight)
      settingsState.layout={...settingsState.layout,...next,operationHeight:LAYOUT_LIMITS.operationHeight}
      applyLayout();scheduleLayoutSave()
    }
    const end=()=>{wfHandle.releasePointerCapture(ev.pointerId);wfHandle.removeEventListener('pointermove',move);wfHandle.removeEventListener('pointerup',end);wfHandle.removeEventListener('pointercancel',end)}
    wfHandle.addEventListener('pointermove',move);wfHandle.addEventListener('pointerup',end);wfHandle.addEventListener('pointercancel',end)
  })
}

function populateSettingsAudioDevices(){const root=document.querySelector('#settings-overlay');if(!root)return;const si=root.querySelector<HTMLSelectElement>('#set-input-device'),so=root.querySelector<HTMLSelectElement>('#set-output-device');const fill=(sel:HTMLSelectElement| null,items:AudioDevice[],current:string,label:string)=>{if(!sel)return;const options=[`<option value="">系统默认</option>`,...items.map(d=>`<option value="${esc(d.id)}">${esc(d.name||d.id)}</option>`)];if(current&&!items.some(d=>d.id===current))options.splice(1,0,`<option value="${esc(current)}">${label}（当前不可用）</option>`);sel.innerHTML=options.join('');sel.value=current};fill(si,audioDevices.inputs,settingsDraft.audio.inputDeviceId,'输入设备');fill(so,audioDevices.outputs,settingsDraft.audio.outputDeviceId,'输出设备')}
function selectedHamlibCapabilities():HamlibCapabilities|null{
  const id=Number(settingsDraft.radio.rigModelId||0)
  return id>0 && hamlibCapabilitiesModelId===id ? hamlibCapabilities : null
}
function requestHamlibCapabilities(modelId:number){
  if(modelId<=0){hamlibCapabilities=null;hamlibCapabilitiesModelId=0;hamlibCapabilitiesPendingModelId=0;const overlay=document.querySelector<HTMLElement>('#settings-overlay');if(overlay)updateRadioControlStates(overlay);return}
  if(hamlibCapabilitiesModelId===modelId && hamlibCapabilities){const overlay=document.querySelector<HTMLElement>('#settings-overlay');if(overlay)updateRadioControlStates(overlay);return}
  if(hamlibCapabilitiesPendingModelId===modelId)return
  hamlibCapabilities=null
  hamlibCapabilitiesModelId=modelId
  hamlibCapabilitiesPendingModelId=modelId
  const overlay=document.querySelector<HTMLElement>('#settings-overlay');if(overlay)updateRadioControlStates(overlay)
  emitBackendEvent('hamlib:capabilities',modelId)
}
function populateHamlibModels(loadCapabilities=false){
  const sel=document.querySelector<HTMLSelectElement>('#set-rig-model');if(!sel)return
  const current=sel.value||String(settingsDraft.radio.rigModelId||0)
  const currentName=settingsDraft.radio.rigName||''
  const models=hamlibModels
  const known=models.some(m=>String(m.modelId)===current)
  const fallback=(current!=='0'&&!known)?`<option value="${esc(current)}" data-name="${esc(currentName||'当前设备')}">${esc(currentName||('Model '+current))}</option>`:''
  const noneSelected=current==='0'?' selected':''
  sel.innerHTML=`<option value="0"${noneSelected}>None</option>`+fallback+models.map(m=>`<option value="${m.modelId}" data-name="${esc(m.label)}">${esc(m.label)}</option>`).join('')
  sel.value=known||current!=='0'?current:'0'
  settingsDraft.radio.rigModelId=num(sel.value,0)
  // Model selection never changes the selected backend.
  const opt=sel.selectedOptions[0]
  if(opt?.dataset.name)settingsDraft.radio.rigName=opt.dataset.name
  else if(settingsDraft.radio.rigModelId===0)settingsDraft.radio.rigName='None'
  if(loadCapabilities)requestHamlibCapabilities(settingsDraft.radio.rigModelId)
}

function populateSerialPorts(){
  const root=document.querySelector('#settings-overlay');if(!root)return
  const sel=root.querySelector<HTMLSelectElement>('#set-ptt-port');if(!sel)return
  const current=String(settingsDraft.radio.pttSerialPort||'').trim().toUpperCase()
  const available=serialPorts.map(x=>String(x).trim().toUpperCase()).filter(Boolean)
  const values=[...new Set(available.concat(current?[current]:[]))]
  sel.innerHTML='<option value="">请选择串口</option>'+values.map(x=>`<option value="${esc(x)}">${esc(x)}</option>`).join('')
  sel.value=current&&values.includes(current)?current:''
  settingsDraft.radio.pttSerialPort=sel.value
  updateRadioControlStates(root)
}

function setSettingsStatus(t:string){const e=document.querySelector('#settings-status');if(e)e.textContent=t}

    function stopPTTTestCountdown(){
      if(pttTestCountdownTimer!==undefined){window.clearInterval(pttTestCountdownTimer);pttTestCountdownTimer=undefined}
      pttTestCountdownDeadline=0
      const button=document.querySelector<HTMLButtonElement>('#radio-test-ptt')
      if(button)button.textContent='测试 PTT'
    }
    function startPTTTestCountdown(){
      stopPTTTestCountdown()
      pttTestCountdownDeadline=Date.now()+30000
      const tick=()=>{
        const remain=Math.max(0,Math.ceil((pttTestCountdownDeadline-Date.now())/1000))
        const button=document.querySelector<HTMLButtonElement>('#radio-test-ptt')
        if(button)button.textContent=remain>0?`停止 PTT（${remain} 秒）`:'正在自动关闭…'
        if(remain<=0 && pttTestCountdownTimer!==undefined){window.clearInterval(pttTestCountdownTimer);pttTestCountdownTimer=undefined}
      }
      tick()
      pttTestCountdownTimer=window.setInterval(tick,200)
    }
    function showPTTReleaseRetry(){
      if(pttTestCountdownTimer!==undefined){window.clearInterval(pttTestCountdownTimer);pttTestCountdownTimer=undefined}
      pttTestCountdownDeadline=0
      const button=document.querySelector<HTMLButtonElement>('#radio-test-ptt')
      if(button)button.textContent='正在释放 PTT…'
    }function collectDraft(){
  const s=cloneSettings(settingsDraft)
  const g=(id:string)=>document.querySelector<HTMLInputElement>(id),q=(id:string)=>document.querySelector<HTMLSelectElement>(id)
  if(g('#set-mycall'))s.myCall=g('#set-mycall')!.value.trim().toUpperCase()
  if(g('#set-grid'))s.myGrid=g('#set-grid')!.value.trim().toUpperCase()
  if(g('#set-follow'))s.autoFollowTail=g('#set-follow')!.checked
  if(g('#set-autostart'))s.autoStartMonitor=g('#set-autostart')!.checked

  if(q('#set-input-device'))s.audio.inputDeviceId=q('#set-input-device')!.value
  if(q('#set-output-device'))s.audio.outputDeviceId=q('#set-output-device')!.value
  if(q('#set-input-channel'))s.audio.inputChannel=q('#set-input-channel')!.value
  if(q('#set-output-channel'))s.audio.outputChannel=q('#set-output-channel')!.value
  if(g('#set-sample'))s.audio.sampleRate=num(g('#set-sample')!.value,s.audio.sampleRate)
  if(g('#set-buffer'))s.audio.bufferMs=num(g('#set-buffer')!.value,s.audio.bufferMs)
  if(g('#set-tx-level'))s.audio.txAudioLevel=num(g('#set-tx-level')!.value,s.audio.txAudioLevel)
  if(g('#set-record-directory'))s.audio.recordDirectory=g('#set-record-directory')!.value.trim()
  if(g('#set-record-enabled'))s.recordEnabled=g('#set-record-enabled')!.checked

  const selectedPTT=document.querySelector<HTMLInputElement>('input[name="pttm"]:checked')
  if(selectedPTT)s.radio.pttMethod=selectedPTT.value==='CAT'?'Rigctl':selectedPTT.value
  const backend=g('#set-backend')?.value||s.radio.backend
  s.radio.backend=backend
  s.radio.enabled=backend!=='none'
  if(q('#set-rig-model')) {s.radio.rigModelId=num(q('#set-rig-model')!.value,0);s.radio.rigName=q('#set-rig-model')!.selectedOptions[0]?.dataset.name||''}
  if(g('#set-rigctld-path'))s.radio.rigctldPath=g('#set-rigctld-path')!.value.trim()
  if(g('#set-host'))s.radio.host=g('#set-host')!.value.trim()
  if(g('#set-port'))s.radio.port=num(g('#set-port')!.value,s.radio.port)
  if(g('#set-serial'))s.radio.serialPort=g('#set-serial')!.value.trim()
  if(q('#set-baud'))s.radio.baud=num(q('#set-baud')!.value,s.radio.baud)
  const db=document.querySelector<HTMLInputElement>('input[name="db"]:checked');if(db)s.radio.dataBits=num(db.value,s.radio.dataBits)
  const sb=document.querySelector<HTMLInputElement>('input[name="sb"]:checked');if(sb)s.radio.stopBits=num(sb.value,s.radio.stopBits)
  const hs=document.querySelector<HTMLInputElement>('input[name="hs"]:checked');if(hs)s.radio.handshake=hs.value
  const ptt=document.querySelector<HTMLInputElement>('input[name="pttm"]:checked');if(ptt)s.radio.pttMethod=normalizePTTMethod(ptt.value)
  if(g('#set-ptt-port'))s.radio.pttSerialPort=g('#set-ptt-port')!.value.trim()
  if(g('#set-force-dtr'))s.radio.forceDTR=g('#set-force-dtr')!.value
  if(g('#set-force-rts'))s.radio.forceRTS=g('#set-force-rts')!.value
  if(q('#set-split'))s.radio.splitMode=q('#set-split')!.value
  if(q('#set-txaudio'))s.radio.txAudioSource=q('#set-txaudio')!.value
  if(g('#set-poll'))s.radio.pollIntervalSec=Math.max(1,Math.round(num(g('#set-poll')!.value,s.radio.pollIntervalSec)))
  if(g('#set-power-swr'))s.radio.readPowerSWR=g('#set-power-swr')!.checked
  if(g('#set-halt-swr'))s.radio.haltOnSWR=g('#set-halt-swr')!.checked
  if(q('#set-mode-radio'))s.radio.mode=q('#set-mode-radio')!.value
  if(g('#set-passband'))s.radio.passbandHz=num(g('#set-passband')!.value,s.radio.passbandHz)

  const frequencyRows=document.querySelectorAll<HTMLTableRowElement>('#frequency-table tbody tr')
  if(frequencyRows.length)s.frequencies=Array.from(frequencyRows).map(tr=>{
    const band=tr.dataset.band||''
    const list=parseFrequencyListMHz(tr.querySelector<HTMLInputElement>('[data-k="list"]')?.value||'')
    let def=Math.round(num(tr.querySelector<HTMLInputElement>('[data-k="default"]')?.value||'',0)*1e6)
    if(!list.length&&def>0)list.push(def)
    if(def<=0||!list.includes(def))def=list[0]||0
    return {band,frequencies:list,defaultHz:def}
  })
  if(g('#set-radio-enabled')){s.radio.enabled=g('#set-radio-enabled')!.checked;s.radio.backend=s.radio.enabled?'rigctld-net':'none'}
  saveRadioProfile(s.radio);settingsDraft=s;return s
}

function normalizeSettingsTab(tab:string){if(tab==='radio'&&!RADIO_SETTINGS_UI_ENABLED)return 'general';return ['general','radio','audio','frequency'].includes(tab)?tab:'general'}
function openSettings(tab='general'){const next=normalizeSettingsTab(tab);settingsTab=next;settingsDraft=cloneSettings(settingsState);renderSettingsModal(next);document.querySelector('#settings-overlay')!.classList.add('open');if(next==='audio')emitBackendEvent('audio:refresh','')}
function closeSettings(){closeAfterSettingsSave=false;stopPTTTestCountdown();emitBackendEvent('radio:test-end',{restore:true});document.querySelector('#settings-overlay')!.classList.remove('open')}
function saveSettings(closeAfterSave=false){const payload=collectDraft();closeAfterSettingsSave=closeAfterSave;emitBackendEvent('settings:save',payload);setSettingsStatus('正在保存…')}

function saveDecoderOption(kind:string,value:string){
  const next=cloneSettings(settingsState)
  switch(kind){
    case 'threads': next.decoder.threads=num(value,next.decoder.threads); break
    case 'threshold': next.decoder.thresholdDb=num(value,next.decoder.thresholdDb); break
    case 'ftol': next.decoder.frequencyToleranceHz=num(value,next.decoder.frequencyToleranceHz); break
    case 'tracker-tolerance': next.decoder.trackerToleranceHz=num(value,next.decoder.trackerToleranceHz); break
    case 'tracker-ttl': next.decoder.trackerTtlMs=num(value,next.decoder.trackerTtlMs); break
    case 'window': next.decodeWindowLimit=Math.max(100,Math.round(num(value,next.decodeWindowLimit))); break
    case 'mode': next.decoder.mode=value; break
    default: return
  }
  settingsState=next
  emitBackendEvent('settings:save',next)
  if(kind==='ftol')emitBackendEvent('jtty:set-tolerance',next.decoder.frequencyToleranceHz)
  const restartNeeded=['threads','mode'].includes(kind)
  setToast(restartNeeded?'解码设置已保存，重新启动监听后生效':'解码设置已保存')
  updateDecodeMenu()
  document.querySelector<HTMLElement>('#decode-popup')?.classList.remove('open')
}
function updateDecodeMenu(){
  const root=document.querySelector('#decode-popup');if(!root)return
  const vals:Record<string,string>={threads:settingsState.decoder.threads<=0?'自动':String(settingsState.decoder.threads),threshold:`${settingsState.decoder.thresholdDb} dB`,ftol:`${settingsState.decoder.frequencyToleranceHz} Hz`,trackerTolerance:`${settingsState.decoder.trackerToleranceHz} Hz`,trackerTtl:`${settingsState.decoder.trackerTtlMs} ms`,window:String(settingsState.decodeWindowLimit),mode:settingsState.decoder.mode||'realtime'}
  root.querySelectorAll<HTMLElement>('[data-decode-value]').forEach(e=>{const k=e.dataset.decodeValue||'';if(vals[k]!==undefined)e.textContent=vals[k]})
}
function wireDecodeMenu(root:Element){
  root.querySelectorAll<HTMLElement>('[data-decode-option]').forEach(item=>item.addEventListener('click',e=>{e.stopPropagation();saveDecoderOption(item.dataset.kind||'',item.dataset.value||'')}))
}
function arrangeRadioSettingsColumns(overlay:HTMLElement){
  const panel=overlay.querySelector<HTMLElement>('.radio-wsjt-v3')
  if(!panel)return
  const columns=panel.querySelector<HTMLElement>('.radio-columns-v3')
  const left=columns?.querySelector<HTMLElement>('.radio-left-v3')
  const right=columns?.querySelector<HTMLElement>('.radio-right-v3')
  if(!columns||!left||!right)return

  // Locate PTT by its actual inputs, not by fixed fieldset text or DOM position.
  const ptt=Array.from(columns.querySelectorAll<HTMLFieldSetElement>('fieldset')).find(fs=>!!fs.querySelector('input[name="pttm"]'))
  if(!ptt)return
  const top=panel.querySelector<HTMLElement>('.radio-topbar-v3')
  const ham=overlay.querySelector<HTMLElement>('.hamlib-box-v3')
  const catTest=columns.querySelector<HTMLButtonElement>('#radio-test-cat')
  const pttTest=columns.querySelector<HTMLButtonElement>('#radio-test-ptt')
  catTest?.remove()
  pttTest?.remove()
  ptt.remove()
  top?.remove()

  // In the old layout, mode/split/data fields are in the right column. Move
  // them beside CAT frequency control; discard only duplicate column headings.
  for(const child of Array.from(right.children)){
    if(child.classList.contains('radio-panel-heading-v6') && /PTT/i.test(child.textContent||'')){
      child.remove()
      continue
    }
    if(child.classList.contains('ptt-control-section-v6') || child.contains(ptt))continue
    if(child.matches('.radio-test-v3') && !child.querySelector('button')){child.remove();continue}
    left.appendChild(child)
  }
  for(const empty of Array.from(columns.querySelectorAll<HTMLElement>('.radio-test-v3'))){
    if(!empty.querySelector('button'))empty.remove()
  }
  // Remove stale heading labels if this function runs after an earlier layout pass.
  for(const heading of Array.from(columns.querySelectorAll<HTMLElement>('.radio-panel-heading-v6')))heading.remove()

  ptt.classList.add('ptt-control-section-v6')
  const legend=ptt.querySelector('legend')
  if(legend)legend.textContent='PTT 控制'
  if(pttTest){
    let actions=ptt.querySelector<HTMLElement>('.ptt-test-actions-v6')
    if(!actions){actions=document.createElement('div');actions.className='radio-test-v3 ptt-test-actions-v6';ptt.appendChild(actions)}
    actions.appendChild(pttTest)
  }
  if(catTest){
    const actions=document.createElement('div');actions.className='radio-test-v3 cat-test-actions-v6';actions.appendChild(catTest);left.appendChild(actions)
  }
  const leftHeading=document.createElement('div');leftHeading.className='radio-panel-heading-v6';leftHeading.textContent='电台控制'
  const rightHeading=document.createElement('div');rightHeading.className='radio-panel-heading-v6';rightHeading.textContent='PTT 控制'
  if(top)left.prepend(top)
  left.prepend(leftHeading)
  right.replaceChildren(rightHeading,ptt)
  if(ham && !left.contains(ham))left.appendChild(ham)
}

function updateRadioControlStates(root:Element){
  const r=settingsDraft.radio, local=r.backend==='hamlib-local', enabled=r.backend!=='none'
  const caps=local?selectedHamlibCapabilities():null
  const method=root.querySelector<HTMLInputElement>('input[name="pttm"]:checked')?.value||r.pttMethod
  const serial=method==='RTS'||method==='DTR'
  const pttPort=root.querySelector<HTMLSelectElement>('#set-ptt-port')
  if(pttPort)pttPort.disabled=!serial
  const note=root.querySelector<HTMLElement>('#hamlib-capability-note')
  if(note)note.textContent=caps?`设备类型：${caps.portType}；PTT：${caps.pttType}；支持模式：${(caps.supportedModes||[]).join(', ')}`:(r.rigModelId?'正在读取型号能力…':'请选择电台型号')
  root.querySelectorAll<HTMLElement>('[data-serial-setting]').forEach(e=>{e.hidden=!!caps&&caps.portType!=='serial'})
  const device=root.querySelector<HTMLInputElement>('#set-serial')
  if(device){device.disabled=!!caps&&caps.portType==='none';device.placeholder=caps?.portType==='network'?'例如 192.168.1.10:50001':'例如 COM3 或 USB 设备路径'}
  const split=root.querySelector<HTMLSelectElement>('#set-split')
  const mode=root.querySelector<HTMLSelectElement>('#set-mode-radio')
  if(mode&&caps?.supportedModes?.length){const current=mode.value||r.mode;const modes=caps.supportedModes;mode.innerHTML=(modes.includes(current)?'':`<option value="${esc(current)}" selected disabled>${esc(current)}（型号未报告支持）</option>`)+modes.map(v=>`<option value="${esc(v)}" ${v===current?'selected':''}>${esc(v)}</option>`).join('')}
  if(split)split.disabled=!enabled||(!!caps&&(!caps.hasGetSplitVFO||!caps.hasSetSplitVFO))
  const audio=root.querySelector<HTMLSelectElement>('#set-txaudio')
  if(audio)audio.disabled=method!=='Rigctl'||(!!caps&&!caps.hasCATPTTMicData)
  const model=Number(root.querySelector<HTMLSelectElement>('#set-rig-model')?.value||r.rigModelId)
  const host=root.querySelector<HTMLInputElement>('#set-host')?.value.trim()
  const port=Number(root.querySelector<HTMLInputElement>('#set-port')?.value||r.port)
  const test=root.querySelector<HTMLButtonElement>('#radio-test-cat')
  if(test)test.disabled=!enabled||(local?model<=0:(!host||port<1||port>65535))
  const ptt=root.querySelector<HTMLButtonElement>('#radio-test-ptt')
  if(ptt){ptt.disabled=method==='VOX'||((local||method==='Rigctl')&&ptt.dataset.online!=='true')||(!local&&serial&&!pttPort?.value);ptt.setAttribute('aria-disabled',String(ptt.disabled))}
}

function radioProfile(r:RadioSettings):RadioProfile {
  const {backend,enabled,configVersion,local,net,...profile}=r
  return structuredClone(profile)
}
function saveRadioProfile(r:RadioSettings){
  if(r.backend==='hamlib-local')r.local=radioProfile(r)
  if(r.backend==='rigctld-net')r.net=radioProfile(r)
  r.configVersion=2
}
function switchRadioBackend(next:string){
  const previous=settingsDraft.radio.backend
  const select=document.querySelector<HTMLSelectElement>('#set-backend')!
  select.value=previous
  const r=collectDraft().radio
  emitBackendEvent('radio:test-end',{restore:false})
  stopPTTTestCountdown()
  const fallback=radioProfile(defaultSettings().radio);fallback.splitMode='None'
  const profile=next==='hamlib-local'?(r.local||fallback):next==='rigctld-net'?(r.net||fallback):radioProfile(r)
  settingsDraft.radio={...r,...structuredClone(profile),backend:next,enabled:next!=='none'}
  hamlibCapabilities=null;hamlibCapabilitiesModelId=0;hamlibCapabilitiesPendingModelId=0
  renderSettingsModal('radio')
}

function updateRadioPTTControls(root:Element){updateRadioControlStates(root)}
function bindSettingsActions(overlay:HTMLElement,tab:string){
  overlay.querySelector('#radio-refresh-serial')?.addEventListener('click',()=>{
    setSettingsStatus('正在刷新本机串口列表…')
    emitBackendEvent('radio:serial-ports','')
  })
  overlay.querySelector('#audio-refresh')?.addEventListener('click',()=>emitBackendEvent('audio:refresh',''))
  overlay.querySelector('#radio-test-cat')?.addEventListener('click',()=>{
        const draft=collectDraft()
        const btn=overlay.querySelector<HTMLButtonElement>('#radio-test-cat')
        if(!btn||btn.disabled)return
        btn.classList.remove('test-ok','test-fail')
        btn.disabled=true
        btn.textContent='正在测试…'
        const ptt=overlay.querySelector<HTMLButtonElement>('#radio-test-ptt')
        if(ptt){ptt.dataset.online='false';ptt.setAttribute('aria-pressed','false');ptt.classList.remove('ptt-active');stopPTTTestCountdown()}
        const host=String(draft.radio.host||'127.0.0.1')
        const port=Number(draft.radio.port||4532)
        setSettingsStatus(`正在连接 rigctld ${host}:${port}，等待频率和模式响应…`)
        const binding=(window as any).go?.app?.App
        if(typeof binding?.TestRadioSettings==='function'){
          let settled=false
          const timer=window.setTimeout(()=>{
            if(settled)return
            settled=true
            btn.disabled=false
            btn.textContent='测试连接'
            setSettingsStatus('连接测试等待超时：8 秒内未收到后端结果，请检查地址、端口和 rigctld 日志。')
          },8000)
          Promise.resolve().then(()=>binding.TestRadioSettings(draft)).then((v:any)=>{
            if(settled)return
            settled=true
            window.clearTimeout(timer)
            handleRadioTestResult({...v,kind:'cat'})
          }).catch((error:any)=>{
            if(settled)return
            settled=true
            window.clearTimeout(timer)
            handleRadioTestResult({kind:'cat',ok:false,error:String(error?.message||error)})
          })
        }else{
          if(!emitBackendEvent('radio:test-cat',draft)){
            btn.disabled=false
            btn.textContent='测试连接'
            setSettingsStatus('连接测试未发送：Wails 事件通道不可用。')
            return
          }
          window.setTimeout(()=>{
            if(!btn.isConnected || !btn.disabled)return
            btn.disabled=false
            btn.textContent='测试连接'
            setSettingsStatus('连接测试等待超时：8 秒内未收到后端结果，请检查地址、端口和 rigctld 日志。')
          },8000)
        }
      })
      overlay.querySelector('#radio-test-ptt')?.addEventListener('click',()=>{
    const btn=overlay.querySelector<HTMLButtonElement>('#radio-test-ptt');if(!btn||btn.disabled)return
    const next=btn.getAttribute('aria-pressed')!=='true'
    const draft=collectDraft()
    btn.dataset.pending=String(next)
    btn.disabled=true
    setSettingsStatus(next?'正在通过 Rigctl/串口启用 PTT，最长发射 30 秒…':'正在关闭 PTT…')
    if(!emitBackendEvent('radio:test-ptt',{on:next,settings:draft})){
      btn.disabled=false
      btn.dataset.pending=''
      setSettingsStatus('PTT 命令发送失败：Wails 事件通道不可用。')
    }
  })
  overlay.querySelector('#hamlib-update')?.addEventListener('click',()=>{const arch=overlay.querySelector<HTMLInputElement>('input[name="hamlib-arch"]:checked')?.value||'64';emitBackendEvent('hamlib:update',arch)})
  overlay.querySelector('#hamlib-revert')?.addEventListener('click',()=>{const arch=overlay.querySelector<HTMLInputElement>('input[name="hamlib-arch"]:checked')?.value||'64';emitBackendEvent('hamlib:revert',arch)})
  overlay.querySelector('#set-backend')?.addEventListener('change',e=>switchRadioBackend((e.target as HTMLSelectElement).value))
  overlay.querySelector('#radio-disconnect')?.addEventListener('click',()=>{emitBackendEvent('radio:test-end',{restore:false});(window as any).go?.app?.App?.DisconnectRadio?.().then(()=>setSettingsStatus('已断开电台')).catch((e:any)=>setSettingsStatus(String(e)))})
  overlay.querySelector('#set-rig-model')?.addEventListener('change',()=>{collectDraft();requestHamlibCapabilities(settingsDraft.radio.rigModelId)})
  overlay.querySelector('#rig-model-search')?.addEventListener('input',e=>{const query=(e.target as HTMLInputElement).value.toLowerCase();overlay.querySelectorAll<HTMLOptionElement>('#set-rig-model option').forEach(o=>o.hidden=o.value!=='0'&&!o.text.toLowerCase().includes(query))})
  overlay.querySelectorAll<HTMLInputElement>('input[name="pttm"]').forEach(e=>e.addEventListener('change',()=>{settingsDraft.radio.pttMethod=normalizePTTMethod(e.value);emitBackendEvent('radio:test-end',{restore:false});const btn=overlay.querySelector<HTMLButtonElement>('#radio-test-ptt');if(btn){btn.dataset.online='false';btn.setAttribute('aria-pressed','false');btn.classList.remove('ptt-active');stopPTTTestCountdown()}updateRadioControlStates(overlay)}))
  overlay.querySelector('#set-host')?.addEventListener('input',()=>updateRadioControlStates(overlay))
  overlay.querySelector('#set-port')?.addEventListener('input',()=>updateRadioControlStates(overlay))
  overlay.querySelector('#set-ptt-port')?.addEventListener('change',()=>{settingsDraft.radio.pttSerialPort=(overlay.querySelector<HTMLSelectElement>('#set-ptt-port')?.value||'').trim();updateRadioControlStates(overlay)})
  overlay.querySelectorAll<HTMLInputElement>('input[name="pttm"]').forEach(e=>e.addEventListener('change',()=>{settingsDraft.radio.pttMethod=e.value;const btn=overlay.querySelector<HTMLButtonElement>('#radio-test-ptt');if(btn){btn.dataset.online='false';btn.setAttribute('aria-pressed','false');btn.classList.remove('ptt-active')}updateRadioControlStates(overlay)}))
  overlay.querySelector('#set-split')?.addEventListener('change',()=>{settingsDraft.radio.splitMode=(overlay.querySelector<HTMLSelectElement>('#set-split')?.value||'Rig');updateRadioControlStates(overlay)})
  if(tab==='radio'){populateSerialPorts();updateRadioPTTControls(overlay);updateRadioControlStates(overlay)}
}
function renderSettingsModal(tab=settingsTab){
  tab=normalizeSettingsTab(tab);settingsTab=tab
  const overlay=document.querySelector('#settings-overlay')!;const s=settingsDraft
  overlay.innerHTML=`<div class="settings-window"><div class="settings-titlebar"><b>JTTY-Go — 设置</b></div>
    <div class="settings-tabs">${[['general','常规'],['radio','电台'],['audio','音频'],['frequency','频率']].map(([id,label])=>{const disabled=id==='radio'&&!RADIO_SETTINGS_UI_ENABLED;return `<button data-tab="${id}" class="${tab===id?'active':''}" ${disabled?'disabled aria-disabled="true" title="Hamlib 电台配置暂未启用"':''}>${label}</button>`}).join('')}</div>
    <div class="settings-body">${settingsPanel(tab)}</div>
    <div class="settings-footer"><span id="settings-status">就绪</span><div><button id="settings-cancel">取消</button><button id="settings-apply">应用</button><button id="settings-ok" class="ok">确定</button></div></div></div>`
  arrangeRadioSettingsColumns(overlay as HTMLElement)
  overlay.querySelectorAll<HTMLButtonElement>('[data-tab]').forEach(b=>b.addEventListener('click',()=>{if(b.disabled)return;collectDraft();const nextTab=normalizeSettingsTab(b.dataset.tab||'general');settingsTab=nextTab;renderSettingsModal(settingsTab);if(settingsTab==='audio')emitBackendEvent('audio:refresh','');}))
  overlay.querySelector('#settings-cancel')!.addEventListener('click',closeSettings)
  overlay.querySelector('#settings-apply')!.addEventListener('click',()=>saveSettings(false))
  overlay.querySelector('#settings-ok')!.addEventListener('click',()=>saveSettings(true))
  populateSettingsAudioDevices();populateHamlibModels(false);populateSerialPorts();bindSettingsActions(overlay as HTMLElement,tab)
  if(tab==='audio')emitBackendEvent('audio:refresh','')
  if(tab==='radio'&&settingsDraft.radio.backend==='hamlib-local'){emitBackendEvent('hamlib:models',false);requestHamlibCapabilities(settingsDraft.radio.rigModelId)}
  if(tab==='radio')emitBackendEvent('radio:serial-ports','')
}
function settingsPanel(tab:string){
  const s=settingsDraft
  if(tab==='audio')return `<div class="audio-panel-v3"><fieldset><legend>声效卡</legend><div class="audio-device-grid-v3"><label>输入(I)：<select id="set-input-device"></select></label><select id="set-input-channel"><option value="Mono" ${s.audio.inputChannel==='Mono'?'selected':''}>单声道</option><option value="Left" ${s.audio.inputChannel==='Left'?'selected':''}>左声道</option><option value="Right" ${s.audio.inputChannel==='Right'?'selected':''}>右声道</option><option value="Both" ${s.audio.inputChannel==='Both'?'selected':''}>双声道</option></select><label>输出(t)：<select id="set-output-device"></select></label><select id="set-output-channel"><option value="Mono" ${s.audio.outputChannel==='Mono'?'selected':''}>单声道</option><option value="Left" ${s.audio.outputChannel==='Left'?'selected':''}>左声道</option><option value="Right" ${s.audio.outputChannel==='Right'?'selected':''}>右声道</option><option value="Both" ${s.audio.outputChannel==='Both'?'selected':''}>双声道</option></select></div><div class="audio-v3-actions"><button id="audio-refresh">刷新</button><label class="check-inline"><input type="checkbox" checked>按字母顺序排列</label><label class="check-inline"><input type="checkbox" checked>隐藏 ALSA 音效卡设备（仅 Linux）</label></div></fieldset><fieldset><legend>保存目录</legend><div class="audio-save-row"><span>目录位置(a)：</span><input id="set-record-directory" value="${esc(s.audio.recordDirectory)}" placeholder="%LOCALAPPDATA%\\JTTY-Go\\record"></div><div class="audio-record-row"><label class="record-enable"><input id="set-record-enabled" type="checkbox" ${s.recordEnabled?'checked':''}>启用录音</label></div></fieldset><fieldset><legend>音频参数</legend><div class="audio-param-grid"><label>采样率<input id="set-sample" type="number" value="${s.audio.sampleRate}" min="8000" max="192000"></label><label>缓冲<input id="set-buffer" type="number" value="${s.audio.bufferMs}" min="5" max="200"> ms</label><label>TX 输出电平<input id="set-tx-level" type="number" value="${s.audio.txAudioLevel}" min="0" max="100"> %</label></div></fieldset></div>`
  if(tab==='radio')return settingsPanelRadio(s)
  if(tab==='frequency')return `<div class="frequency-settings-panel"><div class="frequency-settings-note">每个波段可保存多个频点，使用逗号或空格分隔。默认频点用于点击主界面的波段按钮后设置电台拨盘频率。</div><div class="table-box"><table id="frequency-table"><thead><tr><th class="freq-band-col">波段</th><th>频点列表 (MHz)</th><th class="freq-default-col">默认频点 (MHz)</th></tr></thead><tbody>${s.frequencies.map(f=>`<tr data-band="${esc(f.band)}"><td class="freq-band-name">${esc(f.band)}</td><td><input data-k="list" value="${esc(formatFrequencyListMHz(f.frequencies))}" spellcheck="false"></td><td><input data-k="default" value="${f.defaultHz>0?(f.defaultHz/1e6).toFixed(6):''}" inputmode="decimal" spellcheck="false"></td></tr>`).join('')}</tbody></table></div></div>`

  return `<div class="general-settings"><fieldset><legend>电台资料</legend><label>我的呼号<input id="set-mycall" value="${esc(s.myCall)}" autocomplete="off"></label><label>我的网格<input id="set-grid" value="${esc(s.myGrid)}" autocomplete="off"></label><label class="general-check"><input id="set-autostart" type="checkbox" ${s.autoStartMonitor?'checked':''}><span>启动后自动监听</span></label></fieldset><fieldset><legend>解码显示</legend><label class="general-check"><input id="set-follow" type="checkbox" ${s.autoFollowTail?'checked':''}><span>新解码自动跟随到底部</span></label></fieldset></div>`
}
function settingsPanelRadio(s:Settings){
  const r=s.radio
  const methodRaw=String(r.pttMethod||'VOX').trim().toUpperCase()
  const ptt=methodRaw==='CAT'||methodRaw==='RIGCTL'?'Rigctl':(['DTR','RTS'].includes(methodRaw)?methodRaw:'VOX')
  const split=r.splitMode==='Fake It'?'Fake It':r.splitMode==='None'?'None':'Rig'
  const backend=String(r.backend||'').toLowerCase()
  const enabled=r.backend!=='none'
  const local=r.backend==='hamlib-local'
  return `<div class="radio-net-settings-v4 radio-net-balanced-layout-v7">
    <div class="radio-net-columns-balanced-v7">
      <div class="radio-net-column-balanced-v7 radio-net-control-panel-balanced-v7">
        <div class="radio-net-panel-heading-v7"><strong>电台控制</strong><small>本地 Hamlib / NET rigctl</small></div>
        <div class="radio-net-card-v4 radio-net-subcard-balanced-v7">
          <label>电台后端<select id="set-backend">${[['none','关闭电台控制'],['hamlib-local','本地 Hamlib'],['rigctld-net','NET rigctl']].map(([v,t])=>`<option value="${v}" ${r.backend===v?'selected':''}>${t}</option>`).join('')}</select></label>
          ${local?`<label>搜索型号<input id="rig-model-search" placeholder="厂商 / 型号"></label><label>电台型号<select id="set-rig-model"><option value="${r.rigModelId}">${esc(r.rigName||'请选择型号')}</option></select></label>
          <label>设备路径 / 电台网络地址<input id="set-serial" value="${esc(r.serialPort)}" spellcheck="false" autocomplete="off"></label>
          <div data-serial-setting class="radio-net-radio-fields-v4">
            <label>波特率<select id="set-baud">${[1200,2400,4800,9600,19200,38400,57600,115200].map(v=>`<option value="${v}" ${r.baud===v?'selected':''}>${v}</option>`).join('')}</select></label>
            <fieldset><legend>数据位</legend>${[7,8].map(v=>`<label><input type="radio" name="db" value="${v}" ${r.dataBits===v?'checked':''}>${v}</label>`).join('')}</fieldset>
            <fieldset><legend>停止位</legend>${[1,2].map(v=>`<label><input type="radio" name="sb" value="${v}" ${r.stopBits===v?'checked':''}>${v}</label>`).join('')}</fieldset>
            <fieldset><legend>握手</legend>${['None','XONXOFF','Hardware'].map(v=>`<label><input type="radio" name="hs" value="${v}" ${r.handshake===v?'checked':''}>${v}</label>`).join('')}</fieldset>
            <label>强制 DTR<select id="set-force-dtr">${['none','on','off'].map(v=>`<option ${r.forceDTR===v?'selected':''}>${v}</option>`).join('')}</select></label>
            <label>强制 RTS<select id="set-force-rts">${['none','on','off'].map(v=>`<option ${r.forceRTS===v?'selected':''}>${v}</option>`).join('')}</select></label>
          </div>
          <details><summary>自定义 Hamlib 运行包</summary><label>rigctld.exe 完整路径<input id="set-rigctld-path" value="${esc(r.rigctldPath)}" placeholder="留空使用随包版本"></label></details>
          <p id="hamlib-capability-note" class="radio-net-help-v4" role="status"></p><p class="radio-net-help-v4">本地服务自动管理，版本随完整应用包升级。</p>`:r.backend==='rigctld-net'?`
          <div class="radio-net-endpoint-v4"><label>服务器地址<input id="set-host" value="${esc(r.host||'127.0.0.1')}" spellcheck="false" autocomplete="off"></label><label>TCP 端口<input id="set-port" type="number" min="1" max="65535" value="${r.port||4532}"></label></div><p class="radio-net-help-v4">连接已有服务器，不启动或停止服务器进程。</p>`:'<p class="radio-net-help-v4">电台控制已关闭，可独立配置 VOX 或串口 PTT。</p>'}
        </div>

        <div class="radio-net-card-v4 radio-net-subcard-balanced-v7">
          <div class="radio-net-card-title-v4"><div><strong>电台参数</strong><small>连接成功后应用到电台</small></div></div>
          <div class="radio-net-radio-fields-v4">
            <label><span>模式</span><select id="set-mode-radio"><option value="USB" ${r.mode==='USB'?'selected':''}>USB</option><option value="PKTUSB" ${r.mode==='PKTUSB'?'selected':''}>PKTUSB</option><option value="LSB" ${r.mode==='LSB'?'selected':''}>LSB</option><option value="DIG_U" ${r.mode==='DIG_U'?'selected':''}>DIG_U</option><option value="DIG_L" ${r.mode==='DIG_L'?'selected':''}>DIG_L</option><option value="CW" ${r.mode==='CW'?'selected':''}>CW</option><option value="FM" ${r.mode==='FM'?'selected':''}>FM</option><option value="AM" ${r.mode==='AM'?'selected':''}>AM</option></select></label>
            <label><span>滤波带宽</span><div class="radio-net-input-unit-v4"><input id="set-passband" type="number" min="0" max="100000" value="${Number(r.passbandHz)>=0?r.passbandHz:3000}"><em>Hz</em></div></label>
            <label><span>异频操作</span><select id="set-split"><option value="None" ${split==='None'?'selected':''}>关闭</option><option value="Rig" ${split==='Rig'?'selected':''}>由电台控制</option><option value="Fake It" disabled>Fake It（尚未实现）</option></select></label>
            <label><span>状态轮询</span><div class="radio-net-input-unit-v4"><input id="set-poll" type="number" min="1" max="30" value="${Math.max(1,Number(r.pollIntervalSec)||1)}"><em>秒</em></div></label>
          </div>
          <p class="radio-net-help-v4">模式、异频与带宽由电台实际能力决定。</p>
        </div>
        <div class="radio-net-actions-balanced-v7"><span role="status">${local?'由应用管理本地 Hamlib':'独立电台控制连接'}</span><button id="radio-test-cat">测试连接</button><button id="radio-disconnect">断开</button></div>
      </div>

      <div class="radio-net-column-balanced-v7 radio-net-ptt-column-balanced-v7">
        <div class="radio-net-panel-heading-v7"><strong>PTT 控制</strong><small>与电台频率控制相互独立</small></div>
        <div class="radio-net-card-v4 radio-net-ptt-card-balanced-v7">
          <div class="radio-net-card-title-v4"><div><strong>PTT 发射方式</strong><small>选择控制发射的来源，不影响 rigctl 频率读写。</small></div></div>
          <div class="radio-net-ptt-v4 radio-net-ptt-balanced-v7">
            <label><input type="radio" name="pttm" value="VOX" ${ptt==='VOX'?'checked':''}><span><b>VOX</b><small>由发射音频触发</small></span></label>
            <label><input type="radio" name="pttm" value="Rigctl" ${ptt==='Rigctl'?'checked':''}><span><b>CAT / Rigctl</b><small>由当前后端发送 PTT 命令</small></span></label>
            <label><input type="radio" name="pttm" value="RTS" ${ptt==='RTS'?'checked':''}><span><b>RTS</b><small>使用本机串口 RTS 线路</small></span></label>
            <label><input type="radio" name="pttm" value="DTR" ${ptt==='DTR'?'checked':''}><span><b>DTR</b><small>使用本机串口 DTR 线路</small></span></label>
          </div>
          <div class="radio-net-ptt-port-row-v5"><label class="radio-net-field-v4"><span>PTT 串口</span><select id="set-ptt-port" autocomplete="off"></select></label><button id="radio-refresh-serial" type="button">刷新串口</button></div>
          <p class="radio-net-help-v4">${local?'RTS/DTR 交给本地 Hamlib；留空 PTT 串口时使用 CAT 设备。':'RTS/DTR 使用本机独立串口，必须明确选择。'} VOX 由音频触发。</p>
          <label>CAT PTT 音频输入<select id="set-txaudio">${[['Front','默认输入'],['Mic','麦克风'],['Data','数据输入']].map(([v,t])=>`<option value="${v}" ${r.txAudioSource===v?'selected':''}>${t}</option>`).join('')}</select></label>
          <div class="radio-net-ptt-actions-balanced-v7"><button id="radio-test-ptt" data-online="false" aria-pressed="false">测试 PTT</button></div>
        </div>
        <div class="radio-net-ptt-note-balanced-v7">电台后端与 PTT 方式独立选择，切换后端保留各自配置。</div>
      </div>
    </div>
  </div>`
}

function formatUtcDateTime(iso:string){const d=new Date(iso);if(Number.isNaN(d.getTime()))return '';const p=(n:number)=>String(n).padStart(2,'0');return `${d.getUTCFullYear()}/${d.getUTCMonth()+1}/${d.getUTCDate()} ${p(d.getUTCHours())}:${p(d.getUTCMinutes())}:${p(d.getUTCSeconds())}`}
function showQSORecordDialog(options:{exchangeSent?:string;startUtc?:string}={}){
  const call=setDX(document.querySelector<HTMLInputElement>('#dx')?.value||'',false);
  if(!call){setToast('请先填写 DX 呼号');return}
  const start=qsoStarts.get(call)||options.startUtc||'';
  if(!start){setToast('等待该呼号的第一次信息发送');return}
  const overlay=document.querySelector<HTMLDivElement>('#qso-overlay');if(!overlay)return;
  const endAt=new Date().toISOString();
  const exchange=options.exchangeSent||'';
  overlay.innerHTML=`<div class="qso-dialog"><div class="qso-dialog-title">单击“确定”确认以下通联：</div><div class="qso-form-grid"><label>呼号<input id="qso-call" value="${esc(call)}" readonly></label><label>开始(UTC)<input id="qso-start" value="${esc(formatUtcDateTime(start))}" readonly></label><label>结束(UTC)<input id="qso-end" value="${esc(formatUtcDateTime(endAt))}" readonly></label><label>模式<input id="qso-mode" value="JTTY"></label><label>波段<input id="qso-band" value="${esc(activeBand+'m')}"></label><label>频率(MHz)<input id="qso-frequency" value="${formatDialFrequency(dialFrequency)}"></label><label>发送报告<input id="qso-rst-sent" value="599"></label><label>接收报告<input id="qso-rst-recv" value="599"></label><label>网格<input id="qso-grid" value="${esc(dxGrid)}"></label><label>姓名<input id="qso-name" value=""></label><label>发射功率(W)<input id="qso-power" value=""></label><label>操作员<input id="qso-operator" value="${esc(settingsState.myCall)}"></label><label class="wide">交换发送<input id="qso-exchange-sent" value="${esc(exchange)}"></label><label class="wide">交换接收<input id="qso-exchange-recv" value=""></label><label class="wide">备注<input id="qso-comment" value=""></label></div><div class="qso-dialog-actions"><button id="qso-cancel">取消</button><button id="qso-confirm" class="primary">确定</button></div></div>`;
  overlay.classList.add('open');
  overlay.querySelector('#qso-cancel')!.addEventListener('click',()=>overlay.classList.remove('open'));
  overlay.querySelector('#qso-confirm')!.addEventListener('click',()=>{
    const payload={call,mode:(overlay.querySelector<HTMLInputElement>('#qso-mode')!.value||'JTTY').trim(),band:(overlay.querySelector<HTMLInputElement>('#qso-band')!.value||'20m').trim(),frequency:Math.round(num(overlay.querySelector<HTMLInputElement>('#qso-frequency')!.value,dialFrequency/1e6)*1e6),startUtc:start,endUtc:endAt,rstSent:overlay.querySelector<HTMLInputElement>('#qso-rst-sent')!.value.trim()||'599',rstRcvd:overlay.querySelector<HTMLInputElement>('#qso-rst-recv')!.value.trim()||'599',grid:overlay.querySelector<HTMLInputElement>('#qso-grid')!.value.trim().toUpperCase(),name:overlay.querySelector<HTMLInputElement>('#qso-name')!.value.trim(),powerWatts:num(overlay.querySelector<HTMLInputElement>('#qso-power')!.value,0),operator:overlay.querySelector<HTMLInputElement>('#qso-operator')!.value.trim(),exchangeSent:overlay.querySelector<HTMLInputElement>('#qso-exchange-sent')!.value.trim(),exchangeRecv:overlay.querySelector<HTMLInputElement>('#qso-exchange-recv')!.value.trim(),lastRxMessage:overlay.querySelector<HTMLInputElement>('#qso-comment')!.value.trim()};
    emitBackendEvent('qso:record',payload)
  })
}

function setToast(message:string){const toast=document.querySelector('#toast');if(!toast)return;toast.textContent=message;toast.classList.add('open');window.setTimeout(()=>toast.classList.remove('open'),2600)}
function updateSavePopup(){const all=document.querySelector<HTMLInputElement>('#save-all-log');const mode=document.querySelector<HTMLSelectElement>('#save-log-mode');const rec=document.querySelector<HTMLInputElement>('#save-record');if(all)all.checked=settingsState.decodeLogEnabled;if(mode)mode.value=settingsState.decodeLogMode;if(rec)rec.checked=settingsState.recordEnabled}
function emitSaveOptions(){emitBackendEvent('save:set',{decodeLogEnabled:document.querySelector<HTMLInputElement>('#save-all-log')?.checked??true,decodeLogMode:document.querySelector<HTMLSelectElement>('#save-log-mode')?.value||'single',recordEnabled:document.querySelector<HTMLInputElement>('#save-record')?.checked??false})}

function updateMonitorControls(){
  const top=document.querySelector<HTMLButtonElement>('#monitor-top')
  if(top){top.disabled=receiverBusy;top.textContent=rxRunning?'停止监听(M)':'监听(M)'}
}
function requestMonitor(start:boolean){
  if(receiverBusy)return
  if(start===rxRunning)return
  receiverBusy=true;updateMonitorControls();emitBackendEvent(start?'receiver:start':'receiver:stop','')
  window.setTimeout(()=>{if(receiverBusy){receiverBusy=false;updateMonitorControls()}},12000)
}

function updateDuplexButton(){const btn=document.querySelector<HTMLButtonElement>('#duplex-mode');if(!btn)return;btn.classList.toggle('active',duplexMode);btn.textContent='双工模式'}

function updateTXProgress(percent:number,active:boolean,error=false){const shell=document.querySelector<HTMLElement>('#tx-progress');const fill=document.querySelector<HTMLElement>('#tx-progress-fill');const label=document.querySelector<HTMLElement>('#tx-progress-label');if(!shell||!fill||!label)return;const p=Math.max(0,Math.min(100,Number(percent)||0));fill.style.width=`${p.toFixed(1)}%`;label.textContent=`${Math.round(p)}%`;shell.classList.toggle('transmitting',active);shell.classList.toggle('error',error)}

function installWailsEvents(root:Element){
  const runtime=runtimeApi();if(!runtime?.EventsOn)return false
  runtime.EventsOn('decode:added',(m:DecodeMessage)=>addRow(m,root));runtime.EventsOn('decode:updated',(m:DecodeMessage)=>updateRow(m,root));runtime.EventsOn('waterfall',(f:WaterfallFrame)=>window.dispatchEvent(new CustomEvent('jtty:waterfall',{detail:f})));runtime.EventsOn('qso:dxcall',(v:any)=>setDX(typeof v==='string'?v:v?.call||'',false));runtime.EventsOn('qso:dxgrid',(v:any)=>{const g=String(typeof v==='string'?v:v?.grid||'').toUpperCase();dxGrid=g;const input=document.querySelector<HTMLInputElement>('#dx-grid');if(input)input.value=g})
  runtime.EventsOn('qso:started',(v:any)=>{const call=String(v?.call||'').trim().toUpperCase();if(call&&v?.startUtc)qsoStarts.set(call,String(v.startUtc))})
  runtime.EventsOn('qso:exchange',(v:any)=>{const next=Number(v?.nextSerialNumber);if(Number.isFinite(next)&&next>0){settingsState.exchangeSerialNumber=Math.floor(next);const input=document.querySelector<HTMLInputElement>('#serial-number');if(input)input.value=String(Math.floor(next))}const exchange=String(v?.exchange||'').trim();if(exchange){const start=String(v?.startUtc||new Date().toISOString());window.setTimeout(()=>showQSORecordDialog({exchangeSent:exchange,startUtc:start}),0)}})
  runtime.EventsOn('macro:triggered',(v:any)=>{const msg=String(v?.message||'');if(msg){const tx=document.querySelector<HTMLInputElement>('#tx');if(tx)tx.value=msg}});runtime.EventsOn('tx:tune-state',(v:any)=>{const active=!!v?.active;const tune=document.querySelector<HTMLButtonElement>('#tune-main');if(tune)tune.textContent=active?'停止调谐(T)':'调谐(T)';if(active)updateTXProgress(0,true,false)});runtime.EventsOn('tx:reset',()=>{updateTXProgress(0,false,false);const tune=document.querySelector<HTMLButtonElement>('#tune-main');if(tune)tune.textContent='调谐(T)'});runtime.EventsOn('duplex:state',(v:any)=>{duplexMode=!!v?.enabled;settingsState.duplexMode=duplexMode;settingsDraft.duplexMode=duplexMode;updateDuplexButton()});runtime.EventsOn('tx:started',(v:any)=>{const msg=String(v?.message||'');if(msg){const tx=document.querySelector<HTMLInputElement>('#tx');if(tx)tx.value=msg;addQSOTx(msg,String(v?.utc||new Date().toISOString()),Number(v?.frequencyHz)||txFrequency)}updateTXProgress(0,true,false);setToast('开始发射')});runtime.EventsOn('tx:progress',(v:any)=>{updateTXProgress(Number(v?.percent)||0,v?.transmitting!==false&&Number(v?.percent||0)<100,false)});runtime.EventsOn('tx:finished',()=>{updateTXProgress(100,false,false);setToast('发射完成')});runtime.EventsOn('tx:stopped',()=>{const current=Number(document.querySelector<HTMLElement>('#tx-progress-fill')?.style.width?.replace('%',''))||0;updateTXProgress(current,false,true);setToast('发射已停止')});runtime.EventsOn('tx:error',(v:any)=>{updateTXProgress(Number(document.querySelector<HTMLElement>('#tx-progress-fill')?.style.width?.replace('%',''))||0,false,true);setToast(`发射失败：${v?.error||'未知错误'}`)});runtime.EventsOn('decode:manual-result',(v:any)=>{if(!v?.ok)setToast('当前没有足够的接收数据可立即解码')})
  runtime.EventsOn('record:state',(v:any)=>{recordState={enabled:!!v?.enabled,active:!!v?.active,directory:String(v?.directory||''),path:String(v?.path||'')};const e=document.querySelector('#audio-status');if(e)e.textContent=rxRunning?(recordState.active?'RX ACTIVE · REC':'RX ACTIVE'):'RX STOPPED';const dir=document.querySelector<HTMLInputElement>('#set-record-directory');if(dir&&recordState.directory&&!dir.value)dir.value=recordState.directory})
  runtime.EventsOn('record:error',(v:any)=>setToast(`录音失败：${v?.error||'未知错误'}`));runtime.EventsOn('log:error',(v:any)=>setToast(`ALL.txt 写入失败：${v?.error||'未知错误'}`));runtime.EventsOn('qso:record-result',(v:any)=>{if(v?.ok){document.querySelector('#qso-overlay')?.classList.remove('open');const call=String(v?.qso?.call||'').toUpperCase();if(call)qsoStarts.delete(call);setToast('通联日志已记录')}else setToast(v?.error||'通联日志记录失败')})
  runtime.EventsOn('audio:devices',(v:any)=>{audioDevices={inputs:(v?.inputs||[]),outputs:(v?.outputs||[])};populateSettingsAudioDevices()});runtime.EventsOn('audio:state',(v:any)=>{receiverBusy=false;rxRunning=!!v?.running;const st=document.querySelector('#audio-status');if(st)st.textContent=rxRunning?(recordState.active?'RX ACTIVE · REC':'RX ACTIVE'):'RX STOPPED';document.querySelector('.led')?.classList.toggle('on',rxRunning);updateMonitorControls();if(v?.error)setToast(`监听失败：${v.error}`)})
  runtime.EventsOn('detector:candidate',(c:any)=>{const e=document.querySelector('#candidate-count');if(e)e.textContent=String(Array.isArray(c)?c.length:0)});runtime.EventsOn('frequency:rx',(v:any)=>{const n=Number(typeof v==='number'?v:v?.frequencyHz);if(Number.isFinite(n))applyRXFrequency(n,false)});runtime.EventsOn('frequency:tx',(v:any)=>{const n=Number(typeof v==='number'?v:v?.frequencyHz);if(Number.isFinite(n))applyTXFrequency(n,false)})
  runtime.EventsOn('file:result',(v:any)=>{if(v?.ok){const action=v.action==='open-adi'?'JTTY.adi 日志已打开':v.action==='delete-all'?'ALL.txt 已删除':v.action==='delete-adi'?'JTTY.adi 已删除':'日志目录已打开';setToast(action)}else setToast(`文件操作失败：${v?.error||'unknown'}`)})
  runtime.EventsOn('audio:meter',(v:any)=>{if(Number.isFinite(Number(v?.dbfs)))audioInputDbfs=Math.max(-120,Math.min(0,Number(v.dbfs)));updateMeters()});runtime.EventsOn('radio:meter-capabilities',(v:any)=>{radioMeterCapabilities={strength:!!v?.strength,alc:!!v?.alc,powerWatts:!!v?.powerWatts,powerPercent:!!v?.powerPercent,swr:!!v?.swr};updateMeters()});runtime.EventsOn('radio:meter',(v:any)=>{radioMeterValues={...radioMeterValues,rxDb:finiteOrNull(v?.rxDb),alc:finiteOrNull(v?.alc),powerWatts:finiteOrNull(v?.powerWatts),powerPercent:finiteOrNull(v?.powerPercent),swr:finiteOrNull(v?.swr),transmitting:!!v?.transmitting};updateMeters()});runtime.EventsOn('radio:state',(v:any)=>{radioConnected=!!v?.connected;const e=document.querySelector('#radio-status');if(e){const labels:any={connecting:'正在连接',connected:'已连接',limited:'功能受限',fault:'连接故障',disconnected:'已断开'};e.textContent=`${v?.backend==='hamlib-local'?'HAMLIB':'NET RIGCTL'} ${labels[v?.status]||(radioConnected?'已连接':'已断开')}`;e.setAttribute('title',[v?.error,...(v?.warnings||[])].filter(Boolean).join('；'))}if(v?.error)setSettingsStatus(v.error);if(Number.isFinite(Number(v?.frequencyHz)))setDialFrequency(Number(v.frequencyHz));if(typeof v?.mode==='string'){dialMode=v.mode;const mode=document.querySelector('#dial-mode');if(mode)mode.textContent=dialMode}});runtime.EventsOn('band:state',(v:any)=>{if(typeof v?.band==='string')activeBand=v.band.replace(/m$/i,'');root.querySelectorAll('[data-band]').forEach(x=>x.classList.toggle('active',(x as HTMLElement).dataset.band===activeBand));if(Number.isFinite(Number(v?.frequencyHz)))setDialFrequency(Number(v.frequencyHz))});runtime.EventsOn('frequency:error',(v:any)=>setToast(`频率切换失败：${v?.error||'未知错误'}`))
  runtime.EventsOn('radio:test-result',(v:any)=>handleRadioTestResult(v));runtime.EventsOn('radio:serial-ports',(v:any)=>{serialPorts=Array.isArray(v)?v.map((x:any)=>String(x)).filter(Boolean):[];populateSerialPorts()});runtime.EventsOn('hamlib:models',(v:any)=>{const next=Array.isArray(v)?v:[];const same=next.length===hamlibModels.length&&next.every((m:any,i:number)=>Number(m?.modelId)===Number(hamlibModels[i]?.modelId)&&String(m?.label||'')===String(hamlibModels[i]?.label||''));if(same)return;hamlibModels=next;populateHamlibModels()});runtime.EventsOn('hamlib:capabilities',(v:any)=>{
    const overlay=document.querySelector<HTMLElement>('#settings-overlay');
    const id=Number(v?.modelId||v?.capabilities?.modelId||0);
    const current=Number(overlay?.querySelector<HTMLSelectElement>('#set-rig-model')?.value||settingsDraft.radio.rigModelId||0);
    if(!overlay || id!==current)return;
    if(hamlibCapabilitiesPendingModelId===id)hamlibCapabilitiesPendingModelId=0
    if(v?.available && v?.capabilities){hamlibCapabilities=v.capabilities as HamlibCapabilities;hamlibCapabilitiesModelId=id}
    else {hamlibCapabilities=null;hamlibCapabilitiesModelId=id}
    updateRadioControlStates(overlay);
  });runtime.EventsOn('hamlib:status',(v:any)=>{const i=document.querySelector<HTMLElement>('#hamlib-inuse'),b=document.querySelector<HTMLElement>('#hamlib-backup');if(i){const runtimeVersion=v?.runtimeVerified&&v?.runtimeVersion?String(v.runtimeVersion):'';const dllVersion=v?.dllVerified&&v?.dllVersion?String(v.dllVersion):'';const note=String(v?.note||'');const mismatch=note.includes('不一致');i.textContent=(mismatch?'⚠ ':'')+(runtimeVersion||dllVersion||((v?.dllPath?(String(v.dllPath).split(/[\/]/).pop()??'libhamlib-4.dll'):'libhamlib-4.dll')));i.title=[runtimeVersion?`运行时: ${runtimeVersion}`:'',dllVersion?`DLL: ${dllVersion}`:'',v?.executablePath?`rigctld: ${v.executablePath}`:'',v?.dllPath?`DLL路径: ${v.dllPath}`:'',note].filter(Boolean).join('\n');i.classList.toggle('hamlib-mismatch',mismatch)}if(b)b.textContent=v?.backupExists?'可恢复':'无备份'})
  runtime.EventsOn('settings:state',(v:any)=>{settingsState=mergeSettings(v);applyRXFrequency(settingsState.rxFrequencyHz,false);applyTXFrequency(settingsState.txFrequencyHz,false);applyLayout();setTXAudioLevel(settingsState.audio.txAudioLevel,false);renderMainMacros();updateSavePopup();updateDecodeMenu();updateFrequencyRangeMenu();updateDuplexButton();const serialInput=document.querySelector<HTMLInputElement>('#serial-number');if(serialInput)serialInput.value=String(settingsState.exchangeSerialNumber);if(!document.querySelector('#settings-overlay')?.classList.contains('open'))settingsDraft=cloneSettings(settingsState)});runtime.EventsOn('settings:saved',()=>{setToast('设置已保存');renderMainMacros();applyLayout();updateSavePopup();updateDecodeMenu();updateFrequencyRangeMenu();const serialInput=document.querySelector<HTMLInputElement>('#serial-number');if(serialInput)serialInput.value=String(settingsState.exchangeSerialNumber);renderWaterfall?.();if(closeAfterSettingsSave){closeAfterSettingsSave=false;closeSettings()}});runtime.EventsOn('settings:error',(v:any)=>setToast(`保存失败：${v?.error||'unknown'}`))
  runtime.EventsOn('settings:error',()=>{closeAfterSettingsSave=false})
  
  runtime.EventsOn('settings:saved',()=>{if(closeAfterSettingsSave){closeAfterSettingsSave=false;closeSettings()}})
  runtime.EventsOn('settings:error',()=>{closeAfterSettingsSave=false})
return true
}


function renderMainMacros(){
  const root=document.querySelector('#main-macros'); if(!root)return
  root.innerHTML=settingsState.macros.slice(0,8).map(m=>`<button data-macro="${esc(m.id)}"><b>${esc(m.shortcut||m.id)}</b> ${esc(m.name||m.id)}</button>`).join('')
  root.querySelectorAll<HTMLButtonElement>('[data-macro]').forEach(btn=>{
    btn.addEventListener('click',()=>{const m=settingsState.macros.find(x=>x.id===btn.dataset.macro);if(m)emitBackendEvent('macro:trigger',m.id)})
    btn.addEventListener('contextmenu',(event:MouseEvent)=>{event.preventDefault();event.stopPropagation();const m=settingsState.macros.find(x=>x.id===btn.dataset.macro);if(m)openMacroEditor(m.id)})
  })
}

function openMacroEditor(id:string){
  const macro=settingsState.macros.find(item=>item.id===id)
  if(!macro)return
  document.querySelector('#macro-editor-overlay')?.remove()
  const overlay=document.createElement('div')
  overlay.id='macro-editor-overlay'
  overlay.className='macro-editor-overlay'
  overlay.innerHTML=`<div class="macro-editor-window" role="dialog" aria-modal="true" aria-labelledby="macro-editor-title">
    <div class="macro-editor-titlebar"><b id="macro-editor-title">编辑快捷信息 · ${esc(macro.shortcut||macro.id)}</b><button type="button" data-macro-close aria-label="关闭">×</button></div>
    <div class="macro-editor-body">
      <label class="macro-editor-field"><span>快捷键名称</span><input id="macro-edit-name" type="text" maxlength="40" value="${esc(macro.name||'')}" autocomplete="off"></label>
      <label class="macro-editor-field"><span>快捷键</span><input id="macro-edit-shortcut" type="text" maxlength="12" value="${esc(macro.shortcut||macro.id)}" autocomplete="off" spellcheck="false"></label>
      <label class="macro-editor-field macro-editor-message"><span>发送内容模板</span><textarea id="macro-edit-template" rows="4" spellcheck="false">${esc(macro.template||'')}</textarea></label>
      <div class="macro-editor-options"><label><input id="macro-edit-enabled" type="checkbox" ${macro.enabled?'checked':''}> 启用此快捷信息</label><label><input id="macro-edit-global" type="checkbox" ${macro.global?'checked':''}> 注册为全局快捷键</label></div>
      <div class="macro-editor-help"><b>模板变量</b><span><code>%M</code> 我的呼号</span><span><code>%G</code> 我的网格</span><span><code>%H</code> 对方呼号</span><span><code>%E</code> 交换信息</span><span><code>%Q</code> 当前队列呼号</span></div>
    </div>
    <div class="macro-editor-footer"><span id="macro-editor-status">修改后保存即可生效</span><div><button type="button" data-macro-close>取消</button><button type="button" class="macro-editor-save" id="macro-editor-save">保存</button></div></div>
  </div>`
  document.body.appendChild(overlay)
  const close=()=>overlay.remove()
  overlay.querySelectorAll<HTMLButtonElement>('[data-macro-close]').forEach(button=>button.addEventListener('click',close))
  overlay.addEventListener('mousedown',(event)=>{if(event.target===overlay)close()})
  overlay.addEventListener('keydown',(event)=>{if(event.key==='Escape'){event.preventDefault();close()}})
  overlay.querySelector<HTMLButtonElement>('#macro-editor-save')?.addEventListener('click',()=>{
    const name=(overlay.querySelector<HTMLInputElement>('#macro-edit-name')?.value||'').trim()||macro.id
    const shortcut=(overlay.querySelector<HTMLInputElement>('#macro-edit-shortcut')?.value||'').trim().toUpperCase()||macro.id
    const template=overlay.querySelector<HTMLTextAreaElement>('#macro-edit-template')?.value??macro.template
    const enabled=overlay.querySelector<HTMLInputElement>('#macro-edit-enabled')?.checked??macro.enabled
    const global=overlay.querySelector<HTMLInputElement>('#macro-edit-global')?.checked??macro.global
    const updated:MacroConfig={...macro,name,shortcut,template,enabled,global}
    settingsState={...settingsState,macros:settingsState.macros.map(item=>item.id===id?updated:item)}
    settingsDraft=cloneSettings(settingsState)
    renderMainMacros()
    emitBackendEvent('settings:save',settingsState)
    overlay.remove()
    setToast('快捷信息已保存')
  })
  window.setTimeout(()=>overlay.querySelector<HTMLInputElement>('#macro-edit-name')?.focus(),0)
}

function handleRadioTestResult(v:any){
      const overlay=document.querySelector<HTMLElement>('#settings-overlay')
      if(!overlay?.classList.contains('open'))return
      const kind=String(v?.kind||'')
      if(kind==='cat'){
        const cat=overlay.querySelector<HTMLButtonElement>('#radio-test-cat')
        const ptt=overlay.querySelector<HTMLButtonElement>('#radio-test-ptt')
        if(cat){cat.disabled=false;cat.textContent='测试连接';cat.classList.toggle('test-ok',!!v?.ok);cat.classList.toggle('test-fail',!v?.ok)}
        if(ptt){
          ptt.dataset.online=v?.ok?'true':'false'
          if(!v?.ok){ptt.setAttribute('aria-pressed','false');ptt.classList.remove('ptt-active');stopPTTTestCountdown()}
          updateRadioControlStates(overlay)
        }
        if(v?.capabilities&&settingsDraft.radio.backend==='hamlib-local'){hamlibCapabilities=v.capabilities;hamlibCapabilitiesModelId=settingsDraft.radio.rigModelId}
        setSettingsStatus(v?.ok?`rigctld 连接成功：${v.mode||''} ${v.frequencyHz||''} Hz`:`rigctld 连接失败：${v?.error||v?.frequencyError||v?.modeError||'未收到有效响应'}`)
        return
      }
      if(kind==='ptt'){
        const ptt=overlay.querySelector<HTMLButtonElement>('#radio-test-ptt')
        if(ptt){
          ptt.dataset.pending=''
          if(v?.ok){
            const on=!!v?.on
            ptt.setAttribute('aria-pressed',String(on))
            ptt.classList.toggle('ptt-active',on)
            if(on)startPTTTestCountdown();else stopPTTTestCountdown()
          }else if(v?.retrying){
            ptt.setAttribute('aria-pressed','true')
            ptt.classList.add('ptt-active')
            showPTTReleaseRetry()
          }else{
            ptt.setAttribute('aria-pressed','false')
            ptt.classList.remove('ptt-active')
            stopPTTTestCountdown()
          }
          updateRadioControlStates(overlay)
        }
        if(v?.ok&&v?.on)setSettingsStatus('PTT 已启用，30 秒倒计时开始。再次点击可提前停止。')
        else if(v?.ok&&v?.timeout)setSettingsStatus('PTT 测试达到 30 秒上限，已自动发送关闭命令。')
        else if(v?.ok)setSettingsStatus('PTT 已关闭。')
        else setSettingsStatus(`PTT 控制失败：${v?.error||'未知错误'}${v?.retrying?'，后台会持续尝试关闭。':''}`)
        return
      }
      if(kind==='connect'){
        setSettingsStatus(v?.ok?'rigctld 已连接。':`rigctld 自动连接失败：${v?.error||'未知错误'}`)
      }
    }

    function app(){
  const root=$('#app');
  root.innerHTML=`<div class="shell">
    <header class="menu">
      <div class="brand">JTTY-Go</div>
      <div class="menus"><button id="file-menu">文件</button><button id="save-menu">保存</button><button id="decode-menu">解码</button><button id="range-menu">频率范围</button><button id="config-menu">配置</button></div>
      <div class="status"><span>JTTY</span><span class="divider"></span><span id="radio-status">RADIO DISCONNECTED</span><span class="divider"></span><span id="audio-status">RX STOPPED</span><b class="led"></b></div>
    </header>

    <section class="waterfall-wrap">
      <div class="waterfall-surface" id="waterfall-surface" aria-label="JTTY 频谱瀑布图">
        <canvas id="waterfall"></canvas>
      </div>
    </section>
    <div class="layout-splitter" id="splitter-waterfall" role="separator" aria-orientation="horizontal" aria-label="调整瀑布图高度" title="拖动调整瀑布图高度"></div>

    <main class="content">
      <section class="rx">
        <div class="panel-title"><span>所有解码</span><span class="panel-hint">最新消息在最底部</span></div>
        <div class="table-head"><span>UTC</span><span>SNR</span><span>Freq</span><span>解码内容</span></div>
        <div class="decode-list" id="decodes"></div>
        <button class="new-indicator" id="new-indicator">↓ 新解码</button>
      </section>
      <section class="qso">
        <div class="panel-title"><span>QSO 频率</span><span class="panel-hint">仅显示 TX · 黄色高亮</span></div>
        <div class="qso-table-head"><span>UTC</span><span>Freq</span><span>信息</span></div>
        <div class="qso-decodes" id="qso-decodes"></div>
      </section>
    </main>

    <section class="operation-panel">
      <div class="band-strip">
        ${['160','80','60','40','30','20','17','15','12','10','6','2','70'].map(b=>`<button data-band="${b}" class="${b===activeBand?'active':''}">${b}</button>`).join('')}
      </div>
      <div class="operation-toolbar">
        <button id="record-qso">记录通联(Q)</button>
        <button id="monitor-top" class="green-action">监听(M)</button>
        <button id="clear-main">擦除(E)</button>
        <button id="decode-action">解码(D)</button>
        <button id="stop-tx-main">停止发射(H)</button>
        <button id="tune-main">调谐(T)</button>
        <button id="reset-serial-number" title="将 Serial Number 清除并恢复为 1">清除序号</button>
        <button id="duplex-mode" title="开启后，发射时保持接收监听">双工模式</button>
        <div class="tx-progress" id="tx-progress" aria-label="发射进度"><div class="tx-progress-track"><div class="tx-progress-fill" id="tx-progress-fill"></div></div><span id="tx-progress-label">0%</span></div>
      </div>
      <div class="operation-body">
        <section class="op-left">
          <div class="main-frequency"><input id="dial-freq" value="14.090000" inputmode="decimal" aria-label="电台频率 MHz"><span>MHz</span></div>
          <div class="radio-meters">
            <div class="meter-row" id="meter-rx-alc"><span class="meter-label">接收电平 / ALC</span><div class="meter-track"><div class="meter-fill"></div></div><span class="meter-value">—</span></div>
            <div class="meter-row" id="meter-power"><span class="meter-label">功率</span><div class="meter-track"><div class="meter-fill"></div></div><span class="meter-value">—</span></div>
            <div class="meter-row" id="meter-swr"><span class="meter-label">驻波</span><div class="meter-track"><div class="meter-fill"></div></div><span class="meter-value">—</span></div>
          </div>
          <div class="audio-level-row">
            <span class="audio-level-label">音频电平</span>
            <input id="tx-level" type="range" min="0" max="100" value="65" aria-label="TX 输出音频电平">
            <span id="tx-level-value">65%</span>
          </div>
        </section>

        <section class="op-middle">
          <div class="frequency-settings-row">
            <label class="spin-field"><span>发射</span><input id="op-tx-freq" type="number" min="200" max="5000" step="1" value="1500"><em>Hz</em></label>
            <div class="freq-transfer-buttons" aria-label="复制频率">
              <button id="tx-equals-rx" title="TX = RX">←</button>
              <button id="rx-equals-tx" title="RX = TX">→</button>
            </div>
            <label class="spin-field"><span>接收</span><input id="op-rx-freq" type="number" min="200" max="5000" step="1" value="1500"><em>Hz</em></label>
            <label class="spin-field tolerance-field"><span>容差</span><input id="tolerance" type="number" min="1" max="500" step="1" value="20"><em>Hz</em></label>
          </div>
          <div class="dx-grid">
            <label>DX 呼号</label><input id="dx" autocomplete="off" spellcheck="false">
            <label>DX 网格</label><input id="dx-grid" autocomplete="off" spellcheck="false">
          </div>
          <div class="clock-row">
            <div class="clock-display" id="clock-display">2026 9月 28<br>00:00:00</div>
            <div class="serial-controls">
              <label>Call next<input id="call-next"></label>
              <label>Serial Number<input id="serial-number" type="number" min="1" value="${settingsState.exchangeSerialNumber}"></label>
            </div>
          </div>
          <div class="message-row"><button id="send" class="primary">发送</button><input id="tx" autocomplete="off" spellcheck="false"><button id="clear">清除</button></div>
        </section>

        <section class="op-right">
          <div class="macro-grid" id="main-macros" aria-label="预置信息"></div>
        </section>
      </div>
    </section>

    <footer><span id="health">Initializing…</span><span>候选 <b id="candidate-count">0</b></span><span><b id="count">0</b> 解码</span><span id="rx-freq-bottom">RX 1500</span><span id="tx-freq-bottom">TX 1500</span><span class="footer-hint">左键瀑布图：RX　右键：TX</span></footer>
    <div id="settings-overlay"></div>
    <div id="qso-overlay"></div>
    <div id="toast"></div>
    <div id="file-popup" class="file-popup">
      <button id="file-open-adi">打开 JTTY.adi 日志</button>
      <button id="file-delete-all">删除 ALL.txt</button>
      <button id="file-delete-adi">删除通联日志 JTTY.adi</button>
      <button id="file-open-dir">打开日志目录</button>
      <button id="file-exit">退出软件</button>
    </div>
    <div id="save-popup" class="file-popup save-popup">
      <label class="save-option"><input id="save-all-log" type="checkbox"><span>ALL.txt 解码日志</span></label>
      <label class="save-option save-mode"><span>分拆</span><select id="save-log-mode"><option value="single">全部保存到 ALL.txt</option><option value="year">按年分拆</option><option value="month">按月分拆</option></select></label>
      <label class="save-option"><input id="save-record" type="checkbox"><span>record 录音</span></label>
      <button id="save-apply">应用保存设置</button>
    </div>
    <div id="range-popup" class="menu-popup range-popup">
      ${[2500,2700,3000,3500,4000].map(v=>`<button data-range-max="${v}">0–${v} Hz</button>`).join('')}
    </div>
    <div id="decode-popup" class="menu-popup decode-popup">
      <div class="decode-menu-row has-sub"><span>线程数</span><b data-decode-value="threads"></b><div class="decode-submenu">${['自动:0','1:1','2:2','3:3','4:4'].map(x=>{const [t,v]=x.split(':');return `<button data-decode-option data-kind="threads" data-value="${v}">${t}</button>`}).join('')}</div></div>
      <div class="decode-menu-row has-sub"><span>检出门限</span><b data-decode-value="threshold"></b><div class="decode-submenu">${[4,6,8,10,12,15].map(v=>`<button data-decode-option data-kind="threshold" data-value="${v}">${v} dB</button>`).join('')}</div></div>
      <div class="decode-menu-row has-sub"><span>频率容差</span><b data-decode-value="ftol"></b><div class="decode-submenu">${[5,10,20,50,100,200].map(v=>`<button data-decode-option data-kind="ftol" data-value="${v}">${v} Hz</button>`).join('')}</div></div>
      <div class="decode-menu-row has-sub"><span>追踪容差</span><b data-decode-value="trackerTolerance"></b><div class="decode-submenu">${[5,10,20,50,100].map(v=>`<button data-decode-option data-kind="tracker-tolerance" data-value="${v}">${v} Hz</button>`).join('')}</div></div>
      <div class="decode-menu-row has-sub"><span>追踪超时</span><b data-decode-value="trackerTtl"></b><div class="decode-submenu">${[300,600,1000,1500,2000].map(v=>`<button data-decode-option data-kind="tracker-ttl" data-value="${v}">${v} ms</button>`).join('')}</div></div>
      <div class="decode-menu-row has-sub"><span>解码窗口</span><b data-decode-value="window"></b><div class="decode-submenu">${[500,1000,2000,5000,10000].map(v=>`<button data-decode-option data-kind="window" data-value="${v}">${v}</button>`).join('')}</div></div>
      <div class="decode-menu-row has-sub"><span>解码模式</span><b data-decode-value="mode"></b><div class="decode-submenu"><button data-decode-option data-kind="mode" data-value="realtime">实时</button></div></div>
      
    </div>
  </div>`

  const list=root.querySelector<HTMLDivElement>('#decodes')!
  list.addEventListener('scroll',()=>{const atTail=list.scrollHeight-list.scrollTop-list.clientHeight<12;if(atTail&&!followTail){followTail=true;newSinceScroll=0;root.querySelector('#new-indicator')!.textContent='↓ 新解码'}else if(!atTail){followTail=false}})
  root.querySelector('#new-indicator')!.addEventListener('click',()=>{followTail=true;newSinceScroll=0;list.scrollTop=list.scrollHeight;root.querySelector('#new-indicator')!.textContent='↓ 新解码'})
  root.querySelector('#config-menu')!.addEventListener('click',()=>openSettings('general'))
  root.querySelector('#file-menu')!.addEventListener('click',()=>{root.querySelector('#save-popup')!.classList.remove('open');root.querySelector('#decode-popup')!.classList.remove('open');root.querySelector('#file-popup')!.classList.toggle('open')})
  root.querySelector('#save-menu')!.addEventListener('click',()=>{root.querySelector('#file-popup')!.classList.remove('open');root.querySelector('#decode-popup')!.classList.remove('open');root.querySelector('#save-popup')!.classList.toggle('open')})
  root.querySelector('#decode-menu')!.addEventListener('click',()=>{root.querySelector('#file-popup')!.classList.remove('open');root.querySelector('#save-popup')!.classList.remove('open');root.querySelector('#decode-popup')!.classList.toggle('open');updateDecodeMenu()})
  root.querySelector('#range-menu')!.addEventListener('click',()=>{root.querySelector('#file-popup')!.classList.remove('open');root.querySelector('#save-popup')!.classList.remove('open');root.querySelector('#decode-popup')!.classList.remove('open');root.querySelector('#range-popup')!.classList.toggle('open');updateFrequencyRangeMenu()})
  root.querySelector('#range-popup')!.querySelectorAll<HTMLButtonElement>('[data-range-max]').forEach(b=>b.addEventListener('click',()=>setFrequencyRange(Number(b.dataset.rangeMax))))
  updateFrequencyRangeMenu()
  wireDecodeMenu(root)
  updateDecodeMenu()
  root.querySelector('#file-open-adi')!.addEventListener('click',()=>{emitBackendEvent('file:open-adi','');root.querySelector('#file-popup')!.classList.remove('open')})
  root.querySelector('#file-delete-all')!.addEventListener('click',()=>{if(confirm('确定删除 ALL.txt 及其按年/按月拆分的解码日志？')){emitBackendEvent('file:delete-all','');root.querySelector('#file-popup')!.classList.remove('open')}})
  root.querySelector('#file-delete-adi')!.addEventListener('click',()=>{if(confirm('确定删除 JTTY.adi 通联日志？')){emitBackendEvent('file:delete-adi','');root.querySelector('#file-popup')!.classList.remove('open')}})
  root.querySelector('#file-open-dir')!.addEventListener('click',()=>{emitBackendEvent('file:open-data-dir','');root.querySelector('#file-popup')!.classList.remove('open')})
  root.querySelector('#file-exit')!.addEventListener('click',()=>{root.querySelector('#file-popup')!.classList.remove('open');emitBackendEvent('app:quit','')})

  renderMainMacros()
  const lastMacroKeyAt=new Map<string,number>();
  document.addEventListener('keydown',e=>{
    if(e.repeat||!/^F[1-8]$/.test(e.key))return;
    const m=settingsState.macros.find(x=>x.enabled&&String(x.shortcut||'').trim().toUpperCase()===e.key.toUpperCase());
    if(!m)return;
    const now=performance.now();const last=lastMacroKeyAt.get(m.id)||0;if(now-last<350){e.preventDefault();return}
    lastMacroKeyAt.set(m.id,now);e.preventDefault();e.stopPropagation();emitBackendEvent('macro:trigger',m.id);
  },true)
  updateMonitorControls()

  const submitDialFrequency=()=>{
    const input=root.querySelector<HTMLInputElement>('#dial-freq')!
    const raw=input.value.replace(/\s+/g,'')
    const mhz=Number(raw)
    if(Number.isFinite(mhz)&&mhz>0){
      dialFrequency=Math.round(mhz*1e6)
      emitBackendEvent('frequency:set-dial',dialFrequency)
      input.value=formatDialDisplay(dialFrequency).replace(' ','')
      input.blur()
    }
  }
  root.querySelector<HTMLInputElement>('#dial-freq')!.addEventListener('keydown',e=>{if(e.key==='Enter')submitDialFrequency()})
  root.querySelector<HTMLInputElement>('#dial-freq')!.addEventListener('change',submitDialFrequency)

  root.querySelector<HTMLInputElement>('#op-rx-freq')!.addEventListener('change',e=>setRXFrequency(Number((e.target as HTMLInputElement).value)))
  root.querySelector<HTMLInputElement>('#op-tx-freq')!.addEventListener('change',e=>setTXFrequency(Number((e.target as HTMLInputElement).value)))
  root.querySelector('#tx-equals-rx')!.addEventListener('click',()=>setTXFrequency(rxFrequency))
  root.querySelector('#rx-equals-tx')!.addEventListener('click',()=>setRXFrequency(txFrequency))
  root.querySelector<HTMLInputElement>('#tolerance')!.addEventListener('change',e=>applyTolerance(Number((e.target as HTMLInputElement).value)))
  root.querySelector<HTMLInputElement>('#tolerance')!.addEventListener('keydown',e=>{if(e.key==='Enter'){applyTolerance(Number((e.target as HTMLInputElement).value));(e.target as HTMLInputElement).blur()}})

  root.querySelector<HTMLInputElement>('#tx-level')!.addEventListener('input',e=>setTXAudioLevel(Number((e.target as HTMLInputElement).value),true))
  root.querySelector('#clear')!.addEventListener('click',()=>root.querySelector<HTMLInputElement>('#tx')!.value='')
  root.querySelector<HTMLInputElement>('#dx')!.addEventListener('change',e=>setDX((e.target as HTMLInputElement).value));root.querySelector<HTMLInputElement>('#dx-grid')!.addEventListener('change',e=>{const g=(e.target as HTMLInputElement).value.trim().toUpperCase();dxGrid=g;(e.target as HTMLInputElement).value=g;emitBackendEvent('qso:dxgrid',g)});root.querySelector<HTMLInputElement>('#serial-number')!.addEventListener('change',e=>{const n=Math.max(1,Math.floor(num((e.target as HTMLInputElement).value,settingsState.exchangeSerialNumber)));(e.target as HTMLInputElement).value=String(n);settingsState.exchangeSerialNumber=n;emitBackendEvent('qso:serial-set',n)})
  root.querySelector('#save-apply')!.addEventListener('click',()=>{emitSaveOptions();root.querySelector('#save-popup')!.classList.remove('open')})
  updateSavePopup()
  root.querySelector('#send')!.addEventListener('click',()=>{const call=setDX(root.querySelector<HTMLInputElement>('#dx')?.value||'');const message=root.querySelector<HTMLInputElement>('#tx')!.value;emitBackendEvent('tx:send',{call,message})})
  root.querySelector('#decode-action')!.addEventListener('click',()=>emitBackendEvent('receiver:decode-now',''))
  root.querySelector('#clear-main')!.addEventListener('click',(e)=>{if((e as MouseEvent).button!==0)return;e.preventDefault();rows.clear();decodeRowElements.clear();list.innerHTML='';root.querySelector('#count')!.textContent='0'})
  root.querySelector('#clear-main')!.addEventListener('contextmenu',(e)=>{e.preventDefault();qsoRows.length=0;renderQSOList()})
  root.querySelector('#tune-main')!.addEventListener('click',()=>emitBackendEvent('tx:tune',''))
  root.querySelector('#duplex-mode')!.addEventListener('click',()=>emitBackendEvent('duplex:set',!duplexMode))
  root.querySelector('#reset-serial-number')!.addEventListener('click',()=>{
    const input=root.querySelector<HTMLInputElement>('#serial-number')
    if(input)input.value='1'
    settingsState.exchangeSerialNumber=1
    emitBackendEvent('qso:serial-set',1)
    setToast('Serial Number 已恢复为 1')
  })
  root.querySelector('#stop-tx-main')!.addEventListener('click',()=>emitBackendEvent('tx:stop',''))
  root.querySelectorAll<HTMLButtonElement>('[data-band]').forEach(btn=>btn.addEventListener('click',()=>{const band=btn.dataset.band||'';if(band)emitBackendEvent('band:set',band)}))
  root.querySelector('#monitor-top')!.addEventListener('click',()=>requestMonitor(!rxRunning))
  root.querySelector('#record-qso')!.addEventListener('click',()=>showQSORecordDialog())

  document.addEventListener('click',e=>{const t=e.target as Node;const fileMenu=root.querySelector('#file-menu')!;const saveMenu=root.querySelector('#save-menu')!;const decodeMenu=root.querySelector('#decode-menu')!;const rangeMenu=root.querySelector('#range-menu')!;const filePopup=root.querySelector('#file-popup')!;const savePopup=root.querySelector('#save-popup')!;const decodePopup=root.querySelector('#decode-popup')!;const rangePopup=root.querySelector('#range-popup')!;if(!fileMenu.contains(t)&&!filePopup.contains(t))filePopup.classList.remove('open');if(!saveMenu.contains(t)&&!savePopup.contains(t))savePopup.classList.remove('open');if(!decodeMenu.contains(t)&&!decodePopup.contains(t))decodePopup.classList.remove('open');if(!rangeMenu.contains(t)&&!rangePopup.contains(t))rangePopup.classList.remove('open')},{once:false})

  applyLayout()
  setupLayoutSplitters(root)
  window.addEventListener('resize',()=>{applyLayout();scheduleLayoutSave();window.clearTimeout(windowSizeSaveTimer);windowSizeSaveTimer=window.setTimeout(()=>emitBackendEvent('window:size-save',''),350)})

  renderWaterfall=drawWaterfall(root.querySelector<HTMLCanvasElement>('#waterfall')!,root.querySelector<HTMLElement>('#waterfall-surface')!)
  const updateClock=()=>{const d=new Date(),pad=(n:number)=>String(n).padStart(2,'0');const clock=root.querySelector('#clock-display');if(clock)clock.innerHTML=`${d.getUTCFullYear()} ${d.getUTCMonth()+1}月 ${d.getUTCDate()}<br>${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}:${pad(d.getUTCSeconds())}`}
  updateClock();setInterval(updateClock,1000)
  const live=installWailsEvents(root)
  if(live)emitBackendEvent('settings:get','')
  renderQSOList()
  if(!live){
    const now=Date.now();const demo=[{message:'CQ JA1ABC PM95',snr:-13,dt:0.02,frequencyHz:1450},{message:'DL1XYZ 599 102',snr:-11,dt:-0.01,frequencyHz:1620},{message:'CQ VK3ABC',snr:-17,dt:0.08,frequencyHz:1325},{message:'JA1ABC BH2VSQ',snr:-15,dt:0.03,frequencyHz:1480},{message:'JA6JKQ 599 210',snr:-12,dt:0.01,frequencyHz:1510}]
    demo.forEach((d,i)=>addRow({...d,sequence:i+1,signalUtc:new Date(now+i*1000).toISOString(),receivedUtc:new Date(now+i*1000).toISOString(),confidence:1,callsigns:extractCalls(d.message)},root))
    root.querySelector('#health')!.textContent='UI Preview'
  }else root.querySelector('#health')!.textContent='Wails'
}
function extractCalls(text:string):Callsign[]{const out:Callsign[]=[];const re=/\b[A-Z0-9]{1,3}[0-9][A-Z]{1,4}\b/g;let m:RegExpExecArray|null;while((m=re.exec(text)))out.push({value:m[0],start:m.index,end:m.index+m[0].length,confidence:.8});return out}
function extractGrid(text:string){const m=String(text||'').toUpperCase().match(/\b[A-R]{2}\d{2}(?:[A-X]{2})?\b/);return m?.[0]||''}
app()
