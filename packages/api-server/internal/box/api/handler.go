package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/babelcloud/gbox/packages/api-server/internal/box/service"
	model "github.com/babelcloud/gbox/packages/api-server/pkg/box"
	"github.com/babelcloud/gbox/packages/api-server/pkg/logger"

	"github.com/emicklei/go-restful/v3"
	"github.com/gorilla/websocket"
)

var log = logger.New()

// Local constants replacing models.MediaType*
const (
	mediaTypeRawStream         = "application/vnd.gbox.raw-stream"
	mediaTypeMultiplexedStream = "application/vnd.gbox.multiplexed-stream"
)

// Configure the WebSocket upgrader
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for now, adjust in production!
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// BoxHandler handles HTTP requests for box operations
type BoxHandler struct {
	service service.BoxService
}

// NewBoxHandler creates a new BoxHandler
func NewBoxHandler(service service.BoxService) *BoxHandler {
	return &BoxHandler{
		service: service,
	}
}

// streamServiceOperation is a helper function to handle streaming responses for service operations.
// serviceFunc is expected to write intermediate progress to the progressWriter and return the final data object on success.
func (h *BoxHandler) streamServiceOperation(
	req *restful.Request,
	resp *restful.Response,
	serviceCallParams interface{}, // Parameters to be passed to the serviceFunc
	serviceFunc func(ctx context.Context, params interface{}, progressWriter io.Writer) (finalData interface{}, err error),
	isCreateBox bool, // Flag to determine the final success message structure
) {
	resp.Header().Set("Content-Type", "application/json-stream")
	resp.Header().Set("X-Content-Type-Options", "nosniff")
	resp.Header().Set("Cache-Control", "no-cache")
	resp.Header().Set("Connection", "keep-alive")
	resp.WriteHeader(http.StatusOK)

	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()
		encoder := json.NewEncoder(pw)

		// serviceFunc will use pw for intermediate progress (e.g., image pull)
		// and return the final object or an error.
		finalData, err := serviceFunc(req.Request.Context(), serviceCallParams, pw)

		if err != nil {
			// Encode the final error message to the stream.
			// Intermediate errors (like pull failure) should have been written to pw by serviceFunc's components.
			errorMsg := struct {
				Status string `json:"status"`
				Error  string `json:"error"`
			}{Status: "error", Error: err.Error()}
			if encodeErr := encoder.Encode(errorMsg); encodeErr != nil {
				log.Errorf("Failed to encode error to stream: %v", encodeErr)
			}
			log.Debugf("Streaming operation failed: %v", err) // Log original error for server records
			return
		}

		// Encode the final success message.
		var successPayload interface{}
		if isCreateBox {
			successPayload = struct {
				Status string      `json:"status"`
				Box    interface{} `json:"box"`
			}{Status: "complete", Box: finalData}
		} else {
			// For UpdateBoxImage, finalData is already the complete *model.ImageUpdateResponse
			successPayload = finalData
		}

		if err := encoder.Encode(successPayload); err != nil {
			log.Errorf("Failed to encode success payload to stream: %v", err)
		}
	}()

	// Copy from pipe to response
	if _, err := io.Copy(resp.ResponseWriter, pr); err != nil {
		log.Errorf("Error copying stream to HTTP response: %v", err)
	}
}

// ListBoxes returns all boxes
func (h *BoxHandler) ListBoxes(req *restful.Request, resp *restful.Response) {
	// Parse query parameters into BoxListParams
	params := &model.BoxListParams{}

	// Get filters from query parameters
	queryFilters := req.QueryParameters("filter")
	for _, filter := range queryFilters {
		// Parse filter format: field=value
		parts := strings.SplitN(filter, "=", 2)
		if len(parts) != 2 {
			continue
		}
		params.Filters = append(params.Filters, model.Filter{
			Field:    parts[0],
			Operator: model.FilterOperatorEquals,
			Value:    parts[1],
		})
	}

	result, err := h.service.List(req.Request.Context(), params)
	if err != nil {
		writeError(resp, http.StatusInternalServerError, "ListBoxesError", err.Error())
		return
	}

	resp.WriteEntity(result)
}

