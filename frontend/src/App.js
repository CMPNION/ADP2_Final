import React, {useState} from 'react';
import './App.css';

function App() {
  const [view, setView] = useState('home');
  const [token, setToken] = useState(localStorage.getItem('token')||'')

  const onLogin = (t)=>{ localStorage.setItem('token', t); setToken(t); setView('inventory') }
  const onLogout = ()=>{ localStorage.removeItem('token'); setToken(''); setView('home') }

  return (
    <div className="App">
      <nav className="nav">
        <button onClick={() => setView('home')}>Home</button>
        <button onClick={() => setView('inventory')}>Inventory</button>
        <button onClick={() => setView('reserve')}>Reserve</button>
        <button onClick={() => setView('admin')}>Admin</button>
        <button onClick={() => setView('catalog')}>Catalog</button>
        <button onClick={() => setView('orders')}>Orders</button>
        <button onClick={() => setView('notifications')}>Notifications</button>
        { token ? <button onClick={onLogout}>Logout</button> : <button onClick={() => setView('login')}>Login</button> }
      </nav>
      <main className="main">
        {view === 'home' && <Home />}
        {view === 'inventory' && <Inventory token={token} />}
        {view === 'reserve' && <Reserve token={token} />}
        {view === 'admin' && <Admin token={token} />}
        {view === 'catalog' && <Catalog token={token} />}
        {view === 'orders' && <Orders token={token} />}
        {view === 'notifications' && <Notifications token={token} />}
        {view === 'login' && <Login onLogin={onLogin} />}
      </main>
    </div>
  );
}

function Home() {
  return (
    <div>
      <h2>Omnichannel Inventory - Demo UI</h2>
      <p>Use the navigation to explore inventory, create reservations and manage stock.</p>
    </div>
  );
}

function Inventory({token}) {
  const [sku, setSku] = useState('')
  const [result, setResult] = useState(null)
  const [low, setLow] = useState(null)

  const fetchBySku = async () => {
    const res = await fetch(`/inventory/sku?sku=${encodeURIComponent(sku)}`, { headers: getAuthHeaders(token) })
    const jb = await res.json()
    setResult(jb)
  }
  const fetchLow = async () => {
    const res = await fetch(`/inventory/low`, { headers: getAuthHeaders(token) })
    const jb = await res.json()
    setLow(jb)
  }

  return (
    <div>
      <h3>Get Stock By SKU</h3>
      <input value={sku} onChange={e=>setSku(e.target.value)} placeholder="Enter SKU" />
      <button onClick={fetchBySku}>Fetch</button>
      <pre>{result && JSON.stringify(result, null, 2)}</pre>

      <h3>Low Stock Items</h3>
      <button onClick={fetchLow}>Refresh Low Stock</button>
      <pre>{low && JSON.stringify(low, null, 2)}</pre>
    </div>
  )
}

function Reserve({token}) {
  const [orderId, setOrderId] = useState('')
  const [sku, setSku] = useState('')
  const [qty, setQty] = useState(1)
  const [res, setRes] = useState(null)

  const makeReserve = async () => {
    const body = { order_id: orderId || 'demo-'+Date.now(), items: [{ sku, warehouse_id: '', qty: Number(qty)}] }
    const r = await fetch('/inventory/reserve', { method: 'POST', headers: {...getAuthHeaders(token), 'Content-Type':'application/json'}, body: JSON.stringify(body) })
    const jb = await r.json()
    setRes(jb)
  }

  const release = async () => {
    const body = { order_id: orderId }
    const r = await fetch('/inventory/release', { method: 'POST', headers: {...getAuthHeaders(token), 'Content-Type':'application/json'}, body: JSON.stringify(body) })
    setRes(await r.json())
  }
  const confirm = async () => {
    const body = { order_id: orderId }
    const r = await fetch('/inventory/confirm', { method: 'POST', headers: {...getAuthHeaders(token), 'Content-Type':'application/json'}, body: JSON.stringify(body) })
    setRes(await r.json())
  }

  return (
    <div>
      <h3>Reserve / Release / Confirm</h3>
      <input placeholder="order id" value={orderId} onChange={e=>setOrderId(e.target.value)} />
      <input placeholder="sku" value={sku} onChange={e=>setSku(e.target.value)} />
      <input type="number" value={qty} onChange={e=>setQty(e.target.value)} />
      <div>
        <button onClick={makeReserve}>Reserve</button>
        <button onClick={release}>Release</button>
        <button onClick={confirm}>Confirm</button>
      </div>
      <pre>{res && JSON.stringify(res, null, 2)}</pre>
    </div>
  )
}

