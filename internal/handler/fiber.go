package handler

import (
	"fmt"
	"net/http"

	"github.com/APRSCN/aprsgo/internal/config"
	"github.com/APRSCN/aprsgo/internal/logger"
	"github.com/APRSCN/aprsgo/internal/middleware"
	"github.com/APRSCN/aprsgo/internal/model"

	"github.com/bytedance/sonic"
	"github.com/go-playground/validator/v10"
	fiberzap "github.com/gofiber/contrib/v3/zap"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/gofiber/utils/v2"
	"go.uber.org/zap"
)

var app *fiber.App

// structValidator struct implementation
type structValidator struct {
	validate *validator.Validate
}

// Validate method implementation
func (v *structValidator) Validate(out any) error {
	return v.validate.Struct(out)
}

// fiberAPP provides a fiber app
func fiberAPP() *fiber.App {
	app := fiber.New(fiber.Config{
		JSONEncoder:     sonic.Marshal,
		JSONDecoder:     sonic.Unmarshal,
		ProxyHeader:     fiber.HeaderXForwardedFor,
		StructValidator: &structValidator{validate: validator.New()},
	})

	// Use requestID middleware
	app.Use(requestid.New(requestid.Config{
		Next:      nil,
		Header:    fiber.HeaderXRequestID,
		Generator: utils.UUIDv4,
	}))

	// Use global logger
	app.Use(fiberzap.New(fiberzap.Config{
		Logger: logger.L,
		FieldsFunc: func(c fiber.Ctx) []zap.Field {
			requestID := requestid.FromContext(c)
			return []zap.Field{
				zap.String("requestID", requestID),
			}
		},
	}))

	// Use customer header middleware
	app.Use(middleware.CustomHeader)

	// Root info router handler
	app.All("/api", func(c fiber.Ctx) error {
		return model.RespSuccess(c, map[string]any{
			"poweredBy": fmt.Sprintf("%s %s %s", config.ENName, config.Version, config.Nickname),
		})
	})

	// Register global routes
	Register(app)

	// Not found router handler
	app.Use(func(c fiber.Ctx) error {
		return model.RespNotFound(c)
	})

	return app
}

// RunHTTPServer runs a HTTP server
func RunHTTPServer() {
	// Create Fiber app
	app = fiberAPP()

	// Use fiber as handler
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", config.C.GetString("server.status.host"), config.C.GetInt("server.status.port")),
		Handler: adaptor.FiberApp(app),
	}

	server.SetKeepAlivesEnabled(true)

	// Start HTTP server
	go func() {
		logger.L.Fatal(server.ListenAndServe().Error())
	}()

	if config.C.GetBool("debug") {
		host := config.C.GetString("server.status.host")
		if host == "" {
			host = "[::]"
		}
		visit := host
		if host == "0.0.0.0" || host == "[::]" {
			visit = "localhost"
		}

		logger.L.Info(fmt.Sprintf("Server is running on %s:%d", host, config.C.GetInt("server.status.port")))
		logger.L.Debug(fmt.Sprintf("Visit by http://%s:%d", visit, config.C.GetInt("server.status.port")))
	}
}
