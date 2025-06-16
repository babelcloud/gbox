package docker

import (
	"fmt"

	"github.com/docker/docker/client"

	"github.com/babelcloud/gbox/packages/api-server/config"
	"github.com/babelcloud/gbox/packages/api-server/internal/box/service"
	"github.com/babelcloud/gbox/packages/api-server/internal/tracker"
	"github.com/babelcloud/gbox/packages/api-server/pkg/logger"
)

// Service implements the box service interface using Docker
type Service struct {
	client        *client.Client
	logger        *logger.Logger
	accessTracker tracker.AccessTracker
	imageManager  *ImageManager
}

// NewService creates a new Docker service instance
func NewService(tracker tracker.AccessTracker) (*Service, error) {
	cfg := config.GetInstance()
	dockerHost := cfg.Cluster.Docker.Host

	cli, err := client.NewClientWithOpts(client.WithHost(dockerHost))
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	log := logger.New()

	// Create and start ImageManager
	imageManager := NewImageManager(cli, log)
	imageManager.Start()

	return &Service{
		client:        cli,
		logger:        log,
		accessTracker: tracker,
		imageManager:  imageManager,
	}, nil
}

// Close gracefully shuts down the service
func (s *Service) Close() error {
	if s.imageManager != nil {
		s.imageManager.Stop()
	}
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

func init() {
	service.Register("docker", func(tracker tracker.AccessTracker) (service.BoxService, error) {
		return NewService(tracker)
	})
}
