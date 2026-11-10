const naira = (kobo) =>
  '\u20A6' + (kobo / 100).toLocaleString('en-NG', { minimumFractionDigits: 2, maximumFractionDigits: 2 });

let currentOrder = null;
let orderType = 'dine_in';
let lastPaidOrder = null;

const $ = (id) => document.getElementById(id);

function setConn(on) {
  const dot = $('conn-dot');
  if (dot) {
    dot.classList.toggle('dot-on', on);
    dot.classList.toggle('dot-off', !on);
  }
}

async function post(url, body) {
  const res = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body || {}),
  });
  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText);
    toast(text || 'Request failed');
    return null;
  }
  return res.json();
}

let toastTimer;
function toast(msg) {
  const el = $('toast');
  if (!el) return;
  el.textContent = msg;
  el.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => (el.hidden = true), 2500);
}

function renderOrder(o) {
  currentOrder = o;
  const lines = $('order-lines');
  lines.innerHTML = '';
  if (!o || !o.lines || o.lines.length === 0) {
    lines.innerHTML = `
      <div class="empty-state">
        <div class="empty-cloche-wrap">
          <img class="empty-cloche" src="/static/images/cloche.png" alt="" onerror="this.style.display='none'">
        </div>
        <p class="empty-text">Tap a dish to start the order</p>
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
        <div class="line-controls">
          <button class="qty-btn" data-act="dec" data-line="${l.id}" title="Decrease quantity">&minus;</button>
          <span class="qty-val">${l.quantity}</span>
          <button class="qty-btn" data-act="inc" data-line="${l.id}" title="Increase quantity">&plus;</button>
        </div>
        <span class="line-amount">${naira(l.line_total_kobo)}</span>
        <button class="rm-btn" data-act="rm" data-line="${l.id}" title="Remove item">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="18" y1="6" x2="6" y2="18"></line>
            <line x1="6" y1="6" x2="18" y2="18"></line>
          </svg>
        </button>`;
      row.querySelector('.line-name').textContent = l.name;
      lines.appendChild(row);
    }
  }

  $('order-no').textContent = o ? o.order_no : '—';
  $('subtotal').textContent = naira(o ? o.subtotal_kobo : 0);
  $('vat').textContent = naira(o ? o.vat_kobo : 0);
  $('total').textContent = naira(o ? o.total_kobo : 0);

  const hasItems = !!(o && o.lines && o.lines.length > 0);
  $('pay').disabled = !hasItems;
  $('send').disabled = !hasItems;
  $('clear').disabled = !hasItems && !o;

  if (o && o.type) {
    orderType = o.type;
    document.querySelectorAll('.seg-btn').forEach((b) => b.classList.toggle('active', b.dataset.type === orderType));
    if (o.table_no !== undefined) {
      $('table-no').value = o.table_no || '';
    }
  }
}

// ---- Order Confirmation & Thermal Receipt Printing ----

function showOrderConfirmation(o) {
  lastPaidOrder = o;
  $('modal-order-no').textContent = `Order #${o.order_no}`;

  const isDineIn = o.type === 'dine_in';
  $('modal-order-type').textContent = isDineIn ? 'Dine-in' : 'Takeaway';

  const tableRow = $('modal-table-row');
  if (isDineIn && o.table_no) {
    $('modal-table-no').textContent = `Table ${o.table_no}`;
    tableRow.hidden = false;
  } else {
    tableRow.hidden = true;
  }

  let totalItems = 0;
  if (o.lines) {
    for (const l of o.lines) totalItems += Number(l.quantity);
  }
  $('modal-item-count').textContent = `${totalItems} item${totalItems === 1 ? '' : 's'}`;
  $('modal-total-amount').textContent = naira(o.total_kobo);

  populateReceipt(o);
  $('order-modal').hidden = false;
}

function hideOrderConfirmation() {
  $('order-modal').hidden = true;
}

function populateReceipt(o) {
  const area = $('receipt-print-area');
  if (!area || !o) return;

  const now = new Date();
  const dateStr = now.toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' });
  const timeStr = now.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: true });

  const orderTypeStr = o.type === 'dine_in'
    ? `DINE-IN ${o.table_no ? '(TABLE ' + o.table_no + ')' : ''}`
    : 'TAKEAWAY';

  let linesHtml = '';
  if (o.lines) {
    for (const l of o.lines) {
      linesHtml += `
        <div class="rcpt-item-row">
          <span class="rcpt-item-qty">${l.quantity}x</span>
          <span class="rcpt-item-name">${l.name}</span>
          <span class="rcpt-item-amt">${naira(l.line_total_kobo)}</span>
        </div>
        <div class="rcpt-item-sub">${l.quantity} ${l.unit_label}${l.quantity > 1 ? 's' : ''} @ ${naira(l.unit_price_kobo)}</div>
      `;
    }
  }

  area.innerHTML = `
    <div class="rcpt-header">
      <img src="/static/images/logo.png" class="rcpt-logo" alt="">
      <div class="rcpt-brand">HIFEH TASTY KITCHEN</div>
      <div class="rcpt-sub">RN: 6967663</div>
      <div class="rcpt-contact">Tel: 09038303358, 09079872794</div>
      <div class="rcpt-tagline">Fresh jollof, fried rice &amp; more</div>
    </div>

    <div class="rcpt-divider-star">********************************</div>

    <div class="rcpt-order-callout">
      <div class="rcpt-callout-no">ORDER #${o.order_no}</div>
      <div class="rcpt-callout-type">*** ${orderTypeStr} ***</div>
    </div>

    <div class="rcpt-divider-dashed">--------------------------------</div>

    <div class="rcpt-meta">
      <div><span>Date:</span> <span>${dateStr} ${timeStr}</span></div>
      <div><span>Terminal:</span> <span>Terminal A · Till 1</span></div>
      <div><span>Cashier:</span> <span>${o.user_name || 'Amaka'}</span></div>
      <div><span>Payment:</span> <span>CASH</span></div>
    </div>

    <div class="rcpt-divider-dashed">--------------------------------</div>
    <div class="rcpt-items-header">
      <span>QTY ITEM</span>
      <span>AMOUNT</span>
    </div>
    <div class="rcpt-divider-dashed">--------------------------------</div>

    <div class="rcpt-items-list">
      ${linesHtml}
    </div>

    <div class="rcpt-divider-dashed">--------------------------------</div>

    <div class="rcpt-totals">
      <div class="rcpt-tot-row"><span>Subtotal:</span> <span>${naira(o.subtotal_kobo)}</span></div>
      <div class="rcpt-tot-row"><span>VAT:</span> <span>${naira(o.vat_kobo)}</span></div>
      <div class="rcpt-tot-row rcpt-grand-total">
        <span>TOTAL:</span>
        <span>${naira(o.total_kobo)}</span>
      </div>
      <div class="rcpt-tot-row"><span>Cash Tendered:</span> <span>${naira(o.total_kobo)}</span></div>
      <div class="rcpt-tot-row"><span>Change:</span> <span>₦0.00</span></div>
    </div>

    <div class="rcpt-divider-star">********************************</div>

    <div class="rcpt-footer">
      <div class="rcpt-barcode">||||| |||| |||||| ||| |||||||</div>
      <div>Thank you for dining with us!</div>
      <div>Please keep receipt for collection</div>
      <div class="rcpt-closing">*** HAVE A TASTY DAY! ***</div>
    </div>
  `;
}

