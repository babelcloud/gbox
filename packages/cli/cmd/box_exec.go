package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/internal/profile"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// BoxExecOptions holds command options
type BoxExecOptions struct {
	Interactive bool
	Tty         bool
	BoxID       string
	Command     []string
	WorkingDir  string
}

// NewBoxExecCommand creates a new box exec command
func NewBoxExecCommand() *cobra.Command {
	opts := &BoxExecOptions{}

	cmd := &cobra.Command{
		Use:   "exec [box-id] -- [command] [args...]",
		Short: "Execute a command in a box",
		Long: `usage: gbox-box-exec [-h] [-i] [-t] box_id

Execute a command in a box

positional arguments:
  box_id             ID of the box

options:
  -h, --help         show this help message and exit
  -i, --interactive  Enable interactive mode (with stdin)
  -t, --tty          Force TTY allocation`,
		Example: `    gbox box exec 550e8400-e29b-41d4-a716-446655440000 -- ls -l     # List files in box
    gbox box exec 550e8400-e29b-41d4-a716-446655440000 -t -- bash     # Run interactive bash
    gbox box exec 550e8400-e29b-41d4-a716-446655440000 -i -- cat       # Run cat with stdin`,
		RunE: func(cmd *cobra.Command, args []string) error {
			argsLenAtDash := cmd.ArgsLenAtDash()
			if argsLenAtDash == -1 {
				return fmt.Errorf("command must be specified after '--'")
			}

			if len(args) == 0 || argsLenAtDash == 0 {
				cmd.Help()
				return fmt.Errorf("box ID is required")
			}

			// Get box ID (first argument before --)
			opts.BoxID = args[0]

			// Get command (all arguments after --)
			if argsLenAtDash >= len(args) {
				return fmt.Errorf("command must be specified after '--'")
			}
			opts.Command = args[argsLenAtDash:]

			// Run the command
			return runExec(opts)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			// Only complete the first argument (box-id)
			if len(args) == 0 {
				return completeBoxIDs(cmd, args, toComplete)
			}
			// No completion for subsequent arguments before -- or anything after --
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
	}

	// Add flags
	cmd.Flags().BoolVarP(&opts.Interactive, "interactive", "i", false, "Enable interactive mode (with stdin)")
	cmd.Flags().BoolVarP(&opts.Tty, "tty", "t", false, "Force TTY allocation")
	cmd.Flags().StringVarP(&opts.WorkingDir, "workdir", "w", "", "Working directory inside the container")

	return cmd
}

// runExec implements the exec command functionality
func runExec(opts *BoxExecOptions) error {
	// Resolve box ID prefix from opts.BoxID
	resolvedBoxID, _, err := ResolveBoxIDPrefix(opts.BoxID)
	if err != nil {
		return fmt.Errorf("failed to resolve box ID: %w", err)
	}
	// unified interaction with cloud through WebSocket
	return runExecWebSocket(opts, resolvedBoxID)
}

// runExecWebSocket executes interactive commands through new WebSocket API
func runExecWebSocket(opts *BoxExecOptions, resolvedBoxID string) error {
	pm := profile.NewProfileManager()
	if err := pm.Load(); err != nil {
		// handle error, maybe default to cloud
	}
	// Get effective base URL from profile with priority: GBOX_BASE_URL > profile > default
	effectiveBaseURL := profile.Default.GetEffectiveBaseURL()
	apiBase := effectiveBaseURL

	// convert http(s):// to ws(s)://
	wsBase := apiBase
	if strings.HasPrefix(apiBase, "https://") {
		wsBase = "wss://" + strings.TrimPrefix(apiBase, "https://")
	} else if strings.HasPrefix(apiBase, "http://") {
		wsBase = "ws://" + strings.TrimPrefix(apiBase, "http://")
	}

	wsURL := fmt.Sprintf("%s/boxes/%s/exec", wsBase, resolvedBoxID)

	// parse URL to ensure validity
	parsedURL, err := url.Parse(wsURL)
	if err != nil {
		return fmt.Errorf("invalid websocket url: %v", err)
	}

	headers := http.Header{}
	// Try to set API Key header if available
	apiKey := os.Getenv("GBOX_API_KEY")
	if apiKey == "" {
		// pm is already initialized
		if cur := pm.GetCurrent(); cur != nil {
			apiKey = cur.APIKey
		}
	}
	if apiKey != "" {
		headers.Set("X-API-Key", apiKey)
	}

	conn, _, err := websocket.DefaultDialer.Dial(parsedURL.String(), headers)
	if err != nil {
		return fmt.Errorf("failed to connect websocket: %v", err)
	}
	defer conn.Close()

	// send initialization command
	interactive := opts.Interactive || opts.Tty
	initPayload := map[string]interface{}{
		"command": map[string]interface{}{
			"commands":    opts.Command,
			"interactive": interactive,
			"workingDir":  opts.WorkingDir,
		},
	}
	// TODO If workingDir is not exists, it should be created by the server.
	if err := conn.WriteJSON(initPayload); err != nil {
		return fmt.Errorf("failed to send init payload: %v", err)
	}

	// if TTY is enabled, switch terminal to raw mode
	var oldState *term.State
	if opts.Tty {
		state, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			return fmt.Errorf("failed to set terminal raw mode: %v", err)
		}
		oldState = state
		defer term.Restore(int(os.Stdin.Fd()), oldState)
	}

	errChan := make(chan error, 2)
	// read remote output
	go func() {
		for {
			msgType, data, err := conn.ReadMessage()
			if err != nil {
				// Treat normal close codes as EOF to avoid noisy error message
				if websocket.IsCloseError(err,
					websocket.CloseNormalClosure,    // 1000
					websocket.CloseGoingAway,        // 1001
					websocket.CloseNoStatusReceived, // 1005
					websocket.CloseAbnormalClosure,  // 1006
				) {
					errChan <- io.EOF
				} else {
					errChan <- err
				}
				return
			}

			switch msgType {
			case websocket.TextMessage:
				// try to parse as JSON event
				var evt struct {
					Event   string `json:"event"`
					Data    string `json:"data"`
					Message string `json:"message"`
				}
				if jsonErr := json.Unmarshal(data, &evt); jsonErr == nil && evt.Event != "" {
					switch evt.Event {
					case "stdout":
						os.Stdout.Write([]byte(evt.Data))
					case "stderr":
						os.Stderr.Write([]byte(evt.Data))
					case "end":
						errChan <- io.EOF
						return
					case "error":
						errChan <- fmt.Errorf("%s", evt.Message)
						return
					default:
						os.Stdout.Write(data)
					}
				} else {
					os.Stdout.Write(data)
				}
			case websocket.BinaryMessage:
				// write directly to stdout
				os.Stdout.Write(data)
			}
		}
	}()

	// send local input
	if interactive {
		go func() {
			buffer := make([]byte, 1024)
			for {
				n, err := os.Stdin.Read(buffer)
				if n > 0 {
					if writeErr := conn.WriteMessage(websocket.BinaryMessage, buffer[:n]); writeErr != nil {
						if websocket.IsCloseError(writeErr,
							websocket.CloseNormalClosure,
							websocket.CloseGoingAway,
							websocket.CloseNoStatusReceived,
							websocket.CloseAbnormalClosure,
						) {
							errChan <- io.EOF
						} else {
							errChan <- writeErr
						}
						return
					}
				}
				if err != nil {
					if err != io.EOF {
						errChan <- err
					} else {
						// normal EOF, send close frame
						conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
					}
					return
				}
			}
		}()
	}

	// wait for any goroutine to finish
	err = <-errChan
	if err == io.EOF {
		return nil
	}
	return err
}
