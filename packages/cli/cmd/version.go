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
		Short: "Print the client version information",
		Long:  `Display detailed version information about the GBOX client`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If --version flag was specified, show only the client version
			if cmd.Flag("version").Changed {
				opts.ShortFormat = true
			}
			return runVersion(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.OutputFormat, "output", "o", "text", "Output format (json or text)")
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

	// Display GBOX ASCII art banner with gradient colors
	// Primary color #704FED (purple)
	purple := "\033[38;2;112;79;237m"       // classic purple
	lightPurple := "\033[38;2;142;109;255m" // light purple
	darkPurple := "\033[38;2;82;49;207m"    // dark purple
	// Special highlight color for letter G (brighter gradient purple than primary)
	glowPurple := "\033[38;2;180;130;255m" // glow purple
	reset := "\033[0m"
	bold := "\033[1m"

	gboxBanner := bold + `
` + darkPurple + `   ██████╗ ` + lightPurple + `██████   ` + purple + `██████  ` + lightPurple + `██   ██ 
` + purple + `  ██╔════╝ ` + lightPurple + `██   ██ ` + purple + `██    ██ ` + lightPurple + ` ██ ██  
` + darkPurple + `  ██║  ███╗` + purple + `██████  ` + glowPurple + `██    ██ ` + purple + `  ███   
` + darkPurple + `  ██║   ██║` + purple + `██   ██ ` + glowPurple + `██    ██ ` + purple + ` ██ ██  
` + purple + `  ╚██████╔╝` + lightPurple + `██████  ` + purple + ` ██████  ` + lightPurple + `██   ██
` + glowPurple + `   ╚═════╝ ` + lightPurple + `       ` + purple + `         ` + lightPurple + `        ` + reset

	fmt.Println(gboxBanner)

	if opts.OutputFormat == "json" {
		result := map[string]interface{}{
			"Client": clientInfo,
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

	return nil
}