// GetBox returns a box by ID
func (h *BoxHandler) GetBox(req *restful.Request, resp *restful.Response) {
	boxID := req.PathParameter("id")
	box, err := h.service.Get(req.Request.Context(), boxID)
	if err != nil {
		if err == service.ErrBoxNotFound {
			writeError(resp, http.StatusNotFound, "BoxNotFound", err.Error())
			return
		}
		writeError(resp, http.StatusInternalServerError, "GetBoxError", err.Error())
		return
	}

	resp.WriteEntity(box)
}

// CreateBox creates a new box
//
// If a timeout query parameter is provided and the image doesn't exist locally,
// the API will pull the image asynchronously and return a 202 response
// after the timeout, informing the user that the image is being pulled.
// The user can retry creating the box later.
//
// Accepts the following query parameters:
// - timeout: Image pull timeout duration, e.g. "30s", "1m", etc. If not provided, it will wait for the image pull to complete.
func (h *BoxHandler) CreateBox(req *restful.Request, resp *restful.Response) {
	// Read request body directly into the internal model type
	var createParams model.BoxCreateParams
	if err := req.ReadEntity(&createParams); err != nil {
		writeError(resp, http.StatusBadRequest, "InvalidRequest", err.Error())
		return
	}

	// Parse timeout parameter, default to 0 (wait indefinitely)
	timeoutStr := req.QueryParameter("timeout")
	if timeoutStr != "" {
		var err error
		createParams.Timeout, err = time.ParseDuration(timeoutStr)
		if err != nil {
			writeError(resp, http.StatusBadRequest, "InvalidTimeout", "Invalid timeout format. Use a valid duration (e.g., 30s, 1m)")
			return
		}
	}

	// check if the client wants a stream response
	acceptHeader := req.HeaderParameter("Accept")
	streamRequest := acceptHeader == "application/json-stream"

	if streamRequest {
		// Define the service call for CreateBox compatible with streamServiceOperation
		createBoxServiceCall := func(ctx context.Context, params interface{}, progressWriter io.Writer) (interface{}, error) {
			cp, ok := params.(*model.BoxCreateParams)
			if !ok {
				return nil, fmt.Errorf("internal error: invalid params type for CreateBox service call")
			}
			// h.service.Create will use progressWriter for image pull progress (if any)
			// and return the created Box object or an error.
			return h.service.Create(ctx, cp, progressWriter)
		}
		h.streamServiceOperation(req, resp, &createParams, createBoxServiceCall, true)
		return
	}

	// If timeout is specified, use async pull with timeout
	if createParams.Timeout > 0 {
		// Check if image exists locally first
		imageExists, imageName := h.service.CheckImageExists(req.Request.Context(), &createParams)

		if !imageExists {
			// Start async pull if image doesn't exist
			h.service.EnsureImagePulling(req.Request.Context(), imageName)

			// Set a timeout context
			ctx, cancel := context.WithTimeout(req.Request.Context(), createParams.Timeout)
			defer cancel()

			// Wait for image to be pulled or timeout
			select {
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					// If timeout, return a message that pull is in progress
					log.Infof("Image pull for %s timed out after %s. Sending 202 ImagePullInProgress.", imageName, createParams.Timeout.String())
					resp.WriteHeaderAndEntity(http.StatusAccepted, &model.BoxError{
						Code:    "ImagePullInProgress",
						Message: fmt.Sprintf("Image %s is being pulled, but not yet completed. Please try again later", imageName),
					})
					return
				}
			case <-h.service.WaitForImagePull(imageName):
				// Image pull completed, continue with box creation
			}
		}
	}

	// standard JSON response (non-streaming) - wait for operation to complete
	box, err := h.service.Create(req.Request.Context(), &createParams, nil)
	if err != nil {
		writeError(resp, http.StatusInternalServerError, "CreateBoxError", err.Error())
		return
	}

	resp.WriteHeaderAndEntity(http.StatusCreated, box)
}

