// Seshat shared JS — top bar, fetch dialog, SSE progress, API helpers
const API = window.BACKEND_URL || '/api';

// ── UI text (edit here to customize dialog messages) ──
var MSG = {
  // Top bar
  customBackendWarn: '⚠ 当前为自定义后端，非预期行为。',

  // Fetch dialog
  dialogTitle: '从上游拉取数据',
  idLabel: '拉取一个或一组动画',
  idPlaceholder: '动画 ID，如 51 或 51,288',
  trackerLabel: '拉取或创建 Tracker',
  trackerPlaceholder: 'Tracker 名称',
  btnUserFetch: '拉取用户收藏',
  btnUpdate: '增量更新',
  btnRefreshAll: '刷新全部',
  btnDangerZone: '危险区',
  btnRebuildIndex: '重建索引',
  btnRebuildAll: '重建全部',
  btnExecute: '执行',
  btnCancel: '取消',
  btnClose: '关闭',

  // Confirm messages
  confirmFetchId: function(ids) { return '将从拉取动画 '+ids.split(',').map(function(s){return '#'+s.trim()}).join(', ')+' 的完整数据'; },
  confirmTrackerFetch: function(name) { return '将拉取 Tracker ['+name+'] 中记录的全部动画数据？'; },
  confirmTrackerCreate: function(name) { return '未找到 Tracker ['+name+']。\n是否创建？'; },
  trackerCreated: function(name) { var d = (window.SESHAT_HOME ? window.SESHAT_HOME+"/tracker" : '~/.vSoft/Seshat/tracker'); return 'Tracker ['+name+'] 已创建。请在 '+d+'/'+name+'.toml 中填写动画 ID，然后返回此处重新拉取。'; },
  confirmUserFetch: '将拉取用户收藏列表，存入 Tracker 中。\n拉取成功够，请执行增量更新。',
  confirmUpdate: '将对比本地与上游数据，并添加缺失的数据。',
  confirmRefreshAll: '将从上游拉取全部 Tracker 数据，覆盖已有内容。本地多余数据不会删除。',
  confirmRebuildIndex: '将从本地 JSON 中重建所有索引',
  confirmRebuildAll: function() { var d = window.SESHAT_HOME || '~/.vSoft/Seshat'; return '将删除 '+d+'/data 的全部数据，并完整重建。'; },

  // Validation
  errInvalidTrackerName: 'Tracker 名称仅允许大小写字母、数字、短横线和下划线',
  errUserFetchFailed: '拉取用户收藏失败: ',

  // Progress
  progressConnecting: '连接中…',
  progressDone: '完成',

  // Infobox
  infoboxGender: '性别',
  infoboxBloodType: '血型',
  infoboxBirthday: '生日',
  bloodTypeMap: {1:'A',2:'B',3:'AB',4:'O'},

  // Episode types
  epTypes: {0:'本篇',1:'SP',2:'OP',3:'ED',4:'预告',5:'MAD',6:'其他'},
  // Subject types (used in character appearances)
  subjectTypes: {1:'书籍',2:'动画',3:'音乐',4:'游戏',6:'三次元'},
  genderMap: {male:'男', female:'女'},
  careerMap: {actor:'演员', artist:'艺术家', illustrator:'插画家', producer:'制作人', seiyu:'声优', writer:'作家'},
};

var _remoteURLs = {};
function isRemote(url) { return !!_remoteURLs[url]; }
function markRemote(url) { _remoteURLs[url] = true; }

async function api(url) {
  var r = await fetch(API + url);
  if (!r.ok && window.FALLBACK_URL) {
    var fr = await fetch(window.FALLBACK_URL + url);
    if (fr.ok) { markRemote(url); return fr.json(); }
  }
  if (!r.ok) return null;
  return r.json();
}
// apiLocal always uses local backend for list/index data
function apiLocal(url) {
  return fetch('/api' + url).then(function(r){ return r.ok ? r.json() : null; });
}
function img(kind, id, size) {
  return API + '/v0/' + kind + 's/' + id + '/image?type=' + (size||'grid');
}
// imgOnError returns the onload + onerror attributes for <img> tags with fallback support.
// onload detects the 150x150 placeholder and retries via FALLBACK_URL.
// onerror retries via FALLBACK_URL on network error, then falls back to /images/no-image.png.
// When fallback is triggered, addRemoteGlobe() wraps the image and adds a globe overlay.
function imgOnError(kind, id, size) {
  if (!window.FALLBACK_URL) return 'onerror="this.remove()"';
  var fb = window.FALLBACK_URL + '/v0/' + kind + 's/' + id + '/image?type=' + (size||'grid');
  return 'onload="if(!this._c){this._c=1;if(!this._r&&this.naturalWidth===150&&this.naturalHeight===150){this.src=\''+fb+'\';this._r=1;addRemoteGlobe(this);return;}}if(this._r&&!this._g){addRemoteGlobe(this);}" onerror="if(!this._r){this._r=1;this.src=\''+fb+'\';addRemoteGlobe(this);}else{this.src=\'/images/no-image.png\';}"';
}

