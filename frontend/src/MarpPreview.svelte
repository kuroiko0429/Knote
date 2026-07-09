<script context="module" lang="ts">
  export function detectMarp(src: string): boolean {
    const m = src.match(/^---\r?\n([\s\S]*?)\r?\n---/)
    if (!m) return false
    return /^\s*marp\s*:\s*true\s*$/m.test(m[1])
  }
</script>

<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { GetMarpTheme, SetMarpTheme, GetMarpThemesDir, ListMarpCustomThemes, OpenPath } from '../wailsjs/go/main/App.js'

  export let source: string

  let isMarpFullscreen = false
  let marpTheme: 'default' | 'gaia' | 'uncover' = 'default'
  let marpSections: string[] = []
  let marpNotes: string[] = []
  let marpCss = ''
  let marpSlideIdx = 0
  let marpPresentUrl = ''
  let previewWidth = 0
  let marpScrollEl: HTMLDivElement | null = null
  let customMarpThemes: { name: string; css: string }[] = []
  let _lastMarpSrc = ''
  let _lastRawSrc = ''
  let _marpRenderTimer: ReturnType<typeof setTimeout>
  let _marpInstance: any = null
  let _loadedCustomThemeNames: string[] = []

  async function getMarp(): Promise<any> {
    if (!_marpInstance) {
      const { Marp } = await import('@marp-team/marp-core')
      _marpInstance = new Marp({ html: true })
    }
    for (const t of customMarpThemes) {
      if (!_loadedCustomThemeNames.includes(t.name)) {
        _marpInstance.themeSet.add(t.css)
        _loadedCustomThemeNames.push(t.name)
      }
    }
    return _marpInstance
  }

  async function reloadCustomThemes() {
    customMarpThemes = await ListMarpCustomThemes()
    _marpInstance = null
    _loadedCustomThemeNames = []
    _lastMarpSrc = ''
    renderMarpSlides(source)
  }

  function mountSection(node: HTMLElement, p: { html: string; css: string }) {
    const _so = '<' + 'style>'
    const _sc = '</' + 'style>'
    const render = (p: { html: string; css: string }) => {
      const shadow = node.shadowRoot ?? node.attachShadow({ mode: 'open' })
      const scopedCss = `:host{display:block;width:1280px;height:720px;overflow:hidden}` + p.css.replace(/:root\b/g, ':host')
      shadow.innerHTML = `${_so}${scopedCss}${_sc}${p.html}`
    }
    render(p)
    return { update: render }
  }

  function scrollToMarpSlide(idx: number) {
    marpSlideIdx = idx
    marpScrollEl?.querySelector(`[data-slide-idx="${idx}"]`)?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
  }

  let marpThumbH = Math.max(60, Math.min(320, parseInt(localStorage.getItem('marp-thumb-h') || '112')))
  let marpStripEl: HTMLDivElement | null = null
  $: marpThumbInnerH = Math.max(36, marpThumbH - 22)
  $: marpThumbInnerW = Math.round((marpThumbInnerH * 16) / 9)
  $: marpThumbScale = marpThumbInnerH / 720

  function startThumbResize(e: PointerEvent) {
    const y0 = e.clientY
    const h0 = marpThumbH
    const mv = (ev: PointerEvent) => { marpThumbH = Math.max(60, Math.min(320, h0 - (ev.clientY - y0))) }
    const up = () => {
      localStorage.setItem('marp-thumb-h', String(marpThumbH))
      window.removeEventListener('pointermove', mv)
      window.removeEventListener('pointerup', up)
    }
    window.addEventListener('pointermove', mv)
    window.addEventListener('pointerup', up)
    e.preventDefault()
  }

  let _slideIO: IntersectionObserver | null = null
  function observeSlide(node: HTMLElement) {
    if (_slideIO && (_slideIO.root as HTMLElement | null)?.isConnected === false) {
      _slideIO.disconnect()
      _slideIO = null
    }
    if (!_slideIO) {
      _slideIO = new IntersectionObserver((entries) => {
        let best: IntersectionObserverEntry | null = null
        for (const en of entries) if (en.isIntersecting && (!best || en.intersectionRatio > best.intersectionRatio)) best = en
        if (!best) return
        const idx = parseInt((best.target as HTMLElement).dataset.slideIdx || '0')
        if (idx === marpSlideIdx) return
        marpSlideIdx = idx
        const t = marpStripEl?.querySelector(`[data-thumb-idx="${idx}"]`) as HTMLElement | null
        if (t && marpStripEl) marpStripEl.scrollTo({ left: t.offsetLeft - marpStripEl.clientWidth / 2 + t.clientWidth / 2, behavior: 'smooth' })
      }, { root: node.parentElement, threshold: 0.5 })
    }
    _slideIO.observe(node)
    return { destroy: () => _slideIO?.unobserve(node) }
  }

  function changeMarpTheme(theme: 'default' | 'gaia' | 'uncover') {
    marpTheme = theme
    _lastMarpSrc = ''
    SetMarpTheme(theme)
    renderMarpSlides(source)
  }

  function applyMarpTheme(src: string, theme: string): string {
    const themeStr = `theme: ${theme}`
    const fm = src.match(/^(---\r?\n)([\s\S]*?)(\r?\n---)/)
    if (fm) {
      if (/^theme\s*:/m.test(fm[2])) {
        return src.replace(/^theme\s*:.*$/m, themeStr)
      }
      return src.replace(/^(---\r?\n)/, `$1${themeStr}\n`)
    }
    return `---\n${themeStr}\n---\n${src}`
  }

  async function renderMarpSlides(src: string): Promise<void> {
    if (src !== _lastRawSrc) {
      _lastRawSrc = src
      const fm = src.match(/^---\r?\n([\s\S]*?)\r?\n---/)
      if (fm) {
        const tm = fm[1].match(/^theme\s*:\s*(\S+)/m)
        if (tm && (tm[1] === 'default' || tm[1] === 'gaia' || tm[1] === 'uncover')) {
          marpTheme = tm[1] as 'default' | 'gaia' | 'uncover'
        }
      }
    }
    const cacheKey = src + '\x00' + marpTheme
    if (cacheKey === _lastMarpSrc && marpSections.length > 0) {
      return
    }
    _lastMarpSrc = cacheKey

    const marp = await getMarp()
    const srcWithTheme = applyMarpTheme(src, marpTheme)
    const { html: marpHtml, css: rawCss, comments } = marp.render(srcWithTheme)
    const css = rawCss
      .replace(/@import[^;]+;/g, '')
      .replace(/div\.marpit > svg(?:\[[^\]]*\])? > foreignObject(?:\[[^\]]*\])? > /g, '')
      .replace(/div\.marpit > /g, '')
    const tmp = document.createElement('div')
    tmp.innerHTML = marpHtml
    const sections = Array.from(tmp.querySelectorAll('section'))

    const so = '<' + 'style>'
    const sc = '</' + 'style>'

    const notes = ((comments ?? []) as string[][]).map((c) => c.map((t) => t.trim()).filter(Boolean).join('\n\n'))
    const slidesJson = JSON.stringify(sections.map((s) => s.outerHTML))
    const notesJson = JSON.stringify(notes)
    const pScriptSc = '</' + 'script>'
    const overrideCss =
      `section{display:flex!important;flex-direction:column!important;justify-content:center!important;` +
      `padding:80px 60px!important;font-size:28px!important}` +
      `section>header{position:absolute!important;top:0!important;left:0!important;right:0!important;` +
      `padding:18px 60px!important;color:#888!important;font-size:0.55em!important}` +
      `section>footer{position:absolute!important;bottom:0!important;left:0!important;right:0!important;` +
      `padding:18px 60px!important;color:#888!important;font-size:0.55em!important}`
    const presentHtml =
      `<!DOCTYPE html><html><head>` +
      `<meta charset="utf-8"><meta name="color-scheme" content="light">` +
      `${so}html,body{margin:0;padding:0;background:#000;overflow:hidden;width:100vw;height:100vh}` +
      `#wrap{position:fixed;top:50%;left:50%}` +
      `section{width:1280px!important;height:720px!important;box-sizing:border-box;position:relative;background:#fff;overflow:hidden;transform-origin:top left}` +
      `section>footer{position:absolute!important;bottom:0!important;left:0!important;right:0!important}` +
      `section table{border-collapse:collapse;width:100%}` +
      `section th,section td{border:1px solid #ccc;padding:6px 12px;text-align:left}` +
      `section th{background:#f0f0f0;font-weight:600}` +
      `.ctrl{position:fixed;bottom:24px;left:50%;transform:translateX(-50%);display:flex;gap:14px;align-items:center;` +
      `background:rgba(0,0,0,.5);padding:8px 18px;border-radius:20px;font-family:sans-serif;font-size:14px;color:rgba(255,255,255,.8)}` +
      `.ctrl button{background:none;border:1px solid rgba(255,255,255,.3);color:rgba(255,255,255,.8);padding:4px 10px;border-radius:4px;cursor:pointer;font-size:14px}` +
      `.ctrl button:hover{background:rgba(255,255,255,.15)}` +
      `.ctrl button:disabled{opacity:.3;cursor:default}` +
      `.xbtn{position:fixed;top:14px;right:14px;background:rgba(255,255,255,.1);border:1px solid rgba(255,255,255,.2);` +
      `color:#fff;padding:5px 12px;border-radius:6px;cursor:pointer;font-size:13px;font-family:sans-serif}` +
      `.xbtn:hover{background:rgba(255,255,255,.2)}` +
      `.notes{position:fixed;bottom:76px;left:50%;transform:translateX(-50%);max-width:860px;width:90%;` +
      `max-height:200px;overflow-y:auto;background:rgba(0,0,0,.82);color:#ccc;font-size:15px;font-family:sans-serif;` +
      `padding:12px 16px;border-radius:8px;border:1px solid rgba(255,255,255,.15);white-space:pre-wrap;` +
      `line-height:1.6;display:none;z-index:10;box-sizing:border-box}` +
      `.notes.show{display:block}` +
      `.nkey{position:fixed;bottom:24px;right:14px;background:rgba(255,255,255,.1);border:1px solid rgba(255,255,255,.2);` +
      `color:rgba(255,255,255,.6);padding:5px 10px;border-radius:6px;cursor:pointer;font-size:12px;font-family:sans-serif}` +
      `.nkey:hover{background:rgba(255,255,255,.2)}` +
      `.nkey.on{background:rgba(255,255,200,.12);border-color:rgba(255,255,200,.3);color:rgba(255,255,200,.9)}${sc}` +
      `${so}${css}${sc}` +
      `${so}${overrideCss}${sc}` +
      `</head><body>` +
      `<div id="wrap"></div>` +
      `<div class="ctrl"><button id="p">◀</button><span id="c"></span><button id="n">▶</button></div>` +
      `<button class="xbtn" id="x">✕ 終了</button>` +
      `<div class="notes" id="notes"></div>` +
      `<button class="nkey" id="nkey">N</button>` +
      `<script>` +
      `var sl=${slidesJson};` +
      `var nt=${notesJson};` +
      `var i=0;` +
      `function show(n){` +
      `i=n;` +
      `var w=document.getElementById('wrap');` +
      `w.innerHTML=sl[n];` +
      `var s=w.querySelector('section');` +
      `var scl=Math.min(window.innerWidth/1280,window.innerHeight/720);` +
      `s.style.transform='scale('+scl+')';` +
      `s.style.transformOrigin='top left';` +
      `w.style.width=(1280*scl)+'px';` +
      `w.style.height=(720*scl)+'px';` +
      `w.style.marginLeft='-'+((1280*scl)/2)+'px';` +
      `w.style.marginTop='-'+((720*scl)/2)+'px';` +
      `document.getElementById('c').textContent=(n+1)+' / '+sl.length;` +
      `document.getElementById('p').disabled=n===0;` +
      `document.getElementById('n').disabled=n===sl.length-1;` +
      `updateNotes();}` +
      `function exit(){window.parent.postMessage({type:'marp-exit'},'*');}` +
      `document.addEventListener('keydown',function(e){` +
      `if(e.key==='ArrowRight'||e.key===' '||e.key==='ArrowDown'){if(i<sl.length-1){show(i+1);e.preventDefault();}}` +
      `else if(e.key==='ArrowLeft'||e.key==='ArrowUp'){if(i>0){show(i-1);e.preventDefault();}}` +
      `else if(e.key==='Escape'){exit();}` +
      `else if(e.key==='n'||e.key==='N'){notesVisible=!notesVisible;document.getElementById('nkey').classList.toggle('on',notesVisible);updateNotes();}});` +
      `document.getElementById('p').onclick=function(){if(i>0)show(i-1);};` +
      `document.getElementById('n').onclick=function(){if(i<sl.length-1)show(i+1);};` +
      `document.getElementById('x').onclick=exit;` +
      `window.addEventListener('resize',function(){show(i);});` +
      `window.addEventListener('message',function(e){` +
      `if(e.data&&e.data.type==='marp-next'){if(i<sl.length-1)show(i+1);}` +
      `else if(e.data&&e.data.type==='marp-prev'){if(i>0)show(i-1);}});` +
      `var notesVisible=false;` +
      `function updateNotes(){var nd=document.getElementById('notes');` +
      `var txt=nt[i]||'';nd.textContent=txt;` +
      `nd.className=notesVisible&&txt?'notes show':'notes';}` +
      `document.getElementById('nkey').onclick=function(){` +
      `notesVisible=!notesVisible;document.getElementById('nkey').classList.toggle('on',notesVisible);updateNotes();};` +
      `show(0);` +
      `${pScriptSc}` +
      `</body></html>`
    if (marpPresentUrl) URL.revokeObjectURL(marpPresentUrl)
    marpPresentUrl = URL.createObjectURL(new Blob([presentHtml], { type: 'text/html' }))

    marpSections = sections.map((s) => s.outerHTML)
    marpNotes = notes
    marpCss = css + '\n' + overrideCss

    if (marpSlideIdx >= marpSections.length) marpSlideIdx = 0
  }

  $: marpScale = Math.min(1, Math.max(0.05, (previewWidth - 32) / 1280))

  function trackPreviewWidth(node: HTMLElement) {
    previewWidth = node.clientWidth
    const ro = new ResizeObserver(() => { previewWidth = node.clientWidth })
    ro.observe(node)
    return { destroy: () => ro.disconnect() }
  }

  let _marpStarted = false
  $: if (_marpStarted) {
    clearTimeout(_marpRenderTimer)
    _marpRenderTimer = setTimeout(() => renderMarpSlides(source), 800)
  }

  function onMessage(e: MessageEvent) {
    if (e.data?.type === 'marp-exit') {
      isMarpFullscreen = false
    }
  }

  function onKeydown(e: KeyboardEvent) {
    if (!isMarpFullscreen) return
    const iframe = document.querySelector('.marp-fullscreen-iframe') as HTMLIFrameElement
    if (!iframe?.contentWindow) return
    if (e.key === 'ArrowRight' || e.key === ' ' || e.key === 'ArrowDown') {
      iframe.contentWindow.postMessage({ type: 'marp-next' }, '*')
      e.preventDefault()
    } else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') {
      iframe.contentWindow.postMessage({ type: 'marp-prev' }, '*')
      e.preventDefault()
    } else if (e.key === 'Escape') {
      isMarpFullscreen = false
    }
  }

  onMount(async () => {
    window.addEventListener('message', onMessage)
    window.addEventListener('keydown', onKeydown)
    const savedMarpTheme = await GetMarpTheme()
    if (savedMarpTheme) marpTheme = savedMarpTheme
    customMarpThemes = await ListMarpCustomThemes()
    await renderMarpSlides(source)
    _marpStarted = true
  })

  onDestroy(() => {
    window.removeEventListener('message', onMessage)
    window.removeEventListener('keydown', onKeydown)
    clearTimeout(_marpRenderTimer)
    if (marpPresentUrl) URL.revokeObjectURL(marpPresentUrl)
  })
