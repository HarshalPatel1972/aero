/**
 * Aero phone client.
 *
 * Talks to the desktop app over the LAN. The session key arrives in the URL
 * fragment (#k=...) from the QR code and never leaves this page. Every request
 * carries a single-use auth token, and every file byte, file name and control
 * message is sealed with XChaCha20-Poly1305. The protocol is documented in
 * internal/security/security.go and must stay in sync with it.
 */
(function () {
    'use strict';

    const CHUNK_SIZE = 4 * 1024 * 1024;
    const PARALLEL = 4;
    const RETRIES = 3;
    const NONCE_LEN = 24;

    // ───────────────────────────── DOM ─────────────────────────────

    const $ = (id) => document.getElementById(id);
    const sendBtn = $('sendBtn');
    const bodyContent = $('bodyContent');
    const ringFill = $('ringFill');
    const fileInput = $('fileInput');
    const title = $('title');
    const subtitle = $('subtitle');
    const transferPanel = $('transferPanel');
    const dirIcon = $('dirIcon');
    const fileName = $('fileName');
    const transferStatus = $('transferStatus');
    const speedStat = $('speedStat');
    const etaStat = $('etaStat');
    const cancelBtn = $('cancelBtn');
    const receivePanel = $('receivePanel');
    const receiveFilename = $('receiveFilename');
    const receiveBtn = $('receiveBtn');
    const receiveDoneBtn = $('receiveDoneBtn');
    const recentsDiv = $('recents');
    const conn = $('conn');
    const connText = $('connText');
    const uinf = $('uinf');

    const haptics = {
        tick: () => navigator.vibrate?.(5),
        success: () => navigator.vibrate?.([50, 30, 50]),
        error: () => navigator.vibrate?.([300]),
    };

    // ─────────────────────────── Crypto ────────────────────────────

    const enc = new TextEncoder();
    const dec = new TextDecoder();
    const { xchacha20poly1305 } = window.AeroCipher;

    function b64urlDecode(s) {
        const b64 = s.replace(/-/g, '+').replace(/_/g, '/');
        return Uint8Array.from(atob(b64 + '==='.slice((b64.length + 3) % 4)), (c) => c.charCodeAt(0));
    }

    function b64urlEncode(bytes) {
        let s = '';
        for (let i = 0; i < bytes.length; i++) s += String.fromCharCode(bytes[i]);
        return btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
    }

    function readKey() {
        const m = window.location.hash.match(/[#&]k=([A-Za-z0-9_-]+)/);
        if (!m) return null;
        try {
            const key = b64urlDecode(m[1]);
            return key.length === 32 ? key : null;
        } catch {
            return null;
        }
    }

    const KEY = readKey();

    // AAD labels; must match internal/security/security.go.
    const aad = (...parts) => enc.encode(parts.map(String).join('\0'));
    const AAD = {
        auth: (method, path) => aad('aero/v1/auth', method, path),
        ws: () => aad('aero/v1/ws/s2c'),
        uploadInit: () => aad('aero/v1/upload-init'),
        uploadChunk: (id, i, total) => aad('aero/v1/upload-chunk', id, i, total),
        downloadChunk: (id, i, total) => aad('aero/v1/download-chunk', id, i, total),
    };

    function seal(plain, ad) {
        const nonce = crypto.getRandomValues(new Uint8Array(NONCE_LEN));
        const ct = xchacha20poly1305(KEY, nonce, ad).encrypt(plain);
        const box = new Uint8Array(NONCE_LEN + ct.length);
        box.set(nonce, 0);
        box.set(ct, NONCE_LEN);
        return box;
    }

    function open(box, ad) {
        const bytes = new Uint8Array(box);
        if (bytes.length < NONCE_LEN + 16) throw new Error('message too short');
        return xchacha20poly1305(KEY, bytes.subarray(0, NONCE_LEN), ad).decrypt(bytes.subarray(NONCE_LEN));
    }

    // Chunk crypto runs in a pool of workers so several cores encrypt in
    // parallel; falls back to the main thread if workers are unavailable.
    const cryptoPool = (() => {
        const size = Math.max(1, Math.min(4, (navigator.hardwareConcurrency || 2) - 1));
        const workers = [];
        const pending = new Map();
        let seq = 0;
        let rr = 0;
        try {
            for (let i = 0; i < size; i++) {
                const w = new Worker('/crypto-worker.js');
                w.onmessage = (e) => {
                    const m = e.data;
                    const p = pending.get(m.id);
                    if (!p) return;
                    pending.delete(m.id);
                    m.ok ? p.resolve(new Uint8Array(m.data, m.byteOffset, m.length)) : p.reject(new Error(m.error));
                };
                w.onerror = (e) => console.error('[AERO] crypto worker error', e);
                w.postMessage({ type: 'key', key: KEY });
                workers.push(w);
            }
        } catch (e) {
            console.warn('[AERO] Workers unavailable, encrypting on the main thread', e);
            workers.length = 0;
        }
        const run = (op, data, ad) => {
            if (!workers.length) return Promise.resolve(op === 'seal' ? seal(new Uint8Array(data), ad) : open(data, ad));
            const id = ++seq;
            return new Promise((resolve, reject) => {
                pending.set(id, { resolve, reject });
                workers[rr++ % workers.length].postMessage({ id, op, data, aad: ad }, [data]);
            });
        };
        return {
            /** Seals an ArrayBuffer (which is transferred, i.e. consumed). */
            seal: (buf, ad) => run('seal', buf, ad),
            /** Opens an ArrayBuffer box (transferred). */
            open: (buf, ad) => run('open', buf, ad),
        };
    })();

    /** Single-use token proving knowledge of the key for one request. */
    const authToken = (method, path) => b64urlEncode(seal(new Uint8Array(0), AAD.auth(method, path)));

    /** An error the server says is final (cancelled/expired): never retry. */
    class GoneError extends Error {}

    /**
     * Runs fn with a child AbortSignal that follows `parent`, then detaches it.
     * Listeners left on a long-lived signal keep every finished request (and
     * its multi-megabyte body) alive; that leak crashed Chrome on big files.
     */
    async function withChildSignal(parent, fn) {
        const child = new AbortController();
        const onAbort = () => child.abort();
        if (parent) {
            if (parent.aborted) child.abort();
            else parent.addEventListener('abort', onAbort, { once: true });
        }
        try {
            return await fn(child.signal);
        } finally {
            if (parent) parent.removeEventListener('abort', onAbort);
        }
    }

    async function api(method, path, body, signal, read) {
        return withChildSignal(signal, async (s) => {
            const res = await fetch(path, {
                method,
                body,
                signal: s,
                headers: { 'X-Aero-Auth': authToken(method, path) },
                cache: 'no-store',
            });
            if (res.status === 410) throw new GoneError(await res.text());
            if (!res.ok) throw new Error(`${method} ${path}: ${res.status} ${await res.text()}`);
            return read ? read(res) : res;
        });
    }

    // ───────────────────────── Airflow canvas ──────────────────────
    // Wind-tunnel smoke: streamlines of potential flow around the central
    // circle, with pulses that travel at the local air speed.

    const flow = (() => {
        const canvas = $('flow');
        const g = canvas.getContext('2d');
        const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
        let lines = [];
        let pulses = [];
        let W = 0;
        let H = 0;
        let dpr = 1;
        const style = { color: '#E8ECF0', alpha: 0.6, speed: 50, target: 50, tail: 46 };

        function field(x, y, cx, cy, r2) {
            const dx = x - cx;
            const dy = y - cy;
            const d2 = dx * dx + dy * dy;
            if (d2 < r2) return null;
            const d4 = d2 * d2;
            return [1 - (r2 * (dx * dx - dy * dy)) / d4, (-2 * r2 * dx * dy) / d4];
        }

        function trace() {
            const rect = canvas.getBoundingClientRect();
            W = rect.width;
            H = rect.height;
            dpr = Math.min(devicePixelRatio || 1, 2);
            canvas.width = Math.round(W * dpr);
            canvas.height = Math.round(H * dpr);
            const cx = W / 2;
            const cy = H / 2;
            const r2 = 120 * 120;
            const step = 3;
            const count = 24;
            lines = [];
            for (let i = 0; i < count; i++) {
                const pts = [];
                const sp = [];
                let x = -8;
                let y = (H * (i + 0.5)) / count;
                for (let n = 0; n < 2000 && x < W + 8; n++) {
                    const v = field(x, y, cx, cy, r2);
                    if (!v) break;
                    const m = Math.hypot(v[0], v[1]) || 1e-6;
                    pts.push(x, y);
                    sp.push(Math.min(m, 3));
                    x += (v[0] / m) * step;
                    y += (v[1] / m) * step;
                }
                if (pts.length > 8) lines.push({ pts, sp, n: pts.length / 2 });
            }
            pulses = [];
            lines.forEach((l, li) => {
                for (let k = 0; k < 3; k++) pulses.push({ li, s: Math.random() * l.n });
            });
        }

        let last = 0;
        function frame(t) {
            const dt = last ? Math.min((t - last) / 1000, 0.05) : 0;
            last = t;
            style.speed += (style.target - style.speed) * (1 - Math.exp(-dt * 3));
            g.setTransform(dpr, 0, 0, dpr, 0, 0);
            g.clearRect(0, 0, W, H);
            g.lineWidth = 1;
            g.strokeStyle = '#E8ECF0';
            g.globalAlpha = 0.1;
            for (const l of lines) {
                g.beginPath();
                g.moveTo(l.pts[0], l.pts[1]);
                for (let i = 1; i < l.n; i++) g.lineTo(l.pts[i * 2], l.pts[i * 2 + 1]);
                g.stroke();
            }
            if (!reduced) {
                g.strokeStyle = style.color;
                g.lineWidth = 1.6;
                g.lineCap = 'round';
                const tail = Math.round(style.tail / 3);
                for (const p of pulses) {
                    const l = lines[p.li];
                    const i0 = Math.min(l.n - 1, Math.floor(p.s));
                    p.s += (style.speed * dt * l.sp[i0]) / 3;
                    if (p.s >= l.n) p.s -= l.n;
                    const head = Math.floor(p.s);
                    for (let k = 0; k < 4; k++) {
                        const a = head - Math.round((tail * k) / 4);
                        const b = head - Math.round((tail * (k + 1)) / 4);
                        if (a <= 0) break;
                        g.globalAlpha = style.alpha * (1 - k / 4);
                        g.beginPath();
                        g.moveTo(l.pts[a * 2], l.pts[a * 2 + 1]);
                        for (let i = a - 1; i >= Math.max(0, b); i--) g.lineTo(l.pts[i * 2], l.pts[i * 2 + 1]);
                        g.stroke();
                    }
                }
            }
            g.globalAlpha = 1;
            requestAnimationFrame(frame);
        }

        trace();
        addEventListener('resize', trace);
        requestAnimationFrame(frame);

        return {
            set(mode, bytesPerSec = 0) {
                if (mode === 'busy') {
                    const mb = bytesPerSec / 1048576;
                    Object.assign(style, { color: '#FF5B1F', alpha: 0.95, target: Math.min(340, 80 + Math.log2(1 + mb) * 42), tail: 60 });
                } else if (mode === 'off') {
                    Object.assign(style, { color: '#E8ECF0', alpha: 0.15, target: 12, tail: 34 });
                } else {
                    Object.assign(style, { color: '#E8ECF0', alpha: 0.65, target: 60, tail: 46 });
                }
            },
        };
    })();

    // ───────────────────────── Connection ──────────────────────────

    function setConnection(state) {
        conn.dataset.state = state;
        connText.textContent = { connected: 'Linked', connecting: 'Linking…', disconnected: 'Not linked' }[state];
        if (!busy) flow.set(state === 'connected' ? 'idle' : 'off');
    }

    let retryDelay = 1000;

    function connect() {
        setConnection('connecting');
        const path = '/api/ws';
        const ws = new WebSocket(`ws://${location.host}${path}?auth=${authToken('GET', path)}`);
        ws.binaryType = 'arraybuffer';

        ws.onopen = () => {
            retryDelay = 1000;
            setConnection('connected');
        };
        ws.onclose = () => {
            setConnection('disconnected');
            if (retryDelay >= 8000 && !busy) {
                title.textContent = 'Lost the link';
                subtitle.textContent = 'Is Aero still running on your PC, on the same Wi-Fi? If it was restarted, scan the new QR code.';
            }
            setTimeout(connect, retryDelay);
            retryDelay = Math.min(retryDelay * 2, 15000);
        };
        ws.onmessage = (event) => {
            let msg;
            try {
                msg = JSON.parse(dec.decode(open(event.data, AAD.ws())));
            } catch (e) {
                console.error('[AERO] Rejected message', e);
                return;
            }
            if (msg.type === 'offer') enqueue(() => receiveFile(msg));
            if (msg.type === 'cancel') {
                cancelledOnPC.add(msg.id);
                if (current && current.id === msg.id) current.cancel('pc');
            }
        };
    }

    // Transfers run one at a time so the progress display stays accurate.
    let queue = Promise.resolve();
    const enqueue = (job) => (queue = queue.then(job, job));

    /** The transfer in progress: { id, cancel(reason) }. */
    let current = null;
    const cancelledOnPC = new Set();
    let busy = false;

    // ─────────────────────── Progress display ──────────────────────

    const RING = 304.7;
    const ICON_UP = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><path d="M7 17 17 7M7 7h10v10"/></svg>';
    const ICON_DOWN = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><path d="M17 7 7 17M17 17H7V7"/></svg>';

    let pctText = null;

    function setBody(mode, pct = 0) {
        if (sendBtn.dataset.mode === mode && pctText) {
            // Same mode: just move the numbers (no DOM rebuild per progress event).
            pctText.nodeValue = String(Math.floor(pct));
            ringFill.style.strokeDashoffset = RING * (1 - pct / 100);
            return;
        }
        sendBtn.dataset.mode = mode;
        sendBtn.disabled = mode !== 'idle';
        bodyContent.replaceChildren();
        pctText = null;
        if (mode === 'idle') {
            bodyContent.append(el('svg-up'), el('div', 'display big', 'Send'), el('div', 'small', 'TAP TO CHOOSE'));
            ringFill.style.strokeDashoffset = RING;
        } else {
            const p = el('div', 'pct');
            pctText = document.createTextNode(String(Math.floor(pct)));
            p.append(pctText, el('small', '', '%'));
            bodyContent.append(p, el('div', 'small', mode === 'up' ? 'SENDING' : 'RECEIVING'));
            ringFill.style.strokeDashoffset = RING * (1 - pct / 100);
        }
    }

    function el(tag, cls, text) {
        if (tag === 'svg-up') {
            const s = document.createElement('span');
            s.innerHTML = '<svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="#FF5B1F" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 19V5M5 12l7-7 7 7"/></svg>';
            return s;
        }
        const e = document.createElement(tag);
        if (cls) e.className = cls;
        if (text != null) e.textContent = text;
        return e;
    }

    function startProgress(direction, name, status) {
        busy = true;
        title.textContent = direction === 'up' ? 'Sending to your PC' : 'Incoming from your PC';
        subtitle.textContent = 'Keep this page open until it lands.';
        fileName.textContent = name;
        dirIcon.className = 'dir' + (direction === 'down' ? ' in' : '');
        dirIcon.innerHTML = direction === 'up' ? ICON_UP : ICON_DOWN;
        transferStatus.textContent = status;
        speedStat.textContent = '—';
        etaStat.textContent = '—';
        receivePanel.classList.remove('active');
        transferPanel.classList.add('active');
        setBody(direction, 0);
        flow.set('busy', 0);

        const start = Date.now();
        let lastTime = start;
        let lastBytes = 0;
        let rate = 0;
        let frame = 0;
        let latest = [0, 0];
        return {
            start,
            update(done, total) {
                // Progress events can fire hundreds of times a second; paint at most once per frame.
                latest = [done, total];
                if (frame) return;
                frame = requestAnimationFrame(() => {
                    frame = 0;
                    paint(latest[0], latest[1]);
                });
            },
        };

        function paint(done, total) {
                const pct = total ? Math.min(100, (done / total) * 100) : 100;
                setBody(direction, pct);
                const now = Date.now();
                const dt = (now - lastTime) / 1000;
                if (dt >= 0.5) {
                    const inst = (done - lastBytes) / dt;
                    rate = rate ? rate * 0.6 + inst * 0.4 : inst;
                    speedStat.textContent = formatSpeed(rate);
                    uinf.textContent = formatSpeed(rate);
                    etaStat.textContent = formatEta(rate > 0 ? (total - done) / rate : 0);
                    flow.set('busy', rate);
                    lastTime = now;
                    lastBytes = done;
                }
        }
    }

    function finish(name, ok, direction, totalBytes, startTime) {
        current = null;
        busy = false;
        uinf.textContent = '—';
        transferPanel.classList.remove('active');
        setBody('idle');
        flow.set(conn.dataset.state === 'connected' ? 'idle' : 'off');
        if (ok) {
            const secs = Math.max(0.001, (Date.now() - startTime) / 1000);
            addRecent(name, direction, totalBytes);
            title.textContent = direction === 'sent' ? 'Landed on your PC' : 'Landed here';
            subtitle.textContent = `${formatBytes(totalBytes)} in ${secs.toFixed(1)}s, averaging ${formatSpeed(totalBytes / secs)}. Tap the circle to send more.`;
            haptics.success();
        } else {
            title.textContent = "That one didn't make it";
            subtitle.textContent = 'Check that both devices are on the same Wi-Fi, then try again.';
            haptics.error();
        }
    }

    function resetIdle() {
        title.textContent = 'Send to your PC';
        subtitle.textContent = 'Tap the circle to pick photos, videos or any file. Files from your PC land here automatically.';
    }

    // ─────────────────────── Worker pool helper ────────────────────

    /** Runs task(i) for i in [0, count) with `PARALLEL` workers and retries. */
    async function runChunks(count, task, signal) {
        let next = 0;
        const worker = async () => {
            while (next < count) {
                const i = next++;
                for (let attempt = 1; ; attempt++) {
                    if (signal.aborted) throw new DOMException('Cancelled', 'AbortError');
                    try {
                        await task(i);
                        break;
                    } catch (e) {
                        if (signal.aborted || e instanceof GoneError || attempt >= RETRIES) throw e;
                        await new Promise((r) => setTimeout(r, 500 * attempt));
                    }
                }
            }
        };
        await Promise.all(Array.from({ length: Math.min(PARALLEL, count) }, worker));
    }

    // ─────────────────────── Phone → PC upload ─────────────────────

    /**
     * PUTs one sealed chunk. Every listener is detached when the request ends,
     * so the chunk's memory can be freed immediately.
     */
    function putChunk(path, body, signal, onProgress) {
        return new Promise((resolve, reject) => {
            const xhr = new XMLHttpRequest();
            const onAbort = () => xhr.abort();
            const settle = (fn, value) => {
                signal.removeEventListener('abort', onAbort);
                xhr.upload.onprogress = xhr.onload = xhr.onerror = xhr.onabort = null;
                fn(value);
            };
            xhr.open('PUT', path, true);
            xhr.setRequestHeader('X-Aero-Auth', authToken('PUT', path));
            xhr.upload.onprogress = (e) => e.lengthComputable && onProgress(e.loaded / e.total);
            xhr.onload = () => {
                if (xhr.status === 200) settle(resolve, JSON.parse(xhr.responseText || '{}'));
                else if (xhr.status === 410) settle(reject, new GoneError(xhr.responseText));
                else settle(reject, new Error(`chunk ${xhr.status}: ${xhr.responseText}`));
            };
            xhr.onerror = () => settle(reject, new Error('network error'));
            xhr.onabort = () => settle(reject, new DOMException('Cancelled', 'AbortError'));
            if (signal.aborted) return settle(reject, new DOMException('Cancelled', 'AbortError'));
            signal.addEventListener('abort', onAbort, { once: true });
            xhr.send(body);
        });
    }

    async function sendFile(file, index, count) {
        const controller = new AbortController();
        const { signal } = controller;
        let uploadId = null;
        let reason = 'phone';
        let landed = false;
        current = { id: null, cancel: (r) => { reason = r; controller.abort(); } };

        const progress = startProgress('up', file.name, count > 1 ? `File ${index + 1} of ${count} · ${formatBytes(file.size)}` : formatBytes(file.size));
        try {
            const meta = enc.encode(JSON.stringify({ name: file.name, size: file.size, chunkSize: CHUNK_SIZE }));
            const init = await (await api('POST', '/api/upload', seal(meta, AAD.uploadInit()), signal)).json();
            uploadId = init.id;
            current.id = uploadId;
            const total = init.totalChunks;

            const sent = new Array(total).fill(0);
            const report = () => progress.update(sent.reduce((a, b) => a + b, 0), file.size);

            await runChunks(total, async (i) => {
                const start = i * CHUNK_SIZE;
                const blob = file.slice(start, Math.min(start + CHUNK_SIZE, file.size));
                const len = blob.size;
                // Chrome streams Blob bodies through a fast data pipe; raw typed
                // arrays take a slow copy path (measured ~8x slower: 12 vs 95 MB/s).
                const body = new Blob([await cryptoPool.seal(await blob.arrayBuffer(), AAD.uploadChunk(uploadId, i, total))]);
                const res = await putChunk(`/api/upload/${uploadId}/${i}`, body, signal, (f) => {
                    sent[i] = f * len;
                    report();
                });
                if (res.complete) landed = true;
                sent[i] = len;
                report();
            }, signal);

            finish(file.name, true, 'sent', file.size, progress.start);
        } catch (e) {
            if (landed) {
                // The last chunk landed before the cancel did: the file is complete on the PC.
                finish(file.name, true, 'sent', file.size, progress.start);
                return;
            }
            if (uploadId && reason !== 'pc') {
                // Tell the PC to delete what it has so far.
                api('DELETE', `/api/upload/${uploadId}`).catch(() => {});
            }
            if (e instanceof GoneError || cancelledOnPC.has(uploadId)) cancelled('pc', 'up');
            else if (signal.aborted) cancelled(reason, 'up');
            else {
                console.error('[AERO] Upload failed', e);
                finish(file.name, false);
            }
        }
    }

    // ─────────────────────── PC → phone download ───────────────────

    let lastObjectURL = null;

    async function receiveFile(offer) {
        if (cancelledOnPC.has(offer.id)) return; // cancelled while queued
        const controller = new AbortController();
        const { signal } = controller;
        let reason = 'phone';
        current = { id: offer.id, cancel: (r) => { reason = r; controller.abort(); } };

        const name = String(offer.name);
        const progress = startProgress('down', name, formatBytes(offer.size));
        haptics.tick();
        try {
            const parts = new Array(offer.totalChunks);
            let received = 0;
            await runChunks(offer.totalChunks, async (i) => {
                const box = await api('GET', `/api/download/${offer.id}/${i}`, undefined, signal, (r) => r.arrayBuffer());
                const plain = await cryptoPool.open(box, AAD.downloadChunk(offer.id, i, offer.totalChunks));
                // Wrapping each chunk in a Blob lets the browser page it to disk
                // instead of holding the whole file in memory.
                parts[i] = new Blob([plain]);
                received += plain.length;
                progress.update(received, offer.size);
            }, signal);

            const blob = new Blob(parts, { type: 'application/octet-stream' });
            if (blob.size !== offer.size) throw new Error('size mismatch');
            saveBlob(blob, name);
            finish(name, true, 'received', offer.size, progress.start);
            receiveFilename.textContent = name;
            receivePanel.classList.add('active');
        } catch (e) {
            // Nothing is saved until every chunk is verified, so dropping the
            // received parts here leaves nothing partial on the phone.
            if (e instanceof GoneError || cancelledOnPC.has(offer.id)) cancelled('pc', 'down');
            else if (signal.aborted) {
                api('DELETE', `/api/download/${offer.id}`).catch(() => {});
                cancelled(reason, 'down');
            } else {
                console.error('[AERO] Download failed', e);
                api('DELETE', `/api/download/${offer.id}`).catch(() => {});
                finish(name, false);
            }
        }
    }

    function saveBlob(blob, name) {
        if (lastObjectURL) URL.revokeObjectURL(lastObjectURL);
        lastObjectURL = URL.createObjectURL(blob);
        receiveBtn.href = lastObjectURL;
        receiveBtn.download = name;
        const a = document.createElement('a');
        a.href = lastObjectURL;
        a.download = name;
        document.body.appendChild(a);
        a.click();
        a.remove();
    }

    // ──────────────────────────── UI ───────────────────────────────

    function cancelled(by, direction) {
        current = null;
        busy = false;
        uinf.textContent = '—';
        transferPanel.classList.remove('active');
        setBody('idle');
        flow.set(conn.dataset.state === 'connected' ? 'idle' : 'off');
        title.textContent = by === 'pc' ? 'Cancelled on your PC' : 'Cancelled';
        subtitle.textContent = direction === 'up'
            ? 'Nothing was saved on your PC. Tap the circle to try again.'
            : 'Nothing was saved on this phone.';
        haptics.error();
    }

    const recentFiles = [];

    function addRecent(name, type, size) {
        recentFiles.unshift({ name, type, size });
        if (recentFiles.length > 4) recentFiles.pop();
        renderRecents();
    }

    // Built with textContent so file names can never inject markup.
    function renderRecents() {
        recentsDiv.replaceChildren();
        if (recentFiles.length === 0) return;
        recentsDiv.appendChild(el('div', 'label recents-head', 'Recent flights'));
        for (const f of recentFiles) {
            const row = el('div', 'recent-item');
            const icon = el('span', 'dir' + (f.type === 'received' ? ' in' : ''));
            icon.innerHTML = f.type === 'sent' ? ICON_UP : ICON_DOWN;
            row.append(icon, el('span', 'recent-name', f.name), el('span', 'recent-meta', formatBytes(f.size)));
            recentsDiv.appendChild(row);
        }
    }

    function formatBytes(n) {
        if (n < 1024) return n + ' B';
        if (n < 1048576) return (n / 1024).toFixed(1) + ' KB';
        if (n < 1073741824) return (n / 1048576).toFixed(1) + ' MB';
        return (n / 1073741824).toFixed(2) + ' GB';
    }

    function formatSpeed(bps) {
        if (bps < 1048576) return (bps / 1024).toFixed(0) + ' KB/s';
        return (bps / 1048576).toFixed(1) + ' MB/s';
    }

    function formatEta(seconds) {
        if (!seconds || seconds === Infinity) return '—';
        if (seconds < 60) return Math.ceil(seconds) + 's';
        return Math.floor(seconds / 60) + 'm ' + Math.ceil(seconds % 60) + 's';
    }

    // ────────────────────────── Startup ────────────────────────────

    // Rescanning a new QR code in an open tab only changes the fragment, which
    // browsers do not reload for. Reload so the new key takes effect.
    window.addEventListener('hashchange', () => location.reload());

    if (!KEY) {
        title.textContent = 'Scan the code on your PC';
        subtitle.textContent = 'Open Aero on your computer, press Start, then point this phone’s camera at the QR code.';
        setBody('idle');
        sendBtn.disabled = true;
        sendBtn.dataset.mode = 'off';
        setConnection('disconnected');
        return;
    }

    setBody('idle');
    sendBtn.addEventListener('click', () => fileInput.click());
    fileInput.addEventListener('change', () => {
        const files = Array.from(fileInput.files || []);
        files.forEach((file, i) => enqueue(() => sendFile(file, i, files.length)));
        fileInput.value = '';
    });
    cancelBtn.addEventListener('click', () => current?.cancel('phone'));
    receiveDoneBtn.addEventListener('click', () => {
        receivePanel.classList.remove('active');
        if (lastObjectURL) {
            URL.revokeObjectURL(lastObjectURL);
            lastObjectURL = null;
        }
        resetIdle();
    });

    connect();
})();
