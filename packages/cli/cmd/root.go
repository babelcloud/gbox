package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/config"
	"github.com/babelcloud/gbox/packages/cli/internal/version"
	"github.com/spf13/cobra"
)

var (
	aliasMap = map[string]string{}

	scriptDir string

	rootCmd = &cobra.Command{
		Use:   "gbox",
		Short: "GBOX CLI Tool",
		Long: `GBOX CLI is a command-line tool for managing and operating box and mcp resources. It provides a set of commands to create, manage, and operate these resources.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flag("version").Changed {
				info := version.ClientInfo()
				fmt.Printf("GBOX version %s, build %s\n", info["Version"], info["GitCommit"])
				return nil
			}
			return cmd.Help()
		},
	}
)

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting executable path: %v\n", err)
		os.Exit(1)
	}

	_, isGoRun := os.LookupEnv("CLI_DEV_MODE")

	if isGoRun {
		projectRoot := filepath.Clean(getProjectRoot())
		scriptDir = filepath.Join(projectRoot, "/cmd/script")
	} else {
		realExePath, err := filepath.EvalSymlinks(exePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving symlink: %v\n", err)
			os.Exit(1)
		}
		exeDir := filepath.Dir(realExePath)
		scriptDir = filepath.Join(exeDir, "cmd", "script")
	}

	rootCmd.Flags().BoolP("version", "v", false, "Print version information and exit")

	for alias, cmd := range aliasMap {
		createAliasCommand(alias, cmd)
	}

	rootCmd.AddCommand(NewBoxCommand())
	rootCmd.AddCommand(NewMcpCommand())
	rootCmd.AddCommand(NewVersionCommand())
	rootCmd.AddCommand(NewAdbExposeCommand())
	rootCmd.AddCommand(NewDeviceConnectCommand())
	rootCmd.AddCommand(NewPruneCommand())

	// Add unified server command with subcommands
	rootCmd.AddCommand(NewServerCmd())

	// Enable custom help output ordering
	setupHelpCommand(rootCmd)
}

func createAliasCommand(alias, targetCmd string) {
	parts := strings.Split(targetCmd, " ")
	aliasCmd := &cobra.Command{
		Use:   alias,
		Short: fmt.Sprintf("Alias for '%s'", targetCmd),
		RunE: func(cmd *cobra.Command, args []string) error {
			allArgs := append(parts[1:], args...)
			return executeScript(parts[0], allArgs)
		},
	}

	// Hide certain alias commands from help
	if alias == "setup" || alias == "cleanup" || alias == "export" {
		aliasCmd.Hidden = true
	}

	rootCmd.AddCommand(aliasCmd)
}

func executeScript(cmdName string, args []string) error {
	scriptPath := filepath.Join(scriptDir, fmt.Sprintf("gbox-%s", cmdName))

	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return fmt.Errorf("script not found: %s", scriptPath)
	}

	cmd := exec.Command(scriptPath)
	if len(args) > 0 {
		cmd = exec.Command(scriptPath, args...)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func getProjectRoot() string {
	projectRoot := config.GetProjectRoot()
	if projectRoot != "" {
		return projectRoot
	}

	currentDir, err := os.Getwd()
	if err == nil {
		return currentDir
	}

	return "."
}
