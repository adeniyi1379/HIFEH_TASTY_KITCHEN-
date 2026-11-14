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

function lineSub(l) {
  if (Number(l.unit_price_kobo) === 0) {
    return `${l.quantity} ${l.unit_label}${l.quantity > 1 ? 's' : ''} · included in combo`;
  }
  return `${l.quantity} ${l.unit_label}${l.quantity > 1 ? 's' : ''} &times; ${naira(l.unit_price_kobo)}`;
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
          <div class="line-sub">${lineSub(l)}</div>
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

  const isEdit = !!(o && o.is_edit);
  const editBadge = $('order-edit-badge');
  if (editBadge) editBadge.hidden = !isEdit;

  const payBtn = $('pay');
  const payLabel = $('pay-label');
  const payIcon = $('pay-icon');
  const clearLabel = $('clear-label');

  if (isEdit) {
    if (payBtn) payBtn.classList.add('btn-action-save');
    if (payLabel) payLabel.textContent = 'Save Changes';
    if (payIcon) {
      payIcon.innerHTML = `
        <path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"></path>
        <polyline points="17 21 17 13 7 13 7 21"></polyline>
        <polyline points="7 3 7 8 15 8"></polyline>
      `;
    }
    if (clearLabel) clearLabel.textContent = 'Cancel Edit';
  } else {
    if (payBtn) payBtn.classList.remove('btn-action-save');
    if (payLabel) payLabel.textContent = 'Pay (cash)';
    if (payIcon) {
      payIcon.innerHTML = `
        <rect x="2" y="6" width="20" height="12" rx="2"></rect>
        <circle cx="12" cy="12" r="2"></circle>
        <line x1="6" y1="12" x2="6.01" y2="12"></line>
        <line x1="18" y1="12" x2="18.01" y2="12"></line>
      `;
    }
    if (clearLabel) clearLabel.textContent = 'Clear';
  }

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

  populateReceipt(o, false);
  $('order-modal').hidden = false;
}

function hideOrderConfirmation() {
  if ($('order-modal')) $('order-modal').hidden = true;
}

function showEditConfirmation(o) {
  lastPaidOrder = o;
  const noEl = $('edit-modal-order-no');
  if (noEl) noEl.textContent = `Order #${o.order_no}`;

  const isDineIn = o.type === 'dine_in';
  const typeEl = $('edit-modal-order-type');
  if (typeEl) typeEl.textContent = isDineIn ? 'Dine-in' : 'Takeaway';

  const tableRow = $('edit-modal-table-row');
  const tableNoEl = $('edit-modal-table-no');
  if (tableRow && tableNoEl) {
    if (isDineIn && o.table_no) {
      tableNoEl.textContent = `Table ${o.table_no}`;
      tableRow.hidden = false;
    } else {
      tableRow.hidden = true;
    }
  }

  let totalItems = 0;
  if (o.lines) {
    for (const l of o.lines) totalItems += Number(l.quantity);
  }
  const itemCountEl = $('edit-modal-item-count');
  if (itemCountEl) itemCountEl.textContent = `${totalItems} item${totalItems === 1 ? '' : 's'}`;
  const totalEl = $('edit-modal-total-amount');
  if (totalEl) totalEl.textContent = naira(o.total_kobo);

  populateReceipt(o, true);
  if ($('order-edit-modal')) $('order-edit-modal').hidden = false;
}

function hideEditConfirmation() {
  if ($('order-edit-modal')) $('order-edit-modal').hidden = true;
}

