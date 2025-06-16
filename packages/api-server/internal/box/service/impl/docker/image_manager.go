package docker

import (
	"context"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"

	"github.com/babelcloud/gbox/packages/api-server/pkg/logger"
)

// ImageManager manages Docker images in the background
type ImageManager struct {
	client       *client.Client
	logger       *logger.Logger
	stopChan     chan struct{}
	wg           sync.WaitGroup
	interval     time.Duration
	imageTargets []string // Images to manage
}

// NewImageManager creates a new ImageManager
func NewImageManager(dockerClient *client.Client, logger *logger.Logger) *ImageManager {
	// Default interval: 6 hours
	interval := 6 * time.Hour

	// Default images to manage - use the same logic as GetImage("")
	imageTargets := []string{
		GetImage(""), // Use GetImage to get the properly formatted default image
	}

	return &ImageManager{
		client:       dockerClient,
		logger:       logger,
		stopChan:     make(chan struct{}),
		interval:     interval,
		imageTargets: imageTargets,
	}
}

// Start begins the background image management process
func (im *ImageManager) Start() {
	im.wg.Add(1)
	go im.run()
	im.logger.Info("ImageManager started with %v interval", im.interval)
}

// Stop gracefully stops the image management process
func (im *ImageManager) Stop() {
	close(im.stopChan)
	im.wg.Wait()
	im.logger.Info("ImageManager stopped")
}

// run is the main loop that executes image management tasks
func (im *ImageManager) run() {
	defer im.wg.Done()

	// Execute immediately on startup
	im.logger.Info("ImageManager: Executing initial image management cycle")
	im.manageImages()

	// Then execute periodically
	ticker := time.NewTicker(im.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			im.logger.Info("ImageManager: Starting periodic image management cycle")
			im.manageImages()
		case <-im.stopChan:
			im.logger.Info("ImageManager: Received stop signal")
			return
		}
	}
}

// manageImages performs the actual image management tasks
func (im *ImageManager) manageImages() {
	ctx := context.Background()

	for _, imageTarget := range im.imageTargets {
		im.logger.Info("ImageManager: Processing image %s", imageTarget)

		// Ensure image has a tag using unified logic
		imageWithTag := EnsureImageTag(imageTarget)

		// Parse image to get repo and tag for cleanup operations
		repo, tag, ok := parseImageTag(imageWithTag)
		if !ok {
			im.logger.Error("ImageManager: Failed to parse image %s", imageWithTag)
			continue
		}

		// Check if latest version exists locally
		targetImage, _, err := im.client.ImageInspectWithRaw(ctx, imageWithTag)
		targetExists := err == nil

		if !targetExists {
			im.logger.Info("ImageManager: Target image %s not found locally, pulling...", imageWithTag)
			if err := im.pullImage(ctx, imageWithTag); err != nil {
				im.logger.Error("ImageManager: Failed to pull image %s: %v", imageWithTag, err)
				continue
			}
			im.logger.Info("ImageManager: Successfully pulled image %s", imageWithTag)

			// Refresh target image info after pull
			targetImage, _, err = im.client.ImageInspectWithRaw(ctx, imageWithTag)
			if err != nil {
				im.logger.Error("ImageManager: Failed to inspect pulled image %s: %v", imageWithTag, err)
				continue
			}
		} else {
			im.logger.Info("ImageManager: Target image %s already exists locally", imageWithTag)
		}

		// Clean up old versions
		im.cleanupOldImages(ctx, repo, tag, targetImage.ID)
	}

	im.logger.Info("ImageManager: Image management cycle completed")
}

// pullImage pulls an image without progress reporting
func (im *ImageManager) pullImage(ctx context.Context, imageWithTag string) error {
	reader, err := im.client.ImagePull(ctx, imageWithTag, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

	// Wait for pull to complete without streaming progress
	_, err = WaitForResponse(reader)
	return err
}

// cleanupOldImages removes outdated versions of an image
func (im *ImageManager) cleanupOldImages(ctx context.Context, repo, targetTag, targetImageID string) {
	// List all images with the same repository
	filterArgs := filters.NewArgs()
	filterArgs.Add("reference", repo+":*")

	images, err := im.client.ImageList(ctx, types.ImageListOptions{
		Filters: filterArgs,
	})
	if err != nil {
		im.logger.Error("ImageManager: Failed to list images for cleanup: %v", err)
		return
	}

	for _, img := range images {
		// Skip the target image
		if img.ID == targetImageID {
			continue
		}

		// Check if this image has the same repository but different tag
		for _, repoTag := range img.RepoTags {
			if currentRepo, currentTag := parseRepoTag(repoTag); currentRepo == repo && currentTag != targetTag {
				im.logger.Info("ImageManager: Removing outdated image %s (ID: %s)", repoTag, img.ID[:12])

				// Remove the outdated image
				_, err := im.client.ImageRemove(ctx, img.ID, types.ImageRemoveOptions{
					Force:         false, // Don't force remove if image is in use
					PruneChildren: true,
				})
				if err != nil {
					im.logger.Warn("ImageManager: Failed to remove outdated image %s: %v", repoTag, err)
				} else {
					im.logger.Info("ImageManager: Successfully removed outdated image %s", repoTag)
				}
			}
		}
	}
}

// parseRepoTag parses a repo:tag string and returns repo and tag
func parseRepoTag(repoTag string) (string, string) {
	repo, tag, _ := parseImageTag(repoTag)
	return repo, tag
}
