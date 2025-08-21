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
        <h3>Actived Rooms</h3>
        <table id="tblActived"><thead><tr><th>Room</th><th>Players</th><th>Server</th><th>Alloc</th><th>Created At</th></tr></thead><tbody></tbody></table>
      </div>
      <div class="card">
        <h3>Fulfilled Rooms</h3>
        <table id="tblFulfilled"><thead><tr><th>Room</th><th>Players</th><th>Server</th><th>End Reason</th><th>Graceful At</th><th>Created At</th></tr></thead><tbody></tbody></table>
      </div>
      <div class="card">
        <h3>Dead Rooms</h3>
        <table id="tblDead"><thead><tr><th>Room</th><th>Players</th><th>Fail Reason</th><th>Dead At</th><th>Created At</th></tr></thead><tbody></tbody></table>
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
  function renderActived(items){
    const tb = document.querySelector('#tblActived tbody'); if(!tb) return; tb.innerHTML='';
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
  function renderFulfilled(items){
    const tb = document.querySelector('#tblFulfilled tbody'); tb.innerHTML='';
    (items||[]).forEach(it=>{
      const server = (it.server_ip&&it.port)? (it.server_ip+':'+it.port) : '';
      const tr=document.createElement('tr');
      // Cell players tách 2 dòng: winner và scores
      const winner = it.winner ? ('<div><b>Winner:</b> '+it.winner+'</div>') : '';
      let scores = '';
      if(it.scores){
        const parts = [];
        for (const k in it.scores){ parts.push(k+': '+it.scores[k]); }
        scores = '<div><b>Scores:</b> '+parts.join(', ')+'</div>';
      }
      const playersCell = winner + scores;
      tr.innerHTML = '<td class="mono">'+(it.room_id||'')+'</td>'+
                     '<td>'+(playersCell|| (it.players||[]).join(', '))+'</td>'+
                     '<td>'+server+'</td>'+
                     '<td>'+(it.end_reason||'')+'</td>'+
                     '<td>'+ts2(it.graceful_at_unix||it.graceful_at||0)+'</td>'+
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
                     '<td>'+ts2(it.dead_at_unix||it.dead_at||0)+'</td>'+
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
      renderActived(data.actived_rooms||[]);
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

// (đã loại bỏ) WebClientHTML do cmd/web bị xoá khỏi project