function populateReceipt(o, isEdit) {
  const area = $('receipt-print-area');
  if (!area || !o) return;

  const now = new Date();
  const dateStr = now.toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' });
  const timeStr = now.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: true });

  const orderTypeStr = o.type === 'dine_in'
    ? `DINE-IN ${o.table_no ? '(TABLE ' + o.table_no + ')' : ''}`
    : 'TAKEAWAY';

  const editCallout = isEdit ? `
    <div style="font-weight:900;text-align:center;padding:4px 0;font-size:0.92rem;letter-spacing:1px;color:#000;">
      *** UPDATED RECEIPT (EDITED) ***
    </div>` : '';

  let linesHtml = '';
  if (o.lines) {
    for (const l of o.lines) {
      const sub = Number(l.unit_price_kobo) === 0
        ? `${l.quantity} ${l.unit_label}${l.quantity > 1 ? 's' : ''} · included in combo`
        : `${l.quantity} ${l.unit_label}${l.quantity > 1 ? 's' : ''} @ ${naira(l.unit_price_kobo)}`;
      linesHtml += `
        <div class="rcpt-item-row">
          <span class="rcpt-item-qty">${l.quantity}x</span>
          <span class="rcpt-item-name">${l.name}</span>
          <span class="rcpt-item-amt">${naira(l.line_total_kobo)}</span>
        </div>
        <div class="rcpt-item-sub">${sub}</div>
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

    ${editCallout}

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
      <div><span>Payment:</span> <span>${isEdit ? 'EDITED / SAVED' : 'CASH'}</span></div>
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
if ($('modal-done-btn')) {
  $('modal-done-btn').addEventListener('click', () => {
    hideOrderConfirmation();
    startNewOrder();
  });
}

if ($('modal-print-btn')) {
  $('modal-print-btn').addEventListener('click', () => {
    if (lastPaidOrder) {
      populateReceipt(lastPaidOrder, false);
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
}

// Close modal when clicking outside card
if ($('order-modal')) {
  $('order-modal').addEventListener('click', (e) => {
    if (e.target === $('order-modal')) {
      hideOrderConfirmation();
      startNewOrder();
    }
  });
}

// Edit Modal events
if ($('edit-modal-done-btn')) {
  $('edit-modal-done-btn').addEventListener('click', () => {
    hideEditConfirmation();
    startNewOrder();
  });
}

if ($('edit-modal-print-btn')) {
  $('edit-modal-print-btn').addEventListener('click', () => {
    if (lastPaidOrder) {
      populateReceipt(lastPaidOrder, true);
    }
    setTimeout(() => {
      window.print();
      setTimeout(() => {
        hideEditConfirmation();
        startNewOrder();
      }, 400);
    }, 80);
  });
}

if ($('order-edit-modal')) {
  $('order-edit-modal').addEventListener('click', (e) => {
    if (e.target === $('order-edit-modal')) {
      hideEditConfirmation();
      startNewOrder();
    }
  });
}

window.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') {
    if ($('order-modal') && !$('order-modal').hidden) {
      hideOrderConfirmation();
      startNewOrder();
    }
    if ($('order-edit-modal') && !$('order-edit-modal').hidden) {
      hideEditConfirmation();
      startNewOrder();
    }
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
  if (card.dataset.trackInventory === 'true' && parseInt(card.dataset.stock, 10) <= 0) {
    toast(`Out of stock: ${card.dataset.name} is finished!`);
    return;
  }
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
  if (currentOrder && currentOrder.is_edit) {
    const o = await post('/api/order/save');
    if (o) {
      showEditConfirmation(o);
      renderOrder(null);
    }
    return;
  }
  const o = await post('/api/order/pay');
  if (o) {
    showOrderConfirmation(o);
    renderOrder(null);
  }
});

$('clear').addEventListener('click', async () => {
  if (currentOrder && currentOrder.is_edit) {
    if (!confirm(`Cancel editing Order #${currentOrder.order_no}? Any unsaved changes will be discarded.`)) {
      return;
    }
  }
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
es.addEventListener('inventory', (e) => {
  try {
    const items = JSON.parse(e.data);
    updateInventoryBadges(items);
    if (tillInventoryModal && !tillInventoryModal.hidden) {
      renderTillInventory(items);
    }
  } catch (err) {}
});

