package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Install and configure all command dependencies",
	Long: `Install and configure all dependencies required by GBOX commands.

This includes:
  • Node.js and npm (for Appium)
  • Android Debug Bridge (ADB)
  • FRP client (frpc)
  • Appium Server
  • Appium Drivers (uiautomator2, etc.)
  • Appium Plugins (inspector, etc.)

Examples:
  # Setup all dependencies
  gbox setup

  # Setup without Appium
  GBOX_APPIUM_DISABLED=true gbox setup
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return executeSetup(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

// dependency represents a software dependency to be installed
type dependency struct {
	name              string
	displayName       string
	emoji             string
	checkInstalled    func() bool
	getVersion        func() string
	install           func() error
	manualInstallHelp []string
	required          bool
	skipable          bool
}

func executeSetup(_ *cobra.Command, _ []string) error {
	fmt.Println("🚀 GBOX Dependencies Setup")
	fmt.Println()

	// Define all dependencies
	dependencies := []dependency{
		{
			name:        "nodejs",
			displayName: "Node.js and npm",
			emoji:       "📦",
			checkInstalled: func() bool {
				return device_connect.CheckNodeInstalled() == nil
			},
			getVersion: func() string {
				node := getNodeVersion()
				npm := getNpmVersion()
				if node != "" && npm != "" {
					return fmt.Sprintf("Node.js %s, npm %s", node, npm)
				}
				return ""
			},
			install: installNodeJS,
			manualInstallHelp: []string{
				"🍎 macOS:         brew install node",
				"🐧 Ubuntu/Debian: sudo apt-get install nodejs npm",
				"🪟 Windows:       Download from https://nodejs.org/",
			},
			required: true,
			skipable: false,
		},
		{
			name:           "adb",
			displayName:    "Android Debug Bridge (ADB)",
			emoji:          "📦",
			checkInstalled: checkAdbInstalled,
			getVersion: func() string {
				version := getAdbVersion()
				if version != "" {
					return version
				}
				return ""
			},
			install: installADB,
			manualInstallHelp: []string{
				"🍎 macOS:         brew install android-platform-tools",
				"🐧 Ubuntu/Debian: sudo apt-get install android-tools-adb",
				"🪟 Windows:       Download from https://developer.android.com/studio/releases/platform-tools",
			},
			required: false,
			skipable: true,
		},
		{
			name:           "frpc",
			displayName:    "FRP client (frpc)",
			emoji:          "📦",
			checkInstalled: checkFrpcInstalled,
			getVersion:     getFrpcVersion,
			install:        installFrpc,
			manualInstallHelp: []string{
				"🍎 macOS:         brew install frpc",
				"🐧 Linux:         Visit https://github.com/fatedier/frp/releases",
			},
			required: false,
			skipable: true,
		},
	}

	// Process each dependency
	for _, dep := range dependencies {
		if err := processDependency(dep); err != nil {
			if dep.required {
				return err
			}
			// Non-required dependencies continue on error
		}
	}

	// Handle Appium separately (special case with env var)
	if err := setupAppium(); err != nil {
		return err
	}

	// Setup complete
	fmt.Println("🎉 Setup Complete! All dependencies are installed. You can now have fun with all GBOX commands :)")

	return nil
}

// processDependency handles the check and installation of a single dependency
func processDependency(dep dependency) error {
	fmt.Printf("%s Checking %s...\n", dep.emoji, dep.displayName)

	if dep.checkInstalled() {
		version := dep.getVersion()
		if version != "" {
			fmt.Printf("✅ %s (%s) is already installed\n", dep.displayName, version)
		} else {
			fmt.Printf("✅ %s is already installed\n", dep.displayName)
		}
		fmt.Println()
		return nil
	}

	// Not installed - attempt installation
	fmt.Printf("⚠️  %s not found\n", dep.displayName)
	fmt.Printf("⚙️  Installing %s...\n", dep.displayName)

	if err := dep.install(); err != nil {
		fmt.Printf("⚠️  Failed to install %s: %v\n\n", dep.displayName, err)
		fmt.Printf("Please install %s manually:\n", dep.displayName)
		for _, help := range dep.manualInstallHelp {
			fmt.Printf("  %s\n", help)
		}
		fmt.Println()

		if dep.required {
			return fmt.Errorf("failed to install required dependency: %s", dep.displayName)
		}
		return nil // Non-required dependencies don't fail the setup
	}

	// Installation successful
	version := dep.getVersion()
	if version != "" {
		fmt.Printf("✅ %s (%s) installed\n", dep.displayName, version)
	} else {
		fmt.Printf("✅ %s installed\n", dep.displayName)
	}
	fmt.Println()

	return nil
}

// setupAppium handles the Appium installation
func setupAppium() error {
	if os.Getenv("GBOX_APPIUM_DISABLED") == "true" {
		fmt.Println("ℹ️  Appium installation is disabled (GBOX_APPIUM_DISABLED=true)")
		fmt.Println()
		return nil
	}

	fmt.Println("📦 Setting up Appium automation...")
	appiumCfg := device_connect.GetAppiumConfig()

	if err := device_connect.InstallAppium(appiumCfg); err != nil {
		return fmt.Errorf("failed to install Appium: %v", err)
	}
	fmt.Println()

	return nil
}

func installNodeJS() error {
	// Try to detect OS and install Node.js
	if _, err := exec.LookPath("brew"); err == nil {
		// macOS with Homebrew
		cmd := exec.Command("brew", "install", "node")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	if _, err := exec.LookPath("apt-get"); err == nil {
		// Debian/Ubuntu
		return runAsRoot("apt-get", "install", "-y", "nodejs", "npm")
	}

	if _, err := exec.LookPath("yum"); err == nil {
		// RHEL/CentOS
		return runAsRoot("yum", "install", "-y", "nodejs", "npm")
	}

	return fmt.Errorf("unable to detect package manager")
}

// getNodeVersion returns the Node.js version
func getNodeVersion() string {
	cmd := exec.Command("node", "--version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// getNpmVersion returns the npm version
func getNpmVersion() string {
	cmd := exec.Command("npm", "--version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// getAdbVersion returns the ADB version
func getAdbVersion() string {
	cmd := exec.Command("adb", "version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	// ADB output format: "Android Debug Bridge version X.Y.Z"
	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		parts := strings.Fields(lines[0])
		if len(parts) >= 5 {
			return parts[4] // version number
		}
	}
	return ""
}

// getFrpcVersion returns the frpc version
func getFrpcVersion() string {
	cmd := exec.Command("frpc", "--version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
