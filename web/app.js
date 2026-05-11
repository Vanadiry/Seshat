// ── Shared top bar (injected into all pages) ──
(function initTopbar() {
  const bar = document.getElementById('topbar');
  if (!bar) return;
  bar.innerHTML = `
    <h1 onclick="location.href='/'">Seshat</h1>
    <nav>
      <wa-button variant="neutral" appearance="plain" size="s" onclick="location.href='/'">动画</wa-button>
      <wa-button variant="neutral" appearance="plain" size="s" onclick="location.href='/tags.html'">标签</wa-button>
    </nav>
    <div class="spacer"></div>
    <wa-button variant="neutral" size="s" appearance="plain" id="width-btn" onclick="toggleWidth()" title="切换页面宽度">⇔</wa-button>
    <wa-button variant="brand" size="s" pill onclick="openFetch()">+ 拉取</wa-button>
    <wa-input id="search-input" placeholder="搜索…" size="s" style="width:200px;" onkeydown="if(event.key==='Enter')location.href='/?q='+encodeURIComponent(this.value)">
      <wa-icon name="search" slot="start"></wa-icon>
    </wa-input>
  `;
})();

function toggleWidth() {
  const main = document.querySelector('.main');
  const w = main.style.maxWidth === '900px' ? '1200px' : '900px';
  main.style.maxWidth = w;
  document.getElementById('width-btn').textContent = w === '900px' ? '⇔' : '⇔';
}

// ── API helpers ──
const API = location.port === '3000' ? 'http://127.0.0.1:8080' : '';
const EP_TYPES = {0:'本篇',1:'特别篇',2:'OP',3:'ED',4:'预告',5:'MAD',6:'其他'};

function imgUrl(path) {
  if (!path) return '';
  return API + '/api/v1/images/' + path.replace('images/', '');
}

function parseInfobox(raw) {
  if (!raw) return {};
  let data;
  try { data = JSON.parse(raw); } catch(e) { return {}; }
  const out = {};
  for (const item of data || []) {
    if (item.key && typeof item.value === 'string') out[item.key] = item.value;
  }
  return out;
}

async function api(url) {
  const res = await fetch(API + url);
  return res.json();
}

// ── Fetch dialog + SSE progress ──
function openFetch() {
  let d = document.getElementById('fetch-dialog');
  if (!d) {
    d = document.createElement('dialog');
    d.id = 'fetch-dialog';
    d.style.cssText = 'background:#1a1a23;color:#e4e4e7;border:1px solid #27272a;border-radius:12px;padding:24px;width:400px;max-width:90vw;';
    d.innerHTML = `<h3 style="margin:0 0 16px;">拉取动画</h3>
      <div style="display:flex;flex-direction:column;gap:12px;">
        <input id="fetch-id-input" placeholder="动画 ID，如 51" style="padding:10px 12px;border-radius:8px;border:1px solid #3f3f46;background:#0f0f13;color:#e4e4e7;font-size:14px;" />
        <input id="batch-ids-input" placeholder="批量：51, 288, 9717" style="padding:10px 12px;border-radius:8px;border:1px solid #3f3f46;background:#0f0f13;color:#e4e4e7;font-size:14px;" />
        <div style="display:flex;gap:8px;justify-content:flex-end;margin-top:8px;">
          <button id="fetch-cancel" style="padding:8px 16px;border-radius:8px;border:1px solid #3f3f46;background:transparent;color:#e4e4e7;cursor:pointer;">取消</button>
          <button id="fetch-submit" style="padding:8px 16px;border-radius:8px;border:none;background:#6366f1;color:#fff;cursor:pointer;">拉取</button>
        </div>
      </div>`;
    document.body.appendChild(d);
    document.getElementById('fetch-cancel').onclick = () => d.close();
    document.getElementById('fetch-submit').onclick = doFetch;
  }
  d.showModal();
}

async function doFetch() {
  const id = document.getElementById('fetch-id-input').value.trim();
  const batch = document.getElementById('batch-ids-input').value.trim();
  const ids = batch ? batch.split(',').map(s=>s.trim()).filter(Boolean) : id ? [id] : [];
  if (!ids.length) return;
  document.getElementById('fetch-dialog').close();
  for (const sid of ids) {
    const res = await fetch(API+'/api/v1/subjects/fetch', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({id:parseInt(sid)})});
    const d = await res.json();
    if (d.data?.task_id) startSSE(d.data.task_id);
  }
}

function startSSE(taskId) {
  const area = document.getElementById('progress-area');
  if (!area) return;
  const bar = document.getElementById('progress-bar');
  const text = document.getElementById('progress-text');
  area.style.display = 'block'; bar.value = 0; bar.indeterminate = true;
  text.textContent = '连接中...';
  const evt = new EventSource(API+'/api/v1/tasks/'+taskId+'/events');
  evt.onmessage = function(e) {
    const d = JSON.parse(e.data);
    if (d.step==='error') { bar.indeterminate=false; bar.value=100; text.textContent='❌ '+d.message; setTimeout(()=>{area.style.display='none';evt.close();},6000); return; }
    if (d.step==='complete') { bar.indeterminate=false; bar.value=100; text.textContent='完成'; setTimeout(()=>{area.style.display='none';evt.close();location.reload();},1500); }
    else if (d.done!==undefined&&d.total) { bar.indeterminate=false; bar.value=Math.round(d.done/d.total*100); text.textContent=d.step+': '+d.done+'/'+d.total; }
    else { text.textContent=d.step+': '+(d.status||'处理中...'); }
  };
  evt.onerror = ()=>evt.close();
}
