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
  if (_personMap && Object.keys(_personMap).length) return Promise.resolve(_personMap);
  return api('/v0/person-names').then(function(data) {
    _personMap = data || {};
    _personRegex = null;
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
      if (typeof v === 'string') {
        items.push([ib[i].key, linkifyPerson(v), 0]);
      } else if (Array.isArray(v)) {
        if (v.length > 0 && typeof v[0] === 'object' && v[0].k) {
          // Nested key-value objects like aliases: [{k: "纯假名", v: "..."}]
          items.push([ib[i].key, '', 0]);
          for (var j = 0; j < v.length; j++) {
            if (v[j].k) items.push([v[j].k, linkifyPerson(v[j].v||''), 1]);
          }
        } else {
          // Simple string array
          for (var j = 0; j < v.length; j++) {
            if (typeof v[j] === 'string') { items.push([ib[i].key, linkifyPerson(v[j]), 0]); break; }
          }
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

// extractCNFromInfobox extracts 简体中文名 from character/person detail.
function extractCN(d) {
  var ib = d.infobox;
  if (typeof ib === 'string') { try { ib = JSON.parse(ib); } catch(e) { return ''; } }
  if (!Array.isArray(ib)) return '';
  for (var i=0; i<ib.length; i++) {
    if (ib[i].key === '简体中文名' && typeof ib[i].value === 'string') return ib[i].value;
  }
  return '';
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
      <div id="ptext" class="hidden text-[12px] text-sub px-5 pb-1 max-w-[1200px] mx-auto"></div>
    </div>`;

  document.getElementById('btn-fetch').addEventListener('click', openFetch);
});

// ── Fetch dialog ──
var dlgInited = false, confirmCb = null;

function initDialog() {
  if (dlgInited) return;
  dlgInited = true;
  var dlg = document.createElement('div');
  dlg.id = 'dlg-overlay';
  dlg.className = 'hidden fixed inset-0 z-[100] bg-black/60 flex items-center justify-center';
  dlg.innerHTML =
    '<div class="bg-[#24242e] border border-[rgba(255,255,255,.12)] rounded-xl p-6 w-[460px] max-w-[90vw] max-h-[90vh] overflow-y-auto">'+
    '<div class="flex items-center gap-3 mb-4">'+
    '<button id="btn-close-dlg" class="w-7 h-7 rounded-full border border-[rgba(255,255,255,.2)] flex items-center justify-center text-sub hover:text-white hover:border-[rgba(255,255,255,.4)] shrink-0 no-underline text-xs cursor-pointer bg-transparent" title="关闭">&times;</button>'+
    '<h3 class="text-base font-bold">拉取数据</h3></div>'+

    // ID input row
    '<label class="text-xs text-sub block mb-1">拉取指定动画的完整数据到本地</label>'+
    '<div class="flex gap-2 mb-4">'+
    '<input id="fetch-input" placeholder="动画 ID，如 51 或 51,288" class="flex-1 px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-[#151518] text-sm">'+
    '<button id="btn-do-fetch" class="w-9 h-9 rounded-lg bg-[#FE8A95] text-white font-bold cursor-pointer border-0 hover:opacity-90 flex items-center justify-center" title="拉取">&check;</button></div>'+

    // Tracker input row
    '<label class="text-xs text-sub block mb-1">拉取指定 Tracker 列表中的全部动画</label>'+
    '<div class="flex gap-2 mb-4">'+
    '<input id="tracker-input" placeholder="Tracker 名称" class="flex-1 px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-[#151518] text-sm">'+
    '<button id="btn-tracker-fetch" class="w-9 h-9 rounded-lg bg-[#FE8A95] text-white font-bold cursor-pointer border-0 hover:opacity-90 flex items-center justify-center" title="拉取">&check;</button></div>'+

    '<div class="border-t border-[rgba(255,255,255,.06)] mb-4"></div>'+

    // Action buttons
    '<div class="grid grid-cols-2 gap-2 mb-2">'+
    '<button id="btn-user" class="px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#30303b] hover:text-white">拉取用户收藏</button>'+
    '<button id="btn-update" class="px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#30303b] hover:text-white">增量更新</button>'+
    '</div>'+
    '<button id="btn-all" class="w-full px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#30303b] hover:text-white mb-2">刷新全部</button>'+

    // Danger zone
    '<div class="border-t border-[rgba(255,255,255,.06)] mb-3"></div>'+
    '<button id="btn-danger-toggle" class="w-full px-3 py-2 rounded-lg border border-[rgba(255,0,0,.3)] bg-transparent text-[#dc2626] text-sm cursor-pointer hover:bg-[#3a1a1a]">危险区</button>'+
    '<div id="danger-zone" class="hidden mt-2 grid grid-cols-2 gap-2">'+
    '<button id="btn-index" class="px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#30303b] hover:text-white">重建索引</button>'+
    '<button id="btn-images" class="px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#30303b] hover:text-white">重建图像</button>'+
    '<button id="btn-deep" class="px-3 py-2 rounded-lg bg-[#dc2626] text-white text-sm font-semibold cursor-pointer border-0 hover:opacity-90 col-span-2">重建全部</button>'+
    '</div>'+

    // Confirmation overlay
    '<div id="confirm-overlay" class="hidden absolute inset-0 bg-black/70 rounded-xl flex items-center justify-center z-10">'+
    '<div class="bg-[#2a2a30] border border-[rgba(255,255,255,.12)] rounded-lg p-5 w-[340px] max-w-[85vw]">'+
    '<p id="confirm-msg" class="text-sm mb-4 leading-relaxed"></p>'+
    '<button id="btn-confirm-exec" class="w-full px-4 py-2 rounded-lg bg-[#FE8A95] text-white text-sm font-semibold cursor-pointer border-0 hover:opacity-90 mb-2">执行</button>'+
    '<button id="btn-confirm-cancel" class="w-full px-4 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#30303b]">取消</button>'+
    '</div></div></div></div>';

  dlg.addEventListener('click', function(e) { if (e.target === dlg) closeFetch(); });
  document.body.appendChild(dlg);

  document.getElementById('btn-close-dlg').addEventListener('click', closeFetch);
  document.getElementById('btn-confirm-cancel').addEventListener('click', closeConfirm);

  // Danger zone toggle (expand only, no collapse)
  document.getElementById('btn-danger-toggle').addEventListener('click', function() {
    document.getElementById('danger-zone').classList.remove('hidden');
    this.remove();
  });

  // Wire up action buttons with confirmation
  document.getElementById('btn-do-fetch').addEventListener('click', function() {
    var ids = document.getElementById('fetch-input').value.trim();
    if (!ids) return;
    confirmAction('将从上游拉取动画 '+ids.split(',').map(function(s){return '#'+s.trim()}).join(', ')+' 的完整数据（角色、人员、剧集、图片）', doFetch);
  });
  document.getElementById('btn-tracker-fetch').addEventListener('click', function() {
    var name = document.getElementById('tracker-input').value.trim();
    if (!name) return;
    confirmAction('将从上游拉取 Tracker ['+name+'] 中的全部动画数据', doTrackerFetch);
  });
  document.getElementById('btn-user').addEventListener('click', function() {
    confirmAction('将从上游拉取用户收藏列表，存入 Tracker [user] 中（不会覆盖已有 Tracker）', doUserFetch);
  });
  document.getElementById('btn-update').addEventListener('click', function() {
    confirmAction('对比 Tracker 与本地缓存，仅拉取新增的动画（不会删除已有数据）', doFetchUpdate);
  });
  document.getElementById('btn-all').addEventListener('click', function() {
    confirmAction('将从上游重新拉取全部 Tracker 数据，覆盖已有内容（本地多余数据不会删除）', doRefreshAll);
  });
  document.getElementById('btn-index').addEventListener('click', function() {
    confirmAction('扫描本地已缓存的 JSON 文件，重建所有索引（不会请求上游）', doFetchIndex);
  });
  document.getElementById('btn-images').addEventListener('click', function() {
    confirmAction('将删除并重新下载全部图像（根据现有 list 文件）', doFetchImages);
  });
  document.getElementById('btn-deep').addEventListener('click', function() {
    confirmAction('⚠ 这将删除本地全部缓存文件，从上游完整重建。此操作不可逆。', doDeepRebuild);
  });
}

function confirmAction(msg, cb) {
  document.getElementById('confirm-msg').textContent = msg;
  document.getElementById('confirm-overlay').style.display = 'flex';
  confirmCb = cb;
  document.getElementById('btn-confirm-exec').onclick = function() {
    closeConfirm();
    cb();
  };
}
function closeConfirm() {
  document.getElementById('confirm-overlay').style.display = 'none';
  confirmCb = null;
}

function openFetch() {
  initDialog();
  document.getElementById('dlg-overlay').style.display = 'flex';
  document.getElementById('fetch-input').focus();
}
function closeFetch() {
  document.getElementById('dlg-overlay').style.display = 'none';
  closeConfirm();
}

async function doFetch() {
  var v = document.getElementById('fetch-input').value.trim();
  if (!v) return;
  closeFetch();
  var ids = v.split(',').map(function(s){return parseInt(s.trim())}).filter(Boolean);
  if (!ids.length) return;
  var res = await fetch(API + '/v0/fetch/subject', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({ids:ids})});
  var d = await res.json();
  if (d.task_id) startProgress(d.task_id);
}

async function doTrackerFetch() {
  var v = document.getElementById('tracker-input').value.trim();
  if (!v) return;
  closeFetch();
  var res = await fetch(API + '/v0/fetch/tracker', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({name:v})});
  var d = await res.json();
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
  closeFetch();
  var res = await fetch(API + '/v0/fetch/all', {method:'POST'});
  var d = await res.json();
  if (d.task_id) startProgress(d.task_id);
}

async function doDeepRebuild() {
  closeFetch();
  var res = await fetch(API + '/v0/fetch/deep', {method:'POST'});
  var d = await res.json();
  if (d.task_id) startProgress(d.task_id);
}

async function doFetchUpdate() {
  closeFetch();
  var res = await fetch(API + '/v0/fetch/update', {method:'POST'});
  var d = await res.json();
  if (d.task_id) startProgress(d.task_id);
}

async function doFetchImages() {
  closeFetch();
  var res = await fetch(API + '/v0/fetch/images', {method:'POST'});
  var d = await res.json();
  if (d.task_id) startProgress(d.task_id);
}

async function doFetchIndex() {
  closeFetch();
  var res = await fetch(API + '/v0/fetch/index', {method:'POST'});
  var d = await res.json();
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
