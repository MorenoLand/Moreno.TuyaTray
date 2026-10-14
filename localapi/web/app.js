(() => {
const $ = s => document.querySelector(s)
const H = window.HouseUtil
const S = {devs: [], layout: {showHouse: true, slots: {}, homeName: ''}, view: 'main', sceneTab: '', sel: '', filter: '', scene: '', custom: false, placing: false, sig: '', busy: false}
const svg = p => `<svg viewBox="0 0 24 24">${p}</svg>`
const IC = {
  home: svg('<path d="M4 11l8-7 8 7v9H4z"/><path d="M10 20v-6h4v6"/>'),
  back: svg('<path d="M15 5l-7 7 7 7"/>'),
  power: svg('<path d="M12 3v8"/><path d="M6.4 6.8a8 8 0 1 0 11.2 0"/>'),
  sliders: svg('<path d="M4 8h9M17 8h3M4 16h3M11 16h9"/><circle cx="15" cy="8" r="2"/><circle cx="9" cy="16" r="2"/>'),
  refresh: svg('<path d="M20 11a8 8 0 1 0-2.3 5.7"/><path d="M20 4v7h-7"/>'),
  bulb: svg('<path d="M8.5 14.5A6 6 0 1 1 15.5 14.5c-.9.8-1.5 1.8-1.5 3h-4c0-1.2-.6-2.2-1.5-3z"/><path d="M10 20.5h4"/>'),
  lamp: svg('<path d="M7 3h10l3 9H4z"/><path d="M12 12v8"/><path d="M8 21h8"/>'),
  plug: svg('<rect x="6" y="9" width="12" height="7" rx="3"/><path d="M9 9V4M15 9V4M12 16v4"/>'),
  spark: svg('<path d="M12 3l2.4 6.6L21 12l-6.6 2.4L12 21l-2.4-6.6L3 12l6.6-2.4z"/>'),
  sun: svg('<circle cx="12" cy="12" r="4"/><path d="M12 2v3M12 19v3M2 12h3M19 12h3M5 5l2 2M17 17l2 2M19 5l-2 2M7 17l-2 2"/>'),
  sofa: svg('<rect x="3" y="11" width="18" height="7" rx="3"/><path d="M6 11V8a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v3M6 18v2M18 18v2"/>'),
  film: svg('<rect x="5" y="3" width="14" height="18" rx="2"/><path d="M10 3v18M14 3v18M5 8h5M14 8h5M5 12h5M14 12h5M5 16h5M14 16h5"/>'),
  moon: svg('<path d="M20 14.5A8 8 0 1 1 9.5 4a6.5 6.5 0 0 0 10.5 10.5z"/>'),
  fan: svg('<circle cx="12" cy="12" r="1.6"/><path d="M12 10.4C10 6 12 3.5 14.5 4.5 16 5.2 15 8 12 10.4zM13.5 12.8C18 12 20 14 19 16.5 18.2 18 15.800 17 13.500 12.800zM10.5 12.800C9.500 17.500 6.500 18.500 5.200 16.500 4.300 15 6.500 13 10.500 12.800z"/>'),
  bed: svg('<path d="M3 18V6M3 14h18v4M21 14v-2a3 3 0 0 0-3-3h-7v5"/><circle cx="7" cy="11" r="1.6"/>'),
  kitchen: svg('<path d="M5 10h14v8a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2zM3 10h18M9 7h6M3 14h2M19 14h2"/>'),
  bath: svg('<path d="M3 12h18l-2 6H5zM6 12V6a2 2 0 0 1 3-1.700M7 18l-1 2M17 18l1 2"/>'),
  desk: svg('<path d="M3 8h18M5 8v12M19 8v12M11 12h6v4h-6z"/>'),
  plant: svg('<path d="M7 14h10l-1.500 6h-7zM12 14V4M12 13c-3 0-5-2-5-5M12 11c3 0 5-2 5-5"/>'),
  door: svg('<rect x="7" y="3" width="10" height="18" rx="1"/><path d="M4 21h16"/><circle cx="14" cy="12" r=".8"/>')
}
const SCENES = [['Bright', 'sun'], ['Cozy', 'sofa'], ['Movie', 'film'], ['Night', 'moon'], ['Away', 'door']]
const HUES = [0, 28, 52, 125, 180, 230, 300]
const FX = {
  Night: 'linear-gradient(135deg,#121a40,#78607a,#ecaa64)', Reading: 'linear-gradient(135deg,#f6e2c4,#fff8ec)', Cocktail: 'linear-gradient(135deg,#ee46be,#7846f0,#4682fa)',
  Leisure: 'linear-gradient(135deg,#3c6e96,#aa9696,#f2aa78)', Soft: 'linear-gradient(135deg,#ffcdba,#ffe8d6,#f0d6ec)', Blinking: 'conic-gradient(#ff3c46,#3cf064,#466eff,#ff3c46)',
  Rainbow: 'conic-gradient(#f33,#fd2,#3e5,#3de,#46f,#e3e,#f33)', Rave: 'conic-gradient(#ff2846 0 25%,#181424 0 50%,#ff2846 0 75%,#181424 0)', Nature: 'linear-gradient(135deg,#1e8246,#78c85a,#28965a)'
}
const EFFECTS = ['Night', 'Reading', 'Cocktail', 'Leisure', 'Soft', 'Blinking', 'Rainbow', 'Rave', 'Nature']
const lights = () => S.devs.filter(d => d.kind === 'light')
const esc = s => String(s).replace(/[&<>"]/g, c => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;'}[c]))
const clamp = (v, a, b) => Math.max(a, Math.min(b, v))
const rgb = c => `rgb(${c[0] | 0},${c[1] | 0},${c[2] | 0})`
let toastT = 0
function toast(msg) { let t = $('.toast'); if (!t) { t = document.createElement('div'); t.className = 'toast'; document.body.appendChild(t) } t.textContent = msg; clearTimeout(toastT); toastT = setTimeout(() => t.remove(), 3500) }
async function api(method, path, body) {
  const r = await fetch(path, {method, cache: 'no-store', headers: body ? {'Content-Type': 'application/json'} : {}, body: body ? JSON.stringify(body) : undefined})
  if (r.status === 204) return null
  const j = await r.json().catch(() => null)
  if (!r.ok) throw new Error((j && j.detail) || r.statusText)
  return j
}
const fire = (m, p, b) => api(m, p, b).catch(e => toast(e.message))
const throttle = (fn, ms) => { let t = 0, pend = null; return (...a) => { const n = Date.now(); if (n - t >= ms) { t = n; fn(...a) } else { pend = a; setTimeout(() => { if (pend) { t = Date.now(); fn(...pend); pend = null } }, ms - (n - t)) } } }
const send = {
  bright: throttle((id, v) => ids(id).forEach(i => fire('POST', `/devices/${i}/brightness`, {percent: v})), 80),
  temp: throttle((id, v) => ids(id).forEach(i => fire('POST', `/devices/${i}/temperature`, {percent: v})), 80),
  colour: throttle((id, h, s) => ids(id).forEach(i => fire('POST', `/devices/${i}/colour`, {hue: h, saturation: s})), 80)
}
const groupIds = () => lights().filter(d => d.online && (S.filter === '' || d.room === S.filter)).map(d => d.id)
const ids = id => id === '*' ? groupIds() : [id]
function groupDev() {
  const sc = groupIds().map(i => S.devs.find(d => d.id === i)).filter(Boolean)
  if (!sc.length) return null
  const lit = sc.filter(d => d.on), col = lit.filter(d => d.mode === 'colour' && d.supportsColour)
  const avg = k => lit.length ? Math.round(lit.reduce((a, d) => a + (d[k] || 0), 0) / lit.length) : 0
  return {id: '*', name: S.filter === '' ? 'All lights' : S.filter, room: `${sc.length} lights`, kind: 'light', online: true, on: lit.length > 0, mode: col.length * 2 > lit.length ? 'colour' : 'white', brightness: lit.length ? avg('brightness') : 100, temperature: avg('temperature'), hue: col.length ? col[0].hue : 30, saturation: col.length ? col[0].saturation : 100, supportsTemperature: sc.some(d => d.supportsTemperature), supportsColour: sc.some(d => d.supportsColour), supportsEffects: sc.some(d => d.supportsEffects), effect: ''}
}
const get = id => id === '*' ? groupDev() : S.devs.find(d => d.id === id)
function patch(id, f) { ids(id).forEach(i => { const d = S.devs.find(x => x.id === i); if (d) f(d) }); S.sig = '' }
const act = {
  power(id, on) { patch(id, d => d.on = on); ids(id).forEach(i => fire('POST', `/devices/${i}/power`, {on})); S.scene = ''; render() },
  all(on) { S.devs.forEach(d => { if (d.kind === 'light') d.on = on }); fire('POST', '/lights/power', {on}); S.scene = ''; render() },
  roomAll(room, on) { S.devs.forEach(d => { if (d.kind === 'light' && d.room === room) d.on = on }); fire('POST', `/rooms/${encodeURIComponent(room)}/power`, {on}); S.scene = ''; render() },
  scene(n) { S.scene = n; fire('POST', `/scenes/${n}`); setTimeout(poll, 400); render() }
}
function acc(d) { return d.mode === 'colour' && d.supportsColour ? rgb(H.hsv(d.hue, Math.max(d.saturation, 50) / 100, 1)) : '#d9944a' }
const BI = {bulb: IC.bulb, lamp: IC.lamp, strip: IC.spark, plug: IC.plug, tv: IC.film, sofa: IC.sofa, sun: IC.sun, moon: IC.moon, door: IC.door, home: IC.home, fan: IC.fan, bed: IC.bed, kitchen: IC.kitchen, bath: IC.bath, desk: IC.desk, plant: IC.plant}
function customIcon(key) {
  const v = (S.layout.icons || {})[key] || ''
  if (v.startsWith('b:') && BI[v.slice(2)]) return BI[v.slice(2)]
  if (v.startsWith('i:')) return `<img src="/icons/img/${esc(v.slice(2))}" alt="">`
  return ''
}
function iconFor(d) {
  const c = customIcon('d:' + d.id); if (c) return c
  const n = d.name.toLowerCase()
  if (d.kind === 'switch') return IC.plug
  if (/strip|tv|back|led|tree/.test(n)) return IC.spark
  if (/lamp|floor|desk/.test(n)) return IC.lamp
  return IC.bulb
}
const inScene = d => /^scene/.test(d.mode)
const sceneView = d => d.supportsEffects && (inScene(d) || S.sceneTab === d.id)
const sub = d => !d.online ? `Offline · ${d.room}` : !d.on ? `Off · ${d.room}` : d.kind === 'switch' ? `On · ${d.room}` : `${d.brightness}% · ${d.room}`
function header() {
  const l = lights(), on = l.filter(d => d.on && d.online).length, off = l.filter(d => !d.online).length
  $('#home').textContent = S.layout.homeName || 'My Home'
  $('#sub').innerHTML = `<i style="background:${on ? '#3cdc6e' : '#646c88'}"></i>${on ? `${on} of ${l.length} lights on` : 'No lights on'}${off ? ` <em>· ${off} offline</em>` : ''}`
  $('#back').innerHTML = S.view === 'main' ? IC.home : IC.back
  $('#refresh').innerHTML = IC.refresh
  $('#tools').innerHTML = IC.sliders
  $('#tools').classList.toggle('on', S.view === 'tools')
  $('#all').innerHTML = IC.power
  $('#all').classList.toggle('on', on > 0)
  const bg = l.filter(d => d.on && d.online)
  if (bg.length) {
    let r = 0, g = 0, b = 0, w = 0
    bg.forEach(d => { const c = H.lightColour(d); const k = .3 + .7 * d.brightness / 100; r += c[0] * k; g += c[1] * k; b += c[2] * k; w += k })
    const f = bg.length / Math.max(1, l.length), m = .3 + .4 * f
    document.body.style.setProperty('--bg1', rgb([20 + (r / w * .55 - 20) * m, 23 + (g / w * .42 - 23) * m, 60 + (b / w * .42 - 60) * m]))
  } else document.body.style.setProperty('--bg1', '#141738')
}
let house
function hero() {
  const show = S.layout.showHouse && lights().length > 0 && (S.view === 'main' || S.view === 'detail')
  const el = $('#hero')
  el.classList.toggle('none', !show)
  el.classList.toggle('detail', show && S.view === 'detail')
  $('#herohint').textContent = S.placing ? 'Tap a spot to place this light' : ''
  if (!house) house = new window.House($('#house'), {
    toggle: id => { const d = get(id); if (d && d.online) act.power(id, !d.on) },
    place: i => { const id = S.sel; S.layout.slots[id] = i; S.placing = false; fire('PUT', `/devices/${id}/slot`, {slot: i}); render() }
  })
  if (S.plan) house.setPlan(S.plan)
  house.set(S.devs, S.layout)
  house.select(S.view === 'detail' && S.sel !== '*' ? S.sel : '')
  house.place(S.placing && S.view === 'detail')
}
function mainView() {
  const l = lights(), rooms = [...new Set(S.devs.map(d => d.room))]
  const cnt = r => l.filter(d => d.on && d.online && (r === '' || d.room === r)).length
  const pills = ['', ...rooms].map(r => `<div class="pill ${S.filter === r ? 'sel' : ''}" data-act="filter" data-room="${esc(r)}">${r !== '' && customIcon('r:' + r) ? `<span class="pm">${customIcon('r:' + r)}</span>` : ''}${r === '' ? 'All' : esc(r)}${r !== '' && cnt(r) ? `<u>${cnt(r)}</u>` : ''}</div>`).join('')
  const scope = l.filter(d => S.filter === '' || d.room === S.filter)
  const onN = scope.filter(d => d.on && d.online).length
  let html = `<div class="scenes">${SCENES.map(([n, i]) => `<div class="chip ${S.scene === n ? 'sel' : ''}" data-act="scene" data-name="${n}">${IC[i]}<span>${n}</span></div>`).join('')}</div>`
  html += `<div class="pills" id="pills">${pills}</div>`
  html += `<div class="whole"><b>${S.filter === '' ? 'Whole home' : esc(S.filter)}</b><span>· ${onN} of ${scope.length} on</span><button class="sm" data-act="groupopen">Style</button><button class="sm" data-act="alloff">All off</button><button class="sm" data-act="allon">All on</button></div>`
  const groups = [], at = {}
  for (const d of S.devs) {
    if (S.filter !== '' && d.room !== S.filter) continue
    if (!(d.room in at)) { at[d.room] = groups.length; groups.push({room: d.room, devs: []}) }
    groups[at[d.room]].devs.push(d)
  }
  html += '<div class="rooms">'
  for (const g of groups) {
    const wide = groups.length < 2 || g.devs.length > 1 || g.devs[0].kind === 'switch'
    html += `<section class="room${wide ? ' wide' : ''}">${S.filter === '' ? `<div class="sect">${customIcon('r:' + g.room) ? `<span class="pm">${customIcon('r:' + g.room)}</span>` : ''}${esc(g.room)}</div>` : ''}<div class="grid">${g.devs.map(tileHtml).join('')}</div></section>`
  }
  return html + '</div>'
}
function tileHtml(d) {
  const on = d.on && d.online
  if (d.kind === 'switch') return `<div class="tile row ${on ? 'on' : 'off'} ${d.online ? '' : 'dead'}" style="--acc:#d9944a" data-id="${d.id}"><div class="ico" data-act="toggle">${iconFor(d)}</div><div class="tx"><b>${esc(d.name)}</b><span>${esc(sub(d))}</span></div><div class="pw" data-act="toggle">${IC.power}</div></div>`
  return `<div class="tile ${on ? 'on' : 'off'} ${d.online ? '' : 'dead'}" style="--acc:${acc(d)}" data-id="${d.id}" data-lt="1"><div class="ico" data-act="toggle">${iconFor(d)}</div><div class="tx"><b>${esc(d.name)}</b><span>${esc(sub(d))}</span></div><i class="bar"><u style="width:${d.brightness}%"></u></i></div>`
}
function detailView(d) {
  const on = d.on && d.online, a = H.lightColour(d)
  let h = `<div class="dhead"><div><h2>${esc(d.name)}</h2><p><span>${esc(d.room)} · ${!d.online ? 'Offline' : on ? 'On' : 'Off'}</span>${d.id === '*' ? '' : '<button class="sm" data-act="room">Change room</button>'}${d.kind === 'light' && S.layout.showHouse && d.id !== '*' ? `<button class="sm" data-act="place">${S.placing ? 'Cancel' : 'Place in house'}</button>` : ''}</p></div><div class="hbtn power dpow ${on ? 'on' : ''}" data-act="dpower">${IC.power}</div></div>`
  if (d.kind === 'switch') return h + `<div class="big"><b style="color:${on ? '#fff' : '#646c88'}">${on ? 'On' : 'Off'}</b></div>`
  h += `<div class="big"><b id="num" style="color:${d.on ? '#fff' : '#646c88'}">${d.on ? d.brightness : 0}</b><span>%</span>`
  const sv = sceneView(d)
  if (d.supportsColour || d.supportsEffects) h += `<div class="seg"><button class="${d.mode !== 'colour' && !sv ? 'sel' : ''}" data-act="white">White</button>${d.supportsColour ? `<button class="${d.mode === 'colour' && !sv ? 'sel' : ''}" data-act="colourmode">Colour</button>` : ''}${d.supportsEffects ? `<button class="${sv ? 'sel' : ''}" data-act="scenemode">Scene</button>` : ''}</div>`
  h += '</div>'
  if (sv) return h + `<div class="fx">${EFFECTS.map(n => `<button class="${inScene(d) && d.effect === n ? 'sel' : ''}" data-act="effect" data-name="${n}"><i style="background:${FX[n]}"></i><span>${n}</span></button>`).join('')}</div>`
  h += `<input type="range" id="sl-b" min="1" max="100" value="${Math.max(1, d.brightness)}" style="background:linear-gradient(90deg,#1e2236,${rgb(H.hsv(0, 0, 0).map((_, i) => a[i] + (255 - a[i]) * .35))})">`
  if (d.supportsTemperature && d.mode !== 'colour') h += `<div class="lbl">Temperature</div><input type="range" class="thin" id="sl-t" min="0" max="100" value="${d.temperature}" style="background:linear-gradient(90deg,#ffaa5a,#aacdff)">`
  if (d.supportsColour) {
    if (S.custom) {
      h += `<div class="lbl">Colour<button class="sm" data-act="presets">Presets</button></div>`
      h += `<input type="range" class="thin" id="sl-h" min="0" max="359" value="${d.hue}" style="background:linear-gradient(90deg,#f00,#ff0,#0f0,#0ff,#00f,#f0f,#f00)">`
      h += `<input type="range" class="thin" id="sl-s" min="0" max="100" value="${d.saturation}" style="background:linear-gradient(90deg,#fff,${rgb(H.hsv(d.hue, 1, 1))})">`
    } else {
      h += `<div class="lbl">Colour</div><div class="sw">${HUES.map(hu => `<button class="${d.mode === 'colour' && Math.abs(d.hue - hu) < 14 ? 'sel' : ''}" data-act="swatch" data-h="${hu}" style="background:${rgb(H.hsv(hu, 1, 1))}"></button>`).join('')}<button class="rb" data-act="custom"></button></div>`
    }
  }
  if (S.layout.showHouse) h += '<div class="hint">Scroll over the home to zoom, drag to pan, double-click to reset. Tap a glow to toggle that light.</div>'
  return h
}
function modalRoom(d) {
  const rooms = [...new Set(S.devs.map(x => x.room))]
  $('#modal').innerHTML = `<div class="card"><h3>Move to room</h3><p>${esc(d.name)}</p><div class="chips">${rooms.map(r => `<button class="${r === d.room ? 'sel' : ''}" data-r="${esc(r)}">${esc(r)}</button>`).join('')}<button data-r="">Reset to Tuya room</button></div><input type="text" id="nr" placeholder="New room name"><div class="acts"><button id="mc">Cancel</button><button class="pri" id="ma">Add</button></div></div>`
  $('#modal').classList.add('on')
  const close = () => $('#modal').classList.remove('on')
  const set = r => { patch(d.id, x => { if (r) x.room = r }); fire('PUT', `/devices/${d.id}/room`, {room: r}).then(() => poll()); close(); render() }
  $('#modal').querySelectorAll('[data-r]').forEach(b => b.onclick = () => set(b.dataset.r))
  $('#mc').onclick = close
  $('#ma').onclick = () => { const v = $('#nr').value.trim(); if (v) set(v) }
  $('#modal').onclick = e => { if (e.target.id === 'modal') close() }
}
function render() {
  if (S.view === 'detail' && !get(S.sel)) S.view = 'main'
  header(); hero()
  $('#app').classList.toggle('tools', S.view === 'tools')
  const el = $('#body')
  const html = S.view === 'detail' ? detailView(get(S.sel)) : S.view === 'tools' ? toolsView() : mainView()
  if (el.dataset.sig !== html) { el.dataset.sig = html; el.innerHTML = html; if (S.view !== el.dataset.v) { el.dataset.v = S.view; el.style.animation = 'none'; void el.offsetWidth; el.style.animation = '' } }
  if (S.view === 'detail') bindDetail()
  if (S.view === 'tools') bindTools()
}
function bindDetail() {
  const d = get(S.sel); if (!d) return
  const b = $('#sl-b'), t = $('#sl-t'), hh = $('#sl-h'), ss = $('#sl-s')
  const busy = (el, up) => { if (!el) return; el.onpointerdown = () => { S.busy = true }; el.onpointerup = () => { S.busy = false }; el.oninput = () => up(+el.value) }
  busy(b, v => { patch(d.id, x => { x.brightness = v; x.on = true }); const n = $('#num'); if (n) n.textContent = v; send.bright(d.id, v) })
  busy(t, v => { patch(d.id, x => { x.temperature = v; x.mode = 'white' }); send.temp(d.id, v) })
  busy(hh, v => { patch(d.id, x => { x.hue = v; x.mode = 'colour' }); if (ss) ss.style.background = `linear-gradient(90deg,#fff,${rgb(H.hsv(v, 1, 1))})`; send.colour(d.id, v, Math.max(+(ss ? ss.value : 100), 60)) })
  busy(ss, v => { patch(d.id, x => { x.saturation = v; x.mode = 'colour' }); send.colour(d.id, +hh.value, v) })
}
const DAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
const ACTIONS = [['on', 'Turn on'], ['off', 'Turn off'], ['bright:25', 'Dim to 25%'], ['bright:50', 'Dim to 50%'], ['bright:100', 'Full brightness'], ['scene:Cozy', 'Cozy scene'], ['scene:Movie', 'Movie scene'], ['scene:Night', 'Night scene'], ['scene:Bright', 'Bright scene'], ['scene:Away', 'Away (everything off)']].concat(EFFECTS.map(n => ['effect:' + n, n + ' effect']))
const AREAS = [['Whole screen', [0, 0, 1, 1]], ['Left half', [0, 0, .5, 1]], ['Right half', [.5, 0, 1, 1]], ['Top half', [0, 0, 1, .5]], ['Bottom half', [0, .5, 1, 1]], ['Centre', [.25, .25, .75, .75]]]
const T = {plans: null, planText: null, planMsg: '', rules: [], timer: {active: false, seconds: 0}, show: {state: {running: false}, config: {mode: 'screen', devices: [], region: [0, 0, 1, 1], split: false, gain: 100, max_bright: 100}}, edit: null, msg: '', sig: '', end: 0}
const two = n => String(n).padStart(2, '0')
const fmtT = at => { const [h, m] = at.split(':').map(Number); return S.layout.clock24 ? `${two(h)}:${two(m)}` : `${h % 12 || 12}:${two(m)} ${h >= 12 ? 'PM' : 'AM'}` }
const actLabel = a => (ACTIONS.find(x => x[0] === a) || [a, a])[1]
const tgtLabel = t => t === 'everything' ? 'Everything' : t === 'all' || !t ? 'All lights' : t.startsWith('room:') ? t.slice(5) : t.startsWith('device:') ? ((get(t.slice(7)) || {}).name || t) : t
const dayLabel = ds => { if (!ds || !ds.length || ds.length === 7) return 'Every day'; const s = [...ds].sort().join(); return s === '1,2,3,4,5' ? 'Weekdays' : s === '0,6' ? 'Weekends' : [...ds].sort().map(d => DAYS[d]).join(', ') }
const ruleLine = r => `${dayLabel(r.days)} · ${tgtLabel(r.target)} · ${actLabel(r.action)}`
const mmss = s => `${Math.floor(s / 60)}:${two(s % 60)}`
async function loadPlans() { try { T.plans = await api('GET', '/plans') } catch (e) { T.plans = null } }
async function loadTools() {
  try {
    if (!T.plans) await loadPlans()
    const [rules, timer, show] = await Promise.all([api('GET', '/schedules'), api('GET', '/timer'), api('GET', '/show')])
    const sig = JSON.stringify([rules, timer.active, show])
    T.timer = timer; T.end = Date.now() + timer.seconds * 1000
    if (sig === T.sig) return false
    show.config.devices = show.config.devices || []
    if (!show.config.region || show.config.region[2] - show.config.region[0] < .02) show.config.region = [0, 0, 1, 1]
    T.sig = sig; T.rules = rules; T.show = show
    return true
  } catch (e) { toast(e.message); return false }
}
setInterval(() => { const el = $('#tleft'); if (el && T.timer.active) el.textContent = mmss(Math.max(0, Math.round((T.end - Date.now()) / 1000))) }, 1000)
function toolsView() {
  if (T.edit) return ruleEditor()
  const t = T.timer, c = T.show.config, st = T.show.state
  let h = '<h2 class="ph">Automation</h2>'
  h += '<section class="panel"><div class="ptitle">Sleep timer</div>'
  h += t.active ? `<div class="bigt" id="tleft">${mmss(t.seconds)}</div><p class="note">Everything turns off when it reaches zero.</p><button class="sm" data-act="timercancel">Cancel timer</button>` : `<p class="note">Turn everything off in</p><div class="row">${[15, 30, 60, 120].map(m => `<button class="btn" data-act="timer" data-m="${m}">${m < 60 ? m + ' min' : m / 60 + ' h'}</button>`).join('')}</div>`
  h += '</section><section class="panel"><div class="ptitle">Schedules</div>'
  h += T.rules.map((r, i) => `<div class="rule ${r.enabled ? '' : 'off'}"><div class="rt" data-act="ruleedit" data-i="${i}">${fmtT(r.at)}</div><div class="rx" data-act="ruleedit" data-i="${i}"><b>${esc(r.name || actLabel(r.action))}</b><span>${esc(ruleLine(r))}</span></div><label class="tg"><input type="checkbox" data-act="ruletoggle" data-i="${i}" ${r.enabled ? 'checked' : ''}><i></i></label></div>`).join('')
  h += '<button class="btn pri" data-act="ruleadd">Add a schedule</button></section>'
  h += `<section class="panel"><div class="ptitle">Light show</div><div class="seg wide"><button class="${c.mode !== 'sound' ? 'sel' : ''}" data-act="showmode" data-v="screen">Screen colours</button><button class="${c.mode === 'sound' ? 'sel' : ''}" data-act="showmode" data-v="sound">PC sound</button></div>`
  h += `<p class="note">Lights to drive (network-connected bulbs only)</p><div class="chips">${lights().map(d => `<button class="${c.devices.includes(d.id) ? 'sel' : ''}" data-act="showlight" data-id2="${d.id}">${esc(d.name)}</button>`).join('')}</div>`
  if (c.mode !== 'sound') {
    const cur = AREAS.findIndex(a => a[1].every((v, i) => Math.abs(v - c.region[i]) < .001))
    h += `<p class="note">Screen area</p><div class="chips">${AREAS.map((a, i) => `<button class="${i === cur ? 'sel' : ''}" data-act="showarea" data-i="${i}">${a[0]}</button>`).join('')}</div>`
    h += `<label class="chk"><input type="checkbox" data-act="showsplit" ${c.split ? 'checked' : ''}> Left to right across lights</label>`
  } else h += `<p class="note">Sensitivity <b id="gv">${c.gain || 100}%</b></p><input type="range" class="thin" id="show-gain" min="20" max="400" value="${c.gain || 100}" style="background:linear-gradient(90deg,#1e2236,#78aaff)">`
  h += `<p class="note">Maximum brightness <b id="mv">${c.max_bright || 100}%</b></p><input type="range" class="thin" id="show-max" min="10" max="100" value="${c.max_bright || 100}" style="background:linear-gradient(90deg,#1e2236,#ffd696)">`
  h += st.running ? `<button class="btn" data-act="showstop">Stop light show</button><p class="note ok">Running · ${st.lights} lights${st.skipped ? ` · ${st.skipped} skipped, not reachable on the network` : ''}</p>` : `<button class="btn pri" data-act="showstart">Start light show</button>${T.msg || st.error ? `<p class="note bad">${esc(T.msg || st.error)}</p>` : ''}`
  const rms = [...new Set(S.devs.map(d => d.room))]
  const icRow = (key, label, sub) => { const cur = (S.layout.icons || {})[key] || ''; return `<div class="icrow"><div class="icp">${customIcon(key) || (key.startsWith('r:') ? BI.home : BI.bulb)}</div><div class="rx"><b>${esc(label)}</b><span>${esc(sub)}</span></div><select class="icsel" data-key="${esc(key)}"><option value="" ${cur === '' ? 'selected' : ''}>Default</option>${Object.keys(BI).map(n => `<option value="b:${n}" ${cur === 'b:' + n ? 'selected' : ''}>${n}</option>`).join('')}${cur.startsWith('i:') ? '<option value="i" selected>Custom image</option>' : ''}</select><label class="sm up">Image<input type="file" accept="image/*" class="icfile" data-key="${esc(key)}" hidden></label></div>` }
  h += `<section class="panel"><div class="ptitle">Icons</div>${rms.map(r => icRow('r:' + r, r, 'Room')).join('')}${S.devs.map(d => icRow('d:' + d.id, d.name, d.room)).join('')}</section>`
  const pl = T.plans
  h += `</section><section class="panel"><div class="ptitle">Home design</div>${pl ? `<div class="chips">${pl.plans.map(p => `<button class="${p.id === S.planId ? 'sel' : ''}" data-act="plan" data-p="${esc(p.id)}">${esc(p.name)}</button>`).join('')}</div>` : ''}<p class="note">Design your own: edit the JSON below and apply. Saved as house.json next to the settings.</p><button class="btn" data-act="planedit">Edit design JSON</button>${T.planText !== null ? `<textarea id="plan-json" spellcheck="false">${esc(T.planText)}</textarea><div class="row"><button class="btn" data-act="planclose">Close</button><button class="btn pri" data-act="planapply">Apply design</button></div>${T.planMsg ? `<p class="note bad">${esc(T.planMsg)}</p>` : ''}` : ''}</section>`
  h += `<section class="panel"><div class="ptitle">Display</div><label class="chk"><input type="checkbox" data-act="clock24" ${S.layout.clock24 ? 'checked' : ''}> Use 24-hour time</label></section>`
  return h
}
function ruleEditor() {
  const {r, i} = T.edit
  const [hh, mm] = r.at.split(':').map(Number), pm = hh >= 12, h12 = hh % 12 || 12
  const rooms = [...new Set(lights().map(d => d.room))]
  const targets = [['everything', 'Everything'], ['all', 'All lights']].concat(rooms.map(x => ['room:' + x, x]))
  if (!targets.some(t => t[0] === r.target)) targets.push([r.target, tgtLabel(r.target)])
  const hours = S.layout.clock24 ? Array.from({length: 24}, (_, k) => [k, two(k)]) : Array.from({length: 12}, (_, k) => [k + 1, String(k + 1)])
  const hv = S.layout.clock24 ? hh : h12
  const has = d => !r.days.length || r.days.includes(d)
  const mins = Array.from({length: 12}, (_, k) => k * 5).concat(mm % 5 ? [mm] : []).sort((a, b) => a - b)
  let h = `<h2 class="ph">${i >= 0 ? 'Edit schedule' : 'New schedule'}</h2><section class="panel">`
  h += `<div class="timesel"><select id="re-h">${hours.map(x => `<option value="${x[0]}" ${x[0] === hv ? 'selected' : ''}>${x[1]}</option>`).join('')}</select><b>:</b><select id="re-m">${mins.map(m => `<option value="${m}" ${m === mm ? 'selected' : ''}>${two(m)}</option>`).join('')}</select>${S.layout.clock24 ? '' : `<select id="re-p"><option ${pm ? '' : 'selected'}>AM</option><option ${pm ? 'selected' : ''}>PM</option></select>`}</div>`
  h += `<p class="note">Repeat</p><div class="days">${DAYS.map((d, k) => `<button class="${has(k) ? 'sel' : ''}" data-act="reday" data-d="${k}">${d[0]}</button>`).join('')}</div>`
  h += `<p class="note">Lights</p><select id="re-t">${targets.map(t => `<option value="${esc(t[0])}" ${t[0] === r.target ? 'selected' : ''}>${esc(t[1])}</option>`).join('')}</select>`
  h += `<p class="note">Do</p><select id="re-a">${ACTIONS.map(a => `<option value="${a[0]}" ${a[0] === r.action ? 'selected' : ''}>${a[1]}</option>`).join('')}</select>`
  h += `<p class="note">Name (optional)</p><input type="text" id="re-n" value="${esc(r.name || '')}" placeholder="Lights out"></section>`
  h += `<div class="row">${i >= 0 ? '<button class="btn" data-act="redel">Delete</button>' : '<button class="btn" data-act="recancel">Cancel</button>'}<button class="btn pri" data-act="resave">Save</button></div>`
  return h
}
function bindTools() {
  if (T.edit) {
    const r = T.edit.r
    const clock = () => { let h = +$('#re-h').value; if (!S.layout.clock24) h = h % 12 + ($('#re-p').value === 'PM' ? 12 : 0); r.at = `${two(h)}:${two(+$('#re-m').value)}` }
    ;['#re-h', '#re-m', '#re-p'].forEach(s => { const el = $(s); if (el) el.onchange = clock })
    $('#re-t').onchange = e => { r.target = e.target.value }
    $('#re-a').onchange = e => { r.action = e.target.value }
    $('#re-n').oninput = e => { r.name = e.target.value }
    return
  }
  document.querySelectorAll('.icsel').forEach(s => { s.onchange = async () => { if (s.value === 'i') return; await fire('PUT', '/icons', {key: s.dataset.key, value: s.value}); S.layout.icons = S.layout.icons || {}; if (s.value) S.layout.icons[s.dataset.key] = s.value; else delete S.layout.icons[s.dataset.key]; S.sig = ''; render() } })
  document.querySelectorAll('.icfile').forEach(f => { f.onchange = async () => { const file = f.files[0]; if (!file) return; try { const r = await fetch('/icons/image?key=' + encodeURIComponent(f.dataset.key), {method: 'POST', body: file}); const j = await r.json(); if (!r.ok) throw new Error(j.detail || r.statusText); S.layout.icons = S.layout.icons || {}; S.layout.icons[f.dataset.key] = j.value; S.sig = ''; render() } catch (e) { toast(e.message) } } })
  const gain = $('#show-gain'), mx = $('#show-max'), c = T.show.config
  if (gain) gain.oninput = () => { c.gain = +gain.value; $('#gv').textContent = c.gain + '%'; T.sig = '' }
  if (mx) mx.oninput = () => { c.max_bright = +mx.value; $('#mv').textContent = c.max_bright + '%'; T.sig = '' }
}
async function toolsAct(a, t) {
  const c = T.show.config, i = +t.dataset.i
  const save = async rules => { try { T.rules = await api('PUT', '/schedules', rules); T.edit = null; T.sig = '' } catch (e) { toast(e.message) } render() }
  switch (a) {
    case 'timer': await fire('POST', '/timer', {minutes: +t.dataset.m, target: 'everything', action: 'off'}); await loadTools(); render(); break
    case 'timercancel': await fire('DELETE', '/timer'); await loadTools(); render(); break
    case 'ruleadd': { const d = new Date(Date.now() + 36e5); T.edit = {i: -1, r: {enabled: true, at: `${two(d.getHours())}:00`, days: [], target: 'all', action: 'off', name: ''}}; render(); break }
    case 'ruleedit': T.edit = {i, r: JSON.parse(JSON.stringify(T.rules[i]))}; T.edit.r.days = T.edit.r.days || []; render(); break
    case 'ruletoggle': { const rules = JSON.parse(JSON.stringify(T.rules)); rules[i].enabled = t.checked; await save(rules); break }
    case 'reday': { const d = +t.dataset.d, r = T.edit.r, cur = r.days.length ? r.days : [0, 1, 2, 3, 4, 5, 6]; const nx = cur.includes(d) ? cur.filter(x => x !== d) : [...cur, d]; r.days = nx.length === 7 ? [] : nx; render(); break }
    case 'resave': { const rules = JSON.parse(JSON.stringify(T.rules)); T.edit.i >= 0 ? rules[T.edit.i] = T.edit.r : rules.push(T.edit.r); await save(rules); break }
    case 'redel': await save(T.rules.filter((_, k) => k !== T.edit.i)); break
    case 'recancel': T.edit = null; render(); break
    case 'showmode': c.mode = t.dataset.v; T.sig = ''; render(); break
    case 'showlight': { const id = t.dataset.id2; c.devices = c.devices.includes(id) ? c.devices.filter(x => x !== id) : [...c.devices, id]; T.sig = ''; render(); break }
    case 'showarea': c.region = AREAS[i][1]; T.sig = ''; render(); break
    case 'showsplit': c.split = t.checked; T.sig = ''; break
    case 'showstart': try { T.msg = ''; T.show = await api('POST', '/show/start', c) } catch (e) { T.msg = e.message } T.sig = ''; render(); break
    case 'showstop': await fire('POST', '/show/stop'); await loadTools(); render(); break
    case 'plan': try { const r = await api('PUT', '/plan', {preset: t.dataset.p}); S.plan = r.plan; S.planId = r.id; S.layout.plan = r.id } catch (e) { toast(e.message) } S.sig = ''; render(); break
    case 'planedit': try { const r = await fetch('/plan/custom', {cache: 'no-store'}); T.planText = await r.text(); T.planMsg = '' } catch (e) { toast(e.message) } render(); break
    case 'planclose': T.planText = null; render(); break
    case 'planapply': { const txt = $('#plan-json').value; try { const r = await fetch('/plan/custom', {method: 'PUT', headers: {'Content-Type': 'application/json'}, body: txt}); const j = await r.json(); if (!r.ok) throw new Error(j.detail || r.statusText); S.plan = j.plan; S.planId = j.id; S.layout.plan = j.id; T.planText = txt; T.planMsg = ''; T.plans = null; await loadPlans() } catch (e) { T.planMsg = e.message } S.sig = ''; render(); break }
    case 'clock24': S.layout.clock24 = t.checked; await fire('PUT', '/preferences', {clock24: t.checked}); S.sig = ''; render(); break
  }
}
document.addEventListener('click', e => {
  const t = e.target.closest('[data-act]'); if (!t) return
  if (S.suppress) { S.suppress = false; return }
  const a = t.dataset.act, tile = t.closest('[data-id]'), id = tile ? tile.dataset.id : S.sel, d = get(id)
  switch (a) {
    case 'filter': S.filter = t.dataset.room; render(); break
    case 'groupopen': S.sel = '*'; S.view = 'detail'; S.custom = false; S.placing = false; S.sceneTab = ''; window.scrollTo(0, 0); render(); break
    case 'scene': act.scene(t.dataset.name); break
    case 'allon': S.filter === '' ? act.all(true) : act.roomAll(S.filter, true); break
    case 'alloff': S.filter === '' ? act.all(false) : act.roomAll(S.filter, false); break
    case 'toggle': if (d && d.online) act.power(id, !d.on); break
    case 'dpower': if (d && d.online) act.power(d.id, !d.on); break
    case 'white': S.sceneTab = ''; patch(d.id, x => x.mode = 'white'); ids(d.id).forEach(i => fire('POST', `/devices/${i}/white`)); render(); break
    case 'scenemode': S.sceneTab = d.id; render(); break
    case 'effect': patch(d.id, x => { x.mode = 'scene'; x.effect = t.dataset.name; x.on = true }); ids(d.id).filter(i => (S.devs.find(x => x.id === i) || {}).supportsEffects).forEach(i => fire('POST', `/devices/${i}/effect`, {name: t.dataset.name})); render(); break
    case 'colourmode': S.sceneTab = ''; patch(d.id, x => { x.mode = 'colour'; x.saturation = Math.max(x.saturation, 100) }); ids(d.id).forEach(i => fire('POST', `/devices/${i}/colour`, {hue: d.hue, saturation: Math.max(d.saturation, 100)})); render(); break
    case 'swatch': patch(d.id, x => { x.mode = 'colour'; x.hue = +t.dataset.h; x.saturation = 100; x.on = true }); ids(d.id).forEach(i => fire('POST', `/devices/${i}/colour`, {hue: +t.dataset.h, saturation: 100})); render(); break
    case 'custom': S.custom = true; render(); break
    case 'presets': S.custom = false; render(); break
    case 'room': modalRoom(d); break
    case 'place': S.placing = !S.placing; render(); break
    default: toolsAct(a, t)
  }
})
document.addEventListener('click', e => {
  const tile = e.target.closest('.tile[data-id]')
  if (tile && !e.target.closest('[data-act]') && !S.suppress && S.view === 'main') { S.sel = tile.dataset.id; S.view = 'detail'; S.custom = false; S.placing = false; window.scrollTo(0, 0); render() }
  else if (S.suppress) S.suppress = false
})
$('#back').onclick = () => { if (S.view === 'tools' && T.edit) { T.edit = null; render(); return } if (S.view !== 'main') { S.view = 'main'; S.placing = false; render() } }
$('#tools').onclick = () => { S.view = S.view === 'tools' ? 'main' : 'tools'; T.edit = null; S.placing = false; window.scrollTo(0, 0); if (S.view === 'tools') loadTools().then(render); else render() }
$('#refresh').onclick = () => { fire('POST', '/refresh'); setTimeout(poll, 1500); toast('Refreshing devices') }
$('#all').onclick = () => { const on = lights().some(d => d.on && d.online); act.all(!on) }
let drag = null
document.addEventListener('pointerdown', e => {
  const tile = e.target.closest('.tile[data-lt]'), pills = e.target.closest('#pills')
  if (tile && !e.target.closest('.ico')) drag = {kind: 'dim', tile, x: e.clientX, y: e.clientY, moved: false}
  else if (pills) drag = {kind: 'pills', el: pills, x: e.clientX, sl: pills.scrollLeft, moved: false}
})
document.addEventListener('pointermove', e => {
  if (!drag) return
  if (!(e.buttons & 1)) { endDrag(false); return }
  if (drag.kind === 'pills') { const dx = e.clientX - drag.x; if (Math.abs(dx) > 5) { drag.moved = true; drag.el.scrollLeft = drag.sl - dx } return }
  const dx = e.clientX - drag.x, dy = e.clientY - drag.y
  if (!drag.moved) { if (Math.abs(dx) > 9 && Math.abs(dx) > Math.abs(dy)) { drag.moved = true; S.busy = true } else return }
  const r = drag.tile.getBoundingClientRect(), v = clamp(Math.round((e.clientX - r.left) / r.width * 100), 1, 100)
  const id = drag.tile.dataset.id
  patch(id, x => { x.brightness = v; x.on = true })
  drag.tile.classList.add('on'); drag.tile.querySelector('.bar u').style.width = v + '%'; drag.tile.querySelector('.tx span').textContent = `${v}% · ${get(id).room}`
  send.bright(id, v)
})
function endDrag(real) { if (real && drag && drag.moved) { S.suppress = true; setTimeout(() => { S.suppress = false }, 80) } S.busy = false; drag = null }
document.addEventListener('pointerup', () => endDrag(true))
document.addEventListener('pointercancel', () => endDrag(false))
window.addEventListener('blur', () => endDrag(false))
document.addEventListener('mouseleave', () => endDrag(false))
async function poll() {
  if (S.busy) return
  try {
    const [d, l] = await Promise.all([api('GET', '/devices'), api('GET', '/layout')])
    if (l.plan !== S.planId) { const r = await api('GET', '/plan'); S.plan = r.plan; S.planId = r.id; S.sig = ''; T.plans = null }
    const sig = JSON.stringify([d, l])
    if (sig !== S.sig) { S.sig = sig; S.devs = d; S.layout = l; render() }
    if (S.view === 'tools' && (await loadTools()) && !T.edit) render()
  } catch (e) { toast(e.message) }
}
let timer = setInterval(poll, 1500)
document.addEventListener('visibilitychange', () => { clearInterval(timer); if (!document.hidden) { poll(); timer = setInterval(poll, 1500) } })
poll()
})()