// ── Remote globe marker helpers ──
// globeIcon returns an <img> tag for the globe marker. Use in headers and section titles.
function globeIcon(cls) {
  return '<img src="/assets/global.svg" class="'+(cls||'w-5 h-5')+' shrink-0" title="此数据来自远程">';
}
// addRemoteGlobe wraps a remote-fallback image and adds a globe overlay at top-left corner.
function addRemoteGlobe(el) {
  if (el._g) return;
  el._g = 1;
  var wrap = document.createElement('span');
  var s = 'position:relative;display:inline-block;line-height:0;vertical-align:top;';
  if (el.classList.contains('w-full')) s += 'width:100%;';
  if (el.style.width) s += 'width:' + el.style.width + ';';
  if (el.style.height) s += 'height:' + el.style.height + ';';
  wrap.style.cssText = s;
  el.parentNode.insertBefore(wrap, el);
  wrap.appendChild(el);
  var globe = document.createElement('img');
  globe.src = '/assets/global.svg';
  globe.style.cssText = 'position:absolute;top:4px;left:4px;z-index:10;width:20px;height:20px;pointer-events:none;';
  globe.title = '图片来自远程';
  wrap.appendChild(globe);
}

// ── Name → ID lookup maps (subjects/characters/persons) ──
var _subjectMap = null, _charMap = null, _personMap = null;
function loadPersonMap() {
  if (_personMap && Object.keys(_personMap).length) return Promise.resolve();
  return Promise.all([
    api('/v0/subjects/name'),
    api('/v0/characters/name'),
    api('/v0/persons/name')
  ]).then(function(results) {
    _subjectMap = results[0] || {};
    _charMap = results[1] || {};
    _personMap = results[2] || {};
    _personRegex = null;
  });
}

// ── Local name lookup (subjects > characters > persons) ──
function lookupLocalName(name) {
  if (_subjectMap && _subjectMap[name]) return '/subject.html?id=' + _subjectMap[name];
  if (_charMap && _charMap[name]) return '/character.html?id=' + _charMap[name];
  if (_personMap && _personMap[name]) return '/person.html?id=' + _personMap[name];
  return null;
}

// ── Regex building for name matching ──
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

function buildRegexForMap(map) {
  if (!map) return null;
  var names = Object.keys(map);
  if (!names.length) return null;
  names.sort(function(a,b){return b.length-a.length});
  var escaped = names.map(function(n){return n.replace(/[.*+?^${}()|[\]\\]/g,'\\$&')});
  return new RegExp(escaped.join('|'), 'g');
}

function linkifyByMap(text, map, hrefPrefix) {
  if (!text || !map || !Object.keys(map).length) return text;
  var re = buildRegexForMap(map);
  if (!re) return text;
  return text.replace(re, function(match) {
    return '<a href="' + hrefPrefix + map[match] + '" class="text-[#FE8A95] hover:underline">' + match + '</a>';
  });
}

// Keep existing linkifyPerson for backward compat (used in infobox)
function linkifyPerson(text) { return linkifyByMap(text, _personMap, '/person.html?id='); }

// Apply all three name matchers, skipping text already inside <a> tags
function linkifyAllNames(text) {
  if (!text) return text;
  var parts = text.split(/(<a\b[^>]*>.*?<\/a>)/);
  for (var i = 0; i < parts.length; i++) {
    if (parts[i].indexOf('<a') === 0) continue;
    var t = parts[i];
    t = linkifyByMap(t, _subjectMap, '/subject.html?id=');
    t = linkifyByMap(t, _charMap, '/character.html?id=');
    t = linkifyPerson(t);
    parts[i] = t;
  }
  return parts.join('');
}

// ── linkifyURL：检测 URL 并转为可点击链接 ──
function linkifyURL(text) {
  if (!text) return text;
  return text.replace(/(https?:\/\/[^\s<]+)/g, '<a href="$1" target="_blank" rel="noopener" class="text-[#FE8A95] hover:underline"><img src="/assets/link.svg" class="w-3.5 h-3.5 inline-block mr-0.5 align-text-bottom">$1</a>');
}

