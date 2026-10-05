/**
 * Aero crypto worker: seals and opens file chunks off the main thread so
 * several CPU cores can encrypt in parallel. Same construction as app.js:
 * box = nonce(24) || XChaCha20-Poly1305 ciphertext || tag(16).
 */
importScripts('/cipher.js');

const NONCE_LEN = 24;
let key = null;

self.onmessage = (e) => {
    const m = e.data;
    if (m.type === 'key') {
        key = new Uint8Array(m.key);
        return;
    }
    try {
        let out;
        if (m.op === 'seal') {
            const nonce = crypto.getRandomValues(new Uint8Array(NONCE_LEN));
            const ct = self.AeroCipher.xchacha20poly1305(key, nonce, m.aad).encrypt(new Uint8Array(m.data));
            out = new Uint8Array(NONCE_LEN + ct.length);
            out.set(nonce, 0);
            out.set(ct, NONCE_LEN);
        } else {
            const box = new Uint8Array(m.data);
            if (box.length < NONCE_LEN + 16) throw new Error('message too short');
            out = self.AeroCipher.xchacha20poly1305(key, box.subarray(0, NONCE_LEN), m.aad).decrypt(box.subarray(NONCE_LEN));
        }
        self.postMessage({ id: m.id, ok: true, data: out.buffer, byteOffset: out.byteOffset, length: out.length }, [out.buffer]);
    } catch (err) {
        self.postMessage({ id: m.id, ok: false, error: String(err) });
    }
};
