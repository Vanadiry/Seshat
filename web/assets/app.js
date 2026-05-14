// Seshat shared JS — top bar, fetch dialog, SSE progress, API helpers
document.addEventListener('dragstart', function(e) { if (e.target.tagName === 'IMG') e.preventDefault(); });

// Drag-to-scroll on .thin-scroll containers
(function() {
  var el, startX, scrollLeft, down = false, dragged = false;
  document.addEventListener('mousedown', function(e) {
    el = e.target.closest('.thin-scroll');
    if (!el) return;
    down = true; dragged = false;
    startX = e.pageX - el.getBoundingClientRect().left;
    scrollLeft = el.scrollLeft;
  });
  document.addEventListener('mousemove', function(e) {
    if (!down || !el) return;
    var x = e.pageX - el.getBoundingClientRect().left;
    if (!dragged && Math.abs(x - startX) < 5) return;
    dragged = true;
    e.preventDefault();
    el.style.cursor = 'grabbing';
    el.style.userSelect = 'none';
    el.scrollLeft = scrollLeft - (x - startX);
  });
  document.addEventListener('mouseup', function() {
    if (!down) return;
    down = false;
    if (el) { el.style.cursor = ''; el.style.userSelect = ''; }
    el = null;
  });
  // Prevent click after drag
  document.addEventListener('click', function(e) {
    if (dragged) { e.stopPropagation(); e.preventDefault(); dragged = false; }
  }, true);
})();

const API = window.BACKEND_URL || '/api';

