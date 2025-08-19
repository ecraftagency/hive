package ui

// AgentUIHTML: trang dashboard của Agent tại /ui
type _ string

const AgentUIHTML = `<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <title>Agent Rooms Dashboard</title>
  <style>
    body { font-family: ui-sans-serif,system-ui,-apple-system; max-width: 1000px; margin: 24px auto; color:#111 }
    h2 { margin: 8px 0 16px; }
    .row { display: flex; gap: 16px; }
    .col { flex: 1; }
    table { border-collapse: collapse; width: 100%; }
    th, td { border: 1px solid #ddd; padding: 6px; font-size: 14px; }
    th { background: #f7f7f7; text-align: left; }
    #status { color: #555; margin-bottom: 10px; }
  </style>
</head>
<body>
  <h2>Agent Rooms Dashboard</h2>
  <div id="status"></div>
  <div class="row">
    <div class="col">
      <h3>Waiting</h3>
      <table id="tblWaiting"><thead><tr><th>Room</th><th>Player</th><th>Enqueue At</th></tr></thead><tbody></tbody></table>
    </div>
    <div class="col">
      <h3>Matched</h3>
      <table id="tblMatched"><thead><tr><th>Room</th><th>Players</th><th>Server</th><th>Alloc</th></tr></thead><tbody></tbody></table>
    </div>
  </div>
<script>
(function(){
  const statusEl = document.getElementById('status');
  function renderWaiting(items){
    const tbody = document.querySelector('#tblWaiting tbody');
    tbody.innerHTML = '';
    (items||[]).forEach(it => {
      const tr = document.createElement('tr');
      const t = (ts=> ts? new Date(ts*1000).toLocaleTimeString(): '')(it.enqueue_at_unix);
      tr.innerHTML = '<td>'+(it.room_name||'')+'</td><td>'+(it.player_id||'')+'</td><td>'+t+'</td>';
      tbody.appendChild(tr);
    });
  }
  function renderMatched(items){
    const tbody = document.querySelector('#tblMatched tbody');
    tbody.innerHTML = '';
    (items||[]).forEach(it => {
      const server = (it.server_ip&&it.port)? (it.server_ip+':'+it.port) : '';
      const players = (it.players||[]).join(', ');
      const tr = document.createElement('tr');
      tr.innerHTML = '<td>'+(it.room_id||'')+'</td><td>'+players+'</td><td>'+server+'</td><td>'+(it.allocation_id||'')+'</td>';
      tbody.appendChild(tr);
    });
  }
  async function refresh(){
    try {
      statusEl.textContent = 'Refreshing ...';
      const res = await fetch('/rooms');
      const data = await res.json();
      renderWaiting(data.waiting||[]);
      renderMatched(data.matched||[]);
      statusEl.textContent = 'Last update: '+new Date().toLocaleTimeString();
    } catch(e){
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

// WebClientHTML: trang client. Dùng fmt.Sprintf(WebClientHTML, defaultPlayerName)
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
        <input id="roomName" placeholder="room name" />
        <button id="btnCreate">Create Room</button>
        <button id="btnJoin">Join Room</button>
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
  let hbTimer = null;
  let currentHost='', currentPort=0;

  function log(msg){ const ts = new Date().toISOString(); logEl.textContent += '['+ts+'] '+msg+'\n'; logEl.scrollTop = logEl.scrollHeight; }
  const val = id => (document.getElementById(id).value||'').trim();

  async function apiGet(path){ const res = await fetch(path, { credentials: 'same-origin' }); if(!res.ok){ throw new Error(res.status + ': ' + await res.text()); } return res.json(); }

  // Heartbeat trực tiếp tới server
  function startHeartbeat(host, port, playerId){
    if(hbTimer){ clearInterval(hbTimer); }
    currentHost = host; currentPort = port;
    const url = 'http://'+host+':'+port+'/heartbeat?player_id='+encodeURIComponent(playerId);
    hbTimer = setInterval(async function(){
      try{
        await fetch(url, { mode: 'cors' });
        log('heartbeat ok '+host+':'+port);
      }catch(e){ log('heartbeat failed: '+e.message); }
    }, 3000);
  }

  async function pollRoom(roomId, playerId){
    log('Polling room info for '+roomId+' ...'); const deadline = Date.now()+120000;
    while(Date.now()<deadline){
      try{
        const info = await apiGet('/api/room_info?room_id='+encodeURIComponent(roomId));
        if(info && info.host_ip){
          const host = info.host_ip; let port = 0; if(info.ports){ port = info.ports.http || (Object.values(info.ports)[0] || 0); }
          if(host && port){ const url = 'http://'+host+':'+port+'/'; serverEl.textContent = url; serverEl.href = url; log('Server ready '+url); startHeartbeat(host, port, playerId); return; }
        }
      }catch(e){}
      await new Promise(function(r){ setTimeout(r,2000); });
    }
    log('Server info not ready yet');
  }

  async function createRoom(){ const roomName = val('roomName'); const playerId = val('playerId'); if(!roomName||!playerId){ log('Please input room name and player id'); return } log('Creating room '+roomName+' ...'); try{ const data = await apiGet('/api/create_room?room_name='+encodeURIComponent(roomName)+'&player_id='+encodeURIComponent(playerId)); if(data.error){ log('Create error: '+data.error); return } log('Room enqueued. Polling for server after match ...'); await pollRoom(roomName, playerId); }catch(e){ log('Create failed: '+e.message); } }

  async function joinRoom(){ const playerId = val('playerId'); if(!playerId){ log('Please input player id'); return } log('Calling /join_room ...'); try{ const jr = await apiGet('/api/join_room?player_id='+encodeURIComponent(playerId)); if(jr.error){ log('Waiting for match: '+jr.error); return } const roomId = jr.room_id||''; log('Matched room '+roomId); await pollRoom(roomId, playerId); }catch(e){ log('join_room failed: '+e.message); } }

  document.getElementById('btnCreate').addEventListener('click', createRoom);
  document.getElementById('btnJoin').addEventListener('click', joinRoom);
  document.getElementById('btnRand').addEventListener('click', function(){ const pool=%[2]s; const pick = pool[Math.floor(Math.random()*pool.length)]+'-'+Math.floor(Math.random()*1000); document.getElementById('playerId').value = pick; });
  document.getElementById('playerId').value = '%[1]s';
})();
</script>
</body>
</html>`
