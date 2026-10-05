package security

import (
	"encoding/hex"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGoToJavaScript seals a download chunk in Go and opens it with the phone
// client's bundled cipher via Node.js. Skipped when Node is not installed.
func TestGoToJavaScript(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not installed")
	}
	key := make([]byte, KeySize)
	for i := range key {
		key[i] = byte(i)
	}
	s, _ := NewSession(key)
	box, err := s.Seal([]byte("from go"), DownloadChunkAAD("abc", 2, 5))
	if err != nil {
		t.Fatal(err)
	}
	cipher, _ := filepath.Abs("../transfer/web/cipher.js")
	script := `
const fs = require('fs'), vm = require('vm');
const ctx = { crypto: globalThis.crypto }; vm.createContext(ctx);
vm.runInContext(fs.readFileSync(process.argv[1], 'utf8') + ';this.AeroCipher = AeroCipher;', ctx);
const box = new Uint8Array(Buffer.from(process.argv[2], 'hex'));
const key = Uint8Array.from({ length: 32 }, (_, i) => i);
const aad = new TextEncoder().encode(['aero/v1/download-chunk', 'abc', 2, 5].join('\0'));
const pt = ctx.AeroCipher.xchacha20poly1305(key, box.subarray(0, 24), aad).decrypt(box.subarray(24));
process.stdout.write(Buffer.from(pt).toString());
`
	out, err := exec.Command(node, "-e", script, cipher, hex.EncodeToString(box)).CombinedOutput()
	if err != nil || string(out) != "from go" {
		t.Fatalf("node could not open Go box: %v: %s", err, out)
	}
}
