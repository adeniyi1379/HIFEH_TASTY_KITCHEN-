const $ = (id) => document.getElementById(id);

const naira = (kobo) =>
  '\u20A6' + (kobo / 100).toLocaleString('en-NG', { minimumFractionDigits: 2, maximumFractionDigits: 2 });

let mirrorUser = localStorage.getItem('tatsy_mirror') || '';
let cashiers = [];

function setConn(on) {
  const dot = $('conn-dot');
  if (dot) {
    dot.classList.toggle('dot-on', on);
    dot.classList.toggle('dot-off', !on);
  }
}

// ---- setup: choose the cashier to mirror ----

async function loadCashiers() {
  try {
    const res = await fetch('/api/cashiers');
    cashiers = await res.json();
  } catch {
    cashiers = [];
  }
  renderSyncLabel();
  renderPicker();
}

function renderSyncLabel() {
  const c = cashiers.find((x) => x.id === mirrorUser);
  $('sync-btn').textContent = c ? 'Mirror: ' + c.name : 'Mirror: All';
}

function renderPicker() {
  const list = $('picker-list');
  list.innerHTML = '';
  if (cashiers.length === 0) {
    list.innerHTML = '<p class="muted small">No one is signed in yet. The till login appears here.</p>';
    return;
  }
  for (const c of cashiers) {
    const btn = document.createElement('button');
    btn.className = 'picker-item' + (c.id === mirrorUser ? ' active' : '');
    btn.dataset.user = c.id;
    btn.innerHTML = `<span class="picker-name"></span><span class="muted small"> · ${c.role}</span>`;
    btn.querySelector('.picker-name').textContent = c.name;
    list.appendChild(btn);
  }
}

$('sync-btn').addEventListener('click', () => {
  renderPicker();
  $('picker').hidden = false;
});

$('picker').addEventListener('click', (e) => {
  const item = e.target.closest('.picker-item');
  if (item) {
    mirrorUser = item.dataset.user;
    localStorage.setItem('tatsy_mirror', mirrorUser);
    renderPicker();
    renderSyncLabel();
    refresh();
  }
});

$('picker-close').addEventListener('click', () => ($('picker').hidden = true));

// ---- menu grid (reflection) ----

$('cat-tabs').addEventListener('click', (e) => {
  const tab = e.target.closest('.tab');
  if (!tab) return;
  document.querySelectorAll('.tab').forEach((t) => t.classList.toggle('active', t === tab));
  const cat = tab.dataset.cat;
  document.querySelectorAll('.item-card').forEach((c) => {
    c.classList.toggle('hidden', cat !== 'all' && c.dataset.cat !== cat);
  });
});

// ---- live order mirror ----

function accepts(o) {
  return !o || !mirrorUser || o.user_id === mirrorUser;
}

let payConfirmTimer = null;

function showPayConfirm(o) {
  const overlay = $('pay-confirm');
  if (!overlay) return;
  $('confirm-order-no').textContent = `Order #${o.order_no}`;
  $('confirm-amount').textContent = naira(o.total_kobo);
  overlay.hidden = false;
  // Auto-dismiss after 6 seconds
  clearTimeout(payConfirmTimer);
  payConfirmTimer = setTimeout(() => {
    overlay.hidden = true;
  }, 6000);
}

// Tap anywhere on overlay to dismiss early
document.addEventListener('click', (e) => {
  const overlay = $('pay-confirm');
  if (overlay && !overlay.hidden && e.target === overlay) {
    clearTimeout(payConfirmTimer);
    overlay.hidden = true;
  }
});

function renderOrder(o) {
  const paid = o && o.status === 'paid';

  $('order-no').textContent = o ? o.order_no : '—';
  document.querySelectorAll('.seg-btn').forEach((b) =>
    b.classList.toggle('active', !!o && b.dataset.type === o.type));
  $('table-no').value = o && o.table_no ? o.table_no : '';

  const lines = $('order-lines');
  lines.innerHTML = '';
  if (!o || !o.lines || o.lines.length === 0) {
    lines.innerHTML = `
      <div class="empty-state">
        <div class="empty-cloche-wrap">
          <img class="empty-cloche" src="/static/images/cloche.png" alt="" onerror="this.style.display='none'">
        </div>
        <p class="empty-text">Cashier is starting a new order</p>
      </div>`;
  } else {
    for (const l of o.lines) {
      const row = document.createElement('div');
      row.className = 'line';
      row.innerHTML = `
        <div class="line-info">
          <div class="line-name"></div>
          <div class="line-sub">${l.quantity} ${l.unit_label}${l.quantity > 1 ? 's' : ''} &times; ${naira(l.unit_price_kobo)}</div>
        </div>
        <span class="line-amount">${naira(l.line_total_kobo)}</span>`;
      row.querySelector('.line-name').textContent = l.name;
      lines.appendChild(row);
    }
  }

  $('subtotal').textContent = naira(o ? o.subtotal_kobo : 0);
  $('vat').textContent = naira(o ? o.vat_kobo : 0);
  $('total').textContent = naira(o ? o.total_kobo : 0);

  const banner = $('status-banner');
  banner.classList.toggle('paid', !!paid);
  banner.textContent = paid ? 'Paid — thank you!' : 'Pay at the counter';

  // Show full-screen confirmation on customer display when order is paid
  if (paid) showPayConfirm(o);
}

async function refresh() {
  try {
    const res = await fetch('/api/current-order');
    const o = await res.json();
    renderOrder(accepts(o) ? o : null);
  } catch {}
}

const es = new EventSource('/api/events');
es.onopen = () => setConn(true);
es.onerror = () => setConn(false);
es.addEventListener('order', (e) => {
  const o = JSON.parse(e.data);
  if (accepts(o)) renderOrder(o);
});

// Live clock update
function updateClock() {
  const el = $('footer-clock');
  if (!el) return;
  const now = new Date();
  const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
  const m = months[now.getMonth()];
  const d = now.getDate();
  let h = now.getHours();
  const ampm = h >= 12 ? 'PM' : 'AM';
  h = h % 12;
  h = h ? h : 12;
  const min = String(now.getMinutes()).padStart(2, '0');
  el.textContent = `${m} ${d} ${h}:${min} ${ampm}`;
}
setInterval(updateClock, 1000);
updateClock();

renderOrder(null);
refresh();
loadCashiers();
setInterval(loadCashiers, 15000);
