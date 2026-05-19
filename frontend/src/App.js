import React, { useMemo, useState } from 'react';
import './App.css';

const ORDER_STATUS_OPTIONS = ['CREATED', 'PENDING', 'CONFIRMED', 'PAID', 'SHIPPED', 'CANCELLED', 'COMPLETED'];

function App() {
  const [view, setView] = useState('home');
  const [token, setToken] = useState(localStorage.getItem('token') || '');
  const [role, setRole] = useState(() => parseRoleFromToken(localStorage.getItem('token') || ''));
  const [toasts, setToasts] = useState([]);

  const notify = (type, message) => {
    const id = Date.now() + Math.random();
    setToasts((prev) => [...prev, { id, type, message }]);
    setTimeout(() => setToasts((prev) => prev.filter((t) => t.id !== id)), 3500);
  };

  const onLogin = (t) => {
    localStorage.setItem('token', t);
    setToken(t);
    try {
      setRole(parseRoleFromToken(t));
    } catch (e) {
      setRole('');
    }
    setView('inventory');
    notify('success', 'Успешный вход');
  };

  const onLogout = () => {
    localStorage.removeItem('token');
    setToken('');
    setRole('');
    setView('home');
    notify('info', 'Вы вышли из системы');
  };

  const tabs = useMemo(() => {
    const base = [
      ['home', 'Home'],
      ['inventory', 'Inventory'],
      ['reserve', 'Reserve'],
      ['catalog', 'Catalog'],
      ['orders', 'Orders'],
      ['notifications', 'Notifications'],
    ];
    if (role === 'admin') {
      base.splice(3, 0, ['admin', 'Admin']);
    }
    return base;
  }, [role]);

  return (
    <div className="App">
      <ToastStack toasts={toasts} />
      <nav className="nav">
        {tabs.map(([key, label]) => (
          <button key={key} className={view === key ? 'active' : ''} onClick={() => setView(key)}>
            {label}
          </button>
        ))}
        {token ? (
          <button className="logout" onClick={onLogout}>Logout</button>
        ) : (
          <button className={view === 'login' ? 'active' : ''} onClick={() => setView('login')}>Login</button>
        )}
      </nav>

      <main className="main">
        {view === 'home' && <Home />}
        {view === 'inventory' && <InventoryView token={token} notify={notify} />}
        {view === 'reserve' && <ReserveView token={token} notify={notify} />}
        {view === 'admin' && <AdminView token={token} notify={notify} />}
        {view === 'catalog' && <CatalogView token={token} notify={notify} />}
        {view === 'orders' && <OrdersView token={token} notify={notify} />}
        {view === 'notifications' && <NotificationsView token={token} notify={notify} />}
        {view === 'login' && <LoginView onLogin={onLogin} notify={notify} />}
      </main>
    </div>
  );
}

function Home() {
  return (
    <section className="card">
      <h2>Omnichannel Inventory - Demo UI</h2>
      <p>Демо-интерфейс для Inventory, Catalog, Orders и Notifications.</p>
    </section>
  );
}

