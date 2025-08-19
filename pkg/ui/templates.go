package ui

// AgentUIHTML: trang dashboard của Agent tại /ui
type _ string

const AgentUIHTML = `<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <title>Agent Rooms Dashboard</title>
  <style>
    :root{ --bg:#0f172a; --card:#111827; --text:#e5e7eb; --muted:#9ca3af; --accent:#22d3ee; }
    body { font-family: ui-sans-serif,system-ui,-apple-system; background:var(--bg); color:var(--text); margin:0; }
    .wrap{ max-width:1200px; margin:32px auto; padding:16px; }
    h2 { margin: 8px 0 16px; }
    .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
    .card { background:var(--card); border:1px solid #1f2937; border-radius: 10px; padding: 12px; box-shadow: 0 6px 24px rgba(0,0,0,.25) }
    table { border-collapse: collapse; width: 100%; }
    th, td { border: 1px solid #374151; padding: 8px; font-size: 14px; }
    th { background: #1f2937; text-align: left; }
    #status { color: var(--muted); margin-bottom: 12px; }
    .mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
  </style>
</head>
<body>
  <div class="wrap">
    <h2>Agent Dashboard</h2>
    <div id="status"></div>

    <div class="grid">
      <div class="card">
        <h3>Open Tickets</h3>
        <table id="tblTickets"><thead><tr><th>Ticket ID</th><th>Player</th><th>Enqueue At</th><th>Status</th></tr></thead><tbody></tbody></table>
      </div>
      <div class="card">
        <h3>Opened Rooms</h3>
        <table id="tblOpened"><thead><tr><th>Room</th><th>Players</th><th>Created At</th><th>Status</th></tr></thead><tbody></tbody></table>
      </div>
      <div class="card">
        <h3>Fulfilled Rooms</h3>
        <table id="tblFulfilled"><thead><tr><th>Room</th><th>Players</th><th>Server</th><th>Alloc</th><th>Created At</th></tr></thead><tbody></tbody></table>
      </div>
      <div class="card">
        <h3>Dead Rooms</h3>
        <table id="tblDead"><thead><tr><th>Room</th><th>Players</th><th>Reason</th><th>Created At</th></tr></thead><tbody></tbody></table>
      </div>
    </div>
  </div>

<script>
(function(){
  const statusEl = document.getElementById('status');

  function ts2(t){ return t? new Date(t*1000).toLocaleTimeString(): '' }
  function renderTickets(items){
    const tb = document.querySelector('#tblTickets tbody'); tb.innerHTML='';
    (items||[]).forEach(it=>{
      const tr=document.createElement('tr');
      tr.innerHTML = '<td class="mono">'+(it.ticket_id||'')+'</td>'+
                     '<td>'+(it.player_id||'')+'</td>'+
                     '<td>'+ts2(it.enqueue_at_unix)+'</td>'+
                     '<td>'+(it.status||'')+'</td>';
      tb.appendChild(tr);
    });
  }
  function renderOpened(items){
    const tb = document.querySelector('#tblOpened tbody'); tb.innerHTML='';
    (items||[]).forEach(it=>{
      const players = (it.players||[]).join(', ');
      const tr=document.createElement('tr');
      tr.innerHTML = '<td class="mono">'+(it.room_id||it.RoomID||'')+'</td>'+
                     '<td>'+players+'</td>'+
                     '<td>'+ts2(it.created_at_unix||it.created_at||0)+'</td>'+
                     '<td>'+(it.status||'OPENED')+'</td>';
      tb.appendChild(tr);
    });
  }
  function renderFulfilled(items){
    const tb = document.querySelector('#tblFulfilled tbody'); tb.innerHTML='';
    (items||[]).forEach(it=>{
      const players = (it.players||[]).join(', ');
      const server = (it.server_ip&&it.port)? (it.server_ip+':'+it.port) : '';
      const tr=document.createElement('tr');
      tr.innerHTML = '<td class="mono">'+(it.room_id||'')+'</td>'+
                     '<td>'+players+'</td>'+
                     '<td>'+server+'</td>'+
                     '<td class="mono">'+(it.allocation_id||'')+'</td>'+
                     '<td>'+ts2(it.created_at_unix||it.created_at||0)+'</td>';
      tb.appendChild(tr);
    });
  }
  function renderDead(items){
    const tb = document.querySelector('#tblDead tbody'); tb.innerHTML='';
    (items||[]).forEach(it=>{
      const players = (it.players||[]).join(', ');
      const tr=document.createElement('tr');
      tr.innerHTML = '<td class="mono">'+(it.room_id||'')+'</td>'+
                     '<td>'+players+'</td>'+
                     '<td>'+(it.fail_reason||'')+'</td>'+
                     '<td>'+ts2(it.created_at_unix||it.created_at||0)+'</td>';
      tb.appendChild(tr);
    });
  }

  async function refresh(){
    try{
      statusEl.textContent = 'Refreshing...';
      const res = await fetch('/admin/overview');
      const data = await res.json();
      renderTickets(data.open_tickets||[]);
      renderOpened(data.opened_rooms||[]);
      renderFulfilled(data.fulfilled_rooms||[]);
      renderDead(data.dead_rooms||[]);
      statusEl.textContent = 'Last update: '+new Date().toLocaleTimeString();
    }catch(e){
      statusEl.textContent = 'Refresh failed: '+e.message;
    }
  }

  refresh();
  setInterval(refresh, 3000);
})();
</script>
</body>
</html>`

