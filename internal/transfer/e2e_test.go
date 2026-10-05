package transfer

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

// TestManualE2E runs a real server for driving the phone client from a
// browser. It is skipped unless AERO_E2E is set:
//
//	AERO_E2E=1 AERO_E2E_DIR=<download dir> AERO_E2E_OFFER=<file to send> \
//	  go test -run TestManualE2E -v -timeout 10m ./internal/transfer
//
// Open the printed URL, send a file from the page, and the server offers
// AERO_E2E_OFFER to the page once it connects.
func TestManualE2E(t *testing.T) {
	if os.Getenv("AERO_E2E") == "" {
		t.Skip("set AERO_E2E=1 to run")
	}
	dir := os.Getenv("AERO_E2E_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	// AERO_E2E_PC_CANCEL_AT=<percent> cancels an incoming upload from the PC side.
	cancelAt, _ := strconv.ParseFloat(os.Getenv("AERO_E2E_PC_CANCEL_AT"), 64)
	var s *Server
	var cancelOnce sync.Once
	s, err := Start(Options{
		IP: "127.0.0.1", DownloadDir: dir,
		OnEvent: func(e Event) {
			fmt.Printf("EVENT %s %s %s %.0f%% %s\n", e.Direction, e.Status, e.Filename, e.Progress, e.Speed)
			if cancelAt > 0 && e.Status == "progress" && e.Progress >= cancelAt {
				cancelOnce.Do(func() { go s.Cancel(e.ID) })
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(context.Background())
	fmt.Println("URL", s.URL())

	offer := os.Getenv("AERO_E2E_OFFER")
	deadline := time.Now().Add(8 * time.Minute)
	for time.Now().Before(deadline) {
		if offer != "" && s.PhoneCount() > 0 {
			time.Sleep(2 * time.Second)
			if err := s.OfferFile(offer); err != nil {
				t.Fatal(err)
			}
			fmt.Println("OFFERED", offer)
			offer = ""
		}
		time.Sleep(200 * time.Millisecond)
	}
}