function Admin({token}) {
  const [sku, setSku] = useState('')
  const [widFrom, setWidFrom] = useState('')
  const [widTo, setWidTo] = useState('')
  const [qty, setQty] = useState(1)
  const [msg, setMsg] = useState(null)
  const [warehouses, setWarehouses] = useState([])
  const [newWarehouse, setNewWarehouse] = useState({ name: '', location: '' })
  const [activeTab, setActiveTab] = useState('transfer')

  const transfer = async () => {
    const body = { sku, from_warehouse: widFrom, to_warehouse: widTo, qty: Number(qty) }
    const r = await fetch('/inventory/transfer', { method: 'POST', headers: {...getAuthHeaders(token), 'Content-Type':'application/json'}, body: JSON.stringify(body) })
    setMsg(await r.json())
  }

  const fetchWarehouses = async () => {
    const r = await fetch('/inventory/warehouses', { headers: getAuthHeaders(token) })
    const jb = await r.json()
    setWarehouses(jb.warehouses || [])
  }

  const createWarehouse = async () => {
    const body = { name: newWarehouse.name, location: newWarehouse.location }
    const r = await fetch('/inventory/warehouse', { method: 'POST', headers: {...getAuthHeaders(token), 'Content-Type':'application/json'}, body: JSON.stringify(body) })
    const jb = await r.json()
    setMsg(jb)
    setNewWarehouse({ name: '', location: '' })
    fetchWarehouses()
  }

  const addReceipt = async () => {
    const body = { items: [{ sku, warehouse_id: widFrom, qty: Number(qty) }] }
    const r = await fetch('/inventory/receipt', { method: 'POST', headers: {...getAuthHeaders(token), 'Content-Type':'application/json'}, body: JSON.stringify(body) })
    const jb = await r.json()
    setMsg(jb)
  }

  const updateSafety = async () => {
    const body = { sku, level: Number(qty) }
    const r = await fetch('/inventory/safety', { method: 'POST', headers: {...getAuthHeaders(token), 'Content-Type':'application/json'}, body: JSON.stringify(body) })
    const jb = await r.json()
    setMsg(jb)
  }

  return (
    <div>
      <h3>Admin Panel</h3>
      <div className="admin-tabs">
        <button onClick={() => setActiveTab('transfer')} className={activeTab === 'transfer' ? 'active' : ''}>Transfer</button>
        <button onClick={() => setActiveTab('receipt')} className={activeTab === 'receipt' ? 'active' : ''}>Add Stock</button>
        <button onClick={() => setActiveTab('safety')} className={activeTab === 'safety' ? 'active' : ''}>Safety Level</button>
        <button onClick={() => setActiveTab('warehouse')} className={activeTab === 'warehouse' ? 'active' : ''}>Warehouses</button>
      </div>

      {activeTab === 'transfer' && (
        <div className="admin-section">
          <h4>Transfer Stock</h4>
          <input placeholder="sku" value={sku} onChange={e=>setSku(e.target.value)} />
          <input placeholder="from warehouse" value={widFrom} onChange={e=>setWidFrom(e.target.value)} />
          <input placeholder="to warehouse" value={widTo} onChange={e=>setWidTo(e.target.value)} />
          <input type="number" value={qty} onChange={e=>setQty(e.target.value)} />
          <button onClick={transfer}>Transfer</button>
        </div>
      )}

      {activeTab === 'receipt' && (
        <div className="admin-section">
          <h4>Add Stock Receipt</h4>
          <input placeholder="sku" value={sku} onChange={e=>setSku(e.target.value)} />
          <input placeholder="warehouse id" value={widFrom} onChange={e=>setWidFrom(e.target.value)} />
          <input type="number" placeholder="qty" value={qty} onChange={e=>setQty(e.target.value)} />
          <button onClick={addReceipt}>Add Receipt</button>
        </div>
      )}

      {activeTab === 'safety' && (
        <div className="admin-section">
          <h4>Update Safety Stock Level</h4>
          <input placeholder="sku" value={sku} onChange={e=>setSku(e.target.value)} />
          <input type="number" placeholder="safety level" value={qty} onChange={e=>setQty(e.target.value)} />
          <button onClick={updateSafety}>Update Level</button>
        </div>
      )}

      {activeTab === 'warehouse' && (
        <div className="admin-section">
          <h4>Manage Warehouses</h4>
          <button onClick={fetchWarehouses}>Load Warehouses</button>
          <div>
            <h5>Create New</h5>
            <input placeholder="name" value={newWarehouse.name} onChange={e=>setNewWarehouse({...newWarehouse, name: e.target.value})} />
            <input placeholder="location" value={newWarehouse.location} onChange={e=>setNewWarehouse({...newWarehouse, location: e.target.value})} />
            <button onClick={createWarehouse}>Create</button>
          </div>
          <h5>Existing:</h5>
          <ul>
            {warehouses.map((w,i) => <li key={i}>{w.name} ({w.location}) - {w.is_active ? 'Active' : 'Inactive'}</li>)}
          </ul>
        </div>
      )}

      <pre>{msg && JSON.stringify(msg, null, 2)}</pre>
    </div>
  )
}