// ServerUIHTML: trang UI của game server. Dùng fmt.Sprintf(ServerUIHTML, roomID, port).
const ServerUIHTML = `<!doctype html>
<html><head><meta charset="utf-8"/><title>Server Room</title>
<style>
  :root{ --bg:#0f172a; --card:#111827; --text:#e5e7eb; --muted:#9ca3af; --accent:#22d3ee; }
  body{font-family: ui-sans-serif,system-ui,-apple-system; background:var(--bg); color:var(--text); margin:0;}
  .wrap{max-width:900px;margin:32px auto;padding:16px}
  .card{background:var(--card);border:1px solid #1f2937;border-radius:10px;padding:16px;margin-bottom:16px; box-shadow: 0 6px 24px rgba(0,0,0,.25)}
  h2,h3{margin:8px 0 12px}
  table{border-collapse:collapse;width:100%%}
  th,td{border:1px solid #374151;padding:8px}
  th{background:#1f2937;text-align:left}
  a{color:var(--accent)}
  #log{white-space:pre-line;font-family:ui-monospace, SFMono-Regular; background:#0b1220; border-radius:8px; padding:12px;}
  .summary{color:#93c5fd;margin-bottom:8px}
</style>
</head><body>
<div class="wrap">
  <div class="card">
    <h2>Room: %s</h2>
    <div id="server">Listening on :%s</div>
    <div class="summary" id="summary"></div>
  </div>
  <div class="card">
    <h3>Players</h3>
    <table><thead><tr><th>Player</th><th>State</th><th>Last Seen</th></tr></thead><tbody id="tbody"></tbody></table>
  </div>
  <div class="card">
    <h3>Log</h3>
    <pre id="log"></pre>
  </div>
</div>
<script>
async function load(){
  try{
    const res=await fetch('/players');
    const d=await res.json();
    const tb=document.getElementById('tbody');
    tb.innerHTML='';
    let connected=0, total=0;
    (d.players||[]).forEach(p=>{
      total++;
      if(p.state==='connected') connected++;
      const tr=document.createElement('tr');
      const t=p.last_seen_unix? new Date(p.last_seen_unix*1000).toLocaleTimeString():'';
      tr.innerHTML='<td>'+p.player_id+'</td><td>'+p.state+'</td><td>'+t+'</td>';
      tb.appendChild(tr);
    });
    document.getElementById('summary').textContent = 'Connected: '+connected+' / '+total;
  }catch(e){
    document.getElementById('log').textContent+='load failed: '+e+'\n';
  }
}
setInterval(load,2000); load();
</script>
</body></html>`

