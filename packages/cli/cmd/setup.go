package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/babelcloud/gbox/packages/cli/internal/device_connect"
	"github.com/spf13/cobra"
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
  # Interactive setup
  gbox setup

  # Non-interactive setup (use defaults)
  gbox setup -y

  # Setup without Appium
  GBOX_APPIUM_DISABLED=true gbox setup
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return executeSetup(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.Flags().BoolP("yes", "y", false, "Non-interactive mode (use all defaults)")
}

func executeSetup(cmd *cobra.Command, args []string) error {
	nonInteractive, _ := cmd.Flags().GetBool("yes")

	fmt.Println("🚀 GBOX Dependencies Setup")

	if nonInteractive {
		fmt.Println("ℹ️  Running in non-interactive mode")
		fmt.Println()
	}

	// Check Node.js and npm
	fmt.Println("📦 Checking Node.js and npm...")
	if err := device_connect.CheckNodeInstalled(); err != nil {
		fmt.Println("⚠️  Node.js or npm not found")
		fmt.Println()
		fmt.Println("Please install Node.js first:")
		fmt.Println("  🍎 macOS:         brew install node")
		fmt.Println("  🐧 Ubuntu/Debian: sudo apt-get install nodejs npm")
		fmt.Println("  🪟 Windows:       Download from https://nodejs.org/")
		fmt.Println()

		if !nonInteractive {
			return fmt.Errorf("Node.js is required. Please install it and run 'gbox setup' again")
		}

		// In non-interactive mode, try to install
		fmt.Println("⚙️  Attempting to install Node.js...")
		if err := installNodeJS(); err != nil {
			return fmt.Errorf("failed to install Node.js: %v", err)
		}

		// Display versions after installation
		nodeVersion := getNodeVersion()
		npmVersion := getNpmVersion()
		if nodeVersion != "" && npmVersion != "" {
			fmt.Printf("✅ Node.js (%s) and npm (%s) installed\n", nodeVersion, npmVersion)
		} else {
			fmt.Println("✅ Node.js installed")
		}
	} else {
		// Display versions
		nodeVersion := getNodeVersion()
		npmVersion := getNpmVersion()
		if nodeVersion != "" && npmVersion != "" {
			fmt.Printf("✅ Node.js (%s) and npm (%s) are already installed\n", nodeVersion, npmVersion)
		} else {
			fmt.Println("✅ Node.js and npm are already installed")
		}
	}
	fmt.Println()

	// Check and install ADB
	fmt.Println("📦 Checking Android Debug Bridge (ADB)...")
	if !checkAdbInstalled() {
		fmt.Println("⚠️  ADB not found")
		fmt.Println()

		if !nonInteractive {
			fmt.Print("Install ADB now? [Y/n]: ")
			var response string
			fmt.Scanln(&response)
			if response != "" && response != "Y" && response != "y" {
				fmt.Println("⏭️  Skipping ADB installation")
				goto checkFrpc
			}
		}

		fmt.Println("⚙️  Installing ADB...")
		if err := installADB(); err != nil {
			fmt.Printf("⚠️  Failed to install ADB: %v\n", err)
			fmt.Println()
			fmt.Println("Please install ADB manually:")
			fmt.Println("  🍎 macOS:         brew install android-platform-tools")
			fmt.Println("  🐧 Ubuntu/Debian: sudo apt-get install android-tools-adb")
			fmt.Println("  🪟 Windows:       Download from https://developer.android.com/studio/releases/platform-tools")
			fmt.Println()
		} else {
			// Display version after installation
			adbVersion := getAdbVersion()
			if adbVersion != "" {
				fmt.Printf("✅ ADB (%s) installed\n", adbVersion)
			} else {
				fmt.Println("✅ ADB installed")
			}
		}
	} else {
		// Display version
		adbVersion := getAdbVersion()
		if adbVersion != "" {
			fmt.Printf("✅ ADB (%s) is already installed\n", adbVersion)
		} else {
			fmt.Println("✅ ADB is already installed")
		}
	}
	fmt.Println()

checkFrpc:
	// Check and install frpc
	fmt.Println("📦 Checking FRP client (frpc)...")
	if !checkFrpcInstalled() {
		fmt.Println("⚠️  frpc not found")
		fmt.Println()

		if !nonInteractive {
			fmt.Print("Install frpc now? [Y/n]: ")
			var response string
			fmt.Scanln(&response)
			if response != "" && response != "Y" && response != "y" {
				fmt.Println("⏭️  Skipping frpc installation")
				goto checkAppium
			}
		}

		fmt.Println("⚙️  Installing frpc...")
		if err := installFrpc(); err != nil {
			fmt.Printf("⚠️  Failed to install frpc: %v\n", err)
			fmt.Println()
			fmt.Println("Please install frpc manually:")
			fmt.Println("  🍎 macOS:         brew install frpc")
			fmt.Println("  🐧 Linux:         Visit https://github.com/fatedier/frp/releases")
			fmt.Println()
		} else {
			// Display version after installation
			frpcVersion := getFrpcVersion()
			if frpcVersion != "" {
				fmt.Printf("✅ frpc (%s) installed\n", frpcVersion)
			} else {
				fmt.Println("✅ frpc installed")
			}
		}
	} else {
		// Display version
		frpcVersion := getFrpcVersion()
		if frpcVersion != "" {
			fmt.Printf("✅ frpc (%s) is already installed\n", frpcVersion)
		} else {
			fmt.Println("✅ frpc is already installed")
		}
	}
	fmt.Println()

checkAppium:
	// Check and install Appium (if not disabled)
	if os.Getenv("GBOX_APPIUM_DISABLED") == "true" {
		fmt.Println("ℹ️  Appium installation is disabled (GBOX_APPIUM_DISABLED=true)")
		fmt.Println()
	} else {
		fmt.Println("📦 Setting up Appium automation...")
		appiumCfg := device_connect.GetAppiumConfig()

		if err := device_connect.InstallAppium(appiumCfg); err != nil {
			return fmt.Errorf("failed to install Appium: %v", err)
		}
		fmt.Println()
	}

	// Setup complete
	fmt.Println("🎉 Setup Complete! All dependencies are installed. You can now have fun with all GBOX commands :)")

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
		cmd := exec.Command("sudo", "apt-get", "install", "-y", "nodejs", "npm")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	if _, err := exec.LookPath("yum"); err == nil {
		// RHEL/CentOS
		cmd := exec.Command("sudo", "yum", "install", "-y", "nodejs", "npm")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
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
