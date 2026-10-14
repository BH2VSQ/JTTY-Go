// Optional browser regression: use an existing Chromium/Edge installation.
// JTTY_PLAYWRIGHT_MODULE may point at playwright-core installed outside the repo.
import assert from 'node:assert/strict';
import {readFile,mkdir} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';
import {resolve,dirname} from 'node:path';
const {chromium}=await import(process.env.JTTY_PLAYWRIGHT_MODULE||'playwright-core');
const root=resolve(dirname(fileURLToPath(import.meta.url)),'../..');
const source=await readFile(resolve(root,'internal/app/app.go'),'utf8');
const probeSource=source.slice(source.indexOf('func (a *App) registerHamlibEvents'));
const requests=[...probeSource.matchAll(/on\("([^"]+)"/g)].map(m=>m[1]);
const responses=['hamlib:models','hamlib:capabilities','hamlib:status','radio:serial-ports'];
assert(responses.every(name=>!requests.includes(name)),'Go reply names must not register request handlers');
const modelRequest=requests.find(name=>name.startsWith('hamlib:models'));
const capsRequest=requests.find(name=>name.startsWith('hamlib:capabilities'));
const portsRequest=requests.find(name=>name.startsWith('radio:serial-ports'));
const models=process.env.JTTY_MODELS_JSON?JSON.parse(await readFile(process.env.JTTY_MODELS_JSON,'utf8')):
  Array.from({length:700},(_,i)=>({modelId:i+1,label:`Hamlib test model ${String(i+1).padStart(4,'0')}`}));
const output=resolve(root,'.codex/verification/freeze-ui');await mkdir(output,{recursive:true});
const browser=await chromium.launch({headless:true,executablePath:process.env.JTTY_BROWSER||'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'});
try {
  const results=[];
  for(const viewport of [{width:1360,height:820},{width:1094,height:680},{width:1280,height:640}]) {
    const page=await browser.newPage({viewport});page.setDefaultTimeout(8000);
    const errors=[];page.on('pageerror',e=>errors.push(String(e)));
    await page.addInitScript(({models,modelRequest,capsRequest,portsRequest})=>{
      const js={},go={};let count=0;window.__saved=[];window.__events=[];
      // Unlike the earlier UI mock, Go replies go through BOTH listener sets.
      const emit=(name,payload)=>{
        if(++count>1500)throw Error('event storm');
        window.__events.push({name,payload});
        if(go[name])setTimeout(()=>go[name](payload),0);
        js[name]?.forEach(fn=>fn(payload));
      };
      go[modelRequest]=()=>emit('hamlib:models',models);
      go[capsRequest]=modelId=>emit('hamlib:capabilities',{modelId,available:true,capabilities:{modelId,portType:'serial',pttType:'Rig capable',hasGetFreq:true,hasSetFreq:true,hasGetSplitVFO:true,hasSetSplitVFO:true,hasCATPTTMicData:false,supportedModes:['USB','PKTUSB']}});
      go[portsRequest]=()=>emit('radio:serial-ports',['COM3','COM17']);
      go['settings:save']=payload=>{window.__saved.push(structuredClone(payload));emit('settings:state',payload);emit('settings:saved',{})};
      window.runtime={EventsOn:(name,fn)=>(js[name]??=[]).push(fn),EventsEmit:emit};
    },{models,modelRequest,capsRequest,portsRequest});
    await page.goto(process.env.JTTY_UI_URL||'http://127.0.0.1:8766');
    await page.locator('#config-menu').click();await page.locator('[data-tab="radio"]').click();
    const start=Date.now();
    for(let i=0;i<10;i++) {
      await page.locator('#set-backend').selectOption('hamlib-local');
      await page.waitForFunction(()=>document.querySelectorAll('#set-rig-model option').length>100);
      await page.locator('#set-backend').selectOption('rigctld-net');
    }
    await page.locator('input[name="pttm"][value="DTR"]').check();
    assert.equal(await page.locator('#set-backend').inputValue(),'rigctld-net');
    assert.equal(await page.locator('#set-ptt-port').inputValue(),'');
    await page.locator('#set-ptt-port').selectOption('COM3');
    await page.locator('#radio-test-ptt').scrollIntoViewIfNeeded();
    const geometry=await page.evaluate(()=>{
      const button=document.querySelector('#radio-test-ptt').getBoundingClientRect();
      const footer=document.querySelector('.settings-footer').getBoundingClientRect();
      const body=document.querySelector('.settings-body');
      return {buttonBottom:button.bottom,footerTop:footer.top,overflow:getComputedStyle(body).overflowY,columns:getComputedStyle(document.querySelector('.radio-net-ptt-v4')).gridTemplateColumns};
    });
    assert(geometry.buttonBottom<=geometry.footerTop,'footer must not cover PTT controls');
    assert.equal(geometry.overflow,'auto');
    assert.equal(geometry.columns.split(' ').length,2,'compact PTT choices use two columns');
    await page.screenshot({path:resolve(output,`net-${viewport.width}x${viewport.height}.png`)});
    await page.locator('#set-host').fill('192.168.1.20');await page.locator('#set-port').fill('4534');
    await page.locator('#set-backend').selectOption('hamlib-local');
    await page.locator('#set-rig-model').selectOption(String(models[0].modelId));
    await page.locator('#set-serial').fill('COM17');
    await page.locator('#radio-test-cat').scrollIntoViewIfNeeded();
    const visible=await page.locator('#radio-test-cat').boundingBox();const footer=await page.locator('.settings-footer').boundingBox();
    assert(visible.y+visible.height<=footer.y,'local connect button must be scrollable above footer');
    await page.screenshot({path:resolve(output,`local-${viewport.width}x${viewport.height}.png`)});
    await page.locator('#settings-apply').click();
    await page.waitForFunction(()=>window.__saved.length===1);
    const saved=await page.evaluate(()=>window.__saved[0]);
    assert.equal(saved.radio.local.serialPort,'COM17');assert.equal(saved.radio.net.host,'192.168.1.20');
    await page.locator('#settings-cancel').click();
    const events=await page.evaluate(()=>window.__events.map(x=>x.name));
    assert.equal(events.filter(x=>x===modelRequest).length,11);
    assert.equal(events.filter(x=>x==='hamlib:models').length,11);
    assert.deepEqual(errors,[]);
    results.push({viewport,modelCount:models.length,switches:20,elapsedMS:Date.now()-start,geometry});
    await page.close();
  }
  console.log(JSON.stringify(results,null,2));
} finally {await browser.close()}
