<script lang="ts">
  import { onMount } from 'svelte'
  import { W, H, BYTES, pack, unpack } from './bitmap'
  import { renderText, SIZES } from './text'
  import { CLEAR, BLACK, WHITE, line, rect, floodRegion } from './draw'

  type Tool = 'pencil' | 'line' | 'rect' | 'fill' | 'eraser'
  const tools: [Tool, string][] = [
    ['pencil', 'Pencil'], ['line', 'Line'], ['rect', 'Rectangle'], ['fill', 'Fill'], ['eraser', 'Eraser'],
  ]
  const STORAGE_KEY = 'love-letter'
  const MAX_HISTORY = 50
  const MAX_ZOOM = 16

  let text = ''
  let fontSize = 20
  let tool: Tool = 'pencil'
  let color: typeof BLACK | typeof WHITE = BLACK
  let brush = 1
  let drawing = new Uint8Array(W * H)
  let textBits = new Uint8Array(W * H).fill(1)
  // Inverted shows the text layer white-on-black; drawing colours are literal.
  let inverted = false
  type Snapshot = { drawing: Uint8Array; inverted: boolean }
  let undoStack: Snapshot[] = []
  let redoStack: Snapshot[] = []
  let status = ''
  let sending = false
  let canvas: HTMLCanvasElement
  // The viewport scrolls; the sheet inside it is the canvas scaled by zoom.
  let viewport: HTMLDivElement
  let sheet: HTMLDivElement
  let zoom = 1
  let canvasCtx: CanvasRenderingContext2D
  let loaded = false

  // Cursor mode: touches move a crosshair like a trackpad instead of drawing
  // under the finger, so the pixel being edited is never covered.
  let cursorMode = false
  let cursorX = W / 2 // fractional position; the pixel is the floor
  let cursorY = H / 2
  let penDown = false
  $: cx = Math.floor(cursorX)
  $: cy = Math.floor(cursorY)
  $: cursorSize = tool === 'fill' ? 1 : brush

  // Pointers currently down on the canvas, in client coordinates.
  const pointers = new Map<number, { x: number; y: number }>()
  // Set once a second finger lands, until every finger lifts.
  let pinched = false
  let downAt = 0
  let travel = 0

  // Active stroke.
  let base: Uint8Array | null = null
  let startX = 0
  let startY = 0
  let lastX = 0
  let lastY = 0

  function composite(): Uint8Array {
    const out = new Uint8Array(W * H)
    const flip = inverted ? 1 : 0
    for (let i = 0; i < out.length; i++) {
      const d = drawing[i]
      out[i] = d === CLEAR ? textBits[i] ^ flip : d === WHITE ? 1 : 0
    }
    return out
  }

  function paint() {
    if (!canvasCtx) return
    const comp = composite()
    const img = canvasCtx.createImageData(W, H)
    for (let i = 0; i < comp.length; i++) {
      const v = comp[i] ? 255 : 0
      img.data[i * 4] = img.data[i * 4 + 1] = img.data[i * 4 + 2] = v
      img.data[i * 4 + 3] = 255
    }
    canvasCtx.putImageData(img, 0, 0)
  }

  // Re-render text whenever its inputs change, then repaint.
  $: textBits = renderText(text, fontSize)
  $: textBits, paint()

  // Persist text, font size and cursor mode on change.
  $: if (loaded) save(text, fontSize, cursorMode)

  function toBase64(bytes: Uint8Array): string {
    let s = ''
    for (const b of bytes) s += String.fromCharCode(b)
    return btoa(s)
  }

  function fromBase64(s: string): Uint8Array {
    return Uint8Array.from(atob(s), c => c.charCodeAt(0))
  }

  function save(t = text, s = fontSize, c = cursorMode) {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify({ text: t, fontSize: s, cursorMode: c, inverted, drawing: toBase64(drawing) }))
    } catch {
      // Storage unavailable; work simply isn't persisted.
    }
  }

  function restore() {
    try {
      const raw = localStorage.getItem(STORAGE_KEY)
      if (!raw) return
      const s = JSON.parse(raw)
      text = typeof s.text === 'string' ? s.text : ''
      const n = Number(s.fontSize)
      if (SIZES.includes(n)) fontSize = n
      cursorMode = s.cursorMode === true
      const d = fromBase64(s.drawing)
      if (d.length === W * H) drawing = d
      inverted = s.inverted === true
    } catch {
      // Corrupt state; start fresh.
    }
  }

  function commit(before: Uint8Array, wasInverted = inverted) {
    undoStack.push({ drawing: before, inverted: wasInverted })
    if (undoStack.length > MAX_HISTORY) undoStack.shift()
    redoStack = []
    undoStack = undoStack
    redoStack = redoStack
    save()
    paint()
  }

  function undo() {
    const prev = undoStack.pop()
    if (!prev) return
    redoStack.push({ drawing, inverted })
    ;({ drawing, inverted } = prev)
    undoStack = undoStack
    redoStack = redoStack
    save()
    paint()
  }

  function redo() {
    const next = redoStack.pop()
    if (!next) return
    undoStack.push({ drawing, inverted })
    ;({ drawing, inverted } = next)
    undoStack = undoStack
    redoStack = redoStack
    save()
    paint()
  }

  function mutate(fn: () => void) {
    const before = drawing
    drawing = drawing.slice()
    fn()
    commit(before)
  }

  function clearDrawing() {
    mutate(() => drawing.fill(CLEAR))
  }

  function invert() {
    const before = drawing
    drawing = drawing.map(d => (d === BLACK ? WHITE : d === WHITE ? BLACK : CLEAR))
    inverted = !inverted
    commit(before, !inverted)
  }

  async function loadFromScreen() {
    try {
      const res = await fetch('/api/screen')
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const bytes = new Uint8Array(await res.arrayBuffer())
      if (bytes.length !== BYTES) throw new Error(`unexpected size ${bytes.length}`)
      const px = unpack(bytes)
      // White stays transparent so text typed afterwards shows through.
      const before = drawing
      const wasInverted = inverted
      drawing = px.map(v => (v ? CLEAR : BLACK))
      inverted = false
      text = ''
      commit(before, wasInverted)
      status = 'Loaded from screen.'
    } catch (e) {
      status = `Load failed: ${e instanceof Error ? e.message : e}`
    }
  }

  async function send() {
    sending = true
    status = 'Sending…'
    try {
      const res = await fetch('/api/screen', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/octet-stream' },
        body: pack(composite()),
      })
      if (!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`)
      status = 'Sent!'
    } catch (e) {
      status = `Send failed: ${e instanceof Error ? e.message : e}`
    } finally {
      sending = false
    }
  }

  function pos(e: PointerEvent): [number, number] {
    const r = canvas.getBoundingClientRect()
    const x = Math.floor(((e.clientX - r.left) / r.width) * W)
    const y = Math.floor(((e.clientY - r.top) / r.height) * H)
    return [Math.min(W - 1, Math.max(0, x)), Math.min(H - 1, Math.max(0, y))]
  }

  function plotter(): (x: number, y: number) => void {
    const v = tool === 'eraser' ? CLEAR : color
    return (x, y) => { drawing[y * W + x] = v }
  }

  function strokeStart(x: number, y: number) {
    if (base) return
    if (tool === 'fill') {
      const region = floodRegion(composite(), x, y)
      mutate(() => { for (const i of region) drawing[i] = color })
      return
    }
    base = drawing
    drawing = drawing.slice()
    startX = lastX = x
    startY = lastY = y
    if (tool === 'pencil' || tool === 'eraser') {
      line(x, y, x, y, brush, plotter())
      paint()
    }
  }

  function strokeMove(x: number, y: number) {
    if (!base) return
    if (tool === 'pencil' || tool === 'eraser') {
      line(lastX, lastY, x, y, brush, plotter())
      lastX = x
      lastY = y
    } else {
      drawing = base.slice()
      const shape = tool === 'line' ? line : rect
      shape(startX, startY, x, y, brush, plotter())
    }
    paint()
  }

  function strokeEnd() {
    if (!base) return
    const before = base
    base = null
    commit(before)
  }

  function strokeCancel() {
    if (!base) return
    drawing = base
    base = null
    paint()
  }

  // Sets the zoom, keeping the canvas point under the given client position
  // (default: the middle of the viewport) where it is.
  function setZoom(z: number, ax?: number, ay?: number) {
    const vr = viewport.getBoundingClientRect()
    ax ??= vr.left + vr.width / 2
    ay ??= vr.top + vr.height / 2
    const r = sheet.getBoundingClientRect()
    const fx = (ax - r.left) / r.width
    const fy = (ay - r.top) / r.height
    zoom = Math.min(MAX_ZOOM, Math.max(1, z))
    sheet.style.width = `${zoom * 100}%`
    const nr = sheet.getBoundingClientRect()
    viewport.scrollLeft += nr.left + fx * nr.width - ax
    viewport.scrollTop += nr.top + fy * nr.height - ay
  }

  // Steps to the next power of two; zooms around the cursor when it is shown.
  function zoomStep(dir: 1 | -1) {
    const l = Math.log2(zoom)
    const z = 2 ** (dir > 0 ? Math.floor(l) + 1 : Math.ceil(l) - 1)
    if (cursorMode) {
      const r = sheet.getBoundingClientRect()
      const s = r.width / W
      setZoom(z, r.left + (cx + 0.5) * s, r.top + (cy + 0.5) * s)
      keepCursorInView()
    } else {
      setZoom(z)
    }
  }

  // Scrolls the viewport so the cursor stays clear of its edges.
  function keepCursorInView() {
    const vr = viewport.getBoundingClientRect()
    const r = sheet.getBoundingClientRect()
    const s = r.width / W
    const m = Math.min(32, vr.width / 4, vr.height / 4)
    const px = r.left + (cx + 0.5) * s
    const py = r.top + (cy + 0.5) * s
    viewport.scrollLeft += Math.min(0, px - vr.left - m) + Math.max(0, px - vr.right + m)
    viewport.scrollTop += Math.min(0, py - vr.top - m) + Math.max(0, py - vr.bottom + m)
  }

  // Moves the cursor by a distance in canvas pixels.
  function moveCursor(dx: number, dy: number) {
    cursorX = Math.min(W - 0.01, Math.max(0, cursorX + dx))
    cursorY = Math.min(H - 0.01, Math.max(0, cursorY + dy))
    cx = Math.floor(cursorX)
    cy = Math.floor(cursorY)
    if (penDown) strokeMove(cx, cy)
    keepCursorInView()
  }

  function toggleCursor() {
    cursorMode = !cursorMode
    if (!cursorMode) return
    // Start in the middle of whatever is on show.
    const vr = viewport.getBoundingClientRect()
    const r = sheet.getBoundingClientRect()
    cursorX = ((vr.left + vr.width / 2 - r.left) / r.width) * W
    cursorY = ((vr.top + vr.height / 2 - r.top) / r.height) * H
    moveCursor(0, 0)
  }

  function penOn(e: PointerEvent) {
    if (penDown || e.button > 0) return
    ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
    penDown = true
    strokeStart(cx, cy)
  }

  function penOff() {
    if (!penDown) return
    penDown = false
    strokeEnd()
  }

  function onDown(e: PointerEvent) {
    if (e.button > 0) return
    canvas.setPointerCapture(e.pointerId)
    pointers.set(e.pointerId, { x: e.clientX, y: e.clientY })
    if (pointers.size === 2) {
      // A second finger means pinch/pan; drop whatever the first one drew.
      if (!penDown) strokeCancel()
      pinched = true
    }
    if (pinched || pointers.size > 1) return
    if (cursorMode && e.pointerType === 'touch') {
      downAt = e.timeStamp
      travel = 0
      return
    }
    strokeStart(...pos(e))
  }

  function onMove(e: PointerEvent) {
    const p = pointers.get(e.pointerId)
    if (!p) return
    const dx = e.clientX - p.x
    const dy = e.clientY - p.y
    if (pointers.size === 2) {
      const span = () => {
        const [a, b] = [...pointers.values()]
        return { d: Math.hypot(a.x - b.x, a.y - b.y), x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 }
      }
      const from = span()
      p.x = e.clientX
      p.y = e.clientY
      const to = span()
      if (from.d > 0) setZoom((zoom * to.d) / from.d, from.x, from.y)
      viewport.scrollLeft -= to.x - from.x
      viewport.scrollTop -= to.y - from.y
      return
    }
    p.x = e.clientX
    p.y = e.clientY
    if (pinched || pointers.size > 1) return
    if (cursorMode && e.pointerType === 'touch') {
      travel += Math.abs(dx) + Math.abs(dy)
      const s = sheet.getBoundingClientRect().width / W
      moveCursor(dx / s, dy / s)
      return
    }
    strokeMove(...pos(e))
  }

  function onUp(e: PointerEvent) {
    if (!pointers.delete(e.pointerId)) return
    if (pinched) {
      if (!pointers.size) pinched = false
      return
    }
    if (cursorMode && e.pointerType === 'touch') {
      // A tap plots at the cursor, like a trackpad click.
      if (e.type === 'pointerup' && !penDown && travel < 10 && e.timeStamp - downAt < 300) {
        strokeStart(cx, cy)
        strokeEnd()
      }
      return
    }
    strokeEnd()
  }

  function onKey(e: KeyboardEvent) {
    if (!(e.metaKey || e.ctrlKey) || e.altKey) return
    const k = e.key.toLowerCase()
    if (k !== 'z' && k !== 'y') return
    const t = e.target as HTMLElement
    if (t.closest('textarea, input')) return
    e.preventDefault()
    if (k === 'y' || e.shiftKey) redo()
    else undo()
  }

  onMount(() => {
    canvasCtx = canvas.getContext('2d')!
    restore()
    loaded = true
    paint()
  })
</script>

<svelte:window on:keydown={onKey} />

<div class="editor">
  <div class="stage">
    <div class="frame">
      <div class="viewport" class:zoomed={zoom > 1} bind:this={viewport}>
        <div class="sheet" bind:this={sheet}>
          <canvas
            bind:this={canvas}
            width={W}
            height={H}
            on:pointerdown={onDown}
            on:pointermove={onMove}
            on:pointerup={onUp}
            on:pointercancel={onUp}
          ></canvas>
          {#if cursorMode}
            <div
              class="cursor"
              style="left: {((cx - (cursorSize >> 1)) / W) * 100}%; top: {((cy - (cursorSize >> 1)) / H) * 100}%; width: {(cursorSize / W) * 100}%; height: {(cursorSize / H) * 100}%"
            ></div>
          {/if}
        </div>
      </div>
    </div>

    <div class="row">
      <button disabled={!undoStack.length} on:click={undo}>Undo</button>
      <button disabled={!redoStack.length} on:click={redo}>Redo</button>
      <span class="sep"></span>
      <button aria-label="Zoom out" disabled={zoom <= 1} on:click={() => zoomStep(-1)}>−</button>
      <button title="Reset zoom" disabled={zoom <= 1} on:click={() => setZoom(1)}>{Math.round(zoom * 10) / 10}×</button>
      <button aria-label="Zoom in" disabled={zoom >= MAX_ZOOM} on:click={() => zoomStep(1)}>+</button>
      <span class="sep"></span>
      <button class:active={cursorMode} aria-pressed={cursorMode} on:click={toggleCursor}>Cursor</button>
    </div>

    {#if cursorMode}
      <button
        class="draw"
        class:active={penDown}
        on:pointerdown={penOn}
        on:pointerup={penOff}
        on:pointercancel={penOff}
        on:contextmenu|preventDefault
      >Hold to draw · or tap the canvas for a dot</button>
    {/if}
  </div>

  <div class="row">
    {#each tools as [t, label]}
      <button class:active={tool === t} aria-pressed={tool === t} on:click={() => (tool = t)}>{label}</button>
    {/each}
  </div>

  <div class="row">
    <button class:active={color === BLACK} aria-pressed={color === BLACK} on:click={() => (color = BLACK)}>Black</button>
    <button class:active={color === WHITE} aria-pressed={color === WHITE} on:click={() => (color = WHITE)}>White</button>
    <span class="sep"></span>
    {#each [1, 2, 3, 4] as n}
      <button class:active={brush === n} aria-pressed={brush === n} on:click={() => (brush = n)}>{n}px</button>
    {/each}
  </div>

  <textarea bind:value={text} rows="3" placeholder="Type something…"></textarea>

  <div class="row">
    <label>Font size
      <select bind:value={fontSize}>
        {#each SIZES as n}<option value={n}>{n}px</option>{/each}
      </select>
    </label>
  </div>

  <div class="row">
    <button on:click={clearDrawing}>Clear drawing</button>
    <button on:click={invert}>Invert</button>
    <button on:click={loadFromScreen}>Load from screen</button>
  </div>

  <div class="row">
    <button class="send" disabled={sending} on:click={send}>{sending ? 'Sending…' : 'Send to screen'}</button>
    <span class="status" role="status">{status}</span>
  </div>
</div>

<style>
  .editor {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    max-width: 800px;
    margin: 0 auto;
  }
  .stage {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  /* Keep the canvas, undo and zoom on screen while the other tools scroll. */
  @media (min-height: 600px) {
    .stage {
      position: sticky;
      top: 0;
      z-index: 1;
      padding: 0.5rem 0;
      background-color: var(--bg);
    }
  }
  .frame {
    border: 2px solid currentColor;
  }
  .viewport {
    aspect-ratio: 200 / 96;
    overflow: hidden;
  }
  .viewport.zoomed {
    overflow: auto;
  }
  .sheet {
    position: relative;
    overflow: hidden;
  }
  canvas {
    display: block;
    width: 100%;
    aspect-ratio: 200 / 96;
    image-rendering: pixelated;
    touch-action: none;
    cursor: crosshair;
  }
  /* Outlines the brush footprint, with arms so it is findable at low zoom. */
  .cursor {
    --c: #ff2d95;
    position: absolute;
    pointer-events: none;
    outline: 2px solid var(--c);
  }
  .cursor::before,
  .cursor::after {
    content: '';
    position: absolute;
    left: 50%;
    top: 50%;
    width: 2px;
    height: 48px;
    background: linear-gradient(var(--c) 0 14px, transparent 14px 34px, var(--c) 34px);
    transform: translate(-50%, -50%);
  }
  .cursor::after {
    transform: translate(-50%, -50%) rotate(90deg);
  }
  .draw {
    padding: 1em;
    touch-action: none;
    user-select: none;
    -webkit-user-select: none;
    -webkit-touch-callout: none;
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
    align-items: center;
    justify-content: center;
  }
  .row button {
    padding: 0.4em 0.8em;
  }
  button.active {
    border-color: #646cff;
    background-color: #646cff;
    color: #fff;
  }
  button:disabled {
    opacity: 0.5;
    cursor: default;
  }
  .sep {
    width: 0.6rem;
  }
  textarea {
    font: inherit;
    width: 100%;
    box-sizing: border-box;
    padding: 0.5em;
  }
  select {
    font: inherit;
  }
  .status {
    min-height: 1.5em;
  }
</style>
