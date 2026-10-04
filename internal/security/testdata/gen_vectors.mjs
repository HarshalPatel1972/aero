// Generates JS-sealed vectors with the exact phone-client bundle so Go tests
// can prove the two implementations interoperate.
// Run: node internal/security/testdata/gen_vectors.mjs > internal/security/testdata/js_vectors.json
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

const src = readFileSync(new URL('../../transfer/web/cipher.js', import.meta.url), 'utf8');
const ctx = { crypto: globalThis.crypto };
vm.createContext(ctx);
vm.runInContext(src + ';this.AeroCipher = AeroCipher;', ctx);
const { xchacha20poly1305 } = ctx.AeroCipher;

const key = new Uint8Array(32).map((_, i) => i);
const enc = new TextEncoder();
const aad = (...p) => enc.encode(p.map(String).join('\0'));
const hex = (b) => Buffer.from(b).toString('hex');
const b64url = (b) => Buffer.from(b).toString('base64url');

function seal(plain, ad) {
  const nonce = crypto.getRandomValues(new Uint8Array(24));
  const ct = xchacha20poly1305(key, nonce, ad).encrypt(plain);
  return new Uint8Array([...nonce, ...ct]);
}

console.log(JSON.stringify({
  key: hex(key),
  token: { method: 'PUT', path: '/api/upload/abc/0', value: b64url(seal(new Uint8Array(0), aad('aero/v1/auth', 'PUT', '/api/upload/abc/0'))) },
  uploadInit: { plain: '{"name":"photo.jpg","size":5,"chunkSize":65536}', box: hex(seal(enc.encode('{"name":"photo.jpg","size":5,"chunkSize":65536}'), aad('aero/v1/upload-init'))) },
  uploadChunk: { id: 'abc', index: 3, total: 7, plain: 'hello chunk', box: hex(seal(enc.encode('hello chunk'), aad('aero/v1/upload-chunk', 'abc', 3, 7))) },
}, null, 2));