function InventoryView({ token, notify }) {
  const [sku, setSku] = useState('');
  const [warehouseID, setWarehouseID] = useState('');
  const [stockBySku, setStockBySku] = useState(null);
  const [lowStock, setLowStock] = useState(null);
  const [warehouseStocks, setWarehouseStocks] = useState(null);

  const loadSku = async () => {
    const q = (sku || '').trim();
    if (!q) {
      notify('error', 'SKU пустой');
      return;
    }
    let data;
    try {
      data = await requestJSON(`/inventory/sku?sku=${encodeURIComponent(q)}`, { method: 'GET' }, token);
    } catch (e) {
      setStockBySku(null);
      notify('error', e.message || 'Ошибка загрузки SKU');
      return;
    }
    setStockBySku(data);
    if (Array.isArray(data?.stocks) && data.stocks.length === 0) {
      notify('warn', 'SKU не найден');
    } else {
      notify('success', 'Остатки по SKU загружены');
    }
  };
  const loadLow = async () => {
    const data = await requestJSON('/inventory/low', { method: 'GET' }, token);
    setLowStock(data);
    notify('success', 'Low-stock список обновлен');
  };
  const loadWarehouseStocks = async () => {
    const data = await requestJSON(`/inventory/warehouse/stocks?warehouse_id=${encodeURIComponent(warehouseID)}`, { method: 'GET' }, token);
    setWarehouseStocks(data);
    notify('success', 'Остатки склада загружены');
  };

  return (
    <div className="grid">
      <section className="card">
        <h3>Get Stock By SKU</h3>
        <input value={sku} onChange={(e) => setSku(e.target.value)} placeholder="Enter SKU" />
        <button onClick={() => wrapAction(loadSku, notify)}>Fetch</button>
        <ResultPanel title="SKU Result" data={stockBySku} />
      </section>
      <section className="card">
        <h3>Low Stock Items</h3>
        <button onClick={() => wrapAction(loadLow, notify)}>Refresh Low Stock</button>
        <ResultPanel title="Low Stock" data={lowStock} />
      </section>
      <section className="card">
        <h3>Stocks By Warehouse</h3>
        <input value={warehouseID} onChange={(e) => setWarehouseID(e.target.value)} placeholder="warehouse_id" />
        <button onClick={() => wrapAction(loadWarehouseStocks, notify)}>Load Warehouse Stocks</button>
        <ResultPanel title="Warehouse Stocks" data={warehouseStocks} />
      </section>
    </div>
  );
}

function ReserveView({ token, notify }) {
  const [orderId, setOrderId] = useState('');
  const [sku, setSku] = useState('');
  const [qty, setQty] = useState(1);
  const [result, setResult] = useState(null);

  const reserve = async () => {
    const body = buildReservePayload(orderId, sku, qty);
    if (!body) {
      notify('error', 'Заполни order id, sku и qty > 0');
      return;
    }
    const order = await requestJSON(`/orders?order_id=${encodeURIComponent(orderId.trim())}`, { method: 'GET' }, token);
    if (!isMutableOrderStatus(order?.status)) {
      notify('error', 'Ордер уже финализирован');
      return;
    }
    const data = await requestJSON('/inventory/reserve', jsonPost(body), token);
    setResult(data);
    notify('success', 'Резервация выполнена');
  };
  const release = async () => {
    if (!orderId.trim()) {
      notify('error', 'Заполни order id');
      return;
    }
    const order = await requestJSON(`/orders?order_id=${encodeURIComponent(orderId.trim())}`, { method: 'GET' }, token);
    if (!isMutableOrderStatus(order?.status)) {
      notify('error', 'Ордер уже финализирован');
      return;
    }
    // Release отменяет ВСЕ резервации по заказу - это финальная отмена
    await requestJSON('/inventory/release', jsonPost({ order_id: orderId.trim() }), token);
    setResult({ order_id: orderId.trim(), action: 'all_reservations_released' });
    notify('success', 'Все резервации по заказу отменены');
  };
  const confirm = async () => {
    const data = await requestJSON('/inventory/confirm', jsonPost({ order_id: orderId }), token);
    setResult(data);
    notify('success', 'Списание подтверждено');
  };

  return (
    <section className="card">
      <h3>Reserve / Confirm / Release</h3>
      <p className="hint">Reserve добавляет товары к заказу. Confirm финализирует списание. Release отменяет ВСЕ резервации (финальная отмена).</p>
      <input placeholder="order id" value={orderId} onChange={(e) => setOrderId(e.target.value)} />
      <input placeholder="sku" value={sku} onChange={(e) => setSku(e.target.value)} />
      <input type="number" value={qty} onChange={(e) => setQty(e.target.value)} />
      <div>
        <button onClick={() => wrapAction(reserve, notify)}>Reserve</button>
        <button onClick={() => wrapAction(confirm, notify)}>Confirm</button>
        <button onClick={() => wrapAction(release, notify)}>Release (отмена)</button>
      </div>
      <ResultPanel title="Reserve Result" data={result} />
    </section>
  );
}