// ── UI text (edit here to customize dialog messages) ──
var MSG = {
  // Top bar
  customBackendWarn: '自定义后端模式，所有数据将从远程获取',
  fallbackWarn: '回退模式，缺失数据将从回退端点获取',
  bothWarn: '自定义后端和回退模式，请确保你明白此配置',
  // Fetch dialog
  dialogTitle: '管理数据',
  idLabel: '拉取动画',
  idPlaceholder: '动画 ID，如 51 或 51, 288',
  trackerLabel: '拉取或创建 Tracker',
  trackerPlaceholder: 'Tracker 名称',
  btnUserFetch: '刷新用户数据',
  btnUpdate: '增量更新',
  btnRefreshAll: '刷新全部',
  btnDangerZone: '危险区',
  btnRebuildIndex: '重建索引',
  btnRebuildAll: '重建全部',
  btnExecute: '执行',
  btnCancel: '取消',
  btnClose: '关闭',
  btnImportColl: '导入收藏至 Tracker',
  confirmImportColl: '将从用户收藏列表中读取数据，并存入 Tracker。\n之后拉取数据时将能够拉取你的收藏。',
  importCollDone: function(n) { return '已将 '+n+' 个条目导入至 tracker/user.json'; },

  // Confirm messages
  confirmFetchId: function(ids) { return '将从拉取动画 '+ids.split(',').map(function(s){return '#'+s.trim()}).join(', ')+' 的完整数据'; },
  confirmTrackerFetch: function(name) { return '将拉取 Tracker ['+name+'] 中记录的全部动画数据？'; },
  confirmTrackerCreate: function(name) { return '未找到 Tracker ['+name+']。\n是否创建？'; },
  trackerCreated: function(name) { var d = (window.SESHAT_HOME ? window.SESHAT_HOME+"/tracker" : '~/.vSoft/Seshat/tracker'); return 'Tracker ['+name+'] 已创建。请在 '+d+'/'+name+'.toml 中填写动画 ID，然后返回此处重新拉取。'; },
  confirmUserFetch: function() { return '将拉取 '+window.USERNAME+' 的用户数据与收藏列表'; },
  confirmUpdate: '将对比本地与上游数据，并添加缺失的数据。',
  confirmRefreshAll: '将从上游拉取全部 Tracker 数据，覆盖已有内容。本地多余数据不会删除。',
  confirmRebuildIndex: '将从本地 JSON 中重建所有索引',
  confirmRebuildAll: function() { var d = window.SESHAT_HOME || '~/.vSoft/Seshat'; return '将删除 '+d+'/data 的全部数据，并完整重建。'; },

  // Validation
  errInvalidTrackerName: 'Tracker 名称仅允许大小写字母、数字、短横线和下划线',
  errUserFetchFailed: '刷新用户数据失败: ',
  errNoUsername: '未在配置中设置用户名（[user] username），无法刷新用户数据。',

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
// onload detects no-image placeholder by URL and retries via FALLBACK_URL.
// onerror retries via FALLBACK_URL on network error, then falls back to placeholder.
// When fallback is triggered, addRemoteGlobe() wraps the image and adds a globe overlay.
function imgOnError(kind, id, size) {
  if (!window.FALLBACK_URL) return 'onerror="this.remove()"';
  var fb = window.FALLBACK_URL + '/v0/' + kind + 's/' + id + '/image?type=' + (size||'grid');
  return 'onload="if(!this._c){this._c=1;if(!this._r&&this.currentSrc.indexOf(\'no-image\')>=0){this.src=\''+fb+'\';this._r=1;addRemoteGlobe(this);return;}}if(this._r&&!this._g){addRemoteGlobe(this);}" onerror="if(!this._r){this._r=1;this.src=\''+fb+'\';addRemoteGlobe(this);}else{this.src=\'/assets/no-image.png\';}"';
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
  if (_subjectMap && _subjectMap[name]) return '/subject?id=' + _subjectMap[name];
  if (_charMap && _charMap[name]) return '/character?id=' + _charMap[name];
  if (_personMap && _personMap[name]) return '/person?id=' + _personMap[name];
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
function linkifyPerson(text) { return linkifyByMap(text, _personMap, '/person?id='); }

// Apply all three name matchers, skipping text already inside <a> tags
function linkifyAllNames(text) {
  if (!text) return text;
  var parts = text.split(/(<a\b[^>]*>.*?<\/a>)/);
  for (var i = 0; i < parts.length; i++) {
    if (parts[i].indexOf('<a') === 0) continue;
    var t = parts[i];
    t = linkifyByMap(t, _subjectMap, '/subject?id=');
    t = linkifyByMap(t, _charMap, '/character?id=');
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
    if (p === '/subject') p = '/';
    else if (p === '/character') p = '/characters';
    else if (p === '/person') p = '/persons';
    else if (p === '/tag') p = '/tags';
    if (href === '/' && (p === '/')) return 'bg-[#30303b] text-white';
    if (href !== '/' && p.startsWith(href)) return 'bg-[#30303b] text-white';
    return 'hover:bg-[#30303b] hover:text-white';
  }

  tb.style.cssText = 'position:sticky;top:0;z-index:50';
  var custom = !!window.BACKEND_URL;
  var fallback = !!window.FALLBACK_URL;
  var both = custom && fallback;
  var topBg = both ? 'bg-[#7c3aed]' : (custom ? 'bg-[#dc2626]' : (fallback ? 'bg-[#92400e]' : 'bg-[#1c1c22]'));
  var topBord = (custom||fallback) ? 'border-[rgba(255,255,255,.2)]' : 'border-[rgba(255,255,255,.12)]';
  var warnText = both ? MSG.bothWarn : (custom ? MSG.customBackendWarn : (fallback ? MSG.fallbackWarn : ''));
  var navStyle = function(href) {
    var c = navCls(href);
    if (custom||fallback) {
      if (c.indexOf('bg-') >= 0) return c;
      return 'text-white hover:bg-[#ffffff22]';
    }
    return c;
  };
  tb.innerHTML = `
    <div class="${topBg} border-b ${topBord}">
      <div class="max-w-[1200px] mx-auto px-5 flex items-center gap-3 h-12 relative">
        <h1 class="text-lg font-bold cursor-pointer shrink-0" onclick="location.href='/'">Seshat</h1>
        <nav class="flex gap-1 ml-2 items-center">
          <a href="/" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle('/')}">动画</a>
          <a href="/characters" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle('/characters')}">角色</a>
          <a href="/persons" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle('/persons')}">人物</a>
          <a href="/tags" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle('/tags')}">标签</a>
          <a href="/stats" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle('/stats')}">统计</a>
        </nav>
        <div class="flex-1"></div>
        ${warnText ? '<span class="absolute left-1/2 -translate-x-1/2 text-[#FFCA28] text-sm font-bold pointer-events-none">'+warnText+'</span>' : ''}
        <nav class="flex gap-1 items-center">
          <a href="/rating" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle('/rating')}">评分</a>
          <a href="/search" class="no-underline px-3 py-1.5 rounded-lg text-sm ${navStyle('/search')}">搜索</a>
        </nav>
        <button id="btn-fetch" class="px-4 py-1.5 rounded-lg bg-[#FE8A95] text-white text-sm font-semibold cursor-pointer border-0 hover:opacity-90">管理数据</button>
        <div class="relative">
          <img id="user-avatar" src="" class="w-8 h-8 rounded-lg object-cover bg-[#202028] cursor-pointer shrink-0" onclick="toggleUserMenu(event)" style="display:none">
          <div id="user-placeholder" class="w-8 h-8 rounded-lg cursor-pointer shrink-0" onclick="toggleUserMenu(event)" style="background:linear-gradient(135deg,#ff6b6b,#feca57,#48dbfb,#ff9ff3,#54a0ff);background-size:300% 300%;animation:rainbow 3s ease infinite"></div>
          <div id="user-dropdown" class="absolute right-0 top-full mt-2 bg-[#1c1c22] border border-[rgba(255,255,255,.15)] rounded-xl shadow-2xl py-2 w-[180px] z-[300] overflow-hidden" style="opacity:0;pointer-events:none;transition:opacity .2s ease">
            <div id="user-dropdown-name" class="px-4 pt-1 pb-2 text-sm font-bold text-center"></div>
            <div class="border-t border-[rgba(255,255,255,.06)] my-1"></div>
            <a href="/settings" class="block no-underline px-4 py-1.5 text-sm text-sub hover:bg-[#30303b] hover:text-white">设置</a>
            <div class="border-t border-[rgba(255,255,255,.06)] my-1"></div>
            <a href="/doc/api" class="block no-underline px-4 py-1.5 text-sm text-sub hover:bg-[#30303b] hover:text-white">API 文档</a>
            <a href="https://github.com/Vanadiry/Seshat" target="_blank" class="block no-underline px-4 py-1.5 text-sm text-sub hover:bg-[#30303b] hover:text-white">项目 GitHub</a>
          </div>
        </div>
      </div>
    </div>
    <style>
      @keyframes rainbow { 0%{background-position:0% 50%} 50%{background-position:100% 50%} 100%{background-position:0% 50%} }
      @keyframes imgFadeIn { from{opacity:0} to{opacity:1} }
      img, .grid > *, #results > div, #view > div, #appearances-section > div, #chars-section > div, #subjects-section > div, #stats-list > div, #history-area > div { animation: imgFadeIn .35s ease }
    </style>`;

  document.getElementById('btn-fetch').addEventListener('click', openFetch);

  // Reconnect to active tasks after page refresh
  api('/v0/tasks').then(function(d) {
    if (d && d.tasks && d.tasks.length) {
      for (var i=0; i<d.tasks.length; i++) {
        startProgress(d.tasks[i].id, d.tasks[i].label);
      }
    }
  });

  // Load user avatar
  if (window.USERNAME) {
    api('/v0/users/' + window.USERNAME).then(function(u) {
      if (u && u.id) {
        document.getElementById('user-dropdown-name').textContent = u.nickname || u.username;
        var av = document.getElementById('user-avatar');
        av.onerror = function() { av.style.display = 'none'; document.getElementById('user-placeholder').style.display = ''; };
        av.src = API + '/v0/users/' + window.USERNAME + '/avatar?type=large';
        av.style.display = '';
        document.getElementById('user-placeholder').style.display = 'none';
      }
    });
  }
});

function toggleUserMenu(e) {
  e.stopPropagation();
  var dd = document.getElementById('user-dropdown');
  var show = dd.style.opacity === '0' || dd.style.opacity === '';
  dd.style.opacity = show ? '1' : '0';
  dd.style.pointerEvents = show ? 'auto' : 'none';
}
document.addEventListener('click', function() {
  var dd = document.getElementById('user-dropdown');
  dd.style.opacity = '0';
  dd.style.pointerEvents = 'none';
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
    '<button id="btn-close-dlg" class="w-7 h-7 rounded-full border border-[rgba(255,255,255,.2)] flex items-center justify-center text-sub hover:text-white hover:border-[rgba(255,255,255,.4)] shrink-0 no-underline text-xs cursor-pointer bg-transparent" title="'+MSG.btnClose+'">&times;</button>'+
    '<h3 class="text-base font-bold">'+MSG.dialogTitle+'</h3></div>'+

    // ID input row
    '<label class="text-xs text-sub block mb-1">'+MSG.idLabel+'</label>'+
    '<div class="flex gap-2 mb-4">'+
    '<input id="fetch-input" placeholder="'+MSG.idPlaceholder+'" pattern="[0-9, ]*" class="flex-1 px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-[#151518] text-sm">'+
    '<button id="btn-do-fetch" class="w-9 h-9 rounded-lg bg-[#FE8A95] text-white font-bold cursor-pointer border-0 hover:opacity-90 flex items-center justify-center" title="拉取">&check;</button></div>'+

    // Tracker input row
    '<label class="text-xs text-sub block mb-1">'+MSG.trackerLabel+'</label>'+
    '<div class="flex gap-2 mb-4">'+
    '<input id="tracker-input" placeholder="'+MSG.trackerPlaceholder+'" pattern="[a-zA-Z0-9_\\-]*" class="flex-1 px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-[#151518] text-sm">'+
    '<button id="btn-tracker-fetch" class="w-9 h-9 rounded-lg bg-[#FE8A95] text-white font-bold cursor-pointer border-0 hover:opacity-90 flex items-center justify-center" title="拉取或创建">&check;</button></div>'+

    // Action buttons
    '<div class="flex flex-col gap-2 mb-2">'+
    '<div class="grid grid-cols-2 gap-2">'+
    '<button id="btn-update" class="px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#30303b] hover:text-white">'+MSG.btnUpdate+'</button>'+
    '<button id="btn-all" class="px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#30303b] hover:text-white">'+MSG.btnRefreshAll+'</button>'+
    '</div>'+
    '<div class="grid grid-cols-2 gap-2">'+
    '<button id="btn-user" class="px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#30303b] hover:text-white">'+MSG.btnUserFetch+'</button>'+
    '<button id="btn-import-coll" class="px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#30303b] hover:text-white">'+MSG.btnImportColl+'</button>'+
    '</div></div>'+

    // Danger zone
    '<button id="btn-danger-toggle" class="w-full px-3 py-2 rounded-lg border border-[rgba(255,0,0,.3)] bg-transparent text-[#dc2626] text-sm cursor-pointer hover:bg-[#3a1a1a]">'+MSG.btnDangerZone+'</button>'+
    '<div id="danger-zone" class="hidden mt-2 grid grid-cols-3 gap-2">'+
    '<button id="btn-index" class="px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#30303b] hover:text-white">'+MSG.btnRebuildIndex+'</button>'+
    '<button id="btn-rebuild-elo" class="px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#30303b] hover:text-white">重建 ELO</button>'+
    '<button id="btn-deep" class="px-3 py-2 rounded-lg bg-[#dc2626] text-white text-sm font-semibold cursor-pointer border-0 hover:opacity-90">'+MSG.btnRebuildAll+'</button>'+
    '</div>'+

    // Confirmation overlay
    '<div id="confirm-overlay" class="hidden absolute inset-0 bg-black/70 rounded-xl flex items-center justify-center z-10">'+
    '<div class="bg-[#2a2a30] border border-[rgba(255,255,255,.12)] rounded-lg p-5 w-[340px] max-w-[85vw]">'+
    '<p id="confirm-msg" class="text-sm mb-4 leading-relaxed"></p>'+
    '<button id="btn-confirm-exec" class="w-full px-4 py-2 rounded-lg bg-[#FE8A95] text-white text-sm font-semibold cursor-pointer border-0 hover:opacity-90 mb-2">'+MSG.btnExecute+'</button>'+
    '<button id="btn-confirm-cancel" class="w-full px-4 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#30303b]">'+MSG.btnCancel+'</button>'+
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
    confirmAction(MSG.confirmFetchId(ids), doFetch);
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
        confirmAction(MSG.confirmTrackerFetch(name), doTrackerFetch);
      } else {
        confirmAction(MSG.confirmTrackerCreate(name), function() {
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
    if (!window.USERNAME) { showError(MSG.errNoUsername); return; }
    confirmAction(MSG.confirmUserFetch(), doUserFetch);
  });
  document.getElementById('btn-import-coll').addEventListener('click', function() {
    confirmAction('将用户收藏列表导入为 Tracker，之后可参与增量更新。', function() {
      closeFetch();
      fetch(API + '/v0/tracker/import-collections', {method:'POST'}).then(function(r){return r.json()}).then(function(d) {
        if (d.error) showError(d.error);
        else showError(MSG.importCollDone(d.count));
      });
    });
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
  document.getElementById('btn-rebuild-elo').addEventListener('click', function() {
    confirmAction('将清空当前 ELO 分数并从对战历史完整重建', function() {
      closeFetch();
      fetch(API + '/v0/elo/rebuild', {method:'POST'}).then(function(r){return r.json()}).then(function(d) {
        showError(d.status==='ok' ? '已重建 '+d.count+' 条 ELO 分数' : '重建失败');
      });
    });
  });
}

function confirmAction(msg, cb) {
  document.getElementById('confirm-msg').textContent = msg;
  document.getElementById('confirm-overlay').style.display = 'flex';
  confirmCb = cb;
  document.getElementById('btn-confirm-exec').style.display = 'block';
  document.getElementById('btn-confirm-cancel').textContent = MSG.btnCancel;
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

// ── 进度通知横幅（右下角浮动）──
function startProgress(taskId, label) {
  var banner = document.getElementById('progress-banner');
  if (!banner) {
    banner = document.createElement('div');
    banner.id = 'progress-banner';
    banner.className = 'fixed bottom-4 right-4 z-[200] bg-[#1c1c22] border border-[rgba(255,255,255,.15)] rounded-xl shadow-2xl p-4 max-w-[320px] min-w-[260px]';
    banner.style.opacity = '0';
    banner.style.pointerEvents = 'none';
    banner.innerHTML = '<div class="flex items-center gap-2"><span id="pb-label" class="text-sm font-bold truncate"></span><span id="pb-phase" class="text-xs text-sub shrink-0"></span></div><div id="pb-detail" class="text-xs text-sub mt-1"></div><div id="pb-bar" class="mt-2 h-1 rounded-full bg-[#30303b] overflow-hidden"><div id="pb-fill" class="h-full bg-[#FE8A95] w-0 transition-[width] duration-300"></div></div>';
    document.body.appendChild(banner);
  }
  banner.style.opacity = '1';
  banner.style.pointerEvents = 'auto';
  banner.style.transition = 'opacity .3s';
  banner.style.borderColor = 'rgba(255,255,255,.15)';
  var fill = document.getElementById('pb-fill');
  fill.style.width = '0%';
  fill.style.background = '#FE8A95';
  fill.style.transition = 'width 300ms';
  document.getElementById('pb-label').textContent = label || MSG.progressConnecting;
  document.getElementById('pb-detail').textContent = '';
  document.getElementById('pb-phase').textContent = '';

  var taskLabel = label || '';
  var evt = new EventSource(API + '/v0/task/' + taskId);
  evt.onmessage = function(e) {
    var d = JSON.parse(e.data);
    if (d.label && !taskLabel) { taskLabel = d.label; document.getElementById('pb-label').textContent = d.label; }
    if (d.step === 'complete' || d.step === 'done') {
      document.getElementById('pb-label').textContent = taskLabel;
      document.getElementById('pb-phase').textContent = '';
      document.getElementById('pb-detail').textContent = MSG.progressDone;
      fill.style.width = '100%';
      fill.style.background = '#22c55e';
      fill.style.transition = 'none';
      banner.style.borderColor = 'rgba(34,197,94,.4)';
      evt.close();
      // 5 second countdown
      var remaining = 5000;
      var interval = setInterval(function() {
        remaining -= 50;
        if (remaining <= 0) {
          clearInterval(interval);
          banner.style.opacity = '0';
          banner.style.pointerEvents = 'none';
          setTimeout(function() { if (typeof onFetchDone === 'function') onFetchDone(); }, 300);
        } else {
          fill.style.width = Math.round(remaining / 5000 * 100) + '%';
        }
      }, 50);
    } else if (typeof d.done === 'number' && d.total) {
      fill.style.width = Math.round(d.done/d.total*100) + '%';
      document.getElementById('pb-phase').textContent = (d.phase && d.phases) ? d.phase + '/' + d.phases : '';
      var parts = [];
      if (d.phase_name) parts.push(d.phase_name);
      parts.push(d.done + '/' + d.total);
      if (d.speed) parts.push(d.speed);
      document.getElementById('pb-detail').textContent = parts.join('，');
    } else if (d.status) {
      document.getElementById('pb-detail').textContent = d.status;
    }
  };
  evt.onerror = function() { evt.close(); };
}