// WebClientHTML: trang client. Dùng fmt.Sprintf(WebClientHTML, defaultPlayerName, namesPoolJSON, agentBaseURL)
const WebClientHTML = `<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <title>Dummy Client</title>
  <style>
    :root{ --bg:#0f172a; --card:#111827; --text:#e5e7eb; --muted:#9ca3af; --accent:#22d3ee; }
    body { font-family: ui-sans-serif,system-ui,-apple-system; background:var(--bg); color:var(--text); margin:0; }
    .wrap{ max-width:800px; margin:32px auto; padding:16px; }
    .card{ background:var(--card); border:1px solid #1f2937; border-radius:10px; padding:16px; margin-bottom:16px; box-shadow:0 6px 24px rgba(0,0,0,.25)}
    input, button { padding: 10px 12px; margin: 6px 4px; border-radius:8px; border:1px solid #374151; background:#0b1220; color:var(--text); }
    button{ background:#0ea5e9; color:white; border:none; cursor:pointer }
    button:hover{ filter:brightness(1.1) }
    a{ color: var(--accent); }
    #log { white-space: pre-line; background:#0b1220; border-radius:10px; padding:12px; font-family: ui-monospace, SFMono-Regular; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="card">
      <h2>Dummy Web Client</h2>
      <div>
        <input id="playerId" placeholder="player id" />
        <button id="btnRand">Random Name</button>
      </div>
      <div style="margin: 12px 0;">
        <button id="btnJoin">Join</button>
        <button id="btnCancel" disabled>Cancel Ticket</button>
      </div>
      <div><b>Server:</b> <a id="server" href="#" target="_blank"></a></div>
    </div>

    <div class="card">
      <h3>Log</h3>
      <div id="log"></div>
    </div>
  </div>
<script>
(function(){
  const logEl = document.getElementById('log');
  const serverEl = document.getElementById('server');
  const btnJoin = document.getElementById('btnJoin');
  const btnCancel = document.getElementById('btnCancel');
  const AGENT = '%[3]s';
  let hbTimer = null;
  let currentTicketId = '';

  function log(msg){ const ts = new Date().toISOString(); logEl.textContent += '['+ts+'] '+msg+'\n'; logEl.scrollTop = logEl.scrollHeight; }
  const val = id => (document.getElementById(id).value||'').trim();

  async function apiGet(url){ const res = await fetch(url); if(!res.ok){ throw new Error(res.status + ': ' + await res.text()); } return res.json(); }
  async function apiPost(url, body){ const res = await fetch(url, { method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify(body||{}) }); if(!res.ok){ throw new Error(res.status + ': ' + await res.text()); } return res.json(); }

  function startHeartbeat(host, port, playerId){ if(hbTimer){ clearInterval(hbTimer); } const url = 'http://'+host+':'+port+'/heartbeat?player_id='+encodeURIComponent(playerId); hbTimer = setInterval(async function(){ try{ await fetch(url, { mode:'cors' }); log('heartbeat ok '+host+':'+port); }catch(e){ log('heartbeat failed: '+e.message); } }, 3000); }

  async function pollRoom(roomId, playerId){ log('Polling room info for '+roomId+' ...'); const deadline = Date.now()+120000; while(Date.now()<deadline){ try{ const info = await apiGet(AGENT+'/rooms/'+encodeURIComponent(roomId)); log('Room info: '+JSON.stringify(info)); 
    
    // Case 1: RoomState from Redis - start heartbeat immediately when FULFILLED
    if(info && info.status==='FULFILLED'){
      if(info.server_ip && (info.port > 0 || parseInt(info.port) > 0)){
        const url = 'http://'+info.server_ip+':'+info.port+'/';
        serverEl.textContent = url;
        serverEl.href = url;
        log('Server ready (Redis) '+url);
        startHeartbeat(info.server_ip, info.port, playerId);
        return;
      } else {
        log('FULFILLED but no server info yet, keep polling...');
      }
    }
    
    // Case 2: RoomInfo from Nomad (has host_ip, ports)
    if(info && info.host_ip && info.ports){
      const port = info.ports.http || Object.values(info.ports)[0];
      if(port){
        const url = 'http://'+info.host_ip+':'+port+'/';
        serverEl.textContent = url;
        serverEl.href = url;
        log('Server ready (Nomad) '+url);
        startHeartbeat(info.host_ip, port, playerId);
        return;
      }
    }
    
    if(info && info.status==='DEAD'){ log('Room DEAD: '+(info.fail_reason||'')); return }
    
  }catch(e){ log('pollRoom error: '+e.message); } await new Promise(r=>setTimeout(r,2000)); } log('Server info not ready yet'); }

  async function pollTicket(ticketId, playerId){ log('Polling ticket '+ticketId+' ...'); while(true){ try{ const st = await apiGet(AGENT+'/tickets/'+encodeURIComponent(ticketId)); if(st.status==='OPENED'){ await new Promise(r=>setTimeout(r,2000)); continue } if(st.status==='MATCHED' && st.room_id){ log('Matched room '+st.room_id); await pollRoom(st.room_id, playerId); break } if(st.status==='EXPIRED' || st.status==='REJECTED'){ log('Ticket '+st.status); btnCancel.disabled = true; currentTicketId=''; break } }catch(e){ log('ticket poll failed: '+e.message); break } }
  }

  async function join(){ const playerId = val('playerId'); if(!playerId){ log('Please input player id'); return } try{ const rs = await apiPost(AGENT+'/tickets', { player_id: playerId }); if(rs.status==='REJECTED'){ log('Ticket rejected'); return } currentTicketId = rs.ticket_id; log('Ticket OPENED '+currentTicketId); btnCancel.disabled = false; await pollTicket(currentTicketId, playerId); }catch(e){ log('join failed: '+e.message); }
  }

  async function cancelTicket(){ if(!currentTicketId){ return } try{ const rs = await apiPost(AGENT+'/tickets/'+encodeURIComponent(currentTicketId)+'/cancel', {}); log('Cancel result: '+(rs.status||'')); btnCancel.disabled = true; currentTicketId=''; }catch(e){ log('cancel failed: '+e.message); }
  }

  document.getElementById('btnRand').addEventListener('click', function(){ const pool=%[2]s; const pick = pool[Math.floor(Math.random()*pool.length)]+'-'+Math.floor(Math.random()*1000); document.getElementById('playerId').value = pick; });
  document.getElementById('playerId').value = '%[1]s';
  btnJoin.addEventListener('click', join);
  btnCancel.addEventListener('click', cancelTicket);
})();
</script>
</body>
</html>`