function AdminView({ token, notify }) {
  const [sku, setSku] = useState('');
  const [widFrom, setWidFrom] = useState('');
  const [widTo, setWidTo] = useState('');
  const [qty, setQty] = useState(1);
  const [msg, setMsg] = useState(null);
  const [warehouses, setWarehouses] = useState([]);
  const [newWarehouse, setNewWarehouse] = useState({ name: '', location: '' });
  const [updateWarehouse, setUpdateWarehouse] = useState({ id: '', name: '', location: '', is_active: true });
  const [activeTab, setActiveTab] = useState('transfer');

  const transfer = async () => {
    const data = await requestJSON('/inventory/transfer', jsonPost({ sku, from_warehouse: widFrom, to_warehouse: widTo, qty: Number(qty) }), token);
    setMsg(data);
    notify('success', 'Трансфер выполнен');
  };
  const fetchWarehouses = async () => {
    const data = await requestJSON('/inventory/warehouses', { method: 'GET' }, token);
    setWarehouses(data.warehouses || []);
    notify('success', 'Склады загружены');
  };
  const createWarehouse = async () => {
    const data = await requestJSON('/inventory/warehouse', jsonPost({ name: newWarehouse.name, location: newWarehouse.location }), token);
    setMsg(data);
    setNewWarehouse({ name: '', location: '' });
    await fetchWarehouses();
    notify('success', 'Склад создан');
  };
  const updateWarehouseInfo = async () => {
    const data = await requestJSON('/inventory/warehouse', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(updateWarehouse),
    }, token);
    setMsg(data);
    notify('success', 'Склад обновлен');
  };
  const addReceipt = async () => {
    const data = await requestJSON('/inventory/receipt', jsonPost({ items: [{ sku, warehouse_id: widFrom, qty: Number(qty) }] }), token);
    setMsg(data);
    notify('success', 'Поставка добавлена');
  };
  const updateSafety = async () => {
    const data = await requestJSON('/inventory/safety', jsonPost({ sku, warehouse_id: widFrom, level: Number(qty) }), token);
    setMsg(data);
    notify('success', 'Safety level обновлен');
  };

  return (
    <section className="card">
      <h3>Admin Panel</h3>
      <div className="nav">
        <button className={activeTab === 'transfer' ? 'active' : ''} onClick={() => setActiveTab('transfer')}>Transfer</button>
        <button className={activeTab === 'receipt' ? 'active' : ''} onClick={() => setActiveTab('receipt')}>Add Stock</button>
        <button className={activeTab === 'safety' ? 'active' : ''} onClick={() => setActiveTab('safety')}>Safety Level</button>
        <button className={activeTab === 'warehouse' ? 'active' : ''} onClick={() => setActiveTab('warehouse')}>Warehouses</button>
      </div>

      {activeTab === 'transfer' && (
        <div>
          <input placeholder="sku" value={sku} onChange={(e) => setSku(e.target.value)} />
          <input placeholder="from warehouse" value={widFrom} onChange={(e) => setWidFrom(e.target.value)} />
          <input placeholder="to warehouse" value={widTo} onChange={(e) => setWidTo(e.target.value)} />
          <input type="number" value={qty} onChange={(e) => setQty(e.target.value)} />
          <button onClick={() => wrapAction(transfer, notify)}>Transfer</button>
        </div>
      )}

      {activeTab === 'receipt' && (
        <div>
          <input placeholder="sku" value={sku} onChange={(e) => setSku(e.target.value)} />
          <input placeholder="warehouse id" value={widFrom} onChange={(e) => setWidFrom(e.target.value)} />
          <input type="number" value={qty} onChange={(e) => setQty(e.target.value)} />
          <button onClick={() => wrapAction(addReceipt, notify)}>Add Receipt</button>
        </div>
      )}

      {activeTab === 'safety' && (
        <div>
          <input placeholder="sku" value={sku} onChange={(e) => setSku(e.target.value)} />
          <input placeholder="warehouse id" value={widFrom} onChange={(e) => setWidFrom(e.target.value)} />
          <input type="number" value={qty} onChange={(e) => setQty(e.target.value)} />
          <button onClick={() => wrapAction(updateSafety, notify)}>Update Level</button>
        </div>
      )}

      {activeTab === 'warehouse' && (
        <div>
          <button onClick={() => wrapAction(fetchWarehouses, notify)}>Load Warehouses</button>
          <input placeholder="name" value={newWarehouse.name} onChange={(e) => setNewWarehouse({ ...newWarehouse, name: e.target.value })} />
          <input placeholder="location" value={newWarehouse.location} onChange={(e) => setNewWarehouse({ ...newWarehouse, location: e.target.value })} />
          <button onClick={() => wrapAction(createWarehouse, notify)}>Create</button>
          <div>
            <h4>Update Warehouse</h4>
            <input placeholder="warehouse id" value={updateWarehouse.id} onChange={(e) => setUpdateWarehouse({ ...updateWarehouse, id: e.target.value })} />
            <input placeholder="name" value={updateWarehouse.name} onChange={(e) => setUpdateWarehouse({ ...updateWarehouse, name: e.target.value })} />
            <input placeholder="location" value={updateWarehouse.location} onChange={(e) => setUpdateWarehouse({ ...updateWarehouse, location: e.target.value })} />
            <select value={String(updateWarehouse.is_active)} onChange={(e) => setUpdateWarehouse({ ...updateWarehouse, is_active: e.target.value === 'true' })}>
              <option value="true">active</option>
              <option value="false">inactive</option>
            </select>
            <button onClick={() => wrapAction(updateWarehouseInfo, notify)}>Update Warehouse</button>
          </div>
          <ul>{warehouses.map((w, i) => (
            <li key={i}>
              <strong>{w.id || w.warehouse_id || i}</strong> — {w.name} ({w.location}) {w.is_active === false ? <span className="muted">[inactive]</span> : null}
            </li>
          ))}</ul>
        </div>
      )}

      <ResultPanel title="Admin Result" data={msg} />
    </section>
  );
}