func (h *BoxHandler) CreateLinuxBox(req *restful.Request, resp *restful.Response) {
	// Read request body directly into the internal model type
	var createParams model.LinuxBoxCreateParam
	if err := req.ReadEntity(&createParams); err != nil {
		writeError(resp, http.StatusBadRequest, "InvalidRequest", err.Error())
		return
	}

	// check if the client wants a stream response
	acceptHeader := req.HeaderParameter("Accept")
	streamRequest := acceptHeader == "application/json-stream"

	if streamRequest {
		// Define the service call for CreateLinuxBox compatible with streamServiceOperation
		createLinuxBoxServiceCall := func(ctx context.Context, params interface{}, progressWriter io.Writer) (interface{}, error) {
			cp, ok := params.(*model.LinuxBoxCreateParam)
			if !ok {
				return nil, fmt.Errorf("internal error: invalid params type for CreateLinuxBox service call")
			}
			return h.service.CreateLinuxBox(ctx, cp, progressWriter)
		}
		h.streamServiceOperation(req, resp, &createParams, createLinuxBoxServiceCall, true)
		return
	}

	// standard JSON response (non-streaming) - wait for operation to complete
	box, err := h.service.CreateLinuxBox(req.Request.Context(), &createParams, nil)
	if err != nil {
		writeError(resp, http.StatusInternalServerError, "CreateLinuxBoxError", err.Error())
		return
	}

	resp.WriteHeaderAndEntity(http.StatusCreated, box)
}

func (h *BoxHandler) CreateAndroidBox(req *restful.Request, resp *restful.Response) {
	writeError(resp, http.StatusNotImplemented, "NotImplemented", "Android box creation is not supported yet, please use the cloud version")
}

// DeleteBox deletes a box by ID
func (h *BoxHandler) DeleteBox(req *restful.Request, resp *restful.Response) {
	boxID := req.PathParameter("id")
	if boxID == "" {
		writeError(resp, http.StatusBadRequest, "InvalidRequest", "Box ID is required")
		return
	}

	// Parse request body directly into model.BoxDeleteParams
	var deleteParams model.BoxDeleteParams
	if err := req.ReadEntity(&deleteParams); err != nil {
		deleteParams.Force = false // Default
	}

	result, err := h.service.Delete(req.Request.Context(), boxID, &deleteParams)
	if err != nil {
		if err == service.ErrBoxNotFound {
			writeError(resp, http.StatusNotFound, "BoxNotFound", err.Error())
			return
		}
		writeError(resp, http.StatusInternalServerError, "DeleteBoxError", err.Error())
		return
	}
	// Write the internal model.BoxDeleteResult directly
	resp.WriteHeaderAndEntity(http.StatusOK, result)
}

// DeleteBoxes deletes all boxes
func (h *BoxHandler) DeleteBoxes(req *restful.Request, resp *restful.Response) {
	// Parse request body directly into model.BoxesDeleteParams
	var deleteParams model.BoxesDeleteParams
	if err := req.ReadEntity(&deleteParams); err != nil {
		// If no request body or error parsing, continue with default force=false
		deleteParams.Force = false // Default
	}

	result, err := h.service.DeleteAll(req.Request.Context(), &deleteParams)
	if err != nil {
		writeError(resp, http.StatusInternalServerError, "DeleteBoxesError", err.Error())
		return
	}
	// Write the internal model.BoxesDeleteResult directly
	resp.WriteHeaderAndEntity(http.StatusOK, result)
}

// ReclaimBoxes reclaims inactive boxes
func (h *BoxHandler) ReclaimBoxes(req *restful.Request, resp *restful.Response) {
	result, err := h.service.Reclaim(req.Request.Context())
	if err != nil {
		writeError(resp, http.StatusInternalServerError, "ReclaimBoxesError", err.Error())
		return
	}
	// Write the internal model.BoxReclaimResult directly
	resp.WriteHeaderAndEntity(http.StatusOK, result)
}

