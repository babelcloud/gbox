package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/babelcloud/gbox/packages/api-server/internal/box/service"
	"github.com/babelcloud/gbox/packages/api-server/internal/tracker"
	model "github.com/babelcloud/gbox/packages/api-server/pkg/box"
	"github.com/stretchr/testify/assert"
)

type mockTracker struct {
	tracker.AccessTracker
}

func TestRegisterAndNew(t *testing.T) {
	t.Run("register and create new service", func(t *testing.T) {
		// Register mock implementation
		service.Register("mock", func(tracker tracker.AccessTracker) (service.BoxService, error) {
			return &mockBoxService{}, nil
		})

		// Create new service
		svc, err := service.New("mock", &mockTracker{})
		assert.NoError(t, err)
		assert.NotNil(t, svc)
	})

	t.Run("unknown implementation", func(t *testing.T) {
		svc, err := service.New("unknown", &mockTracker{})
		assert.Error(t, err)
		assert.Nil(t, svc)
		assert.Contains(t, err.Error(), "unknown box service implementation")
	})

	t.Run("factory returns error", func(t *testing.T) {
		expectedErr := errors.New("factory error")
		service.Register("error", func(tracker tracker.AccessTracker) (service.BoxService, error) {
			return nil, expectedErr
		})

		svc, err := service.New("error", &mockTracker{})
		assert.Error(t, err)
		assert.Nil(t, svc)
		assert.Equal(t, expectedErr, err)
	})
}

// Mock implementation of BoxService for testing
type mockBoxService struct {
	service.BoxService
}

func (m *mockBoxService) Get(ctx context.Context, id string) (*model.Box, error) {
	return nil, nil
}
