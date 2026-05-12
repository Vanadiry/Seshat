// Seshat shared JS — top bar, fetch dialog, SSE progress, API helpers
const API = window.BACKEND_URL || '/api';

async function api(url) {
  const r = await fetch(API + url);
  if (!r.ok) return null;
  return r.json();
}
function img(kind, id, size) { return API + '/v0/' + kind + 's/' + id + '/image?type=' + (size||'grid'); }

// ── Person name → ID lookup for infobox links ──
var _personMap = null;
function loadPersonMap() {
  if (_personMap) return Promise.resolve(_personMap);
  _personMap = {};
  return api('/v0/persons').then(function(list) {
    if (list) for (var i=0; i<list.length; i++) { _personMap[list[i].name] = list[i].id; }
    return _personMap;
  });
}
var _personRegex = null;
function buildPersonRegex() {
  if (_personRegex) return _personRegex;
  var names = Object.keys(_personMap);
  if (!names.length) return null;
  names.sort(function(a,b){return b.length-a.length});
  var escaped = names.map(function(n){return n.replace(/[.*+?^${}()|[\]\\]/g,'\\$&')});
  _personRegex = new RegExp(escaped.join('|'), 'g');
  return _personRegex;
}
function linkifyPerson(text) {
  if (!text || !_personMap || !Object.keys(_personMap).length) return text;
  var re = buildPersonRegex();
  if (!re) return text;
  return text.replace(re, function(match) {
    return '<a href="/person.html?id='+_personMap[match]+'" class="text-[#FE8A95] hover:underline">'+match+'</a>';
  });
}

// infoboxData extracts sidebar info from a subject/character/person detail.
// Handles infobox array + top-level fields (birthday, gender, blood_type).
function infoboxData(d) {
  var items = [];
  // Top-level fields
  if (d.gender) items.push(['性别', linkifyPerson(d.gender)]);
  if (d.blood_type) { var bt = {1:'A',2:'B',3:'AB',4:'O'}; items.push(['血型', bt[d.blood_type]||d.blood_type]); }
  if (d.birth_mon || d.birth_day) {
    var bd = [d.birth_year||'????', String(d.birth_mon||'?').padStart(2,'0'), String(d.birth_day||'?').padStart(2,'0')].join('-');
    items.push(['生日', bd]);
  }
  // infobox array
  var ib = d.infobox;
  if (typeof ib === 'string') { try { ib = JSON.parse(ib); } catch(e) { ib = []; } }
  if (Array.isArray(ib)) {
    for (var i = 0; i < ib.length; i++) {
      if (!ib[i].key) continue;
      var v = ib[i].value;
      if (typeof v === 'string') { items.push([ib[i].key, linkifyPerson(v)]); }
      else if (Array.isArray(v)) {
        // Show first string value from array
        for (var j = 0; j < v.length; j++) {
          if (typeof v[j] === 'string') { items.push([ib[i].key, linkifyPerson(v[j])]); break; }
        }
      }
    }
  }
  return items;
}

// displayName returns the primary display name based on language preference.
// display_lang=chinese: shows name_cn first, falls back to name
// display_lang=original (default): shows name first, falls back to name_cn
function primaryName(name, nameCN) {
  if (window.DISPLAY_LANG === 'chinese') return nameCN || name;
  return name || nameCN;
}
function subName(name, nameCN) {
  if (window.DISPLAY_LANG === 'chinese') { return (nameCN && name && name !== nameCN) ? name : ''; }
  return (nameCN && name !== nameCN) ? nameCN : '';
}

