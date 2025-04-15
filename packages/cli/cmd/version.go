package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/template"

	"github.com/babelcloud/gbox/packages/cli/internal/version"
	"github.com/spf13/cobra"
)

// VersionOptions holds command options
type VersionOptions struct {
	OutputFormat string
	ShortFormat  bool
}

// NewVersionCommand creates a new version command
func NewVersionCommand() *cobra.Command {
	opts := &VersionOptions{}

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the client and server version information",
		Long:  `Display detailed version information about the GBOX client and server`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If --version flag was specified, show only the client version
			if cmd.Flag("version").Changed {
				opts.ShortFormat = true
			}
			return runVersion(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.OutputFormat, "output", "text", "Output format (json or text)")
	flags.BoolVarP(&opts.ShortFormat, "version", "v", false, "Print only the client version number")

	cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json", "text"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

// runVersion executes the version command logic
func runVersion(opts *VersionOptions) error {
	clientInfo := version.ClientInfo()

	// If short format requested, just print the version and exit
	if opts.ShortFormat {
		fmt.Printf("GBOX version %s, build %s\n", clientInfo["Version"], clientInfo["GitCommit"])
		return nil
	}

	// Try to get server info but don't fail if server is not available
	serverInfo, serverErr := version.GetServerInfo()

	if opts.OutputFormat == "json" {
		result := map[string]interface{}{
			"Client": clientInfo,
		}

		if serverErr == nil {
			result["Server"] = serverInfo
		} else {
			result["Server"] = map[string]string{
				"Error": serverErr.Error(),
			}
		}

		jsonData, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format version as JSON: %v", err)
		}
		fmt.Println(string(jsonData))
		return nil
	}

	// Text template for client version output
	const clientTemplate = `Client:
 Version:           {{.Version}}
 API version:       {{.APIVersion}}
 Go version:        {{.GoVersion}}
 Git commit:        {{.GitCommit}}
 Built:             {{.FormattedTime}}
 OS/Arch:           {{.OS}}/{{.Arch}}
`

	tmpl, err := template.New("version").Parse(clientTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse version template: %v", err)
	}

	err = tmpl.Execute(os.Stdout, clientInfo)
	if err != nil {
		return err
	}

	// If server info is available, display it
	if serverErr == nil {
		fmt.Println()

		const serverTemplate = `Server:
 Version:           {{.Version}}
 API version:       {{.APIVersion}}
 Go version:        {{.GoVersion}}
 Git commit:        {{.GitCommit}}
 Built:             {{.FormattedTime}}
 OS/Arch:           {{.OS}}/{{.Arch}}
`

		tmpl, err = template.New("server").Parse(serverTemplate)
		if err != nil {
			return fmt.Errorf("failed to parse server template: %v", err)
		}

		return tmpl.Execute(os.Stdout, serverInfo)
	} else {
		fmt.Printf("\n%s\n", serverErr)
	}

	return nil
}
