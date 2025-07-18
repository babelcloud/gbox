package docker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"

	"github.com/babelcloud/gbox/packages/api-server/config"
	"github.com/babelcloud/gbox/packages/api-server/internal/common"
	model "github.com/babelcloud/gbox/packages/api-server/pkg/box"
	"github.com/babelcloud/gbox/packages/api-server/pkg/id"
)

const defaultStopTimeout = 10 * time.Second

// CreateLinuxBox creates an Alpine Linux box with specific parameters
func (s *Service) CreateLinuxBox(ctx context.Context, params *model.LinuxAndroidBoxCreateParam) (*model.Box, error) {
	// Use Alpine Linux as the default image
	img := GetImage("")

	// Check if image exists - return error if not available
	_, _, err := s.client.ImageInspectWithRaw(ctx, img)
	if err != nil {
		// Image not found, return resource preparation status
		s.logger.Warn("Image %s not available locally, resources are being prepared", img)
		return nil, fmt.Errorf("image resources are being prepared, please try again later (image: %s)", img)
	}

	// Generate box ID
	boxID := id.GenerateBoxID()
	containerName := containerName(boxID)

	tempParams := &model.LinuxAndroidBoxCreateParam{
		Type: "linux",
		Config: model.CreateBoxConfigParam{
			ExpiresIn: params.Config.ExpiresIn,
			Envs:      params.Config.Envs,
			Labels:    params.Config.Labels,
		},
	}

	// Use the same PrepareLabels function as Create method
	labels := PrepareLabels(boxID, tempParams)

	//image labels
	labels["gbox.image"] = img

	// Create share directory for the box
	shareDir := filepath.Join(config.GetInstance().File.Share, boxID)
	if err := os.MkdirAll(shareDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create share directory: %w", err)
	}

	// Prepare mounts (same as Create method)
	var mounts []mount.Mount
	mounts = append(mounts, mount.Mount{
		Type:   mount.TypeBind,
		Source: filepath.Join(config.GetInstance().File.HostShare, boxID),
		Target: common.DefaultShareDirPath,
	})

	// Create container with same logic as Create method
	containerConfig := &container.Config{
		Image:  img,
		Cmd:    GetCommand("", nil), // Use GetCommand for consistent behavior
		Env:    MapToEnv(params.Config.Envs),
		Labels: labels,
	}

	hostConfig := &container.HostConfig{
		Mounts:          mounts,
		PublishAllPorts: true,
	}

	resp, err := s.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, containerName)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := s.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Get container details after start (same as Create method)
	containerInfo, err := s.inspectContainerByID(ctx, boxID)
	if err != nil {
		return nil, fmt.Errorf("failed to get container details after start: %w", err)
	}

	// Update access time on successful creation (same as Create method)
	s.accessTracker.Update(boxID)

	return containerToBox(containerInfo), nil
}

// not implemented
func (s *Service) CreateAndroidBox(ctx context.Context, params *model.AndroidBoxCreateParam) (*model.Box, error) {
	return nil, fmt.Errorf("CreateAndroidBox not implemented")
}

// Start implements Service.Start
func (s *Service) Start(ctx context.Context, id string) (*model.BoxStartResult, error) {
	containerInfo, err := s.getContainerByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if containerInfo.State == "running" {
		// Get full container details for response
		updatedContainerInfo, err := s.inspectContainerByID(ctx, id)
		if err != nil {
			return nil, err
		}
		box := containerToBox(updatedContainerInfo)
		return box, nil
	}

	err = s.client.ContainerStart(ctx, containerInfo.ID, container.StartOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Update access time on successful start
	s.accessTracker.Update(id)

	// Get updated container details after start
	updatedContainerInfo, err := s.inspectContainerByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get container details after start: %w", err)
	}

	box := containerToBox(updatedContainerInfo)
	return box, nil
}

// Stop implements Service.Stop
func (s *Service) Stop(ctx context.Context, id string) (*model.BoxStopResult, error) {
	containerInfo, err := s.getContainerByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if containerInfo.State != "running" {
		// Get full container details for response
		updatedContainerInfo, err := s.inspectContainerByID(ctx, id)
		if err != nil {
			return nil, err
		}
		box := containerToBox(updatedContainerInfo)
		return box, nil
	}

	stopTimeout := int(defaultStopTimeout.Seconds())
	err = s.client.ContainerStop(ctx, containerInfo.ID, container.StopOptions{
		Timeout: &stopTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to stop container: %w", err)
	}

	// Get updated container details after stop
	updatedContainerInfo, err := s.inspectContainerByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get container details after stop: %w", err)
	}

	box := containerToBox(updatedContainerInfo)
	return box, nil
}

