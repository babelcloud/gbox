package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	restful "github.com/emicklei/go-restful/v3"

	"github.com/babelcloud/gbox/packages/api-server/config"
	boxApi "github.com/babelcloud/gbox/packages/api-server/internal/box/api"
	boxService "github.com/babelcloud/gbox/packages/api-server/internal/box/service"
	_ "github.com/babelcloud/gbox/packages/api-server/internal/box/service/impl/docker"
	_ "github.com/babelcloud/gbox/packages/api-server/internal/box/service/impl/k8s"
	browserApi "github.com/babelcloud/gbox/packages/api-server/internal/browser/api"
	browserService "github.com/babelcloud/gbox/packages/api-server/internal/browser/service"
	"github.com/babelcloud/gbox/packages/api-server/internal/common"
	"github.com/babelcloud/gbox/packages/api-server/internal/cron"
	cuaApi "github.com/babelcloud/gbox/packages/api-server/internal/cua/api"
	fileApi "github.com/babelcloud/gbox/packages/api-server/internal/file/api"
	fileService "github.com/babelcloud/gbox/packages/api-server/internal/file/service"
	miscApi "github.com/babelcloud/gbox/packages/api-server/internal/misc/api"
	miscService "github.com/babelcloud/gbox/packages/api-server/internal/misc/service"
	"github.com/babelcloud/gbox/packages/api-server/internal/tracker"
	"github.com/babelcloud/gbox/packages/api-server/pkg/format"
	"github.com/babelcloud/gbox/packages/api-server/pkg/logger"
)

func main() {
	log := logger.New()

	defer func() {
		if r := recover(); r != nil {
			log.Error("Panic recovered: %v", r)
			os.Exit(1)
		}
	}()

	// Initialize configuration
	cfg := config.GetInstance()

	// Initialize Access Tracker
	accessTracker := tracker.NewInMemoryAccessTracker()
	log.Info("Initialized In-Memory Access Tracker")
	log.Info("Box reclaim stop threshold: %s, delete threshold: %s",
		common.FormatDurationConcise(cfg.Cluster.ReclaimStopThreshold),
		common.FormatDurationConcise(cfg.Cluster.ReclaimDeleteThreshold))

	// Initialize services
	boxSvc, err := boxService.New(cfg.Cluster.Mode, accessTracker)
	if err != nil {
		log.Fatal("Failed to initialize box service: %v", err)
	}
	defer func() {
		log.Info("Closing box service...")
		if closer, ok := boxSvc.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				log.Error("Error closing box service: %v", err)
			}
		}
	}()

	fileSvc, err := fileService.New(boxSvc)
	if err != nil {
		log.Fatal("Failed to initialize file service: %v", err)
	}

	miscSvc := miscService.New()

	browserSvc, err := browserService.NewBrowserService(boxSvc)
	if err != nil {
		log.Fatal("Failed to initialize browser service: %v", err)
	}
	defer func() {
		log.Info("Closing browser service...")
		if err := browserSvc.Close(); err != nil {
			log.Error("Error closing browser service: %v", err)
		}
	}()

	// Initialize cron manager (pass fileSvc pointer)
	cronManager := cron.NewManager(log, boxSvc, fileSvc)
	cronManager.Start()
	defer cronManager.Stop()

	// Initialize API handlers
	boxHandler := boxApi.NewBoxHandler(boxSvc)
	fileHandler := fileApi.NewFileHandler(*fileSvc)
	miscHandler := miscApi.NewMiscHandler(miscSvc)
	browserHandler := browserApi.NewHandler(browserSvc)
	cuaHandler := cuaApi.NewCuaHandler()

	// Create REST API container
	container := restful.NewContainer()

	// Create WebService
	ws := new(restful.WebService)
	ws.Path("/api/v1").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	// Register routes
	boxApi.RegisterRoutes(ws, boxHandler)
	fileApi.RegisterRoutes(ws, fileHandler)
	miscApi.RegisterRoutes(ws, miscHandler)
	browserApi.RegisterBrowserRoutes(ws, browserHandler)
	cuaApi.RegisterCuaRoutes(ws, cuaHandler)

	container.Add(ws)

	// Log API endpoints
	endpoints := make([]format.APIEndpoint, 0, len(ws.Routes()))
	for _, route := range ws.Routes() {
		endpoints = append(endpoints, format.APIEndpoint{
			Method:      route.Method,
			Path:        route.Path,
			Description: route.Doc,
		})
	}
	format.LogAPIEndpoints(log, endpoints)

	// Add CORS filter
	cors := restful.CrossOriginResourceSharing{
		AllowedHeaders: []string{"Content-Type", "Accept"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
		AllowedDomains: []string{"*"},
	}
	container.Filter(cors.Filter)

	// Add request logging filter
	container.Filter(func(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
		// Print request line with query parameters
		url := req.Request.URL.Path
		if req.Request.URL.RawQuery != "" {
			url += "?" + req.Request.URL.RawQuery
		}
		log.Info("%s %s %s", req.Request.Method, url, req.Request.Proto)

		// Print headers in debug mode
		if log.IsDebugEnabled() && len(req.Request.Header) > 0 {
			headers := make([]string, 0, len(req.Request.Header))
			for name, values := range req.Request.Header {
				headers = append(headers, fmt.Sprintf("%s: %s", name, values[0]))
			}
			log.Debug("Headers: %s", strings.Join(headers, ", "))
		}

		// Add debug logging for request routing
		log.Debug("Request route: %s", req.SelectedRoutePath())
		log.Debug("Request parameters: %v", req.PathParameters())

		// Process the request
		chain.ProcessFilter(req, resp)

		// Log response status
		log.Debug("Response status: %d", resp.StatusCode())
	})

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Info("%s", format.FormatServerMode(cfg.Cluster.Mode))
	log.Info("Starting server on %s", addr)

	// Get all local IPs
	ips := common.GetLocalIPs()
	log.Info("Accessible URLs:")
	for _, ip := range ips {
		log.Info("  http://%s:%d", ip, cfg.Server.Port)
	}

	// Create a channel to receive OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	server := &http.Server{
		Addr:    addr,
		Handler: container,
	}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-sigChan
	log.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown: %v", err)
	}

	log.Info("Server exited properly")
}
