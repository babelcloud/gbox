package api

import (
	model "github.com/babelcloud/gbox/packages/api-server/pkg/box"
	"github.com/emicklei/go-restful/v3"
)

// RegisterRoutes registers all box-related routes to the WebService
func RegisterRoutes(ws *restful.WebService, boxHandler *BoxHandler) {
	// Box Lifecycle Operations
	ws.Route(ws.GET("/boxes").To(boxHandler.ListBoxes).
		Doc("list all boxes").
		//page and pageSize are only supported for cloud version
		Param(ws.QueryParameter("page", "page number").DataType("float64").Required(false)).
		Param(ws.QueryParameter("pageSize", "page size").DataType("float64").Required(false)).
		Returns(200, "OK", []model.Box{}).
		Returns(500, "Internal Server Error", model.BoxError{}))

	ws.Route(ws.GET("/boxes/{id}").To(boxHandler.GetBox).
		Doc("get a box by ID").
		Param(ws.PathParameter("id", "identifier of the box").DataType("string")).
		Returns(200, "OK", model.Box{}).
		Returns(404, "Not Found", model.BoxError{}).
		Returns(500, "Internal Server Error", model.BoxError{}))

	// ws.Route(ws.POST("/boxes").To(boxHandler.CreateBox).
	// 	Doc("create a box").
	// 	Reads(model.BoxCreateParams{}).
	// 	Produces("application/json", "application/json-stream").
	// 	Param(ws.QueryParameter("timeout", "timeout duration for image pull (e.g. 30s, 1m)").DataType("string").Required(false)).
	// 	Returns(201, "Created", model.Box{}).
	// 	Returns(202, "Accepted", model.BoxError{}).
	// 	Returns(400, "Bad Request", model.BoxError{}).
	// 	Returns(500, "Internal Server Error", model.BoxError{}))

	ws.Route(ws.POST("/boxes/linux").To(boxHandler.CreateLinuxBox).
		Doc("create a linux box").
		Reads(model.LinuxAndroidBoxCreateParam{}).
		Produces("application/json", "application/json-stream").
		Returns(201, "Created", model.Box{}).
		Returns(202, "Accepted", model.BoxError{}).
		Returns(400, "Bad Request", model.BoxError{}).
		Returns(500, "Internal Server Error", model.BoxError{}))

	ws.Route(ws.POST("/boxes/android").To(boxHandler.CreateAndroidBox).
		Doc("create a android box").
		Reads(model.LinuxAndroidBoxCreateParam{}).
		Produces("application/json", "application/json-stream").
		Returns(201, "Created", model.Box{}).
		Returns(202, "Accepted", model.BoxError{}).
		Returns(400, "Bad Request", model.BoxError{}).
		Returns(500, "Internal Server Error", model.BoxError{}))

	ws.Route(ws.DELETE("/boxes/{id}").To(boxHandler.DeleteBox).
		Doc("delete a box").
		Reads(model.BoxDeleteParams{}).
		Returns(200, "OK", model.BoxDeleteResult{}).
		Returns(404, "Not Found", model.BoxError{}).
		Returns(500, "Internal Server Error", model.BoxError{}))

	ws.Route(ws.POST("/boxes/{id}/terminate").To(boxHandler.DeleteBox).
		Doc("terminate a box").
		Param(ws.PathParameter("id", "identifier of the box").DataType("string")).
		Returns(200, "OK", model.BoxDeleteResult{}).
		Returns(404, "Not Found", model.BoxError{}).
		Returns(500, "Internal Server Error", model.BoxError{}))

	// ws.Route(ws.DELETE("/boxes").To(boxHandler.DeleteBoxes).
	// 	Doc("delete all boxes").
	// 	Reads(model.BoxesDeleteParams{}).
	// 	Returns(200, "OK", model.BoxesDeleteResult{}).
	// 	Returns(500, "Internal Server Error", model.BoxError{}))

	// ws.Route(ws.POST("/boxes/reclaim").To(boxHandler.ReclaimBoxes).
	// 	Doc("reclaim inactive boxes").
	// 	Returns(200, "OK", model.BoxReclaimResult{}).
	// 	Returns(500, "Internal Server Error", model.BoxError{}))

	// Box Runtime Operations
	ws.Route(ws.POST("/boxes/{id}/commands").To(boxHandler.ExecBox).
		Doc("execute a command in a box").
		Param(ws.PathParameter("id", "identifier of the box").DataType("string")).
		Reads(model.BoxExecParams{}).
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON).
		Returns(200, "OK", model.BoxExecResult{}).
		Returns(400, "Bad Request", model.BoxError{}).
		Returns(404, "Not Found", model.BoxError{}).
		Returns(409, "Conflict", model.BoxError{}).
		Returns(500, "Internal Server Error", model.BoxError{}))

	ws.Route(ws.POST("/boxes/{id}/run-code").To(boxHandler.RunBox).
		Doc("run code in a box").
		Param(ws.PathParameter("id", "identifier of the box").DataType("string")).
		Reads(model.BoxRunCodeParams{}).
		Returns(200, "OK", model.BoxRunCodeResult{}).
		Returns(400, "Bad Request", model.BoxError{}).
		Returns(404, "Not Found", model.BoxError{}).
		Returns(500, "Internal Server Error", model.BoxError{}))

	ws.Route(ws.POST("/boxes/{id}/start").To(boxHandler.StartBox).
		Doc("start a stopped box").
		Param(ws.PathParameter("id", "identifier of the box").DataType("string")).
		Returns(200, "OK", model.BoxStartResult{}).
		Returns(400, "Bad Request", model.BoxError{}).
		Returns(404, "Not Found", model.BoxError{}).
		Returns(500, "Internal Server Error", model.BoxError{}))

	ws.Route(ws.POST("/boxes/{id}/stop").To(boxHandler.StopBox).
		Doc("stop a running box").
		Param(ws.PathParameter("id", "identifier of the box").DataType("string")).
		Returns(200, "OK", model.BoxStopResult{}).
		Returns(400, "Bad Request", model.BoxError{}).
		Returns(404, "Not Found", model.BoxError{}).
		Returns(500, "Internal Server Error", model.BoxError{}))

	// // WebSocket route for executing commands
	// ws.Route(ws.GET("/boxes/{id}/exec/ws").To(boxHandler.ExecBoxWS).
	// 	Doc("execute a command in a box via WebSocket").
	// 	Param(ws.PathParameter("id", "identifier of the box").DataType("string")).
	// 	Returns(400, "Bad Request", model.BoxError{}). // e.g., missing cmd parameter
	// 	Returns(404, "Not Found", model.BoxError{}).
	// 	Returns(500, "Internal Server Error", model.BoxError{})) // e.g., upgrade failed

	// // Box Archive Operations
	// ws.Route(ws.HEAD("/boxes/{id}/archive").To(boxHandler.HeadArchive).
	// 	Doc("get metadata about files in box").
	// 	Param(ws.PathParameter("id", "identifier of the box").DataType("string")).
	// 	Param(ws.QueryParameter("path", "path to get metadata from").DataType("string").Required(true)).
	// 	Returns(200, "OK", nil).
	// 	Returns(400, "Bad Request", model.BoxError{}).
	// 	Returns(404, "Not Found", model.BoxError{}).
	// 	Returns(500, "Internal Server Error", model.BoxError{}))

	// ws.Route(ws.GET("/boxes/{id}/archive").To(boxHandler.GetArchive).
	// 	Doc("get files from box as tar archive").
	// 	Param(ws.PathParameter("id", "identifier of the box").DataType("string")).
	// 	Param(ws.QueryParameter("path", "path to get files from").DataType("string").Required(true)).
	// 	Produces("application/x-tar").
	// 	Returns(200, "OK", nil).
	// 	Returns(400, "Bad Request", model.BoxError{}).
	// 	Returns(404, "Not Found", model.BoxError{}).
	// 	Returns(500, "Internal Server Error", model.BoxError{}))

	// ws.Route(ws.PUT("/boxes/{id}/archive").To(boxHandler.ExtractArchive).
	// 	Doc("extract tar archive to box").
	// 	Param(ws.PathParameter("id", "identifier of the box").DataType("string")).
	// 	Param(ws.QueryParameter("path", "path to extract files to").DataType("string").Required(true)).
	// 	Consumes("application/x-tar").
	// 	Returns(200, "OK", nil).
	// 	Returns(400, "Bad Request", model.BoxError{}).
	// 	Returns(404, "Not Found", model.BoxError{}).
	// 	Returns(500, "Internal Server Error", model.BoxError{}))

	// Box Filesystem Operations
	ws.Route(ws.GET("/boxes/{id}/fs/list").To(boxHandler.ListFiles).
		Doc("list files in a directory").
		Param(ws.PathParameter("id", "identifier of the box").DataType("string")).
		Param(ws.QueryParameter("path", "path to list files from").DataType("string").Required(false)).
		Param(ws.QueryParameter("depth", "depth of directory listing").DataType("number").Required(false)).
		Produces("application/json").
		Returns(200, "OK", model.BoxFileListResult{}).
		Returns(400, "Bad Request", model.BoxError{}).
		Returns(404, "Not Found", model.BoxError{}).
		Returns(500, "Internal Server Error", model.BoxError{}))

	ws.Route(ws.GET("/boxes/{id}/fs/read").To(boxHandler.ReadFile).
		Doc("read file content").
		Param(ws.PathParameter("id", "identifier of the box").DataType("string")).
		Param(ws.QueryParameter("path", "path to file to read").DataType("string").Required(true)).
		Produces("application/json").
		Returns(200, "OK", model.BoxFileReadResult{}).
		Returns(400, "Bad Request", model.BoxError{}).
		Returns(404, "Not Found", model.BoxError{}).
		Returns(500, "Internal Server Error", model.BoxError{}))

	ws.Route(ws.POST("/boxes/{id}/fs/write").To(boxHandler.WriteFile).
		Doc("write file content").
		Param(ws.PathParameter("id", "identifier of the box").DataType("string")).
		Reads(model.BoxFileWriteParams{}).
		Produces("application/json").
		Returns(200, "OK", model.BoxFileWriteResult{}).
		Returns(400, "Bad Request", model.BoxError{}).
		Returns(404, "Not Found", model.BoxError{}).
		Returns(500, "Internal Server Error", model.BoxError{}))

	// Image management operations - removed /boxes/images/update route as images are now managed by background service

	// these are only supported for cloud version
	ws.Route(ws.POST("/boxes/{id}/actions/click").To(boxHandler.BoxActionClick).
		Doc("click in a box").
		Param(ws.PathParameter("id", "identifier of the box").DataType("string")).
		Reads(model.BoxActionClickParams{}).
		Produces("application/json").
		Returns(200, "OK", model.BoxActionClickResult{}).
		Returns(500, "Internal Server Error", model.BoxError{}))

	ws.Route(ws.POST("/boxes/{id}/actions/drag").To(boxHandler.BoxActionDrag).
		Doc("drag in a box").
		Param(ws.PathParameter("id", "identifier of the box").DataType("string")).
		Reads(model.BoxActionDragParams{}).
		Produces("application/json").
		Returns(200, "OK", model.BoxActionDragResult{}).
		Returns(500, "Internal Server Error", model.BoxError{}))

	ws.Route(ws.POST("/boxes/{id}/actions/move").To(boxHandler.BoxActionMove).
		Doc("move in a box").
		Param(ws.PathParameter("id", "identifier of the box").DataType("string")).
		Reads(model.BoxActionMoveParams{}).
		Produces("application/json").
		Returns(200, "OK", model.BoxActionMoveResult{}).
		Returns(500, "Internal Server Error", model.BoxError{}))

	ws.Route(ws.POST("/boxes/{id}/actions/press").To(boxHandler.BoxActionPress).
		Doc("press in a box").
		Param(ws.PathParameter("id", "identifier of the box").DataType("string")).
		Reads(model.BoxActionPressParams{}).
		Produces("application/json").
		Returns(200, "OK", model.BoxActionPressResult{}).
		Returns(500, "Internal Server Error", model.BoxError{}))

	ws.Route(ws.POST("/boxes/{id}/actions/screenshot").To(boxHandler.BoxActionScreenshot).
		Doc("take a screenshot in a box").
		Param(ws.PathParameter("id", "identifier of the box").DataType("string")).
		Reads(model.BoxActionScreenshotParams{}).
		Produces("application/json").
		Returns(200, "OK", model.BoxActionScreenshotResult{}).
		Returns(500, "Internal Server Error", model.BoxError{}))

	ws.Route(ws.POST("/boxes/{id}/actions/scroll").To(boxHandler.BoxActionScroll).
		Doc("scroll in a box").
		Param(ws.PathParameter("id", "identifier of the box").DataType("string")).
		Reads(model.BoxActionScrollParams{}).
		Produces("application/json").
		Returns(200, "OK", model.BoxActionScrollResult{}).
		Returns(500, "Internal Server Error", model.BoxError{}))

	ws.Route(ws.POST("/boxes/{id}/actions/touch").To(boxHandler.BoxActionTouch).
		Doc("touch in a box").
		Param(ws.PathParameter("id", "identifier of the box").DataType("string")).
		Reads(model.BoxActionTouchParams{}).
		Produces("application/json").
		Returns(200, "OK", model.BoxActionTouchResult{}).
		Returns(500, "Internal Server Error", model.BoxError{}))

	ws.Route(ws.POST("/boxes/{id}/actions/type").To(boxHandler.BoxActionType).
		Doc("type in a box").
		Param(ws.PathParameter("id", "identifier of the box").DataType("string")).
		Reads(model.BoxActionTypeParams{}).
		Produces("application/json").
		Returns(200, "OK", model.BoxActionTypeResult{}).
		Returns(500, "Internal Server Error", model.BoxError{}))

}