function updateInventoryBadges(items) {
  if (!Array.isArray(items)) return;
  items.forEach((it) => {
    const card = document.querySelector(`.item-card[data-item="${it.id}"]`);
    if (!card) return;
    card.dataset.trackInventory = it.track_inventory ? 'true' : 'false';
    card.dataset.stock = it.stock_quantity;

    const existingBadge = card.querySelector('.stock-badge');
    if (existingBadge) existingBadge.remove();

    if (it.track_inventory) {
      const badge = document.createElement('span');
      badge.className = 'stock-badge';
      if (it.stock_quantity <= 0) {
        badge.classList.add('stock-empty');
        badge.textContent = 'Sold Out';
        card.classList.add('is-out-of-stock');
      } else if (it.stock_quantity <= 5) {
        badge.classList.add('stock-low');
        badge.textContent = `${it.stock_quantity} left`;
        card.classList.remove('is-out-of-stock');
      } else {
        badge.classList.add('stock-ok');
        badge.textContent = `${it.stock_quantity} left`;
        card.classList.remove('is-out-of-stock');
      }
      card.prepend(badge);
    } else {
      card.classList.remove('is-out-of-stock');
    }
  });
}

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

// ---- Recent Orders & Manager Authorization ----

const recentOrdersModal = $('recent-orders-modal');
const managerAuthModal = $('manager-auth-modal');

if ($('btn-recent-orders')) {
  $('btn-recent-orders').addEventListener('click', async () => {
    recentOrdersModal.hidden = false;
    await loadRecentOrders();
  });
}

if ($('close-recent-orders')) {
  $('close-recent-orders').addEventListener('click', () => { recentOrdersModal.hidden = true; });
}
if ($('done-recent-orders')) {
  $('done-recent-orders').addEventListener('click', () => { recentOrdersModal.hidden = true; });
}
if ($('btn-cancel-auth')) {
  $('btn-cancel-auth').addEventListener('click', () => { managerAuthModal.hidden = true; });
}

async function loadRecentOrders() {
  const container = $('recent-orders-list');
  if (!container) return;
  container.innerHTML = '<p class="muted small">Loading recent orders...</p>';
  try {
    const res = await fetch('/api/orders/recent');
    const orders = await res.json();
    if (!orders || orders.length === 0) {
      container.innerHTML = '<p class="muted small">No recent orders found.</p>';
      return;
    }
    container.innerHTML = '';
    for (const o of orders) {
      const card = document.createElement('div');
      card.style.cssText = 'background:var(--bg);border-radius:12px;padding:12px 14px;display:flex;justify-content:space-between;align-items:center;border:1px solid var(--line);gap:10px;';
      
      let itemsSummary = '';
      if (o.lines && o.lines.length > 0) {
        itemsSummary = o.lines.map((l) => `${l.quantity}x ${l.name}`).join(', ');
        if (itemsSummary.length > 34) itemsSummary = itemsSummary.substring(0, 34) + '...';
      } else {
        itemsSummary = 'No items';
      }

      const isPaid = o.status === 'paid';
      const isVoid = o.status === 'void';
      const statusBadge = isPaid
        ? '<span class="status-badge status-active">Paid</span>'
        : isVoid
          ? '<span class="status-badge status-inactive">Void</span>'
          : '<span class="status-badge" style="background:#FFF6E0;color:#8A6A0F;">Open</span>';

      card.innerHTML = `
        <div style="flex:1;min-width:0;">
          <div style="display:flex;align-items:center;gap:8px;margin-bottom:3px;">
            <strong style="font-size:0.95rem;">${o.order_no}</strong>
            ${statusBadge}
            <span class="muted small">${o.type === 'dine_in' ? 'Dine-in' : 'Takeaway'}</span>
          </div>
          <div class="muted small" style="white-space:nowrap;overflow:hidden;text-overflow:ellipsis;">${itemsSummary}</div>
          <div style="font-size:0.95rem;font-weight:800;color:var(--ink);margin-top:2px;">${naira(o.total_kobo)}</div>
        </div>
        <div style="display:flex;flex-direction:column;gap:5px;flex-shrink:0;">
          ${isPaid ? `<button class="btn-sm btn-reprint-order" data-id="${o.id}" data-no="${o.order_no}" style="background:var(--paper);border:1px solid #888;color:#555;">🖨️ Reprint</button>` : ''}
          ${!isVoid ? `<button class="btn-sm btn-reopen-order" data-id="${o.id}" data-no="${o.order_no}" style="background:var(--paper);border-color:var(--jollof);color:var(--jollof);">Edit Order</button>` : ''}
          ${isPaid ? `<button class="btn-sm btn-danger btn-void-order" data-id="${o.id}" data-no="${o.order_no}">Void</button>` : ''}
        </div>
      `;
      container.appendChild(card);
    }
  } catch (e) {
    container.innerHTML = '<p class="error small">Failed to load orders.</p>';
  }
}

