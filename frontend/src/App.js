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
        { token ? <button onClick={onLogout}>Logout</button> : <button onClick={() => setView('login')}>Login</button> }
      </nav>
      <main className="main">
        {view === 'home' && <Home />}
        {view === 'inventory' && <Inventory token={token} />}
        {view === 'reserve' && <Reserve token={token} />}
        {view === 'admin' && <Admin token={token} />}
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

export default App;
