package service

import (
	"context"
	"fmt"
	"io"

	"github.com/babelcloud/gbox/packages/api-server/internal/tracker"
	model "github.com/babelcloud/gbox/packages/api-server/pkg/box"
	"github.com/gorilla/websocket"
)

// BoxService defines the interface for box operations
type BoxService interface {
	// Box lifecycle operations
	List(ctx context.Context, params *model.BoxListParams) (*model.BoxListResult, error)
	Get(ctx context.Context, id string) (*model.Box, error)
	CreateLinuxBox(ctx context.Context, params *model.LinuxAndroidBoxCreateParam) (*model.Box, error)
	CreateAndroidBox(ctx context.Context, params *model.AndroidBoxCreateParam) (*model.Box, error)
	Delete(ctx context.Context, id string, params *model.BoxDeleteParams) (*model.BoxDeleteResult, error)
	DeleteAll(ctx context.Context, params *model.BoxesDeleteParams) (*model.BoxesDeleteResult, error)
	Reclaim(ctx context.Context) (*model.BoxReclaimResult, error)

	// Box runtime operations
	Start(ctx context.Context, id string) (*model.BoxStartResult, error)
	Stop(ctx context.Context, id string) (*model.BoxStopResult, error)
	Exec(ctx context.Context, id string, params *model.BoxExecParams) (*model.BoxExecResult, error)
	ExecWS(ctx context.Context, id string, params *model.BoxExecWSParams, wsConn *websocket.Conn) (*model.BoxExecResult, error)
	RunCode(ctx context.Context, id string, params *model.BoxRunCodeParams) (*model.BoxRunCodeResult, error)

	// Box file operations
	GetArchive(ctx context.Context, id string, params *model.BoxArchiveGetParams) (*model.BoxArchiveResult, io.ReadCloser, error)
	HeadArchive(ctx context.Context, id string, params *model.BoxArchiveHeadParams) (*model.BoxArchiveHeadResult, error)
	ExtractArchive(ctx context.Context, id string, params *model.BoxArchiveExtractParams) error

	// Box filesystem operations
	ListFiles(ctx context.Context, id string, params *model.BoxFileListParams) (*model.BoxFileListResult, error)
	ReadFile(ctx context.Context, id string, params *model.BoxFileReadParams) (*model.BoxFileReadResult, error)
	WriteFile(ctx context.Context, id string, params *model.BoxFileWriteParams) (*model.BoxFileWriteResult, error)

	// Box image operations - removed UpdateBoxImage methods as they are now handled by background ImageManager

	// GetExternalPort retrieves the host port mapping for a specific internal port of a box.
	GetExternalPort(ctx context.Context, id string, internalPort int) (int, error)

	// Image management is now handled by background ImageManager service

	// Box action operations, these are only supported for cloud version
	BoxActionClick(ctx context.Context, id string, params *model.BoxActionClickParams) (*model.BoxActionClickResult, error)
	BoxActionDrag(ctx context.Context, id string, params *model.BoxActionDragParams) (*model.BoxActionDragResult, error)
	BoxActionMove(ctx context.Context, id string, params *model.BoxActionMoveParams) (*model.BoxActionMoveResult, error)
	BoxActionPress(ctx context.Context, id string, params *model.BoxActionPressParams) (*model.BoxActionPressResult, error)
	BoxActionScreenshot(ctx context.Context, id string, params *model.BoxActionScreenshotParams) (*model.BoxActionScreenshotResult, error)
	BoxActionScroll(ctx context.Context, id string, params *model.BoxActionScrollParams) (*model.BoxActionScrollResult, error)
}

// Factory creates a new box service instance, accepting an AccessTracker
type Factory func(tracker tracker.AccessTracker) (BoxService, error)

var implementations = make(map[string]Factory)

// Register registers a box service implementation
func Register(name string, factory Factory) {
	implementations[name] = factory
}

// New creates a new box service based on the implementation name, passing the tracker
func New(name string, tracker tracker.AccessTracker) (BoxService, error) {
	factory, ok := implementations[name]
	if !ok {
		return nil, fmt.Errorf("unknown box service implementation: %s", name)
	}
	// Pass the tracker to the factory function
	return factory(tracker)
}