if ($('recent-orders-list')) {
  $('recent-orders-list').addEventListener('click', (e) => {
    const reprintBtn = e.target.closest('.btn-reprint-order');
    if (reprintBtn) {
      reprintReceipt(reprintBtn.dataset.id, reprintBtn.dataset.no);
      return;
    }
    const reopenBtn = e.target.closest('.btn-reopen-order');
    if (reopenBtn) {
      promptManagerAuth(reopenBtn.dataset.id, reopenBtn.dataset.no, 'reopen');
      return;
    }
    const voidBtn = e.target.closest('.btn-void-order');
    if (voidBtn) {
      promptManagerAuth(voidBtn.dataset.id, voidBtn.dataset.no, 'void');
      return;
    }
  });
}

async function reprintReceipt(orderId, orderNo) {
  try {
    // Log the reprint in audit trail
    await fetch(`/api/order/${orderId}/reprint`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: '{}',
    });
    // Fetch full order data
    const res = await fetch(`/api/order/${orderId}`);
    if (!res.ok) { toast('Failed to load order for reprinting'); return; }
    const order = await res.json();
    populateReceipt(order);
    setTimeout(() => { window.print(); }, 150);
    toast(`Receipt reprinted for Order #${orderNo}`);
  } catch (err) {
    toast('Reprint error: ' + err.message);
  }
}

function promptManagerAuth(orderId, orderNo, action) {
  const isManager = document.body.dataset.role === 'manager' || document.body.dataset.role === 'owner';
  $('auth-order-id').value = orderId;
  $('auth-action').value = action;
  $('auth-order-label').textContent = `${action === 'reopen' ? 'Editing' : 'Voiding'} Order #${orderNo}`;
  $('btn-submit-auth').textContent = action === 'reopen' ? 'Confirm & Edit Order' : 'Confirm & Void Order';
  $('manager-auth-pin').value = '';
  $('manager-auth-reason').value = '';

  const pinGroup = $('manager-auth-pin').closest('.form-group');
  if (isManager) {
    if (pinGroup) pinGroup.style.display = 'none';
    $('manager-auth-pin').removeAttribute('required');
    $('manager-auth-reason').focus();
  } else {
    if (pinGroup) pinGroup.style.display = 'block';
    $('manager-auth-pin').setAttribute('required', 'required');
    $('manager-auth-pin').focus();
  }
  managerAuthModal.hidden = false;
}

if ($('manager-auth-form')) {
  $('manager-auth-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const orderId = $('auth-order-id').value;
    const action = $('auth-action').value;
    const pin = $('manager-auth-pin').value.trim();
    const reason = $('manager-auth-reason').value.trim();

    try {
      const res = await fetch(`/api/order/${orderId}/${action}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ manager_pin: pin, reason }),
      });

      if (!res.ok) {
        const err = await res.text();
        alert('Authorization Failed: ' + err);
        return;
      }

      const order = await res.json();
      managerAuthModal.hidden = true;
      if (recentOrdersModal) recentOrdersModal.hidden = true;
      if (managerHubModal) managerHubModal.hidden = true;

      if (action === 'reopen') {
        renderOrder(order);
        toast(`Order #${order.order_no} loaded for editing`);
      } else {
        renderOrder(null);
        toast(`Order #${order.order_no} voided`);
      }
    } catch (err) {
      alert('Request error: ' + err.message);
    }
  });
}

// ---- Till Inventory Modal ----

const tillInventoryModal = $('till-inventory-modal');

if ($('btn-till-inventory')) {
  $('btn-till-inventory').addEventListener('click', async () => {
    tillInventoryModal.hidden = false;
    await loadTillInventory();
  });
}
if ($('close-till-inventory')) {
  $('close-till-inventory').addEventListener('click', () => { tillInventoryModal.hidden = true; });
}
if ($('done-till-inventory')) {
  $('done-till-inventory').addEventListener('click', () => { tillInventoryModal.hidden = true; });
}

