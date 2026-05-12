// Seshat shared JS — top bar, fetch dialog, SSE progress, API helpers
const API = location.port === '3000' ? 'http://127.0.0.1:4000' : '';

function api(url) { return fetch(API + url).then(r => r.json()); }
function img(kind, id, size) { return API + '/images/' + kind + '/' + id + '?type=' + (size||'grid'); }

// ── Top bar injection ──
document.addEventListener('DOMContentLoaded', () => {
  const tb = document.getElementById('topbar');
  if (!tb) return;
  tb.innerHTML = `
    <header class="sticky top-0 z-50 flex items-center gap-3 h-14 px-5 bg-[#16161d] border-b border-[rgba(255,255,255,.12)]">
      <h1 class="text-lg font-bold cursor-pointer" onclick="location.href='/'">Seshat</h1>
      <nav class="flex gap-1 ml-2">
        <a href="/" class="text-sub no-underline px-3 py-1.5 rounded-lg text-sm hover:bg-[#2a2a35] hover:text-white">动画</a>
        <a href="/doc/api" class="text-sub no-underline px-3 py-1.5 rounded-lg text-sm hover:bg-[#2a2a35] hover:text-white">API</a>
      </nav>
      <div class="flex-1"></div>
      <input id="search" placeholder="搜索…" class="px-3 py-1.5 rounded-lg border border-[rgba(255,255,255,.12)] bg-[#0f0f13] text-sm w-44" onkeydown="if(event.key==='Enter')search()">
      <button onclick="openFetch()" class="px-4 py-1.5 rounded-lg bg-[#FE8A95] text-white text-sm font-semibold cursor-pointer border-0">+ 拉取</button>
    </header>`;

  // Progress bar (injected right after topbar)
  const pw = document.createElement('div');
  pw.id = 'pwrap';
  pw.className = 'hidden fixed top-14 left-0 right-0 z-[60] px-5';
  pw.innerHTML = '<div class="h-1 bg-[#2a2a35] rounded overflow-hidden"><div id="pfill" class="h-full bg-[#FE8A95] w-0 transition-[width] duration-300"></div></div><div id="ptext" class="text-[11px] text-sub mt-1"></div>';
  tb.after(pw);
});

// ── Fetch dialog ──
let dlgInited = false;
function initDialog() {
  if (dlgInited) return;
  dlgInited = true;
  const dlg = document.createElement('div');
  dlg.id = 'dlg-overlay';
  dlg.className = 'hidden fixed inset-0 z-[100] bg-black/60 items-center justify-center';
  dlg.innerHTML = `
    <div class="bg-[#1e1e28] border border-[rgba(255,255,255,.12)] rounded-xl p-6 w-[440px] max-w-[90vw] max-h-[90vh] overflow-y-auto">
      <h3 class="text-base font-bold mb-4">拉取数据</h3>
      <label class="text-xs text-sub block mb-1">动画 ID（逗号分隔）</label>
      <input id="fetch-input" placeholder="如 51 或 51,288" class="w-full px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-[#0f0f13] text-sm mb-3">
      <button onclick="doFetch()" class="w-full px-4 py-2 rounded-lg bg-[#FE8A95] text-white text-sm font-semibold cursor-pointer border-0 mb-4 hover:opacity-90">拉取指定动画</button>
      <div class="border-t border-[rgba(255,255,255,.06)] mb-4"></div>
      <label class="text-xs text-sub block mb-1">按 Tracker 名称拉取</label>
      <div class="flex gap-2 mb-3">
        <input id="tracker-input" placeholder="Tracker 名称" class="flex-1 px-3 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-[#0f0f13] text-sm">
        <button onclick="doTrackerFetch()" class="px-4 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#2a2a35] hover:text-white whitespace-nowrap">拉取</button>
      </div>
      <button onclick="doUserFetch()" class="w-full px-4 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#2a2a35] hover:text-white mb-4">拉取用户收藏</button>
      <div class="border-t border-[rgba(255,255,255,.06)] mb-4"></div>
      <div class="flex gap-2 mb-3">
        <button id="btn-refresh" onclick="doRefreshAll()" class="flex-1 px-4 py-2 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#2a2a35] hover:text-white">刷新全部</button>
        <button id="btn-deep" onclick="doDeepRebuild()" class="flex-1 px-4 py-2 rounded-lg bg-[#dc2626] text-white text-sm font-semibold cursor-pointer border-0 hover:opacity-90">深度重建</button>
      </div>
      <button onclick="closeFetch()" class="w-full px-4 py-1.5 rounded-lg border border-[rgba(255,255,255,.12)] bg-transparent text-sub text-sm cursor-pointer hover:bg-[#2a2a35]">取消</button>
    </div>`;
  dlg.addEventListener('click', e => { if (e.target === dlg) closeFetch(); });
  document.body.appendChild(dlg);
}