// ── linkifyBBCode：转换 [url=...]...[/url]，优先匹配本地 name ──
function linkifyBBCode(text) {
  if (!text) return text;
  return text.replace(/\[url=(https?:\/\/[^\]]+)\](.+?)\[\/url\]/g, function(match, url, inner) {
    var local = lookupLocalName(inner);
    if (local) return '<a href="' + local + '" class="text-[#FE8A95] hover:underline">' + inner + '</a>';
    return '<a href="' + url + '" target="_blank" rel="noopener" class="text-[#FE8A95] hover:underline"><img src="/assets/link.svg" class="w-3.5 h-3.5 inline-block mr-0.5 align-text-bottom">' + inner + '</a>';
  });
}

// ── formatSummary：BBCode → name 匹配 → 换行 ──
function formatSummary(text) {
  if (!text) return text;
  return linkifyAllNames(linkifyBBCode(text)).replace(/\r\n|\n/g, '<br><span style="display:block;height:0.5em"></span>');
}

// ── infoboxData：提取侧栏信息（infobox + 顶层字段）──
function infoboxData(d) {
  var items = [];
  // Top-level fields
  if (d.gender) items.push([MSG.infoboxGender, MSG.genderMap[d.gender]||d.gender]);
  if (d.blood_type) { var bt = MSG.bloodTypeMap; items.push([MSG.infoboxBloodType, bt[d.blood_type]||d.blood_type]); }
  if (d.birth_mon || d.birth_day) {
    var bd = [];
    if (d.birth_year) bd.push(d.birth_year+'年');
    if (d.birth_mon) bd.push(d.birth_mon+'月');
    if (d.birth_day) bd.push(d.birth_day+'日');
    items.push([MSG.infoboxBirthday, bd.join('')]);
  }
  // infobox array
  var ib = d.infobox;
  if (typeof ib === 'string') { try { ib = JSON.parse(ib); } catch(e) { ib = []; } }
  if (Array.isArray(ib)) {
    for (var i = 0; i < ib.length; i++) {
      if (!ib[i].key) continue;
      var v = ib[i].value;
      if (typeof v === 'string') {
        items.push([ib[i].key, linkifyURL(linkifyAllNames(linkifyBBCode(v))), 0]);
      } else if (Array.isArray(v)) {
        if (v.length > 0 && typeof v[0] === 'object') {
          if (v[0].k) {
            // Key-value pairs: [{k: "纯假名", v: "..."}]
            items.push([ib[i].key, '', 0]);
            for (var j = 0; j < v.length; j++) {
              if (v[j].k) items.push([v[j].k, linkifyURL(linkifyAllNames(linkifyBBCode(v[j].v||''))), 1]);
            }
          } else if (v[0].v) {
            // Value-only objects: [{v: "クラナド"}]
            items.push([ib[i].key, '', 0]);
            for (var j = 0; j < v.length; j++) {
              if (v[j].v) items.push(['', linkifyURL(linkifyAllNames(linkifyBBCode(v[j].v))), 1]);
            }
          } else {
            // Fallback: show first string from array
            for (var j = 0; j < v.length; j++) {
              if (typeof v[j] === 'string') { items.push([ib[i].key, linkifyURL(linkifyAllNames(linkifyBBCode(v[j]))), 0]); break; }
            }
          }
        } else {
          // Plain string array
          for (var j = 0; j < v.length; j++) {
            if (typeof v[j] === 'string') { items.push([ib[i].key, linkifyURL(linkifyAllNames(linkifyBBCode(v[j]))), 0]); break; }
          }
        }
      }
    }
  }
  return items;
}

// ── primaryName / subName：根据 display_lang 配置决定主副标题顺序 ──
function primaryName(name, nameCN) {
  if (window.PREFER_LANG === 'chinese') return nameCN || name;
  return name || nameCN;
}
function subName(name, nameCN) {
  if (window.PREFER_LANG === 'chinese') { return (nameCN && name && name !== nameCN) ? name : ''; }
  return (nameCN && name !== nameCN) ? nameCN : '';
}