async function loadTillInventory() {
  const container = $('till-inventory-list');
  if (!container) return;
  container.innerHTML = '<p class="muted small">Loading inventory...</p>';
  try {
    const res = await fetch('/api/inventory');
    if (!res.ok) throw new Error('Failed to load inventory');
    const items = await res.json();
    renderTillInventory(items);
  } catch (err) {
    container.innerHTML = `<p class="error small">${err.message}</p>`;
  }
}

function renderTillInventory(items) {
  const container = $('till-inventory-list');
  if (!container) return;
  if (!items || items.length === 0) {
    container.innerHTML = '<p class="muted small">No items found.</p>';
    return;
  }
  container.innerHTML = '';
  items.forEach((it) => {
    const card = document.createElement('div');
    card.style = 'display:flex;align-items:center;justify-content:space-between;gap:12px;padding:10px 12px;background:var(--bg);border:1px solid var(--line);border-radius:12px;';
    
    let stockPill = '<span class="stock-pill untracked">Unlimited</span>';
    if (it.track_inventory) {
      if (it.stock_quantity <= 0) {
        stockPill = '<span class="stock-pill out-of-stock">Finished (0)</span>';
      } else if (it.stock_quantity <= 5) {
        stockPill = `<span class="stock-pill low-stock">${it.stock_quantity} left</span>`;
      } else {
        stockPill = `<span class="stock-pill in-stock">${it.stock_quantity} left</span>`;
      }
    }

    card.innerHTML = `
      <div style="display:flex;align-items:center;gap:10px;min-width:0;">
        <img src="/static/images/${it.id}.png" onerror="this.onerror=null; this.src='/static/images/default_dish.svg';" style="width:36px;height:36px;object-fit:cover;border-radius:6px;flex-shrink:0;">
        <div style="min-width:0;">
          <strong style="font-size:0.92rem;display:block;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;">${it.name}</strong>
          <span class="muted small">per ${it.unit_label}</span>
        </div>
      </div>
      <div style="display:flex;align-items:center;gap:4px;flex-shrink:0;">
        ${stockPill}
        <button class="btn-restock-sm btn-sub btn-till-restock" data-id="${it.id}" data-name="${it.name}" data-delta="-10" title="Reduce 10">−10</button>
        <button class="btn-restock-sm btn-sub btn-till-restock" data-id="${it.id}" data-name="${it.name}" data-delta="-5" title="Reduce 5">−5</button>
        <button class="btn-restock-sm btn-add btn-till-restock" data-id="${it.id}" data-name="${it.name}" data-delta="10" title="Add 10">+10</button>
        <button class="btn-restock-sm btn-add btn-till-restock" data-id="${it.id}" data-name="${it.name}" data-delta="25" title="Add 25">+25</button>
        <button class="btn-sm btn-till-set-stock" data-id="${it.id}" data-name="${it.name}" data-stock="${it.stock_quantity}">Set Stock</button>
      </div>
    `;
    container.appendChild(card);
  });
}

if ($('till-inventory-list')) {
  $('till-inventory-list').addEventListener('click', async (e) => {
    const isManager = document.body.dataset.role === 'manager' || document.body.dataset.role === 'owner';

    const restockBtn = e.target.closest('.btn-till-restock');
    if (restockBtn) {
      const { id, name, delta } = restockBtn.dataset;
      const d = parseInt(delta, 10);
      let pin = '';
      if (!isManager) {
        pin = prompt(`Manager PIN required to ${d < 0 ? 'reduce ' + d : 'add +' + d} to ${name}:`);
        if (!pin) return;
      }
      try {
        const res = await fetch(`/api/inventory/${id}/restock`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            delta: d,
            manager_pin: pin,
            reason: d < 0 ? `Till reduced stock ${d}` : `Till quick restock +${d}`
          })
        });
        if (!res.ok) {
          const text = await res.text();
          alert('Stock adjustment failed: ' + text);
          return;
        }
        toast(`${d < 0 ? 'Reduced ' + d : 'Added +' + d} for ${name}`);
        await loadTillInventory();
      } catch (err) {
        alert('Error: ' + err.message);
      }
      return;
    }

    const setBtn = e.target.closest('.btn-till-set-stock');
    if (setBtn) {
      const { id, name, stock } = setBtn.dataset;
      let pin = '';
      if (!isManager) {
        pin = prompt(`Manager PIN required to set stock for ${name}:`);
        if (!pin) return;
      }
      const val = prompt(`Set exact stock count for ${name}:`, stock || '0');
      if (val === null) return;
      const num = parseInt(val, 10);
      if (isNaN(num) || num < 0) {
        alert('Enter a valid non-negative number');
        return;
      }
      try {
        const res = await fetch(`/api/inventory/${id}/restock`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            stock_quantity: num,
            manager_pin: pin,
            reason: `Till set stock to ${num}`
          })
        });
        if (!res.ok) {
          const text = await res.text();
          alert('Failed to set stock: ' + text);
          return;
        }
        toast(`Stock set to ${num} for ${name}`);
        await loadTillInventory();
      } catch (err) {
        alert('Error: ' + err.message);
      }
      return;
    }
  });
}

