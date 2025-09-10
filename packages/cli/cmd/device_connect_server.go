package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect"
	"github.com/spf13/cobra"
)

// NewDeviceConnectServerCommand creates the start-server subcommand
func NewDeviceConnectServerCommand() *cobra.Command {
	var port int
	
	cmd := &cobra.Command{
		Use:    "start-server",
		Short:  "Start the native device proxy server",
		Hidden: true, // Hidden command for internal use
		RunE: func(cmd *cobra.Command, args []string) error {
			// This command is called by the daemon process
			log.Printf("Starting native device proxy server on port %d", port)
			
			server := device_connect.NewServer(port)
			if err := server.Start(); err != nil {
				return fmt.Errorf("failed to start server: %v", err)
			}
			
			// Wait for interrupt signal
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			<-sigChan
			
			log.Println("Shutting down server...")
			if err := server.Stop(); err != nil {
				log.Printf("Error stopping server: %v", err)
			}
			
			return nil
		},
	}
	
	cmd.Flags().IntVar(&port, "port", device_connect.DefaultPort, "Server port")
	
	return cmd
}