function openFetch() {
  initDialog();
  document.getElementById('dlg-overlay').style.display = 'flex';
  document.getElementById('fetch-input').focus();
}
function closeFetch() {
  document.getElementById('dlg-overlay').style.display = 'none';
  // Reset deep rebuild confirm state
  const btn = document.getElementById('btn-deep');
  if (btn) { btn.textContent = '深度重建'; btn._confirm = false; }
  const btnR = document.getElementById('btn-refresh');
  if (btnR) { btnR.textContent = '刷新全部'; btnR._confirm = false; }
}

async function doFetch() {
  const v = document.getElementById('fetch-input').value.trim();
  if (!v) return;
  closeFetch();
  const ids = v.split(',').map(s => parseInt(s.trim())).filter(Boolean);
  if (!ids.length) return;
  const res = await fetch(API + '/api/v1/fetch/subject', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({ids})});
  const d = await res.json();
  if (d.task_id) startProgress(d.task_id);
}

async function doTrackerFetch() {
  const v = document.getElementById('tracker-input').value.trim();
  if (!v) return;
  closeFetch();
  const res = await fetch(API + '/api/v1/fetch/tracker', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({name:v})});
  const d = await res.json();
  if (d.task_id) startProgress(d.task_id);
}

async function doUserFetch() {
  closeFetch();
  const res = await fetch(API + '/api/v1/fetch/user', {method:'POST'});
  const d = await res.json();
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
  const res = await fetch(API + '/api/v1/fetch', {method:'POST'});
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
  const res = await fetch(API + '/api/v1/fetch/deep', {method:'POST'});
  const d = await res.json();
  if (d.task_id) startProgress(d.task_id);
}

// ── SSE progress ──
function startProgress(taskId) {
  const w = document.getElementById('pwrap');
  if (!w) return;
  w.style.display = 'block';
  const fill = document.getElementById('pfill'), text = document.getElementById('ptext');
  fill.style.width = '0%'; text.textContent = '连接中…';
  const evt = new EventSource(API + '/api/v1/progress/' + taskId);
  evt.onmessage = function(e) {
    const d = JSON.parse(e.data);
    if (d.step === 'complete') {
      fill.style.width = '100%'; text.textContent = '完成'; evt.close();
      setTimeout(() => { w.style.display = 'none'; if (typeof onFetchDone==='function') onFetchDone(); }, 1500);
    } else if (d.done !== undefined && d.total) {
      fill.style.width = Math.round(d.done/d.total*100) + '%';
      text.textContent = (d.step||'') + ' ' + d.done + '/' + d.total + (d.speed?' · '+d.speed:'');
    } else if (d.status) {
      text.textContent = (d.step||'') + ': ' + d.status;
    }
  };
  evt.onerror = () => evt.close();
}

// ── Search ──
function search() {
  const q = document.getElementById('search').value.trim();
  if (!q) { location.href = '/'; return; }
  location.href = '/?q=' + encodeURIComponent(q);
}