// Modal events
$('modal-done-btn').addEventListener('click', () => {
  hideOrderConfirmation();
  startNewOrder();
});

$('modal-print-btn').addEventListener('click', () => {
  if (lastPaidOrder) {
    populateReceipt(lastPaidOrder);
  }
  // Small delay so receipt DOM is painted before print dialog opens
  setTimeout(() => {
    window.print();
    // After print dialog, close modal and start fresh
    setTimeout(() => {
      hideOrderConfirmation();
      startNewOrder();
    }, 400);
  }, 80);
});

// Close modal when clicking outside card
$('order-modal').addEventListener('click', (e) => {
  if (e.target === $('order-modal')) {
    hideOrderConfirmation();
    startNewOrder();
  }
});

async function startNewOrder() {
  // Clear the current till so a new order is ready
  await post('/api/order/clear').catch(() => {});
  renderOrder(null);
}

// ---- events ----

$('item-grid').addEventListener('click', async (e) => {
  const card = e.target.closest('.item-card');
  if (!card) return;
  const o = await post('/api/order/items', { item_id: card.dataset.item });
  if (o) renderOrder(o);
});

$('order-lines').addEventListener('click', async (e) => {
  const btn = e.target.closest('button');
  if (!btn) return;
  const { act, line } = btn.dataset;
  let o = null;
  if (act === 'inc') o = await post(`/api/order/lines/${line}/qty`, { delta: 1 });
  if (act === 'dec') o = await post(`/api/order/lines/${line}/qty`, { delta: -1 });
  if (act === 'rm') o = await post(`/api/order/lines/${line}/remove`);
  if (o) renderOrder(o);
});

$('cat-tabs').addEventListener('click', (e) => {
  const tab = e.target.closest('.tab');
  if (!tab) return;
  document.querySelectorAll('.tab').forEach((t) => t.classList.toggle('active', t === tab));
  const cat = tab.dataset.cat;
  document.querySelectorAll('.item-card').forEach((c) => {
    c.classList.toggle('hidden', cat !== 'all' && c.dataset.cat !== cat);
  });
});

$('order-type').addEventListener('click', async (e) => {
  const btn = e.target.closest('.seg-btn');
  if (!btn) return;
  orderType = btn.dataset.type;
  document.querySelectorAll('.seg-btn').forEach((b) => b.classList.toggle('active', b === btn));
  const tablePill = document.querySelector('.table-pill');
  if (tablePill) {
    tablePill.classList.toggle('disabled', orderType !== 'dine_in');
    $('table-no').disabled = orderType !== 'dine_in';
  }
  if (currentOrder) {
    const o = await post('/api/order/meta', { type: orderType, table_no: $('table-no').value });
    if (o) renderOrder(o);
  }
});

$('table-no').addEventListener('change', async () => {
  if (!currentOrder) return;
  const o = await post('/api/order/meta', { type: orderType, table_no: $('table-no').value });
  if (o) renderOrder(o);
});

$('send').addEventListener('click', async () => {
  const o = await post('/api/order/send');
  if (o) toast(`Order ${o.order_no} is on the display`);
});

$('pay').addEventListener('click', async () => {
  const o = await post('/api/order/pay');
  if (o) {
    showOrderConfirmation(o);
    renderOrder(null);
  }
});

$('clear').addEventListener('click', async () => {
  await post('/api/order/clear');
  renderOrder(null);
});

$('logout').addEventListener('click', async () => {
  await fetch('/api/logout', { method: 'POST' });
  window.location.href = '/login';
});

// Live connection
const es = new EventSource('/api/events');
es.onopen = () => setConn(true);
es.onerror = () => setConn(false);
es.addEventListener('order', (e) => {
  const o = JSON.parse(e.data);
  if (!o) renderOrder(null);
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

// Initialise
renderOrder(null);
fetch('/api/current-order')
  .then((r) => r.json())
  .then((o) => { if (o) renderOrder(o); })
  .catch(() => {});