// ── Top bar injection ──
document.addEventListener('DOMContentLoaded', () => {
  const tb = document.getElementById('topbar');
  if (!tb) return;

  const path = location.pathname;
  function navCls(href) {
    if (href === '/' && (path === '/' || path === '/index.html')) return 'bg-[#30303b] text-white';
    if (href !== '/' && path.startsWith(href)) return 'bg-[#30303b] text-white';
    return 'hover:bg-[#30303b] hover:text-white';
  }

  tb.style.cssText = 'position:sticky;top:0;z-index:50';
  var custom = !!window.BACKEND_URL;
  tb.innerHTML = `
    <div class="${custom ? 'bg-[#dc2626]' : 'bg-[#1c1c22]'} border-b ${custom ? 'border-[rgba(255,255,255,.2)]' : 'border-[rgba(255,255,255,.12)]'}">
      <div class="max-w-[1200px] mx-auto px-5 flex items-center gap-3 h-12">
        <h1 class="text-lg font-bold cursor-pointer shrink-0" onclick="location.href='/'">Seshat</h1>
        <nav class="flex gap-1 ml-2 items-center">
          <a href="/" class="text-sub no-underline px-3 py-1.5 rounded-lg text-sm ${custom ? 'text-white hover:bg-[#ffffff22]' : navCls('/')}">动画</a>
          <a href="/character-list.html" class="text-sub no-underline px-3 py-1.5 rounded-lg text-sm ${custom ? 'text-white hover:bg-[#ffffff22]' : navCls('/character-list.html')}">角色</a>
          <a href="/person-list.html" class="text-sub no-underline px-3 py-1.5 rounded-lg text-sm ${custom ? 'text-white hover:bg-[#ffffff22]' : navCls('/person-list.html')}">人物</a>
          <a href="/tags.html" class="text-sub no-underline px-3 py-1.5 rounded-lg text-sm ${custom ? 'text-white hover:bg-[#ffffff22]' : navCls('/tags.html')}">标签</a>
          <a href="/doc/api" class="text-sub no-underline px-3 py-1.5 rounded-lg text-sm ${custom ? 'text-white hover:bg-[#ffffff22]' : navCls('/doc/api')}">API</a>
          ${custom ? '<span class="text-[#FFCA28] text-sm font-bold ml-2 shrink-0">⚠ 当前为自定义后端，非预期行为。</span>' : ''}
        </nav>
        <div class="flex-1"></div>
        <button onclick="location.href='/search.html'" class="px-3 py-1.5 rounded-lg border ${custom ? 'border-[rgba(255,255,255,.3)] text-white' : 'border-[rgba(255,255,255,.12)] text-sub'} bg-transparent text-sm cursor-pointer hover:bg-[#30303b] hover:text-white" title="搜索">🔍</button>
        <button id="btn-fetch" class="px-4 py-1.5 rounded-lg bg-[#FE8A95] text-white text-sm font-semibold cursor-pointer border-0 hover:opacity-90">+ 拉取</button>
      </div>
      <!-- Progress bar inside topbar -->
      <div id="pwrap" class="hidden" style="height:2px">
        <div id="pfill" class="h-full bg-[#FE8A95] w-0 transition-[width] duration-300"></div>
      </div>
      <div id="ptext" class="hidden text-[11px] text-sub px-5 pb-1 max-w-[1200px] mx-auto"></div>
    </div>`;

  document.getElementById('btn-fetch').addEventListener('click', openFetch);
});

// ── Fetch dialog ──
let dlgInited = false;
function initDialog() {
  if (dlgInited) return;
  dlgInited = true;
  const dlg = document.createElement('div');
  dlg.id = 'dlg-overlay';
  dlg.className = 'hidden fixed inset-0 z-[100] bg-black/60 flex items-center justify-center';
  dlg.innerHTML = `
    <div class="bg-[#24242e] border border-[rgba(255,255,255,.12)] rounded-xl p-6 w-[440px] max-w-[90vw] max-h-[90vh] overflow-y-auto">
      <h3 class="text-base font-bold mb-4">拉取数据</h3>
      <label class="text-xs text-sub block mb-1">动画 ID（逗号分隔）</label>
      <input id="fetch-input" placeholder="如 51 或 51,288" class="w-full px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-[#151518] text-sm mb-3">
      <button id="btn-do-fetch" class="w-full px-4 py-2 rounded-lg bg-[#FE8A95] text-white text-sm font-semibold cursor-pointer border-0 mb-4 hover:opacity-90">拉取指定动画</button>
      <div class="border-t border-[rgba(255,255,255,.06)] mb-4"></div>
      <label class="text-xs text-sub block mb-1">按 Tracker 名称拉取</label>
      <div class="flex gap-2 mb-3">
        <input id="tracker-input" placeholder="Tracker 名称" class="flex-1 px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-[#151518] text-sm">
        <button id="btn-tracker-fetch" class="px-4 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#30303b] hover:text-white whitespace-nowrap">拉取</button>
      </div>
      <button id="btn-user-fetch" class="w-full px-4 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#30303b] hover:text-white mb-4">拉取用户收藏</button>
      <div class="border-t border-[rgba(255,255,255,.06)] mb-4"></div>
      <div class="flex gap-2 mb-3">
        <button id="btn-refresh" class="flex-1 px-4 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#30303b] hover:text-white">刷新全部</button>
        <button id="btn-deep" class="flex-1 px-4 py-2 rounded-lg bg-[#dc2626] text-white text-sm font-semibold cursor-pointer border-0 hover:opacity-90">深度重建</button>
      </div>
      <button id="btn-close-dlg" class="w-full px-4 py-1.5 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#30303b]">取消</button>
    </div>`;
  dlg.addEventListener('click', e => { if (e.target === dlg) closeFetch(); });
  document.body.appendChild(dlg);

  document.getElementById('btn-do-fetch').addEventListener('click', doFetch);
  document.getElementById('btn-tracker-fetch').addEventListener('click', doTrackerFetch);
  document.getElementById('btn-user-fetch').addEventListener('click', doUserFetch);
  document.getElementById('btn-refresh').addEventListener('click', doRefreshAll);
  document.getElementById('btn-deep').addEventListener('click', doDeepRebuild);
  document.getElementById('btn-close-dlg').addEventListener('click', closeFetch);
}

