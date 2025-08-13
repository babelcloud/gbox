package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// Show help information
func showHelp(helpType string) {
	switch helpType {
	case "short":
		fmt.Println("Box management tool")
		return
	case "all":
		fmt.Println("Usage: gbox <command> [arguments]")
		fmt.Println("\nAvailable Commands:")

		// Display alias commands in fixed order
		aliases := []string{"export"}
		for _, alias := range aliases {
			if cmd, ok := aliasMap[alias]; ok {
				parts := strings.Split(cmd, " ")
				scriptPath := filepath.Join(scriptDir, fmt.Sprintf("gbox-%s", parts[0]))
				if _, err := os.Stat(scriptPath); err == nil {
					description := getSubCommandDescription(parts[0], parts[1])
					fmt.Printf("    %-18s %s\n", alias, description)
				}
			}
		}
		fmt.Printf("    %-18s %s\n", "help", "Show help information")

		fmt.Println("\nSub Commands:")
		// Display main commands in fixed order
		for _, cmd := range []string{"box", "mcp"} {
			scriptPath := filepath.Join(scriptDir, fmt.Sprintf("gbox-%s", cmd))
			if _, err := os.Stat(scriptPath); err == nil {
				description := getCommandDescription(cmd)
				fmt.Printf("    %-18s %s\n", cmd, description)
			}
		}

		fmt.Println("\nOptions:")
		fmt.Println("    --help [short|all]  Show this help message (default: all)")

		fmt.Println("\nExamples:")
		fmt.Println("    gbox box create mybox      # Create a new box")
		fmt.Println("    gbox box list              # List all boxes")
		fmt.Println("    gbox export                # Export MCP configuration")

		fmt.Println("\nUse \"gbox <command> --help\" for more information about a command.")
	default:
		fmt.Fprintf(os.Stderr, "Invalid help type: %s\n", helpType)
		fmt.Fprintln(os.Stderr, "Valid types are: short, all")
		os.Exit(1)
	}
}

// Get description for a subcommand
func getSubCommandDescription(cmdName, subCmd string) string {
	scriptPath := filepath.Join(scriptDir, fmt.Sprintf("gbox-%s", cmdName))
	cmd := exec.Command(scriptPath, subCmd, "--help", "short")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Sprintf("%s %s", cmdName, subCmd)
	}
	return strings.TrimSpace(string(output))
}

// Get description for a command
func getCommandDescription(cmdName string) string {
	scriptPath := filepath.Join(scriptDir, fmt.Sprintf("gbox-%s", cmdName))
	cmd := exec.Command(scriptPath, "--help", "short")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Sprintf("%s operations", cmdName)
	}
	return strings.TrimSpace(string(output))
}

// Setup help command
func setupHelpCommand(rootCmd *cobra.Command) {
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		printRootHelpOrdered(cmd)
	})
	rootCmd.SetHelpCommand(&cobra.Command{
		Use:    "help",
		Short:  "Show help information",
		Hidden: false,
		Run: func(cmd *cobra.Command, args []string) {
			printRootHelpOrdered(cmd.Root())
		},
	})
	rootCmd.PersistentFlags().BoolP("help", "", false, "")
	rootCmd.PersistentFlags().MarkHidden("help")
}

// printRootHelpOrdered prints the root help with commands ordered by a custom priority
func printRootHelpOrdered(cmd *cobra.Command) {
	// Priority order for top-level commands
	priority := []string{"login", "box", "device-connect", "port-forward", "mcp", "profile", "version", "completion", "help"}
	priorityIndex := map[string]int{}
	for i, name := range priority {
		priorityIndex[name] = i
	}

	// Header
	if cmd.Long != "" {
		fmt.Fprintln(os.Stdout, cmd.Long)
	} else if cmd.Short != "" {
		fmt.Fprintln(os.Stdout, cmd.Short)
	}

	fmt.Fprintln(os.Stdout, "\nUsage:")
	fmt.Fprintf(os.Stdout, "  %s [flags]\n", cmd.Name())
	fmt.Fprintf(os.Stdout, "  %s [command]\n", cmd.Name())

	// Collect and sort available commands
	commands := []*cobra.Command{}
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.Hidden {
			continue
		}
		commands = append(commands, c)
	}

	// Custom sort by priority, then by name
	sort.SliceStable(commands, func(i, j int) bool {
		ci, cj := commands[i], commands[j]
		pi, okI := priorityIndex[ci.Name()]
		pj, okJ := priorityIndex[cj.Name()]
		if okI && okJ {
			if pi == pj {
				return ci.Name() < cj.Name()
			}
			return pi < pj
		}
		if okI {
			return true
		}
		if okJ {
			return false
		}
		return ci.Name() < cj.Name()
	})

	fmt.Fprintln(os.Stdout, "\nAvailable Commands:")
	for _, c := range commands {
		fmt.Fprintf(os.Stdout, "  %-14s %s\n", c.Name(), c.Short)
	}

	// Flags
	fmt.Fprintln(os.Stdout, "\nFlags:")
	fmt.Fprint(os.Stdout, cmd.Flags().FlagUsages())

	fmt.Fprintf(os.Stdout, "\nUse \"%s [command] --help\" for more information about a command.\n", cmd.Name())
}