function getAuthHeaders(token){ if(!token) return {}; return { 'Authorization': 'Bearer '+token } }

function Login({onLogin}){
  const [username,setUsername]=useState('demo')
  const [password,setPassword]=useState('pass')
  const [role,setRole]=useState('user')
  const [msg,setMsg]=useState(null)

  const doRegister = async ()=>{
    const r = await fetch('http://localhost:8090/auth/register', { method: 'POST', headers: {'Content-Type':'application/json'}, body: JSON.stringify({username,password,role}) })
    setMsg(await r.json())
  }
  const doLogin = async ()=>{
    const r = await fetch('http://localhost:8090/auth/login', { method: 'POST', headers: {'Content-Type':'application/json'}, body: JSON.stringify({username,password}) })
    const jb = await r.json()
    if(jb.token){ onLogin(jb.token) }
    else setMsg(jb)
  }
  return (
    <div>
      <h3>Login / Register</h3>
      <div>
        <input value={username} onChange={e=>setUsername(e.target.value)} placeholder="username" />
        <input value={password} onChange={e=>setPassword(e.target.value)} placeholder="password" />
        <select value={role} onChange={e=>setRole(e.target.value)}>
          <option value="user">user</option>
          <option value="admin">admin</option>
        </select>
      </div>
      <div>
        <button onClick={doRegister}>Register</button>
        <button onClick={doLogin}>Login</button>
      </div>
      <pre>{msg && JSON.stringify(msg,null,2)}</pre>
    </div>
  )
}