function CatalogView({ token, notify }) {
  const [sku, setSku] = useState('');
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [price, setPrice] = useState(0);
  const [productId, setProductId] = useState('');
  const [searchQ, setSearchQ] = useState('');
  const [bulkSku, setBulkSku] = useState('');
  const [msg, setMsg] = useState(null);

  const createProduct = async () => {
    const data = await requestJSON('/catalog/products', jsonPost({ sku, name, description, price: Number(price) }), token);
    setMsg(data);
    notify('success', 'Продукт создан');
  };
  const getProduct = async () => {
    const data = await requestJSON(`/catalog/products?product_id=${encodeURIComponent(productId)}`, { method: 'GET' }, token);
    setMsg(data);
    notify('success', 'Продукт получен');
  };
  const searchProducts = async () => {
    const data = await requestJSON(`/catalog/search?q=${encodeURIComponent(searchQ)}`, { method: 'GET' }, token);
    setMsg(data);
    notify('success', 'Поиск выполнен');
  };
  const updatePrice = async () => {
    const data = await requestJSON('/catalog/price', jsonPost({ product_id: productId, new_price: Number(price) }), token);
    setMsg(data);
    notify('success', 'Цена обновлена');
  };
  const bulkGet = async () => {
    const skus = bulkSku.split(',').map((x) => x.trim()).filter(Boolean);
    const data = await requestJSON('/catalog/bulk', jsonPost({ skus }), token);
    setMsg(data);
    notify('success', 'Bulk запрос выполнен');
  };

  return (
    <section className="card">
      <h3>Catalog Management</h3>
      <input placeholder="SKU" value={sku} onChange={(e) => setSku(e.target.value)} />
      <input placeholder="Name" value={name} onChange={(e) => setName(e.target.value)} />
      <input placeholder="Description" value={description} onChange={(e) => setDescription(e.target.value)} />
      <input type="number" placeholder="Price" value={price} onChange={(e) => setPrice(e.target.value)} />
      <button onClick={() => wrapAction(createProduct, notify)}>Create</button>
      <hr />
      <input placeholder="Product ID" value={productId} onChange={(e) => setProductId(e.target.value)} />
      <button onClick={() => wrapAction(getProduct, notify)}>Get</button>
      <hr />
      <input placeholder="Search query" value={searchQ} onChange={(e) => setSearchQ(e.target.value)} />
      <button onClick={() => wrapAction(searchProducts, notify)}>Search</button>
      <hr />
      <input placeholder="Product ID for price update" value={productId} onChange={(e) => setProductId(e.target.value)} />
      <input type="number" placeholder="New price" value={price} onChange={(e) => setPrice(e.target.value)} />
      <button onClick={() => wrapAction(updatePrice, notify)}>Update Price</button>
      <hr />
      <input placeholder="Bulk SKU list: SKU-1,SKU-2" value={bulkSku} onChange={(e) => setBulkSku(e.target.value)} />
      <button onClick={() => wrapAction(bulkGet, notify)}>Bulk Get</button>
      <ResultPanel title="Catalog Result" data={msg} />
    </section>
  );
}