// ExecBox handles command execution via HTTP Hijacking (existing method)
func (h *BoxHandler) ExecBox(req *restful.Request, resp *restful.Response) {
	boxID := req.PathParameter("id")

	// Get Box status first
	box, err := h.service.Get(req.Request.Context(), boxID)
	if err != nil {
		if err == service.ErrBoxNotFound {
			writeError(resp, http.StatusNotFound, "BoxNotFound", err.Error())
			return
		}
		writeError(resp, http.StatusInternalServerError, "GetBoxError", fmt.Sprintf("Failed to get box status: %v", err))
		return
	}

	// Check if the box is running
	if box.Status != "running" {
		log.Warnf("ExecBox: Box %s is not running (state: %s). Returning 409 Conflict.", boxID, box.Status)
		writeError(resp, http.StatusConflict, "BoxNotRunning", fmt.Sprintf("Box %s is not running (state: %s), please start it first", boxID, box.Status))
		return
	}

	// Read request body directly into model.BoxExecParams
	var execReq model.BoxExecParams
	if err := req.ReadEntity(&execReq); err != nil {
		writeError(resp, http.StatusBadRequest, "InvalidRequest", err.Error())
		return
	}

	// Hijack the connection logic remains the same...
	upgrade := req.HeaderParameter("Upgrade")
	connection := req.HeaderParameter("Connection")
	accept := req.HeaderParameter("Accept")
	if accept == "" {
		accept = mediaTypeMultiplexedStream // Use local constant
	}

	if accept != mediaTypeRawStream && accept != mediaTypeMultiplexedStream {
		writeError(resp, http.StatusNotAcceptable, "UnsupportedMediaType", fmt.Sprintf("Unsupported Accept header: %s", accept))
		return
	}

	httpResp := resp.ResponseWriter
	hijacker, ok := httpResp.(http.Hijacker)
	if !ok {
		writeError(resp, http.StatusInternalServerError, "HijackError", "response does not support hijacking")
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		// Cannot write error after potential partial hijack, just log
		log.Errorf("ExecBox [%s]: Failed to hijack connection: %v", boxID, err)
		return
	}
	defer clientConn.Close()

	// Set the connection in the execReq
	execReq.Conn = clientConn

	// Write HTTP response headers directly to the hijacked connection
	writeResponseHeaders(execReq.Conn, upgrade, connection, execReq.TTY)

	// Execute command and handle streaming using the original service method
	result, err := h.service.Exec(req.Request.Context(), boxID, &execReq)
	if err != nil {
		// Cannot write standard error after hijack, just log
		if err == service.ErrBoxNotFound {
			log.Errorf("ExecBox [%s]: Box not found during exec: %v", boxID, err)
		} else {
			log.Errorf("ExecBox [%s]: Error during hijacked exec: %v", boxID, err)
		}
		// Connection will be closed by defer
		return
	}

	// Log the exit code, cannot write response entity after hijack
	if result != nil {
		log.Infof("ExecBox [%s]: Hijacked command finished with exit code: %d", boxID, result.ExitCode)
	} else {
		log.Warnf("ExecBox [%s]: Hijacked command finished but result was nil", boxID)
	}
}

// ExecBoxWS handles command execution via WebSocket
func (h *BoxHandler) ExecBoxWS(req *restful.Request, resp *restful.Response) {
	boxID := req.PathParameter("id")

	// --- Parameter Parsing from Query ---
	// Example: /ws/boxes/{id}/exec?cmd=bash&arg=-c&arg=ls%20-l&tty=true&workingDir=/path/to/working
	queryParams := req.Request.URL.Query()
	cmd := queryParams["cmd"]                      // Returns a slice
	args := queryParams["arg"]                     // Returns a slice for multiple 'arg' params
	ttyStr := queryParams.Get("tty")               // Get single value
	workingDirStr := queryParams.Get("workingDir") // Get working directory

	if len(cmd) == 0 {
		// Use http.Error for upgrade failures before connection is established
		http.Error(resp.ResponseWriter, "Missing 'cmd' query parameter", http.StatusBadRequest)
		return
	}

	tty := false
	if ttyStr != "" {
		var err error
		tty, err = strconv.ParseBool(ttyStr)
		if err != nil {
			http.Error(resp.ResponseWriter, "Invalid 'tty' query parameter, must be true or false", http.StatusBadRequest)
			return
		}
	}

	execParams := &model.BoxExecWSParams{
		Cmd:        cmd, // Use the first command element
		Args:       args,
		TTY:        tty,
		WorkingDir: workingDirStr, // Set working directory
	}
	//-------------------------------------

	// Upgrade HTTP connection to WebSocket
	wsConn, err := upgrader.Upgrade(resp.ResponseWriter, req.Request, nil)
	if err != nil {
		// Upgrade writes error response itself
		log.Errorf("ExecBoxWS [%s]: Failed to upgrade connection: %v", boxID, err)
		// Don't writeError here, upgrader handles it.
		return
	}
	defer wsConn.Close()

	log.Infof("ExecBoxWS [%s]: WebSocket connection established. TTY: %v, Cmd: %v, Args: %v, WorkingDir: %s", boxID, tty, cmd, args, workingDirStr)

	// Execute command using the WebSocket service method
	// Use context from the original request
	result, err := h.service.ExecWS(req.Request.Context(), boxID, execParams, wsConn)

	if err != nil {
		// Log error. Cannot easily send structured error over WS after exec starts/fails mid-stream.
		// The service layer ExecWS might attempt to send a final error/exit message.
		log.Errorf("ExecBoxWS [%s]: Error during WebSocket exec: %v", boxID, err)
		// Connection will be closed by defer wsConn.Close()
		// Optionally send a specific WebSocket close message with error code?
		// wsConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, err.Error()))
		return
	}

	// Log the successful exit code (service might have already sent it via WS)
	if result != nil {
		log.Infof("ExecBoxWS [%s]: WebSocket command finished with exit code: %d", boxID, result.ExitCode)
	} else {
		// Should ideally not happen if error is nil
		log.Warnf("ExecBoxWS [%s]: WebSocket command finished without error but result was nil", boxID)
	}
	// Final WebSocket closure is handled by defer
}