function Catalog({token}) {
  const [sku, setSku] = useState('')
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [price, setPrice] = useState(0)
  const [productId, setProductId] = useState('')
  const [searchQ, setSearchQ] = useState('')
  const [msg, setMsg] = useState(null)

  const createProduct = async () => {
    const body = { sku, name, description, price: Number(price) }
    const r = await fetch('/catalog/products', { method: 'POST', headers: {...getAuthHeaders(token), 'Content-Type':'application/json'}, body: JSON.stringify(body) })
    setMsg(await r.json())
  }

  const getProduct = async () => {
    const r = await fetch(`/catalog/products?product_id=${encodeURIComponent(productId)}`, { headers: getAuthHeaders(token) })
    setMsg(await r.json())
  }

  const searchProducts = async () => {
    const r = await fetch(`/catalog/search?q=${encodeURIComponent(searchQ)}`, { headers: getAuthHeaders(token) })
    setMsg(await r.json())
  }

  const updatePrice = async () => {
    const body = { product_id: productId, new_price: Number(price) }
    const r = await fetch('/catalog/price', { method: 'POST', headers: {...getAuthHeaders(token), 'Content-Type':'application/json'}, body: JSON.stringify(body) })
    setMsg(await r.json())
  }

  return (
    <div>
      <h3>Catalog Management</h3>
      <div style={{marginBottom: '20px'}}>
        <h4>Create Product</h4>
        <input placeholder="SKU" value={sku} onChange={e=>setSku(e.target.value)} />
        <input placeholder="Name" value={name} onChange={e=>setName(e.target.value)} />
        <input placeholder="Description" value={description} onChange={e=>setDescription(e.target.value)} />
        <input type="number" placeholder="Price" value={price} onChange={e=>setPrice(e.target.value)} />
        <button onClick={createProduct}>Create</button>
      </div>
      <div style={{marginBottom: '20px'}}>
        <h4>Get Product</h4>
        <input placeholder="Product ID" value={productId} onChange={e=>setProductId(e.target.value)} />
        <button onClick={getProduct}>Get</button>
      </div>
      <div style={{marginBottom: '20px'}}>
        <h4>Search Products</h4>
        <input placeholder="Search query" value={searchQ} onChange={e=>setSearchQ(e.target.value)} />
        <button onClick={searchProducts}>Search</button>
      </div>
      <div style={{marginBottom: '20px'}}>
        <h4>Update Price</h4>
        <input placeholder="Product ID" value={productId} onChange={e=>setProductId(e.target.value)} />
        <input type="number" placeholder="New Price" value={price} onChange={e=>setPrice(e.target.value)} />
        <button onClick={updatePrice}>Update</button>
      </div>
      <pre>{msg && JSON.stringify(msg,null,2)}</pre>
    </div>
  )
}

function Orders({token}) {
  const [orderId, setOrderId] = useState('')
  const [customerId, setCustomerId] = useState('')
  const [items, setItems] = useState('')
  const [status, setStatus] = useState('')
  const [msg, setMsg] = useState(null)

  const createOrder = async () => {
    try {
      const itemsArray = JSON.parse(items || '[]')
      const body = { customer_id: customerId, items: itemsArray }
      const r = await fetch('/orders', { method: 'POST', headers: {...getAuthHeaders(token), 'Content-Type':'application/json'}, body: JSON.stringify(body) })
      setMsg(await r.json())
    } catch(e) {
      setMsg({error: 'Invalid JSON in items'})
    }
  }

  const getOrder = async () => {
    const r = await fetch(`/orders?order_id=${encodeURIComponent(orderId)}`, { headers: getAuthHeaders(token) })
    setMsg(await r.json())
  }

  const cancelOrder = async () => {
    const body = { order_id: orderId }
    const r = await fetch('/orders/cancel', { method: 'POST', headers: {...getAuthHeaders(token), 'Content-Type':'application/json'}, body: JSON.stringify(body) })
    setMsg(await r.json())
  }

  const updateStatus = async () => {
    const body = { order_id: orderId, new_status: status }
    const r = await fetch('/orders/status', { method: 'POST', headers: {...getAuthHeaders(token), 'Content-Type':'application/json'}, body: JSON.stringify(body) })
    setMsg(await r.json())
  }

  const calculateTotal = async () => {
    try {
      const itemsArray = JSON.parse(items || '[]')
      const body = { items: itemsArray }
      const r = await fetch('/orders/calculate', { method: 'POST', headers: {...getAuthHeaders(token), 'Content-Type':'application/json'}, body: JSON.stringify(body) })
      setMsg(await r.json())
    } catch(e) {
      setMsg({error: 'Invalid JSON in items'})
    }
  }

  return (
    <div>
      <h3>Order Management</h3>
      <div style={{marginBottom: '20px'}}>
        <h4>Create Order</h4>
        <input placeholder="Customer ID" value={customerId} onChange={e=>setCustomerId(e.target.value)} />
        <textarea placeholder='Items JSON: [{"product_id":"...", "qty":1, "price":10}]' value={items} onChange={e=>setItems(e.target.value)} />
        <button onClick={createOrder}>Create</button>
      </div>
      <div style={{marginBottom: '20px'}}>
        <h4>Get Order</h4>
        <input placeholder="Order ID" value={orderId} onChange={e=>setOrderId(e.target.value)} />
        <button onClick={getOrder}>Get</button>
      </div>
      <div style={{marginBottom: '20px'}}>
        <h4>Update Status</h4>
        <input placeholder="Order ID" value={orderId} onChange={e=>setOrderId(e.target.value)} />
        <input placeholder="New Status" value={status} onChange={e=>setStatus(e.target.value)} />
        <button onClick={updateStatus}>Update</button>
      </div>
      <div style={{marginBottom: '20px'}}>
        <h4>Cancel Order</h4>
        <input placeholder="Order ID" value={orderId} onChange={e=>setOrderId(e.target.value)} />
        <button onClick={cancelOrder}>Cancel</button>
      </div>
      <div style={{marginBottom: '20px'}}>
        <h4>Calculate Total</h4>
        <textarea placeholder='Items JSON: [{"product_id":"...", "qty":1, "price":10}]' value={items} onChange={e=>setItems(e.target.value)} />
        <button onClick={calculateTotal}>Calculate</button>
      </div>
      <pre>{msg && JSON.stringify(msg,null,2)}</pre>
    </div>
  )
}