function OrdersView({ token, notify }) {
  const [orderId, setOrderId] = useState('');
  const [customerId, setCustomerId] = useState('');
  const [status, setStatus] = useState('');
  const [bulkIds, setBulkIds] = useState('');
  const [msg, setMsg] = useState(null);

  const createOrder = async () => {
    const cid = (customerId || '').trim();
    if (!cid) {
      notify('error', 'Customer ID required');
      return;
    }
    try {
      const body = { customer_id: cid };
      const data = await requestJSON('/orders', jsonPost(body), token);
      setMsg(data);
      if (data?.order_id) setOrderId(data.order_id);
      notify('success', 'Ордер создан');
    } catch (e) {
      setMsg({ error: e.message });
      notify('error', e.message || 'Ошибка создания ордера');
    }
  };
  const getOrder = async () => {
    const data = await requestJSON(`/orders?order_id=${encodeURIComponent(orderId)}`, { method: 'GET' }, token);
    setMsg(data);
    notify('success', 'Ордер загружен');
  };
  const cancelOrder = async () => {
    const data = await requestJSON('/orders/cancel', jsonPost({ order_id: orderId }), token);
    setMsg(data);
    notify('success', 'Ордер отменен');
  };
  const updateStatus = async () => {
    const data = await requestJSON('/orders/status', jsonPost({ order_id: orderId, new_status: status }), token);
    setMsg(data);
    notify('success', 'Статус обновлен');
  };
  const calculateTotal = async () => {
    const id = orderId.trim();
    if (!id) {
      notify('error', 'Order ID required');
      return;
    }
    const order = await requestJSON(`/orders?order_id=${encodeURIComponent(id)}`, { method: 'GET' }, token);
    const items = Array.isArray(order?.items) ? order.items : [];
    if (items.length === 0) {
      setMsg({ total: 0, order_id: id });
      notify('success', 'Сумма рассчитана');
      return;
    }
    const data = await requestJSON('/orders/calculate', jsonPost({ items }), token);
    setMsg(data);
    notify('success', 'Сумма рассчитана');
  };
  const bulkGet = async () => {
    const order_ids = bulkIds.split(',').map((x) => x.trim()).filter(Boolean);
    const data = await requestJSON('/orders/bulk', jsonPost({ order_ids }), token);
    setMsg(data);
    notify('success', 'Bulk orders загружены');
  };
  const listOrders = async () => {
    const data = await requestJSON('/orders', { method: 'GET' }, token);
    setMsg(data);
    notify('success', 'Список ордеров загружен');
  };

  return (
    <section className="card">
      <h3>Order Management</h3>
      <div>
        <input placeholder="Customer ID" value={customerId} onChange={(e) => setCustomerId(e.target.value)} />
        <button onClick={() => wrapAction(createOrder, notify)}>Create</button>
      </div>
      <hr />
      <input placeholder="Order ID" value={orderId} onChange={(e) => setOrderId(e.target.value)} />
      <button onClick={() => wrapAction(getOrder, notify)}>Get</button>
      <button onClick={() => wrapAction(cancelOrder, notify)}>Cancel</button>
      <hr />
      <select value={status} onChange={(e) => setStatus(e.target.value)}>
        <option value="">Select status</option>
        {ORDER_STATUS_OPTIONS.map((item) => (
          <option key={item} value={item}>{item}</option>
        ))}
      </select>
      <button onClick={() => wrapAction(updateStatus, notify)}>Update Status</button>
      <button onClick={() => wrapAction(calculateTotal, notify)}>Calculate Total</button>
      <hr />
      <input placeholder="Bulk order IDs: id1,id2" value={bulkIds} onChange={(e) => setBulkIds(e.target.value)} />
      <button onClick={() => wrapAction(bulkGet, notify)}>Bulk Get Orders</button>
      <button onClick={() => wrapAction(listOrders, notify)}>List All Orders</button>
      <ResultPanel title="Orders Result" data={msg} />
    </section>
  );
}