// ── extractCN：从 infobox 提取简体中文名 ──
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
    var p = path;
    if (p === '/subject.html') p = '/';
    else if (p === '/character.html') p = '/character-list.html';
    else if (p === '/person.html') p = '/person-list.html';
    else if (p === '/tags-subject.html') p = '/tags.html';
    if (href === '/' && (p === '/' || p === '/index.html')) return 'bg-[#30303b] text-white';
    if (href !== '/' && p.startsWith(href)) return 'bg-[#30303b] text-white';
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

  // Reconnect to active tasks after page refresh
  api('/v0/tasks').then(function(d) {
    if (d && d.tasks && d.tasks.length) {
      for (var i=0; i<d.tasks.length; i++) {
        startProgress(d.tasks[i].id, d.tasks[i].label);
      }
    }
  });
});

// ── 拉取弹窗：输入行 + 操作按钮 + 危险区 + 二次确认 ──
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
    '<input id="fetch-input" placeholder="动画 ID，如 51 或 51,288" pattern="[0-9, ]*" class="flex-1 px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-[#151518] text-sm">'+
    '<button id="btn-do-fetch" class="w-9 h-9 rounded-lg bg-[#FE8A95] text-white font-bold cursor-pointer border-0 hover:opacity-90 flex items-center justify-center" title="拉取">&check;</button></div>'+

    // Tracker input row
    '<label class="text-xs text-sub block mb-1">拉取或创建 Tracker</label>'+
    '<div class="flex gap-2 mb-4">'+
    '<input id="tracker-input" placeholder="Tracker 名称" pattern="[a-zA-Z0-9_\\-]*" class="flex-1 px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-[#151518] text-sm">'+
    '<button id="btn-tracker-fetch" class="w-9 h-9 rounded-lg bg-[#FE8A95] text-white font-bold cursor-pointer border-0 hover:opacity-90 flex items-center justify-center" title="拉取或创建">&check;</button></div>'+

    

    // Action buttons
    '<div class="grid grid-cols-3 gap-2 mb-2">'+
    '<button id="btn-user" class="px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#30303b] hover:text-white">拉取用户收藏</button>'+
    '<button id="btn-update" class="px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#30303b] hover:text-white">增量更新</button>'+
    '<button id="btn-all" class="px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#30303b] hover:text-white">刷新全部</button>'+
    '</div>'+

    // Danger zone
    '<button id="btn-danger-toggle" class="w-full px-3 py-2 rounded-lg border border-[rgba(255,0,0,.3)] bg-transparent text-[#dc2626] text-sm cursor-pointer hover:bg-[#3a1a1a]">危险区</button>'+
    '<div id="danger-zone" class="hidden mt-2 grid grid-cols-2 gap-2">'+
    '<button id="btn-index" class="px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#30303b] hover:text-white">重建索引</button>'+
    '<button id="btn-deep" class="px-3 py-2 rounded-lg bg-[#dc2626] text-white text-sm font-semibold cursor-pointer border-0 hover:opacity-90">重建全部</button>'+
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
    if (!/^[a-zA-Z0-9_-]+$/.test(name)) { showError(MSG.errInvalidTrackerName); return; }
    // Check if tracker exists
    api('/v0/tracker').then(function(list) {
      var found = false;
      if (list) for (var i=0; i<list.length; i++) { if (list[i].name === name) { found = true; break; } }
      if (found) {
        confirmAction('确认拉取 Tracker ['+name+'] 中的全部动画数据？', doTrackerFetch);
      } else {
        confirmAction('未找到 Tracker ['+name+']。是否创建它？\n创建后请在 Tracker 文件中填写动画 ID，返回此处重新拉取。', function() {
          fetch(API + '/v0/tracker/create', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({name:name})})
            .then(function(r){return r.json()}).then(function(d){
              if (d.error) { showError(d.error); }
              else { showError(MSG.trackerCreated(name)); }
            });
        });
      }
    });
  });
  document.getElementById('btn-user').addEventListener('click', function() {
    if (!window.USERNAME) { showError('未在配置文件中设置用户名（[user] username），无法拉取用户收藏。'); return; }
    confirmAction('将从上游拉取用户 '+window.USERNAME+' 的收藏列表并存入 Tracker。完成后请运行「增量更新」来缓存动画数据。', doUserFetch);
  });
  document.getElementById('btn-update').addEventListener('click', function() {
    confirmAction(MSG.confirmUpdate, doFetchUpdate);
  });
  document.getElementById('btn-all').addEventListener('click', function() {
    confirmAction(MSG.confirmRefreshAll, doRefreshAll);
  });
  document.getElementById('btn-index').addEventListener('click', function() {
    confirmAction(MSG.confirmRebuildIndex, doFetchIndex);
  });
  document.getElementById('btn-deep').addEventListener('click', function() {
    confirmAction(MSG.confirmRebuildAll(), doDeepRebuild);
  });
}