// RunBox runs a command in a box
func (h *BoxHandler) RunBox(req *restful.Request, resp *restful.Response) {
	boxID := req.PathParameter("id")
	// Read request body directly into model.BoxRunParams
	var runReq model.BoxRunParams
	if err := req.ReadEntity(&runReq); err != nil {
		writeError(resp, http.StatusBadRequest, "InvalidRequest", err.Error())
		return
	}

	result, err := h.service.Run(req.Request.Context(), boxID, &runReq)
	if err != nil {
		if err == service.ErrBoxNotFound {
			writeError(resp, http.StatusNotFound, "BoxNotFound", err.Error())
			return
		}
		writeError(resp, http.StatusInternalServerError, "RunBoxError", err.Error())
		return
	}

	// Write the internal model.BoxRunResult directly
	resp.WriteEntity(result)
}

// StartBox starts a stopped box
func (h *BoxHandler) StartBox(req *restful.Request, resp *restful.Response) {
	boxID := req.PathParameter("id")
	result, err := h.service.Start(req.Request.Context(), boxID)
	if err != nil {
		if err == service.ErrBoxNotFound {
			writeError(resp, http.StatusNotFound, "BoxNotFound", err.Error())
			return
		}
		writeError(resp, http.StatusInternalServerError, "StartBoxError", err.Error())
		return
	}
	// Write the internal model.BoxStartResult directly
	resp.WriteHeaderAndEntity(http.StatusOK, result)
}

// StopBox stops a running box
func (h *BoxHandler) StopBox(req *restful.Request, resp *restful.Response) {
	boxID := req.PathParameter("id")
	result, err := h.service.Stop(req.Request.Context(), boxID)
	if err != nil {
		if err == service.ErrBoxNotFound {
			writeError(resp, http.StatusNotFound, "BoxNotFound", err.Error())
			return
		}
		writeError(resp, http.StatusInternalServerError, "StopBoxError", err.Error())
		return
	}
	// Write the internal model.BoxStopResult directly
	resp.WriteHeaderAndEntity(http.StatusOK, result)
}