function NotificationsView({ token, notify }) {
  const [email, setEmail] = useState('');
  const [subject, setSubject] = useState('');
  const [body, setBody] = useState('');
  const [orderId, setOrderId] = useState('');
  const [sku, setSku] = useState('');
  const [warehouseId, setWarehouseId] = useState('');
  const [currentQty, setCurrentQty] = useState(0);
  const [threshold, setThreshold] = useState(0);
  const [msg, setMsg] = useState(null);

  const sendEmail = async () => {
    const data = await requestJSON('/notifications/email', jsonPost({ to: email, subject, body }), token);
    setMsg(data);
    notify('success', 'Email запрос отправлен');
  };
  const sendOrderConfirm = async () => {
    const data = await requestJSON('/notifications/order-confirmation', jsonPost({ order_id: orderId, customer_email: email, order_details: body }), token);
    setMsg(data);
    notify('success', 'Order confirmation отправлен');
  };
  const sendStockAlert = async () => {
    const data = await requestJSON('/notifications/stock-alert', jsonPost({ sku, warehouse_id: warehouseId, current_qty: Number(currentQty), threshold: Number(threshold) }), token);
    setMsg(data);
    notify('success', 'Stock alert отправлен');
  };

  return (
    <section className="card">
      <h3>Notifications</h3>
      <input placeholder="To Email" value={email} onChange={(e) => setEmail(e.target.value)} />
      <input placeholder="Subject" value={subject} onChange={(e) => setSubject(e.target.value)} />
      <textarea placeholder="Body / Details" value={body} onChange={(e) => setBody(e.target.value)} />
      <button onClick={() => wrapAction(sendEmail, notify)}>Send Email</button>
      <hr />
      <input placeholder="Order ID" value={orderId} onChange={(e) => setOrderId(e.target.value)} />
      <button onClick={() => wrapAction(sendOrderConfirm, notify)}>Send Order Confirmation</button>
      <hr />
      <input placeholder="SKU" value={sku} onChange={(e) => setSku(e.target.value)} />
      <input placeholder="Warehouse ID" value={warehouseId} onChange={(e) => setWarehouseId(e.target.value)} />
      <input type="number" placeholder="Current Qty" value={currentQty} onChange={(e) => setCurrentQty(e.target.value)} />
      <input type="number" placeholder="Threshold" value={threshold} onChange={(e) => setThreshold(e.target.value)} />
      <button onClick={() => wrapAction(sendStockAlert, notify)}>Send Stock Alert</button>
      <ResultPanel title="Notifications Result" data={msg} />
    </section>
  );
}

function LoginView({ onLogin, notify }) {
  const [username, setUsername] = useState('demo');
  const [password, setPassword] = useState('pass');
  const [role, setRole] = useState('user');
  const [msg, setMsg] = useState(null);

  const doRegister = async () => {
    const data = await rawRequestJSON('/auth/register', jsonPost({ username, password, role }));
    setMsg(data);
    notify('success', 'Пользователь зарегистрирован');
  };
  const doLogin = async () => {
    const data = await rawRequestJSON('/auth/login', jsonPost({ username, password }));
    if (data.token) onLogin(data.token);
    setMsg(data);
  };

  return (
    <section className="card">
      <h3>Login / Register</h3>
      <input value={username} onChange={(e) => setUsername(e.target.value)} placeholder="username" />
      <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="password" />
      <select value={role} onChange={(e) => setRole(e.target.value)}>
        <option value="user">user</option>
        <option value="admin">admin</option>
      </select>
      <div>
        <button onClick={() => wrapAction(doRegister, notify)}>Register</button>
        <button onClick={() => wrapAction(doLogin, notify)}>Login</button>
      </div>
      <ResultPanel title="Auth Result" data={msg} />
    </section>
  );
}