function openFetch() {
  initDialog();
  document.getElementById('dlg-overlay').style.display = 'flex';
  document.getElementById('fetch-input').focus();
}
function closeFetch() {
  document.getElementById('dlg-overlay').style.display = 'none';
  const btn = document.getElementById('btn-deep');
  if (btn) { btn.textContent = '深度重建'; btn._confirm = false; }
  const btnR = document.getElementById('btn-refresh');
  if (btnR) { btnR.textContent = '刷新全部'; btnR._confirm = false; btnR.classList.remove('bg-[#f59e0b]', 'text-white', 'border-0'); }
}

async function doFetch() {
  const v = document.getElementById('fetch-input').value.trim();
  if (!v) return;
  closeFetch();
  const ids = v.split(',').map(s => parseInt(s.trim())).filter(Boolean);
  if (!ids.length) return;
  const res = await fetch(API + '/v0/fetch/subject', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({ids})});
  const d = await res.json();
  if (d.task_id) startProgress(d.task_id);
}

async function doTrackerFetch() {
  const v = document.getElementById('tracker-input').value.trim();
  if (!v) return;
  closeFetch();
  const res = await fetch(API + '/v0/fetch/tracker', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({name:v})});
  const d = await res.json();
  if (d.task_id) startProgress(d.task_id);
}

async function doUserFetch() {
  closeFetch();
  var res = await fetch(API + '/v0/fetch/user', {method:'POST'});
  if (!res.ok) {
    var d = await res.json().catch(function(){return {error:'请求失败 ('+res.status+')'}});
    alert('拉取用户收藏失败: '+(d.error||'状态 '+res.status));
    return;
  }
  var d = await res.json();
  if (d.task_id) startProgress(d.task_id);
}

async function doRefreshAll() {
  const btn = document.getElementById('btn-refresh');
  if (!btn._confirm) {
    btn._confirm = true;
    btn.textContent = '确认刷新全部？';
    btn.classList.add('bg-[#f59e0b]', 'text-white', 'border-0');
    return;
  }
  closeFetch();
  const res = await fetch(API + '/v0/fetch/all', {method:'POST'});
  const d = await res.json();
  if (d.task_id) startProgress(d.task_id);
}

async function doDeepRebuild() {
  const btn = document.getElementById('btn-deep');
  if (!btn._confirm) {
    btn._confirm = true;
    btn.textContent = '确认深度重建？';
    return;
  }
  closeFetch();
  const res = await fetch(API + '/v0/fetch/deep', {method:'POST'});
  const d = await res.json();
  if (d.task_id) startProgress(d.task_id);
}

// ── SSE progress (inline in topbar) ──
function startProgress(taskId) {
  const w = document.getElementById('pwrap');
  const t = document.getElementById('ptext');
  if (!w || !t) return;
  w.style.display = 'block';
  t.style.display = 'block';
  const fill = document.getElementById('pfill');
  fill.style.width = '0%'; t.textContent = '连接中…';
  const evt = new EventSource(API + '/v0/progress/' + taskId);
  evt.onmessage = function(e) {
    const d = JSON.parse(e.data);
    if (d.step === 'complete') {
      fill.style.width = '100%'; t.textContent = '完成'; evt.close();
      setTimeout(() => { w.style.display = 'none'; t.style.display = 'none'; if (typeof onFetchDone==='function') onFetchDone(); }, 1500);
    } else if (d.done !== undefined && d.total) {
      fill.style.width = Math.round(d.done/d.total*100) + '%';
      t.textContent = (d.step||'') + ' ' + d.done + '/' + d.total + (d.speed?' · '+d.speed:'');
    } else if (d.status) {
      t.textContent = (d.step||'') + ': ' + d.status;
    }
  };
  evt.onerror = () => evt.close();
}