// GetArchive gets files from box as tar archive
func (h *BoxHandler) GetArchive(req *restful.Request, resp *restful.Response) {
	boxID := req.PathParameter("id")
	path := req.QueryParameter("path")

	archiveReq := &model.BoxArchiveGetParams{
		Path: path,
	}

	archiveResp, archive, err := h.service.GetArchive(req.Request.Context(), boxID, archiveReq)
	if err != nil {
		if err == service.ErrBoxNotFound {
			writeError(resp, http.StatusNotFound, "BoxNotFound", err.Error())
			return
		}
		writeError(resp, http.StatusInternalServerError, "GetArchiveError", err.Error())
		return
	}
	defer archive.Close()

	// Convert actual archiveResp to JSON string
	statJSON, err := json.Marshal(archiveResp) // Use actual archiveResp
	if err != nil {
		writeError(resp, http.StatusInternalServerError, "GetArchiveError", fmt.Sprintf("Failed to marshal stat: %v", err))
		return
	}

	// Set response headers
	resp.Header().Set("Content-Type", "application/x-tar")
	resp.Header().Set("X-Gbox-Path-Stat", string(statJSON))
	resp.Header().Set("Last-Modified", archiveResp.Mtime) // Use actual Mtime

	// Copy the archive to response
	_, err = io.Copy(resp.ResponseWriter, archive)

	if err != nil {
		// Log the error, but don't try to writeError as headers might have been sent
		log.Errorf("Failed to copy archive to response for box %s, path %s: %v", boxID, path, err)
		return
	}
}

// HeadArchive gets metadata about files in box
func (h *BoxHandler) HeadArchive(req *restful.Request, resp *restful.Response) {
	boxID := req.PathParameter("id")
	path := req.QueryParameter("path")

	headReq := &model.BoxArchiveHeadParams{
		Path: path,
	}

	stat, err := h.service.HeadArchive(req.Request.Context(), boxID, headReq)
	if err != nil {
		if err == service.ErrBoxNotFound {
			writeError(resp, http.StatusNotFound, "BoxNotFound", err.Error())
			return
		}
		writeError(resp, http.StatusInternalServerError, "HeadArchiveError", err.Error())
		return
	}

	// Convert actual stat to JSON string
	statJSON, err := json.Marshal(stat) // Use actual stat
	if err != nil {
		writeError(resp, http.StatusInternalServerError, "HeadArchiveError", fmt.Sprintf("Failed to marshal stat: %v", err))
		return
	}

	// Set response headers
	resp.Header().Set("Content-Type", "application/json")
	resp.Header().Set("X-Gbox-Path-Stat", string(statJSON))
	resp.WriteHeader(http.StatusOK)
}

// ExtractArchive extracts tar archive to box
func (h *BoxHandler) ExtractArchive(req *restful.Request, resp *restful.Response) {
	boxID := req.PathParameter("id")
	path := req.QueryParameter("path")

	// Read request body
	content, err := io.ReadAll(req.Request.Body)
	if err != nil {
		writeError(resp, http.StatusBadRequest, "InvalidRequest", "Failed to read request body")
		return
	}

	// Use actual params
	extractParams := &model.BoxArchiveExtractParams{
		Path:    path,
		Content: content,
	}

	// Call actual service method
	if err := h.service.ExtractArchive(req.Request.Context(), boxID, extractParams); err != nil {
		if err == service.ErrBoxNotFound {
			writeError(resp, http.StatusNotFound, "BoxNotFound", err.Error())
			return
		}
		writeError(resp, http.StatusInternalServerError, "ExtractArchiveError", err.Error())
		return
	}
	resp.WriteHeader(http.StatusOK)
}

// UpdateBoxImage updates docker images used for boxes, pulling latest and removing outdated versions
func (h *BoxHandler) UpdateBoxImage(req *restful.Request, resp *restful.Response) {
	// parse query parameters
	params := &model.ImageUpdateParams{
		ImageReference: req.QueryParameter("image"),
		DryRun:         req.QueryParameter("dryRun") == "true",
		Force:          req.QueryParameter("force") == "true",
	}

	// check if the client wants a stream response
	acceptHeader := req.HeaderParameter("Accept")
	wantsStreamResponse := acceptHeader == "application/json-stream"

	if wantsStreamResponse && !params.DryRun {
		// Define the service call for UpdateBoxImage compatible with streamServiceOperation
		updateImageServiceCall := func(ctx context.Context, p interface{}, progressWriter io.Writer) (interface{}, error) {
			updateParams, ok := p.(*model.ImageUpdateParams)
			if !ok {
				return nil, fmt.Errorf("internal error: invalid params type for UpdateImage service call")
			}
			// h.service.UpdateBoxImageWithProgress will use progressWriter for image pull progress
			// and return the ImageUpdateResponse object or an error.
			return h.service.UpdateBoxImageWithProgress(ctx, updateParams, progressWriter)
		}
		h.streamServiceOperation(req, resp, params, updateImageServiceCall, false)
		return
	}

	// standard JSON response (non-streaming)
	result, err := h.service.UpdateBoxImage(req.Request.Context(), params)
	if err != nil {
		writeError(resp, http.StatusInternalServerError, "UpdateImageError", err.Error())
		return
	}

	// return result
	resp.WriteEntity(result)
}

