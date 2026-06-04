package handler

import (
	"strings"

	"github.com/APRSCN/aprsgo/internal/network/listener"
	"github.com/gofiber/fiber/v3"
)

// maxSubmitBody bounds the size of an HTTP submit POST body.
const maxSubmitBody = 4096

// Submit handles APRS-IS HTTP packet upload (POST).
//
// The request body must be:
//
//	user CALL pass CODE vers SW VER\r\n
//	PACKET\r\n
//
// with Content-Type "application/octet-stream". On success
// it returns 200 with body "ok\n"; injected packets receive a qAC construct.
func Submit(c fiber.Ctx) error {
	ctype := strings.ToLower(c.Get(fiber.HeaderContentType))
	if !strings.Contains(ctype, "application/octet-stream") {
		return c.Status(fiber.StatusBadRequest).SendString("wrong or missing content-type\n")
	}

	body := c.Body()
	if len(body) == 0 {
		return c.Status(fiber.StatusBadRequest).SendString("empty body\n")
	}
	if len(body) > maxSubmitBody {
		return c.Status(fiber.StatusBadRequest).SendString("body too large\n")
	}

	res, err := listener.SubmitEnvelope(string(body), listener.SubmitHTTP)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error() + "\n")
	}
	if res.Accepted == 0 {
		// Envelope was valid but every packet was rejected (parse/loop/dup).
		return c.Status(fiber.StatusBadRequest).SendString("packet parsing failure\n")
	}

	return c.Status(fiber.StatusOK).SendString("ok\n")
}
