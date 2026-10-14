(() => {
const ORDER = ['loft', 'bed', 'bath', 'kitchen', 'dining', 'hall', 'living']
const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v))
const mix = (a, b, t) => [a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t, a[2] + (b[2] - a[2]) * t]
const shade = (c, k) => c.map(v => Math.min(255, v * k))
const css = (c, a) => `rgba(${c[0] | 0},${c[1] | 0},${c[2] | 0},${a ?? c[3] ?? 1})`
function hsv(h, s, v) {
  h = ((h % 360) + 360) % 360
  const c = v * s, x = c * (1 - Math.abs((h / 60) % 2 - 1)), m = v - c
  const [r, g, b] = h < 60 ? [c, x, 0] : h < 120 ? [x, c, 0] : h < 180 ? [0, c, x] : h < 240 ? [0, x, c] : h < 300 ? [x, 0, c] : [c, 0, x]
  return [(r + m) * 255, (g + m) * 255, (b + m) * 255]
}
const tempColour = t => t < .5 ? mix([255, 170, 90], [255, 247, 235], t * 2) : mix([255, 247, 235], [170, 205, 255], (t - .5) * 2)
const lightColour = d => d.mode === 'colour' && d.supportsColour ? hsv(d.hue, Math.max(d.saturation, 40) / 100, 1) : tempColour((d.temperature || 0) / 100)
const BASE = 0.4
const NIGHT = [16, 22, 54]
const PIXEL_BUDGET = 5e6
const tagOf = (plan, s) => {
  s = (s || '').toLowerCase()
  const keys = Object.keys(plan.words || {}).sort((a, b) => { const ia = ORDER.indexOf(a), ib = ORDER.indexOf(b); return (ia < 0 ? 99 : ia) - (ib < 0 ? 99 : ib) || a.localeCompare(b) })
  for (const k of keys) for (const w of plan.words[k]) if (w && s.includes(w.toLowerCase())) return k
  return ''
}
const tagFor = (plan, d) => { const rt = tagOf(plan, d.room), nt = tagOf(plan, d.name); return rt && rt !== 'living' ? rt : (nt || rt) }
function assign(plan, lights, over) {
  const slots = plan.slots
  const used = new Array(slots.length).fill(false), out = {}
  const pick = tag => { for (let i = 0; i < slots.length; i++) if (!used[i] && ((tag === '' && slots[i].room !== plan.upper) || slots[i].room === tag)) return i; return -1 }
  for (const d of lights) { const i = over && over[d.id]; if (Number.isInteger(i) && i >= 0 && i < slots.length && !used[i]) { used[i] = true; out[d.id] = i } }
  for (const d of lights) { if (d.id in out) continue; let i = pick(tagFor(plan, d)); if (i < 0) i = pick(''); if (i >= 0) { used[i] = true; out[d.id] = i } }
  return out
}
const roomsOf = plan => {
  const seen = new Set(), out = []
  const add = r => { if (r && !seen.has(r)) { seen.add(r); out.push(r) } }
  plan.slots.forEach(s => add(s.room)); plan.floors.forEach(f => add(f.room)); plan.walls.forEach(w => add(w.room))
  plan.items.forEach(it => it.p.forEach(p => add(p.room)))
  return out
}