// ---- Manager Shift Hub Dashboard ----

const managerHubModal = $('manager-hub-modal');

if ($('btn-open-manager-hub')) {
  $('btn-open-manager-hub').addEventListener('click', () => {
    if (managerHubModal) {
      managerHubModal.hidden = false;
      loadHubRecentOrders();
    }
  });
}
if ($('close-manager-hub')) {
  $('close-manager-hub').addEventListener('click', () => { if (managerHubModal) managerHubModal.hidden = true; });
}
if ($('done-manager-hub')) {
  $('done-manager-hub').addEventListener('click', () => { if (managerHubModal) managerHubModal.hidden = true; });
}

// Manager Hub Tabs
if ($('hub-tabs')) {
  $('hub-tabs').addEventListener('click', (e) => {
    const tab = e.target.closest('.tab');
    if (!tab) return;
    document.querySelectorAll('#hub-tabs .tab').forEach((t) => t.classList.toggle('active', t === tab));
    const target = tab.dataset.hubTab;
    document.querySelectorAll('.hub-tab-pane').forEach((p) => {
      p.hidden = p.id !== target;
    });
    if (target === 'hub-recent') loadHubRecentOrders();
  });
}

// Quick restock inside Manager Hub
document.querySelectorAll('.btn-hub-restock').forEach((btn) => {
  btn.addEventListener('click', async () => {
    const { id, name, delta } = btn.dataset;
    const d = parseInt(delta, 10);
    const reason = d < 0 ? `Manager hub reduced stock ${d}` : `Manager hub restock +${d}`;
    try {
      const res = await fetch(`/api/inventory/${id}/restock`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ delta: d, reason: reason })
      });
      if (!res.ok) {
        const text = await res.text();
        alert('Stock adjustment failed: ' + text);
        return;
      }
      toast(`${d < 0 ? 'Reduced ' + d : 'Added +' + d} for ${name}`);
      window.location.reload();
    } catch (err) {
      alert('Error: ' + err.message);
    }
  });
});

document.querySelectorAll('.btn-hub-set-stock').forEach((btn) => {
  btn.addEventListener('click', async () => {
    const { id, name, stock } = btn.dataset;
    const val = prompt(`Set exact stock count for ${name}:`, stock || '0');
    if (val === null) return;
    const num = parseInt(val, 10);
    if (isNaN(num) || num < 0) {
      alert('Enter a valid non-negative number');
      return;
    }
    try {
      const res = await fetch(`/api/inventory/${id}/restock`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ stock_quantity: num, reason: `Manager set stock to ${num}` })
      });
      if (!res.ok) {
        const text = await res.text();
        alert('Failed: ' + text);
        return;
      }
      toast(`Stock set to ${num} for ${name}`);
      window.location.reload();
    } catch (err) {
      alert('Error: ' + err.message);
    }
  });
});

// Toggle dish active inside Manager Hub
document.querySelectorAll('.btn-hub-toggle-dish').forEach((btn) => {
  btn.addEventListener('click', async () => {
    const { id, active } = btn.dataset;
    const next = active !== 'true';
    try {
      const res = await fetch(`/api/admin/menu/items/${id}/toggle`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ active: next })
      });
      if (!res.ok) {
        const text = await res.text();
        alert('Failed: ' + text);
        return;
      }
      toast(`Dish updated`);
      window.location.reload();
    } catch (err) {
      alert('Error: ' + err.message);
    }
  });
});