func (h *BoxHandler) BoxActionClick(req *restful.Request, resp *restful.Response) {
	writeError(resp, http.StatusNotImplemented, "NotImplemented", "BoxActionClick endpoint is not implemented yet")
}

func (h *BoxHandler) BoxActionDrag(req *restful.Request, resp *restful.Response) {
	writeError(resp, http.StatusNotImplemented, "NotImplemented", "BoxActionDrag endpoint is not implemented yet")
}

func (h *BoxHandler) BoxActionMove(req *restful.Request, resp *restful.Response) {
	writeError(resp, http.StatusNotImplemented, "NotImplemented", "BoxActionMove endpoint is not implemented yet")
}

func (h *BoxHandler) BoxActionPress(req *restful.Request, resp *restful.Response) {
	writeError(resp, http.StatusNotImplemented, "NotImplemented", "BoxActionPress endpoint is not implemented yet")
}

func (h *BoxHandler) BoxActionScreenshot(req *restful.Request, resp *restful.Response) {
	writeError(resp, http.StatusNotImplemented, "NotImplemented", "BoxActionScreenshot endpoint is not implemented yet")
}

func (h *BoxHandler) BoxActionScroll(req *restful.Request, resp *restful.Response) {
	writeError(resp, http.StatusNotImplemented, "NotImplemented", "BoxActionScroll endpoint is not implemented yet")
}

func (h *BoxHandler) BoxActionTouch(req *restful.Request, resp *restful.Response) {
	writeError(resp, http.StatusNotImplemented, "NotImplemented", "BoxActionTouch endpoint is not implemented yet")
}

func (h *BoxHandler) BoxActionType(req *restful.Request, resp *restful.Response) {
	writeError(resp, http.StatusNotImplemented, "NotImplemented", "BoxActionType endpoint is not implemented yet")
}

// writeError writes an error response using local apiError struct
func writeError(resp *restful.Response, status int, code, message string) {
	// Ensure headers aren't already written (e.g., after hijack or partial response)
	// A simple check, might not be perfectly robust for all edge cases.
	if resp.ResponseWriter.Header().Get("written") != "true" {
		// Mark headers as written to prevent double writes
		resp.Header().Set("written", "true")
		resp.WriteHeaderAndEntity(status, &model.BoxError{
			Code:    code,
			Message: message,
		})
	} else {
		log.Warnf("Attempted to write error after headers were sent. Status: %d, Code: %s, Msg: %s", status, code, message)
	}
}

// writeResponseHeaders writes HTTP response headers for Hijacked connection
func writeResponseHeaders(w io.Writer, upgrade, connection string, tty bool) {
	if upgrade == "tcp" && connection == "Upgrade" {
		fmt.Fprintf(w, "HTTP/1.1 101 UPGRADED\r\n")
		fmt.Fprintf(w, "Content-Type: %s\r\n", getContentType(tty))
		fmt.Fprintf(w, "Connection: Upgrade\r\n")
		fmt.Fprintf(w, "Upgrade: tcp\r\n")
		log.Debugf("Protocol upgrade requested, using %s", getStreamType(tty))
	} else {
		fmt.Fprintf(w, "HTTP/1.1 200 OK\r\n")
		fmt.Fprintf(w, "Content-Type: %s\r\n", getContentType(tty))
		log.Debugf("No protocol upgrade requested, using %s", getStreamType(tty))
	}
	fmt.Fprintf(w, "\r\n")

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// getContentType returns the appropriate content type based on TTY status
func getContentType(tty bool) string {
	if tty {
		return mediaTypeRawStream
	}
	return mediaTypeMultiplexedStream
}

// getStreamType returns a human-readable stream type description
func getStreamType(tty bool) string {
	if tty {
		return "raw stream (TTY mode)"
	}
	return "multiplexed stream (non-TTY mode)"
}
