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
    const selectBtn = $('selectBtn');
    const fileInput = $('fileInput');
    const transferPanel = $('transferPanel');
    const fileName = $('fileName');
    const transferStatus = $('transferStatus');
    const progressFill = $('progressFill');
    const speedStat = $('speedStat');
    const progressStat = $('progressStat');
    const etaStat = $('etaStat');
    const recentsDiv = $('recents');
    const mainTitle = $('mainTitle');
    const cancelBtn = $('cancelBtn');
    const statsBanner = $('statsBanner');
    const statsContent = $('statsContent');
    const receivePanel = $('receivePanel');
    const receiveFilename = $('receiveFilename');
    const receiveBtn = $('receiveBtn');
    const receiveDoneBtn = $('receiveDoneBtn');
    const connText = $('connText');
    const connDot = $('connDot');

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

    /** Single-use token proving knowledge of the key for one request. */
    const authToken = (method, path) => b64urlEncode(seal(new Uint8Array(0), AAD.auth(method, path)));

    async function api(method, path, body, signal) {
        const res = await fetch(path, {
            method,
            body,
            signal,
            headers: { 'X-Aero-Auth': authToken(method, path) },
            cache: 'no-store',
        });
        if (!res.ok) throw new Error(`${method} ${path}: ${res.status} ${await res.text()}`);
        return res;
    }

    // ───────────────────────── Connection ──────────────────────────

    function setConnection(state) {
        const styles = {
            connected: ['Connected', '#10b981', 'pulse 1s infinite alternate'],
            connecting: ['Connecting…', '#f59e0b', 'none'],
            disconnected: ['Disconnected', '#ef4444', 'none'],
        }[state];
        connText.textContent = styles[0];
        connDot.style.background = styles[1];
        connDot.style.animation = styles[2];
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
            haptics.success();
        };
        ws.onclose = () => {
            setConnection('disconnected');
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
        };
    }

    // Transfers run one at a time so the single progress panel stays accurate.
    let queue = Promise.resolve();
    const enqueue = (job) => (queue = queue.then(job, job));

    let cancelCurrent = null;

    // ─────────────────────── Progress display ──────────────────────

    function startProgress(title, name, status) {
        mainTitle.textContent = title;
        fileName.textContent = name;
        transferStatus.textContent = status;
        progressFill.style.width = '0%';
        progressStat.textContent = '0%';
        speedStat.textContent = '0 MB/s';
        etaStat.textContent = '--';
        receivePanel.classList.remove('active');
        transferPanel.classList.add('active');
        selectBtn.style.display = 'none';

        const start = Date.now();
        let lastTime = start;
        let lastBytes = 0;
        return {
            start,
            update(done, total) {
                const pct = total ? Math.min(100, Math.round((done / total) * 100)) : 100;
                progressFill.style.width = pct + '%';
                progressStat.textContent = pct + '%';
                const now = Date.now();
                const dt = (now - lastTime) / 1000;
                if (dt >= 0.5) {
                    const speed = (done - lastBytes) / dt;
                    speedStat.textContent = formatSpeed(speed);
                    etaStat.textContent = formatEta(speed > 0 ? (total - done) / speed : 0);
                    lastTime = now;
                    lastBytes = done;
                }
            },
        };
    }

    function finishProgress(name, ok, direction, totalBytes, startTime) {
        cancelCurrent = null;
        if (ok && totalBytes && startTime) {
            const secs = Math.max(0.001, (Date.now() - startTime) / 1000);
            statsContent.textContent = `${(totalBytes / secs / 1048576).toFixed(1)} MB/s | ${secs.toFixed(1)}s`;
            statsBanner.classList.add('show');
        }
        mainTitle.textContent = ok ? 'Transfer Complete!' : 'Transfer Failed';
        transferStatus.textContent = ok ? (direction === 'sent' ? 'Sent successfully' : 'Received') : 'Error occurred';
        if (ok) {
            progressFill.style.width = '100%';
            progressStat.textContent = '100%';
            addRecentFile(name, direction);
            haptics.success();
        } else {
            haptics.error();
        }
        return new Promise((resolve) =>
            setTimeout(() => {
                transferPanel.classList.remove('active');
                if (!receivePanel.classList.contains('active')) resetIdle();
                resolve();
            }, 1500)
        );
    }

    function resetIdle() {
        selectBtn.style.display = 'flex';
        mainTitle.textContent = 'Send Files to PC';
        progressFill.style.width = '0%';
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
                        if (signal.aborted || attempt >= RETRIES) throw e;
                        await new Promise((r) => setTimeout(r, 500 * attempt));
                    }
                }
            }
        };
        await Promise.all(Array.from({ length: Math.min(PARALLEL, count) }, worker));
    }

    // ─────────────────────── Phone → PC upload ─────────────────────

    function putChunk(path, box, signal, onProgress) {
        return new Promise((resolve, reject) => {
            const xhr = new XMLHttpRequest();
            xhr.open('PUT', path, true);
            xhr.setRequestHeader('X-Aero-Auth', authToken('PUT', path));
            xhr.upload.onprogress = (e) => e.lengthComputable && onProgress(e.loaded / e.total);
            xhr.onload = () => (xhr.status === 200 ? resolve() : reject(new Error(`chunk ${xhr.status}: ${xhr.responseText}`)));
            xhr.onerror = () => reject(new Error('network error'));
            xhr.onabort = () => reject(new DOMException('Cancelled', 'AbortError'));
            signal.addEventListener('abort', () => xhr.abort(), { once: true });
            xhr.send(box);
        });
    }

    async function sendFile(file) {
        const controller = new AbortController();
        const { signal } = controller;
        let uploadId = null;
        cancelCurrent = () => controller.abort();

        const progress = startProgress('Sending...', file.name, 'Encrypting & uploading...');
        try {
            const meta = enc.encode(JSON.stringify({ name: file.name, size: file.size, chunkSize: CHUNK_SIZE }));
            const init = await (await api('POST', '/api/upload', seal(meta, AAD.uploadInit()), signal)).json();
            uploadId = init.id;
            const total = init.totalChunks;

            const sent = new Array(total).fill(0);
            const report = () => progress.update(sent.reduce((a, b) => a + b, 0), file.size);

            await runChunks(total, async (i) => {
                const start = i * CHUNK_SIZE;
                const blob = file.slice(start, Math.min(start + CHUNK_SIZE, file.size));
                const plain = new Uint8Array(await blob.arrayBuffer());
                const box = seal(plain, AAD.uploadChunk(uploadId, i, total));
                await putChunk(`/api/upload/${uploadId}/${i}`, box, signal, (f) => {
                    sent[i] = f * plain.length;
                    report();
                });
                sent[i] = plain.length;
                report();
            }, signal);

            await finishProgress(file.name, true, 'sent', file.size, progress.start);
        } catch (e) {
            console.error('[AERO] Upload failed', e);
            if (uploadId) api('DELETE', `/api/upload/${uploadId}`).catch(() => {});
            if (signal.aborted) {
                await showCancelled();
            } else {
                await finishProgress(file.name, false);
            }
        }
    }

    // ─────────────────────── PC → phone download ───────────────────

    let lastObjectURL = null;

    async function receiveFile(offer) {
        const controller = new AbortController();
        const { signal } = controller;
        cancelCurrent = () => controller.abort();

        const name = String(offer.name);
        const progress = startProgress('Receiving...', name, formatBytes(offer.size));
        haptics.tick();
        try {
            const parts = new Array(offer.totalChunks);
            let received = 0;
            await runChunks(offer.totalChunks, async (i) => {
                const res = await api('GET', `/api/download/${offer.id}/${i}`, undefined, signal);
                const plain = open(await res.arrayBuffer(), AAD.downloadChunk(offer.id, i, offer.totalChunks));
                // Wrapping each chunk in a Blob lets the browser page it to disk
                // instead of holding the whole file in memory.
                parts[i] = new Blob([plain]);
                received += plain.length;
                progress.update(received, offer.size);
            }, signal);

            const blob = new Blob(parts, { type: 'application/octet-stream' });
            if (blob.size !== offer.size) throw new Error('size mismatch');
            saveBlob(blob, name);
            await finishProgress(name, true, 'received', offer.size, progress.start);
            mainTitle.textContent = 'File Received';
            receiveFilename.textContent = name;
            receivePanel.classList.add('active');
            selectBtn.style.display = 'none';
        } catch (e) {
            console.error('[AERO] Download failed', e);
            if (signal.aborted) {
                await showCancelled();
            } else {
                await finishProgress(name, false);
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

    function showCancelled() {
        cancelCurrent = null;
        mainTitle.textContent = 'Cancelled';
        transferStatus.textContent = 'Transfer cancelled';
        haptics.error();
        return new Promise((resolve) =>
            setTimeout(() => {
                transferPanel.classList.remove('active');
                resetIdle();
                resolve();
            }, 1500)
        );
    }

    const recentFiles = [];

    function addRecentFile(name, type) {
        recentFiles.unshift({ name, type });
        if (recentFiles.length > 3) recentFiles.pop();
        renderRecents();
    }

    // Built with textContent so file names can never inject markup.
    function renderRecents() {
        recentsDiv.replaceChildren();
        if (recentFiles.length === 0) return;
        const header = document.createElement('div');
        header.className = 'recents-header';
        header.textContent = 'Recent Transfers';
        recentsDiv.appendChild(header);
        for (const f of recentFiles) {
            const item = document.createElement('div');
            item.className = 'recent-item';
            for (const [cls, text] of [
                ['recent-icon', f.type === 'sent' ? '↑' : '↓'],
                ['recent-name', f.name],
                ['recent-badge', f.type],
            ]) {
                const el = document.createElement('div');
                el.className = cls;
                el.textContent = text;
                item.appendChild(el);
            }
            recentsDiv.appendChild(item);
        }
    }

    function formatBytes(n) {
        if (n < 1024) return n + ' B';
        if (n < 1048576) return (n / 1024).toFixed(1) + ' KB';
        if (n < 1073741824) return (n / 1048576).toFixed(1) + ' MB';
        return (n / 1073741824).toFixed(2) + ' GB';
    }

    function formatSpeed(bps) {
        if (bps < 1024) return bps.toFixed(0) + ' B/s';
        if (bps < 1048576) return (bps / 1024).toFixed(1) + ' KB/s';
        return (bps / 1048576).toFixed(1) + ' MB/s';
    }

    function formatEta(seconds) {
        if (!seconds || seconds === Infinity) return '--';
        if (seconds < 60) return Math.ceil(seconds) + 's';
        return Math.floor(seconds / 60) + 'm ' + Math.ceil(seconds % 60) + 's';
    }

    // ────────────────────────── Startup ────────────────────────────

    // Rescanning a new QR code in an open tab only changes the fragment, which
    // browsers do not reload for. Reload so the new key takes effect.
    window.addEventListener('hashchange', () => location.reload());

    if (!KEY) {
        mainTitle.textContent = 'Scan the QR code on your PC';
        selectBtn.style.display = 'none';
        setConnection('disconnected');
        return;
    }

    selectBtn.addEventListener('click', () => fileInput.click());
    fileInput.addEventListener('change', () => {
        for (const file of Array.from(fileInput.files || [])) enqueue(() => sendFile(file));
        fileInput.value = '';
    });
    cancelBtn.addEventListener('click', () => cancelCurrent?.());
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