class House {
  constructor(canvas, hooks) {
    this.cv = canvas; this.ctx = canvas.getContext('2d'); this.hooks = hooks
    this.plan = null; this.rooms = []; this.planId = 0
    this.devs = []; this.layout = {slots: {}}; this.sel = ''; this.placing = false; this.sig = ''
    this.cam = {z: 1, x: 0, y: 0}; this.e = {zt: 0, z: 1, x: 0, y: 0}; this.k = {}; this.gl = {}
    this.hits = []; this.slotHits = []; this.tf = {c: [0, 0], f: [0, 0], z: 1, pan: [0, 0]}
    this.cache = {c: document.createElement('canvas'), key: '', rs: 1}
    this.sprites = new Map()
    this.last = 0; this.raf = 0
    this.bind()
    new ResizeObserver(() => this.kick()).observe(canvas)
  }
  setPlan(plan) {
    if (!plan || plan === this.plan) return
    this.plan = plan; this.planId++; this.rooms = roomsOf(plan)
    this.k = {}; for (const r of this.rooms) this.k[r] = BASE
    this.cache.key = ''; this.sig = ''; this.kick()
  }
  set(devs, layout) {
    this.devs = devs.filter(d => d.kind === 'light'); this.layout = layout
    const sig = JSON.stringify([this.devs.map(d => [d.id, d.on, d.online, d.brightness, d.mode, d.hue, d.saturation, d.temperature, d.supportsColour, d.room, d.name]), layout.slots])
    if (sig !== this.sig) { this.sig = sig; this.kick() }
  }
  select(id) { if (id !== this.sel) { this.sel = id; if (!id) this.placing = false; this.kick() } }
  place(on) { if (on !== this.placing) { this.placing = on; this.kick() } }
  reset() { this.cam = {z: 1, x: 0, y: 0}; this.kick() }
  kick() { if (!this.raf && this.plan) { this.last = 0; this.raf = requestAnimationFrame(t => this.tick(t)) } }
  tick(t) {
    const dt = this.last ? clamp((t - this.last) / 1000, 0.001, 0.05) : 0.016; this.last = t; this.raf = 0
    let moving = false
    const ease = (cur, target, rate) => { const n = cur + (target - cur) * (1 - Math.exp(-dt * rate)); if (Math.abs(target - n) > 0.002) { moving = true; return n } return target }
    const slots = this.plan.slots
    const asg = assign(this.plan, this.devs, this.layout.slots)
    const kt = {}; for (const r of this.rooms) kt[r] = BASE
    for (const d of this.devs) { const i = asg[d.id]; if (i != null && d.on && d.online) { const r = slots[i].room; kt[r] = Math.max(kt[r], BASE + (1 - BASE) * (.62 + .38 * d.brightness / 100)) } }
    for (const r of this.rooms) this.k[r] = ease(this.k[r], kt[r], 5)
    for (const d of this.devs) this.gl[d.id] = ease(this.gl[d.id] || 0, (d.on && d.online ? 1 : 0) * (.45 + .55 * d.brightness / 100), 7)
    this.e.zt = ease(this.e.zt, this.sel && !this.placing && this.sel in asg ? 1 : 0, 4.5)
    this.e.z = ease(this.e.z, this.cam.z, 10); this.e.x = ease(this.e.x, this.cam.x, 10); this.e.y = ease(this.e.y, this.cam.y, 10)
    this.draw(asg)
    if (moving) this.kick()
  }
  sprite(col) {
    const key = ((col[0] >> 3) << 10) | ((col[1] >> 3) << 5) | (col[2] >> 3)
    let s = this.sprites.get(key)
    if (!s) {
      if (this.sprites.size > 80) this.sprites.clear()
      s = document.createElement('canvas'); s.width = s.height = 96
      const g = s.getContext('2d'), gr = g.createRadialGradient(48, 48, 0, 48, 48, 48)
      gr.addColorStop(0, css(col, 1)); gr.addColorStop(.4, css(col, .38)); gr.addColorStop(1, css(col, 0))
      g.fillStyle = gr; g.fillRect(0, 0, 96, 96)
      this.sprites.set(key, s)
    }
    return s
  }
  paintStatic(ctx, G) {
    const {at, k, kg, upper} = G, plan = this.plan
    const [W, D] = plan.size
    const col = (room, c) => mix(NIGHT, c, room ? (k[room] ?? BASE) : kg)
    const poly = (pts, c) => { ctx.beginPath(); pts.forEach((p, i) => i ? ctx.lineTo(p[0], p[1]) : ctx.moveTo(p[0], p[1])); ctx.closePath(); ctx.fillStyle = css(c); ctx.fill() }
    const quad = (A, B, C, E, c) => poly([at(...A), at(...B), at(...C), at(...E)], c)
    const floor = (x0, y0, x1, y1, zz, c) => quad([x0, y0, zz], [x1, y0, zz], [x1, y1, zz], [x0, y1, zz], c)
    const box = (x, y, zz, w, d, h, top, left, right) => {
      quad([x, y + d, zz], [x + w, y + d, zz], [x + w, y + d, zz + h], [x, y + d, zz + h], left)
      quad([x + w, y, zz], [x + w, y + d, zz], [x + w, y + d, zz + h], [x + w, y, zz + h], right)
      quad([x, y, zz + h], [x + w, y, zz + h], [x + w, y + d, zz + h], [x, y + d, zz + h], top)
    }
    const disc = (x, y, zz, r, c) => poly(Array.from({length: 16}, (_, i) => at(x + r * Math.cos(i * Math.PI / 8), y + r * Math.sin(i * Math.PI / 8), zz)), c)
    const post = (x, y, zz, r, h, c) => { for (let i = 0; i <= 3; i++) disc(x, y, zz + h * i / 3, r, shade(c, .78 + .07 * i)); disc(x, y, zz + h, r, shade(c, 1.15)) }
    const line = (x0, y0, z0, x1, y1, z1, w, c) => { const p = at(x0, y0, z0), q = at(x1, y1, z1); ctx.beginPath(); ctx.moveTo(p[0], p[1]); ctx.lineTo(q[0], q[1]); ctx.strokeStyle = css(c); ctx.lineWidth = w; ctx.stroke() }
    const lines = (x0, y0, x1, y1, zz, step, c, cols) => { if (cols) for (let x = x0 + step; x < x1 - .01; x += step) line(x, y0, zz, x, y1, zz, .6, c); for (let y = y0 + step; y < y1 - .01; y += step) line(x0, y, zz, x1, y, zz, .6, c) }
    const a = G.a
    { const g = ctx.createRadialGradient(G.ox, G.oy + 3.4 * a, 0, G.ox, G.oy + 3.4 * a, 7.5 * a); g.addColorStop(0, 'rgba(2,3,14,.55)'); g.addColorStop(1, 'rgba(2,3,14,0)'); ctx.fillStyle = g; ctx.fillRect(G.ox - 8 * a, G.oy + 1.2 * a, 16 * a, 5 * a) }
    box(-.14, -.14, -.4, W + .14, D + .14, .4, col('', [70, 64, 86]), col('', [44, 40, 60]), col('', [34, 30, 48]))
    for (const f of plan.floors) {
      const [x0, y0, x1, y1] = f.r, zz = f.z || 0
      floor(x0, y0, x1, y1, zz, col(f.room, f.c))
      const [kind, step] = (f.pat || '').split(':'), st = step && +step > .05 ? +step : .3
      if (kind === 'planks') lines(x0, y0, x1, y1, zz, st, [0, 0, 0, .15], false)
      else if (kind === 'tiles') lines(x0, y0, x1, y1, zz, st, [0, 0, 40, .17], true)
    }
    const wt = col('', [206, 198, 218])
    for (const w of plan.walls) {
      if (w.upper && !upper) continue
      const wc = col(w.room, w.side === 'R' ? [168, 154, 188] : [196, 182, 204])
      box(w.b[0], w.b[1], w.b[2], w.b[3], w.b[4], w.b[5], wt, wc, wc)
    }
    const glass = mix([22, 34, 82], [130, 160, 225], kg * .45)
    for (const w of plan.windows || []) {
      const m = (w.a + w.b) / 2
      if (w.side === 'L') { quad([0, w.a, w.z0], [0, w.b, w.z0], [0, w.b, w.z1], [0, w.a, w.z1], glass); line(0, m, w.z0, 0, m, w.z1, 1.2, wt) }
      else { quad([w.a, 0, w.z0], [w.b, 0, w.z0], [w.b, 0, w.z1], [w.a, 0, w.z1], glass); line(m, 0, w.z0, m, 0, w.z1, 1.2, wt) }
    }
    const drawPart = pt => {
      let base = col(pt.room, pt.c || [128, 128, 128])
      if (pt.sh) base = shade(base, pt.sh)
      switch (pt.t) {
        case 'solid': case 'box': {
          const f = [1.12, .86, .68]; (pt.f || []).forEach((v, i) => { f[i] = v })
          const top = pt.top ? col(pt.topg ? '' : pt.room, pt.top) : shade(base, f[0])
          const left = pt.left ? col(pt.room, pt.left) : shade(base, f[1])
          const right = pt.right ? col(pt.room, pt.right) : shade(base, f[2])
          box(pt.b[0], pt.b[1], pt.b[2], pt.b[3], pt.b[4], pt.b[5], top, left, right); break
        }
        case 'part': {
          const lit = Math.max(...(pt.rooms || []).map(r => k[r] ?? BASE)), pc = mix(NIGHT, [190, 180, 198], lit)
          box(pt.b[0], pt.b[1], 0, pt.b[2], pt.b[3], pt.h || 1.35, shade(pc, 1.06), shade(pc, .86), shade(pc, .66)); break
        }
        case 'disc': disc(pt.p[0], pt.p[1], pt.p[2], pt.r, base); break
        case 'post': post(pt.p[0], pt.p[1], pt.p[2], pt.r, pt.h, base); break
        case 'quad': quad(pt.pts[0], pt.pts[1], pt.pts[2], pt.pts[3], pt.screen ? mix(base, pt.screen, .5 * ((k[pt.room] ?? BASE) - .4)) : base); break
        case 'line': line(pt.a[0], pt.a[1], pt.a[2], pt.b[0], pt.b[1], pt.b[2], pt.w || 1.2, base); break
      }
    }
    plan.items.filter(it => !it.upper || upper)
      .map(it => ({key: it.k[0] + it.k[2] / 2 + it.k[1] + it.k[3] / 2, it}))
      .sort((p, q) => p.key - q.key)
      .forEach(o => o.it.p.forEach(drawPart))
  }
  draw(asg) {
    const cv = this.cv, plan = this.plan, slots = plan.slots, dpr = Math.min(window.devicePixelRatio || 1, 2.5), W = cv.clientWidth, H = cv.clientHeight
    if (!W || !H) return
    const cw = Math.round(W * dpr), ch = Math.round(H * dpr)
    if (cv.width !== cw || cv.height !== ch) { cv.width = cw; cv.height = ch }
    const ctx = this.ctx
    let upper = this.placing; if (plan.upper) for (const i of Object.values(asg)) upper = upper || slots[i].room === plan.upper
    upper = !!(upper && plan.upper)
    const extra = upper ? 1.44 : 0, [PW, PD, PH] = plan.size
    const ext = (PW + PD) / 2 + .9 * PH + .32, lift = .9 * PH
    const a = clamp(Math.min(W / (PW + PD + .4), (H - 8) / (ext + extra)), 12, 60)
    const ox = W / 2, oy = (H - (ext + extra) * a) / 2 + (lift + extra) * a
    const at = (x, y, z) => [ox + (x - y) * a, oy + (x + y) * a * .5 - z * a * .9]
    const kq = {}; let kg = 0
    for (const r of this.rooms) { kq[r] = Math.round(this.k[r] * 60) / 60; kg += kq[r] / this.rooms.length }
    const maxRs = Math.sqrt(PIXEL_BUDGET / (W * H))
    const need = Math.max(1, Math.ceil(this.cam.z * 2) / 2) * dpr
    let rs = this.cache.rs
    if (need > rs + 1e-6 || need < rs / 2 - 1e-6) rs = need
    rs = Math.min(rs, maxRs)
    const key = [this.planId, W, H, rs.toFixed(2), upper ? 1 : 0, ...this.rooms.map(r => Math.round(kq[r] * 60))].join('|')
    if (key !== this.cache.key) {
      const sc = this.cache.c
      sc.width = Math.round(W * rs); sc.height = Math.round(H * rs)
      const g = sc.getContext('2d')
      g.setTransform(rs, 0, 0, rs, 0, 0); g.clearRect(0, 0, W, H)
      this.paintStatic(g, {at, a, ox, oy, k: kq, kg, upper})
      this.cache.key = key; this.cache.rs = rs
    }
    const zt = this.e.zt, z = (1 + .95 * zt) * this.e.z
    const cc = [W / 2, H / 2]
    let F = cc
    if (this.sel && this.sel in asg) { const s = slots[asg[this.sel]], q = at(s.x, s.y, s.z); F = [cc[0] + (q[0] - cc[0]) * zt, cc[1] + (q[1] - cc[1]) * zt] }
    this.tf = {c: cc, f: F, z, pan: [this.e.x, this.e.y]}
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0); ctx.globalCompositeOperation = 'source-over'; ctx.globalAlpha = 1; ctx.clearRect(0, 0, W, H)
    ctx.save(); ctx.translate(cc[0] + this.e.x, cc[1] + this.e.y); ctx.scale(z, z); ctx.translate(-F[0], -F[1])
    ctx.drawImage(this.cache.c, 0, 0, W, H)
    this.hits = []
    const glows = []
    for (const d of this.devs) {
      const i = asg[d.id]; if (i == null) continue
      const s = slots[i], g = at(s.x, s.y, s.z), fl = at(s.x, s.y, 0), br = this.gl[d.id] || 0
      this.hits.push({id: d.id, x: g[0], y: g[1]})
      if (br > .02) glows.push({d, g, fl, br})
    }
    ctx.globalCompositeOperation = 'screen'
    const spr = (x, y, rx, ry, c, kk) => { if (kk < .01) return; ctx.globalAlpha = Math.min(1, kk); ctx.drawImage(this.sprite(c), x - rx, y - ry, rx * 2, ry * 2) }
    for (const {d, g, fl, br} of glows) {
      const c = lightColour(d), sp = d.mode === 'colour' && d.supportsColour ? .62 : 1, warm = mix(c, [255, 238, 205], .35)
      spr(fl[0], fl[1], a * 4.2 * sp, a * 2.2 * sp, c, .2 * br); spr(g[0], g[1], a * 3 * sp, a * 3 * sp, c, .16 * br * sp)
      spr(g[0], g[1], a * 1.5 * sp, a * 1.5 * sp, warm, .3 * br); spr(g[0], g[1], a * .55, a * .55, [255, 250, 236], .55 * br)
    }
    ctx.globalAlpha = 1; ctx.globalCompositeOperation = 'source-over'
    for (const d of this.devs) {
      const i = asg[d.id]; if (i == null) continue
      const s = slots[i], g = at(s.x, s.y, s.z), br = this.gl[d.id] || 0
      if (d.id === this.sel) { ctx.beginPath(); ctx.arc(g[0], g[1], 13, 0, 6.2832); ctx.strokeStyle = 'rgba(255,255,255,.85)'; ctx.lineWidth = 1.8; ctx.stroke() }
      if (br < .5) { ctx.beginPath(); ctx.arc(g[0], g[1], 2.2, 0, 6.2832); ctx.fillStyle = `rgba(150,156,190,${.43 * (1 - br * 2)})`; ctx.fill() }
    }
    this.slotHits = []
    if (this.placing) {
      slots.forEach((s, i) => {
        const g = at(s.x, s.y, s.z), cur = this.sel && asg[this.sel] === i
        ctx.beginPath(); ctx.arc(g[0], g[1], 9, 0, 6.2832); ctx.strokeStyle = cur ? '#ffbe5a' : 'rgba(255,255,255,.55)'; ctx.lineWidth = 1.6; ctx.stroke()
        ctx.beginPath(); ctx.arc(g[0], g[1], 3, 0, 6.2832); ctx.fillStyle = cur ? '#ffbe5a' : 'rgba(255,255,255,.5)'; ctx.fill()
        this.slotHits.push({i, x: g[0], y: g[1]})
      })
    }
    ctx.restore()
    ctx.globalCompositeOperation = 'destination-out'
    const fade = (y0, y1, a0, a1) => { const g = ctx.createLinearGradient(0, y0, 0, y1); g.addColorStop(0, `rgba(0,0,0,${a0})`); g.addColorStop(1, `rgba(0,0,0,${a1})`); ctx.fillStyle = g; ctx.fillRect(0, Math.min(y0, y1), W, Math.abs(y1 - y0)) }
    fade(0, H * .12, 1, 0); fade(H * .82, H, 0, 1)
    ctx.globalCompositeOperation = 'source-over'
  }
  toScene(px, py) { const t = this.tf; return [(px - (t.c[0] + t.pan[0])) / t.z + t.f[0], (py - (t.c[1] + t.pan[1])) / t.z + t.f[1]] }
  bind() {
    const cv = this.cv
    cv.addEventListener('wheel', e => {
      e.preventDefault()
      const dy = clamp(e.deltaMode === 1 ? e.deltaY * 33 : e.deltaY, -90, 90), zn = clamp(this.cam.z * Math.exp(-dy * .0022), 1, 3.4), r = zn / this.cam.z
      const rc = cv.getBoundingClientRect(), s = [e.clientX - rc.left - cv.clientWidth / 2, e.clientY - rc.top - cv.clientHeight / 2]
      this.cam.x = this.cam.x * r + s[0] * (1 - r); this.cam.y = this.cam.y * r + s[1] * (1 - r); this.cam.z = zn
      if (zn === 1) { this.cam.x = 0; this.cam.y = 0 }
      this.limit(); this.kick()
    }, {passive: false})
    let drag = null
    cv.addEventListener('pointerdown', e => { cv.setPointerCapture(e.pointerId); drag = {x: e.clientX, y: e.clientY, moved: false} })
    cv.addEventListener('pointermove', e => {
      if (!drag) return
      if (!(e.buttons & 1)) { drag = null; return }
      const dx = e.clientX - drag.x, dy = e.clientY - drag.y
      if (!drag.moved && Math.hypot(dx, dy) < 5) return
      drag.moved = true; this.cam.x += dx; this.cam.y += dy; drag.x = e.clientX; drag.y = e.clientY; this.limit(); this.kick()
    })
    cv.addEventListener('pointerup', e => {
      const d = drag; drag = null
      if (!d || d.moved) return
      const r = cv.getBoundingClientRect(), p = this.toScene(e.clientX - r.left, e.clientY - r.top)
      if (this.placing && this.slotHits.length) { for (const h of this.slotHits) if (Math.hypot(h.x - p[0], h.y - p[1]) < 16) { this.hooks.place(h.i); return } return }
      for (const h of this.hits) if (Math.hypot(h.x - p[0], h.y - p[1]) < 16) { this.hooks.toggle(h.id); return }
    })
    cv.addEventListener('pointercancel', () => { drag = null })
    cv.addEventListener('dblclick', () => this.reset())
  }
  limit() { const l = (this.cam.z - 1) * this.cv.clientWidth * .42; this.cam.x = clamp(this.cam.x, -l, l); this.cam.y = clamp(this.cam.y, -l * .8, l * .8) }
}
window.House = House
window.HouseUtil = {lightColour, hsv, tempColour}
})()