// Delete implements Service.Delete
func (s *Service) Delete(ctx context.Context, id string, req *model.BoxDeleteParams) (*model.BoxDeleteResult, error) {
	containerInfo, err := s.getContainerByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if containerInfo.State == "running" {
		_, err = s.Stop(ctx, id)
		if err != nil {
			return nil, err
		}
	}

	err = s.client.ContainerRemove(ctx, containerInfo.ID, types.ContainerRemoveOptions{
		Force: req.Force,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to remove container: %w", err)
	}

	// Remove access tracking info on delete
	s.accessTracker.Remove(id)

	return &model.BoxDeleteResult{
		Message: "Box deleted successfully",
	}, nil
}

// DeleteAll implements Service.DeleteAll
func (s *Service) DeleteAll(ctx context.Context, req *model.BoxesDeleteParams) (*model.BoxesDeleteResult, error) {
	// Build filter for gbox containers
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("%s=gbox", labelName))

	containers, err := s.client.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var deletedIDs []string
	for _, container := range containers {
		err := s.client.ContainerRemove(ctx, container.ID, types.ContainerRemoveOptions{
			Force: req.Force,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to remove container %s: %w", container.ID, err)
		}
		deletedIDs = append(deletedIDs, container.Labels[labelID])
		// Remove access tracking info on delete
		s.accessTracker.Remove(container.Labels[labelID])
	}

	return &model.BoxesDeleteResult{
		Count:   len(deletedIDs),
		Message: "Boxes deleted successfully",
		IDs:     deletedIDs,
	}, nil
}

// Reclaim implements Service.Reclaim
func (s *Service) Reclaim(ctx context.Context) (*model.BoxReclaimResult, error) {
	// Get config for thresholds
	cfg := config.GetInstance()
	reclaimStopThreshold := cfg.Cluster.ReclaimStopThreshold
	reclaimDeleteThreshold := cfg.Cluster.ReclaimDeleteThreshold
	s.logger.Info("Starting box reclaim process with stop threshold: %v, delete threshold: %v", reclaimStopThreshold, reclaimDeleteThreshold)

	// Build filter for gbox containers
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("%s=gbox", labelName))

	containers, err := s.client.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var stoppedCount, deletedCount, skippedCount int
	var stoppedIDs, deletedIDs []string

	for _, c := range containers {
		boxID, ok := c.Labels[labelID]
		if !ok {
			s.logger.Warn("Container %s missing %s label, skipping reclaim check", c.ID, labelID)
			continue
		}

		// Check last accessed time
		lastAccessed, found := s.accessTracker.GetLastAccessed(boxID)
		if !found {
			// If tracker didn't have it, GetLastAccessed initialized it to time.Now()
			// Treat this as recently accessed for this cycle.
			s.logger.Debug("Box %s first seen by tracker, skipping reclaim this cycle", boxID)
			skippedCount++
			continue
		}

		// Calculate idle duration using time.Since
		idleDuration := time.Since(lastAccessed)

		// Stop running containers that have been idle longer than the stop threshold
		if c.State == "running" {
			if idleDuration >= reclaimStopThreshold {
				s.logger.Info("Stopping inactive running box %s (idle for %v)", boxID, idleDuration)
				stopTimeout := int(defaultStopTimeout.Seconds())
				err = s.client.ContainerStop(ctx, c.ID, container.StopOptions{
					Timeout: &stopTimeout,
				})
				if err != nil {
					s.logger.Error("Failed to stop container %s: %v", c.ID, err)
					continue // Continue with next container
				}
				stoppedCount++
				stoppedIDs = append(stoppedIDs, boxID)
				// Do NOT remove tracker info here - we need it for the delete threshold check later
			} else {
				// Running but not idle long enough to stop
				s.logger.Debug("Box %s is running but still active (idle for %v), skipping reclaim", boxID, idleDuration)
				skippedCount++
			}
			continue // Process next container after checking running state
		}

		// Delete stopped containers that have been idle longer than the delete threshold
		if c.State == "exited" {
			if idleDuration >= reclaimDeleteThreshold {
				s.logger.Info("Deleting inactive stopped box %s (idle for %v)", boxID, idleDuration)
				err = s.client.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{
					Force: false, // Use false for reclaim, maybe true for explicit delete?
				})
				if err != nil {
					s.logger.Error("Failed to remove container %s: %v", c.ID, err)
					continue // Continue with next container
				}
				deletedCount++
				deletedIDs = append(deletedIDs, boxID)
				s.accessTracker.Remove(boxID) // Remove tracker info after deleting
			} else {
				// Stopped but not idle long enough to delete
				s.logger.Debug("Box %s is stopped but not idle long enough for deletion (idle for %v), skipping deletion", boxID, idleDuration)
				skippedCount++
			}
			continue // Process next container after checking exited state
		}

		// Handle other states if necessary (e.g., created, restarting) - currently skipped
		s.logger.Debug("Box %s is in state '%s', skipping reclaim action", boxID, c.State)
		skippedCount++

	}

	s.logger.Info("Box reclaim finished. Skipped: %d, Stopped: %d, Deleted: %d", skippedCount, stoppedCount, deletedCount)

	return &model.BoxReclaimResult{
		StoppedCount: stoppedCount,
		DeletedCount: deletedCount,
		StoppedIDs:   stoppedIDs,
		DeletedIDs:   deletedIDs,
	}, nil
}