function confirmAction(msg, cb) {
  document.getElementById('confirm-msg').textContent = msg;
  document.getElementById('confirm-overlay').style.display = 'flex';
  confirmCb = cb;
  document.getElementById('btn-confirm-exec').style.display = 'block';
  document.getElementById('btn-confirm-cancel').textContent = '取消';
  document.getElementById('btn-confirm-exec').onclick = function() {
    closeConfirm();
    cb();
  };
}
function showError(msg) {
  document.getElementById('dlg-overlay').style.display = 'flex';
  document.getElementById('confirm-msg').textContent = msg;
  document.getElementById('confirm-overlay').style.display = 'flex';
  document.getElementById('btn-confirm-exec').style.display = 'none';
  document.getElementById('btn-confirm-cancel').textContent = MSG.btnClose;
  confirmCb = null;
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
  if (d.error) { showError(d.error); return; }
  if (d.task_id) startProgress(d.task_id);
}

async function doTrackerFetch() {
  var v = document.getElementById('tracker-input').value.trim();
  if (!v) return;
  var res = await fetch(API + '/v0/fetch/tracker', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({names:[v]})});
  var d = await res.json();
  if (d.error) { showError(d.error); return; }
  closeFetch();
  if (d.task_id) startProgress(d.task_id);
}

async function doUserFetch() {
  var res = await fetch(API + '/v0/fetch/user', {method:'POST'});
  var d = await res.json().catch(function(){return {error:'请求失败 ('+res.status+')'}});
  if (d.error) { showError(MSG.errUserFetchFailed + d.error); return; }
  closeFetch();
  if (d.task_id) startProgress(d.task_id);
}

async function doRefreshAll() {
  closeFetch();
  var res = await fetch(API + '/v0/fetch/all', {method:'POST'});
  var d = await res.json();
  if (d.error) { showError(d.error); return; }
  if (d.task_id) startProgress(d.task_id);
}

async function doDeepRebuild() {
  closeFetch();
  var res = await fetch(API + '/v0/fetch/deep', {method:'POST'});
  var d = await res.json();
  if (d.error) { showError(d.error); return; }
  if (d.task_id) startProgress(d.task_id);
}

async function doFetchUpdate() {
  closeFetch();
  var res = await fetch(API + '/v0/fetch/update', {method:'POST'});
  var d = await res.json();
  if (d.error) { showError(d.error); return; }
  if (d.task_id) startProgress(d.task_id);
}

async function doFetchIndex() {
  closeFetch();
  var res = await fetch(API + '/v0/fetch/index', {method:'POST'});
  var d = await res.json();
  if (d.error) { showError(d.error); return; }
  if (d.task_id) startProgress(d.task_id);
}

// ── SSE 进度条（嵌入顶栏底部）──
function startProgress(taskId, label) {
  var w = document.getElementById('pwrap');
  var t = document.getElementById('ptext');
  if (!w || !t) return;
  w.style.display = 'block';
  t.style.display = 'block';
  var fill = document.getElementById('pfill');
  fill.style.width = '0%';
  t.textContent = (label||MSG.progressConnecting) + ' …';
  var taskLabel = label||'';
  var evt = new EventSource(API + '/v0/task/' + taskId);
  evt.onmessage = function(e) {
    var d = JSON.parse(e.data);
    if (d.label && !taskLabel) taskLabel = d.label;
    if (d.step === 'complete' || d.step === 'done') {
      fill.style.width = '100%'; t.textContent = MSG.progressDone + (taskLabel?' - '+taskLabel:''); evt.close();
      setTimeout(function() { w.style.display = 'none'; t.style.display = 'none'; if (typeof onFetchDone==='function') onFetchDone(); }, 1500);
    } else if (typeof d.done === 'number' && d.total) {
      fill.style.width = Math.round(d.done/d.total*100) + '%';
      var parts = [];
      if (d.phase && d.phases) parts.push(d.phase + '/' + d.phases);
      if (d.phase_name) parts.push(d.phase_name);
      parts.push(d.done + '/' + d.total);
      if (d.speed) parts.push(d.speed);
      t.textContent = (taskLabel ? taskLabel + ' · ' : '') + parts.join('，');
    } else if (d.status) {
      t.textContent = taskLabel + ': ' + d.status;
    }
  };
  evt.onerror = function() { evt.close(); };
}
