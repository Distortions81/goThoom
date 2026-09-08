// Test search and annotation behavior without a browser or external packages.
const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
const source = fs.readFileSync(path.join(__dirname, '../website/help/manual.js'), 'utf8');
const searchIndex = JSON.parse(fs.readFileSync(path.join(__dirname, '../website/help/search-index.json'), 'utf8'));
class Element {
  constructor() { this.children=[];this.attrs={};this.handlers={};this.style={};this.dataset={};this.textContent='';this.hidden=false; }
  addEventListener(name, callback) {this.handlers[name]=callback;}
  append(...children) {this.children.push(...children);}
  replaceChildren(...children) {this.children=children;}
  getAttribute(key) {return this.attrs[key];}
  setAttribute(key,value) {this.attrs[key]=value;}
}
function searchApp(fetch) {
  const input = new Element(), status = new Element(), results = new Element();
  const nodes = {'help-search':input,'help-search-status':status,'help-search-results':results};
  vm.runInNewContext(source,{fetch,document:{querySelectorAll:()=>[],getElementById:id=>nodes[id],createElement:()=>new Element()}});
  return {input,status,results,search:async value=>{input.value=value;await input.handlers.input();}};
}
test('search finds real walkthroughs and control-reference entries',async()=>{
  const app=searchApp(async()=>({ok:true,json:async()=>searchIndex}));
  await app.search('local command');
  assert.ok(app.results.children.some(item=>item.children[0].href==='automation.html#first-go-script'));
  await app.search('sprite cache');
  assert.ok(app.results.children.some(item=>item.children[0].href==='performance.html#sprite-cache'));
  await app.search('auto-size side panels');
  assert.ok(app.results.children.some(item=>item.children[0].href.includes('reference.html#tiled')));
  await app.search('nonexistentxxxxxxxx');
  assert.match(app.status.textContent,/No matching/);assert.equal(app.results.hidden,true);
  await app.search('');assert.match(app.status.textContent,/Search the manual/);
});
test('a cleared query cannot be replaced by an older pending request',async()=>{
  let resolve;
  const app=searchApp(()=>new Promise(r=>{resolve=r;}));
  const pending=app.search('command');await app.search('');
  resolve({ok:true,json:async()=>searchIndex});await pending;
  assert.equal(app.results.hidden,true);assert.match(app.status.textContent,/Search the manual/);
});
test('failed search can be retried and has a useful fallback',async()=>{
  let attempts=0;
  const app=searchApp(async()=>{if(++attempts===1)throw new Error('offline');return {ok:true,json:async()=>searchIndex};});
  await app.search('speech');assert.match(app.status.textContent,/browser’s Find/);
  await app.search('speech');assert.equal(app.results.hidden,false);assert.equal(attempts,2);
});
test('highlight toggle preserves the explanatory text',()=>{
  const toggle=new Element(),overlay=new Element(),legend=new Element(),figure=new Element();
  toggle.attrs['aria-pressed']='true';
  legend.textContent='An explanation of a consequence that is not obvious from the label.';
  figure.querySelector=selector=>({'.help-highlights':overlay,'.help-legend':legend}[selector]);
  toggle.closest=()=>figure;
  vm.runInNewContext(source,{document:{querySelectorAll:()=>[toggle],getElementById:()=>null}});
  toggle.handlers.click();assert.equal(overlay.hidden,true);assert.equal(toggle.attrs['aria-pressed'],'false');
  toggle.handlers.click();assert.equal(overlay.hidden,false);assert.equal(toggle.attrs['aria-pressed'],'true');
  assert.equal(legend.textContent,'An explanation of a consequence that is not obvious from the label.');
});