if ($('btn-refresh-hub-stock')) {
  $('btn-refresh-hub-stock').addEventListener('click', () => window.location.reload());
}
if ($('btn-refresh-hub-orders')) {
  $('btn-refresh-hub-orders').addEventListener('click', () => loadHubRecentOrders());
}

async function loadHubRecentOrders() {
  const container = $('hub-recent-orders-list');
  if (!container) return;
  container.innerHTML = '<p class="muted small">Loading recent orders...</p>';
  try {
    const res = await fetch('/api/orders/recent');
    if (!res.ok) throw new Error('Failed to load orders');
    const orders = await res.json();
    if (!orders || orders.length === 0) {
      container.innerHTML = '<p class="muted small">No recent orders found.</p>';
      return;
    }
    container.innerHTML = '';
    orders.forEach((o) => {
      let itemsSummary = 'No items';
      if (o.lines && o.lines.length > 0) {
        itemsSummary = o.lines.map((l) => `${l.quantity}x ${l.name}`).join(', ');
        if (itemsSummary.length > 36) itemsSummary = itemsSummary.substring(0, 36) + '...';
      }
      const isPaid = o.status === 'paid';
      const isVoid = o.status === 'void';
      const statusBadge = isPaid
        ? '<span class="status-badge status-active">Paid</span>'
        : isVoid
          ? '<span class="status-badge status-inactive">Void</span>'
          : '<span class="status-badge" style="background:#FFF6E0;color:#8A6A0F;">Open</span>';

      const row = document.createElement('div');
      row.style = 'display:flex;align-items:center;justify-content:space-between;gap:12px;padding:10px 12px;background:var(--bg);border:1px solid var(--line);border-radius:12px;';
      row.innerHTML = `
        <div style="flex:1;min-width:0;">
          <div style="display:flex;align-items:center;gap:8px;margin-bottom:2px;">
            <strong style="font-size:0.95rem;">${o.order_no}</strong>
            ${statusBadge}
            <span class="muted small">${o.type === 'dine_in' ? 'Dine-in' : 'Takeaway'}</span>
          </div>
          <div class="muted small" style="white-space:nowrap;overflow:hidden;text-overflow:ellipsis;">${itemsSummary}</div>
          <div style="font-size:0.95rem;font-weight:800;color:var(--ink);">${naira(o.total_kobo)}</div>
        </div>
        <div style="display:flex;gap:6px;flex-shrink:0;">
          ${isPaid ? `<button class="btn-sm btn-reprint-order" data-id="${o.id}" data-no="${o.order_no}">🖨️ Reprint</button>` : ''}
          ${!isVoid ? `<button class="btn-sm btn-reopen-order" data-id="${o.id}" data-no="${o.order_no}" style="background:var(--paper);border-color:var(--jollof);color:var(--jollof);">Edit</button>` : ''}
          ${isPaid ? `<button class="btn-sm btn-danger btn-void-order" data-id="${o.id}" data-no="${o.order_no}">Void</button>` : ''}
        </div>
      `;
      container.appendChild(row);
    });
  } catch (err) {
    container.innerHTML = `<p class="error small">${err.message}</p>`;
  }
}

if ($('hub-recent-orders-list')) {
  $('hub-recent-orders-list').addEventListener('click', (e) => {
    const reprintBtn = e.target.closest('.btn-reprint-order');
    if (reprintBtn) {
      reprintReceipt(reprintBtn.dataset.id, reprintBtn.dataset.no);
      return;
    }
    const reopenBtn = e.target.closest('.btn-reopen-order');
    if (reopenBtn) {
      if (managerHubModal) managerHubModal.hidden = true;
      promptManagerAuth(reopenBtn.dataset.id, reopenBtn.dataset.no, 'reopen');
      return;
    }
    const voidBtn = e.target.closest('.btn-void-order');
    if (voidBtn) {
      promptManagerAuth(voidBtn.dataset.id, voidBtn.dataset.no, 'void');
      return;
    }
  });
}