function Notifications({token}) {
  const [email, setEmail] = useState('')
  const [subject, setSubject] = useState('')
  const [body, setBody] = useState('')
  const [orderId, setOrderId] = useState('')
  const [sku, setSku] = useState('')
  const [warehouseId, setWarehouseId] = useState('')
  const [currentQty, setCurrentQty] = useState(0)
  const [threshold, setThreshold] = useState(0)
  const [msg, setMsg] = useState(null)

  const sendEmail = async () => {
    const reqBody = { to: email, subject, body }
    const r = await fetch('/notifications/email', { method: 'POST', headers: {...getAuthHeaders(token), 'Content-Type':'application/json'}, body: JSON.stringify(reqBody) })
    setMsg(await r.json())
  }

  const sendOrderConfirm = async () => {
    const reqBody = { order_id: orderId, customer_email: email, order_details: body }
    const r = await fetch('/notifications/order-confirmation', { method: 'POST', headers: {...getAuthHeaders(token), 'Content-Type':'application/json'}, body: JSON.stringify(reqBody) })
    setMsg(await r.json())
  }

  const sendStockAlert = async () => {
    const reqBody = { sku, warehouse_id: warehouseId, current_qty: Number(currentQty), threshold: Number(threshold) }
    const r = await fetch('/notifications/stock-alert', { method: 'POST', headers: {...getAuthHeaders(token), 'Content-Type':'application/json'}, body: JSON.stringify(reqBody) })
    setMsg(await r.json())
  }

  return (
    <div>
      <h3>Notifications</h3>
      <div style={{marginBottom: '20px'}}>
        <h4>Send Email</h4>
        <input placeholder="To Email" value={email} onChange={e=>setEmail(e.target.value)} />
        <input placeholder="Subject" value={subject} onChange={e=>setSubject(e.target.value)} />
        <textarea placeholder="Body" value={body} onChange={e=>setBody(e.target.value)} />
        <button onClick={sendEmail}>Send</button>
      </div>
      <div style={{marginBottom: '20px'}}>
        <h4>Send Order Confirmation</h4>
        <input placeholder="Order ID" value={orderId} onChange={e=>setOrderId(e.target.value)} />
        <input placeholder="Customer Email" value={email} onChange={e=>setEmail(e.target.value)} />
        <textarea placeholder="Order Details" value={body} onChange={e=>setBody(e.target.value)} />
        <button onClick={sendOrderConfirm}>Send</button>
      </div>
      <div style={{marginBottom: '20px'}}>
        <h4>Send Stock Alert</h4>
        <input placeholder="SKU" value={sku} onChange={e=>setSku(e.target.value)} />
        <input placeholder="Warehouse ID" value={warehouseId} onChange={e=>setWarehouseId(e.target.value)} />
        <input type="number" placeholder="Current Qty" value={currentQty} onChange={e=>setCurrentQty(e.target.value)} />
        <input type="number" placeholder="Threshold" value={threshold} onChange={e=>setThreshold(e.target.value)} />
        <button onClick={sendStockAlert}>Send Alert</button>
      </div>
      <pre>{msg && JSON.stringify(msg,null,2)}</pre>
    </div>
  )
}

export default App;