async function wrapAction(fn, notify) {
  try {
    await fn();
  } catch (e) {
    notify('error', e.message || 'Ошибка');
  }
}

async function requestJSON(path, options, token) {
  return rawRequestJSON(path, withAuth(options, token));
}

async function rawRequestJSON(path, options) {
  const res = await fetch(path, options);
  const text = await res.text();
  let data = {};
  try {
    data = text ? JSON.parse(text) : {};
  } catch {
    data = { raw: text };
  }
  if (!res.ok) {
    throw new Error(data.error || `HTTP ${res.status}`);
  }
  if (data.error) {
    throw new Error(data.error);
  }
  return data;
}

function withAuth(options, token) {
  const headers = { ...(options?.headers || {}) };
  if (token) headers.Authorization = `Bearer ${token}`;
  return { ...(options || {}), headers };
}

function jsonPost(body) {
  return { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) };
}

function ResultPanel({ title, data }) {
  if (!data) return null;
  return (
    <div className="result-card">
      <h4>{title}</h4>
      <DataTree data={data} />
    </div>
  );
}

function normalizeSku(value) {
  return String(value || '').trim().toLowerCase();
}

function isMutableOrderStatus(value) {
  const status = String(value || '').trim().toUpperCase();
  return status === 'CREATED' || status === 'PENDING';
}

function buildReservePayload(orderId, sku, qty) {
  const id = String(orderId || '').trim();
  const s = String(sku || '').trim();
  const q = Number(qty);
  if (!id || !s || q <= 0) {
    return null;
  }
  return { order_id: id, items: [{ sku: s, warehouse_id: '', qty: q }] };
}

function adjustOrderItems(items, sku, delta) {
  const target = normalizeSku(sku);
  const result = [];
  let matched = false;
  for (const item of items || []) {
    const currentSku = String(item?.sku || '').trim();
    const currentQty = Number(item?.qty || 0);
    if (normalizeSku(currentSku) !== target) {
      if (currentSku && currentQty > 0) {
        result.push({ sku: currentSku, qty: currentQty });
      }
      continue;
    }
    matched = true;
    const nextQty = currentQty + delta;
    if (nextQty > 0) {
      result.push({ sku: currentSku, qty: nextQty });
    }
  }
  if (!matched && delta < 0) {
    return items || [];
  }
  return result;
}

function formatKey(k) {
  if (!k) return '';
  // convert SNAKE_CASE or snake_case or camelCase to Title Case
  const s1 = k.replace(/_/g, ' ');
  const parts = s1.split(/\s+/).filter(Boolean);
  return parts.map(p => p[0]?.toUpperCase() + p.slice(1).toLowerCase()).join(' ');
}

function DataTree({ data }) {
  if (data == null) return <span className="muted">null</span>;
  if (typeof data !== 'object') return <span className="kv-primitive">{String(data)}</span>;
  if (Array.isArray(data)) {
    if (data.length === 0) return <span className="muted">[]</span>;
    return (
      <ul className="data-list">
        {data.map((item, idx) => <li key={idx}><DataTree data={item} /></li>)}
      </ul>
    );
  }
  return (
    <div className="kv-grid">
      {Object.entries(data).map(([k, v]) => (
        <div key={k} className="kv-row">
          <span className="kv-key">{formatKey(k)}</span>
          <span className="kv-value"><DataTree data={v} /></span>
        </div>
      ))}
    </div>
  );
}

function ToastStack({ toasts }) {
  return (
    <div className="toast-stack">
      {toasts.map((t) => (
        <div key={t.id} className={`toast ${t.type}`}>
          {t.message}
        </div>
      ))}
    </div>
  );
}

function parseRoleFromToken(token) {
  if (!token) return '';
  try {
    const parts = token.split('.');
    if (parts.length < 2) return '';
    // browser atob is fine for demo UI
    const raw = atob(parts[1].replace(/-/g, '+').replace(/_/g, '/'));
    const payload = JSON.parse(raw);
    return payload?.role || '';
  } catch (e) {
    return '';
  }
}

export default App;
