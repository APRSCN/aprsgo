package handler

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	config2 "github.com/APRSCN/aprsgo/internal/infra/config"
	"github.com/APRSCN/aprsgo/internal/infra/logger"
	"github.com/APRSCN/aprsgo/internal/network/uplink"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

func testSetup() {
	logger.L = zap.NewNop()
	var c config2.StaticConfig
	c.Server.ID = "TESTING"
	c.Server.BuffSize = 128
	config2.Set(c)
	uplink.Stream = uplink.NewDataStream(10)
}

// newTestApp builds the GoFiber app without the embedded UI (webFS nil).
func newTestApp() *fiber.App {
	return fiberAPP(nil)
}

func TestHTTPSubmitSuccess(t *testing.T) {
	testSetup()
	ch, unsub := uplink.Stream.Subscribe()
	defer unsub()

	app := newTestApp()

	// TEST has passcode 29939.
	body := "user TEST pass 29939 vers httptester 1.0\r\nTEST>APRS,TCPIP*:>http content\r\n"
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 3 * time.Second})
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	select {
	case data := <-ch:
		foundQAC := false
		for _, hop := range data.Data.Path {
			if hop == "qAC" {
				foundQAC = true
			}
		}
		if !foundQAC {
			t.Errorf("expected qAC in path, got %v", data.Data.Path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("packet not injected from HTTP submit")
	}
}

func TestHTTPSubmitWrongContentType(t *testing.T) {
	testSetup()
	app := newTestApp()

	req := httptest.NewRequest("POST", "/", strings.NewReader("x"))
	req.Header.Set("Content-Type", "text/plain")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("status = %d, want 400 for wrong content-type", resp.StatusCode)
	}
}

func TestHTTPSubmitBadPasscode(t *testing.T) {
	testSetup()
	app := newTestApp()

	body := "user TEST pass 1 vers t 1.0\r\nTEST>APRS:>x\r\n"
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("status = %d, want 400 for bad passcode", resp.StatusCode)
	}
}

func TestAPIPing(t *testing.T) {
	testSetup()
	app := newTestApp()

	req := httptest.NewRequest("GET", "/api/ping", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}