</script>

{#if marpSections.length > 0}
  <div class="marp-preview-wrap" use:trackPreviewWidth>
    <div class="marp-slides-scroll" bind:this={marpScrollEl} style="height:calc(100% - {marpThumbH}px)">
      {#each marpSections as sectionHtml, i}
        {@const notes = marpNotes[i] || ''}
        <div class="marp-slide-box" data-slide-idx={i} use:observeSlide style="width:{1280*marpScale}px;height:{720*marpScale}px">
          <div use:mountSection={{html: sectionHtml, css: marpCss}} style="transform:scale({marpScale});transform-origin:top left;width:1280px;height:720px"></div>
        </div>
        {#if notes}
          <div class="marp-notes-panel" style="width:{1280*marpScale}px">{notes}</div>
        {/if}
      {/each}
    </div>
    <div class="marp-thumb-strip" bind:this={marpStripEl} style="height:{marpThumbH}px">
      <div class="marp-thumb-handle" on:pointerdown={startThumbResize}></div>
      {#each marpSections as sectionHtml, i}
        <div class="marp-thumb" data-thumb-idx={i} class:active={marpSlideIdx === i} style="width:{marpThumbInnerW}px;height:{marpThumbInnerH}px" on:click={() => scrollToMarpSlide(i)}>
          <div use:mountSection={{html: sectionHtml, css: marpCss}} style="transform:scale({marpThumbScale});transform-origin:top left;width:1280px;height:720px;pointer-events:none"></div>
          <span class="marp-thumb-num">{i+1}</span>
        </div>
      {/each}
    </div>
    <div class="marp-toolbar">
      <div class="marp-theme-btns">
        <button class:active={marpTheme === 'default'} on:click={() => changeMarpTheme('default')}>Default</button>
        <button class:active={marpTheme === 'gaia'} on:click={() => changeMarpTheme('gaia')}>Gaia</button>
        <button class:active={marpTheme === 'uncover'} on:click={() => changeMarpTheme('uncover')}>Uncover</button>
        {#each customMarpThemes as ct}
          <button class:active={marpTheme === ct.name} on:click={() => changeMarpTheme(ct.name)}>{ct.name}</button>
        {/each}
        <button class="marp-theme-folder-btn" title="テーマフォルダを開く" on:click={async () => { const d = await GetMarpThemesDir(); OpenPath(d) }}>📁</button>
        <button class="marp-theme-folder-btn" title="テーマを再読み込み" on:click={reloadCustomThemes}>↺</button>
      </div>
      <button class="marp-present-btn" on:click={() => (isMarpFullscreen = true)}>▶ 発表</button>
    </div>
  </div>
{/if}

{#if isMarpFullscreen}
  <div class="marp-fullscreen">
    <iframe src={marpPresentUrl} title="Marp Fullscreen" class="marp-fullscreen-iframe"></iframe>
  </div>
{/if}

<style>
  .marp-preview-wrap {
    position: relative;
    width: 100%;
    height: 100%;
    overflow: hidden;
  }

  .marp-slides-scroll {
    width: 100%;
    height: calc(100% - 112px);
    overflow-y: auto;
    overflow-x: hidden;
    padding: 16px 16px 16px;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 16px;
    background: #1a1a1a;
    box-sizing: border-box;
  }

  .marp-slide-box {
    overflow: hidden;
    border-radius: 4px;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.6);
    flex-shrink: 0;
  }

  .marp-notes-panel {
    background: rgba(25, 25, 25, 0.95);
    color: #999;
    font-size: 13px;
    font-family: sans-serif;
    padding: 10px 14px;
    border-radius: 0 0 4px 4px;
    white-space: pre-wrap;
    line-height: 1.6;
    box-sizing: border-box;
  }

  .marp-thumb-strip {
    position: absolute;
    bottom: 0;
    left: 0;
    right: 0;
    height: 112px;
    background: rgba(10, 10, 10, 0.92);
    backdrop-filter: blur(8px);
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    overflow-x: auto;
    overflow-y: hidden;
    z-index: 5;
    border-top: 1px solid rgba(255, 255, 255, 0.08);
    box-sizing: border-box;
  }

  .marp-thumb-strip::-webkit-scrollbar { height: 4px; }
  .marp-thumb-strip::-webkit-scrollbar-thumb { background: rgba(255,255,255,.2); border-radius: 2px; }

  .marp-thumb-handle {
    position: absolute;
    top: -5px;
    left: 0;
    right: 0;
    height: 10px;
    cursor: ns-resize;
    z-index: 6;
  }

  .marp-thumb-handle::after {
    content: '';
    position: absolute;
    top: 4px;
    left: 50%;
    transform: translateX(-50%);
    width: 40px;
    height: 3px;
    border-radius: 2px;
    background: rgba(255, 255, 255, 0.2);
  }

  .marp-thumb-handle:hover::after { background: rgba(255,255,255,.5); }

  .marp-thumb {
    flex-shrink: 0;
    width: 160px;
    height: 90px;
    overflow: hidden;
    border-radius: 3px;
    cursor: pointer;
    border: 2px solid transparent;
    transition: border-color 0.15s;
    position: relative;
  }

  .marp-thumb:hover { border-color: rgba(255,255,255,.4); }
  .marp-thumb.active { border-color: #fff; }

  .marp-thumb-num {
    position: absolute;
    bottom: 3px;
    right: 5px;
    color: rgba(255, 255, 255, 0.8);
    font-size: 9px;
    font-family: sans-serif;
    pointer-events: none;
    text-shadow: 0 0 3px rgba(0, 0, 0, 1);
    z-index: 1;
  }

  .marp-toolbar {
    position: absolute;
    bottom: 16px;
    right: 16px;
    display: flex;
    align-items: center;
    gap: 8px;
    z-index: 10;
    opacity: 0;
    transition: opacity 0.15s;
  }

  .marp-preview-wrap:hover .marp-toolbar {
    opacity: 1;
  }

  .marp-theme-btns {
    display: flex;
    gap: 4px;
    background: rgba(0, 0, 0, 0.6);
    padding: 4px;
    border-radius: 6px;
    border: 1px solid rgba(255, 255, 255, 0.15);
  }

  .marp-theme-btns button {
    background: none;
    border: none;
    color: rgba(255, 255, 255, 0.6);
    padding: 4px 10px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 12px;
    transition: background 0.1s, color 0.1s;
  }

  .marp-theme-btns button:hover {
    background: rgba(255, 255, 255, 0.1);
    color: #fff;
  }

  .marp-theme-btns button.active {
    background: rgba(255, 255, 255, 0.2);
    color: #fff;
  }

  .marp-theme-folder-btn {
    background: none;
    border: none;
    color: rgba(255, 255, 255, 0.5);
    padding: 4px 6px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 13px;
  }

  .marp-theme-folder-btn:hover {
    background: rgba(255, 255, 255, 0.1);
    color: #fff;
  }

  .marp-present-btn {
    background: rgba(0, 0, 0, 0.65);
    color: #fff;
    border: 1px solid rgba(255, 255, 255, 0.25);
    padding: 6px 14px;
    border-radius: 6px;
    cursor: pointer;
    font-size: 13px;
  }

  .marp-present-btn:hover {
    background: rgba(0, 0, 0, 0.85);
  }

  .marp-fullscreen {
    position: fixed;
    inset: 0;
    z-index: 1000;
    background: #000;
  }

  .marp-fullscreen-iframe {
    width: 100%;
    height: 100%;
    border: none;
  }
</style>
