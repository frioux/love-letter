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
  let canvasCtx: CanvasRenderingContext2D
  let loaded = false

  // Active pointer gesture.
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

  // Persist text and font size on change.
  $: if (loaded) save(text, fontSize)

  function toBase64(bytes: Uint8Array): string {
    let s = ''
    for (const b of bytes) s += String.fromCharCode(b)
    return btoa(s)
  }

  function fromBase64(s: string): Uint8Array {
    return Uint8Array.from(atob(s), c => c.charCodeAt(0))
  }

  function save(t = text, s = fontSize) {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify({ text: t, fontSize: s, inverted, drawing: toBase64(drawing) }))
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

  function onDown(e: PointerEvent) {
    if (base || e.button > 0) return
    canvas.setPointerCapture(e.pointerId)
    const [x, y] = pos(e)
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

  function onMove(e: PointerEvent) {
    if (!base) return
    const [x, y] = pos(e)
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

  function onUp() {
    if (!base) return
    const before = base
    base = null
    commit(before)
  }

  function onKey(e: KeyboardEvent) {
    if (!(e.metaKey || e.ctrlKey) || e.key.toLowerCase() !== 'z') return
    const t = e.target as HTMLElement
    if (t.closest('textarea, input')) return
    e.preventDefault()
    if (e.shiftKey) redo()
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
  <canvas
    bind:this={canvas}
    width={W}
    height={H}
    on:pointerdown={onDown}
    on:pointermove={onMove}
    on:pointerup={onUp}
    on:pointercancel={onUp}
  ></canvas>

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
    <button disabled={!undoStack.length} on:click={undo}>Undo</button>
    <button disabled={!redoStack.length} on:click={redo}>Redo</button>
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
  canvas {
    width: 100%;
    aspect-ratio: 200 / 96;
    image-rendering: pixelated;
    touch-action: none;
    border: 2px solid currentColor;
    box-sizing: border-box;
    cursor: crosshair;
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
