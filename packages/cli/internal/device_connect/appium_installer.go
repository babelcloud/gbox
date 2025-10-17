package device_connect

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/briandowns/spinner"

	"github.com/babelcloud/gbox/packages/cli/config"
)

// UISpinner wraps spinner for elegant terminal output
type UISpinner struct {
	sp    *spinner.Spinner
	debug bool
}

// NewUISpinner creates a new spinner with the given message
func NewUISpinner(debug bool, message string) *UISpinner {
	s := &UISpinner{debug: debug}

	if !debug {
		// Use dots spinner style (CharSet 14)
		s.sp = spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		s.sp.Prefix = "  "
		s.sp.Suffix = " " + message
		s.sp.Start()
	} else {
		fmt.Printf("[DEBUG] %s\n", message)
	}

	return s
}

// Success stops the spinner and prints a success message
func (s *UISpinner) Success(message string) {
	if !s.debug && s.sp != nil {
		s.sp.Stop()
		fmt.Printf("\r\033[K  ✓ %s\n", message) // \033[K clears the line
	} else if s.debug {
		fmt.Printf("[DEBUG] ✓ %s\n", message)
	}
}

// Fail stops the spinner and prints an error message
func (s *UISpinner) Fail(message string) {
	if !s.debug && s.sp != nil {
		s.sp.Stop()
		fmt.Printf("\r\033[K  ✗ %s\n", message)
	} else if s.debug {
		fmt.Printf("[DEBUG] ✗ %s\n", message)
	}
}

// Stop stops the spinner without printing anything
func (s *UISpinner) Stop() {
	if !s.debug && s.sp != nil {
		s.sp.Stop()
		fmt.Print("\r\033[K") // Clear the line
	}
}

// AppiumConfig holds the configuration for Appium installation
type AppiumConfig struct {
	InstallAppium bool
	Drivers       []string
	Plugins       []string
}

// GetAppiumConfig reads Appium installation configuration from environment variables
func GetAppiumConfig() AppiumConfig {
	cfg := AppiumConfig{
		InstallAppium: true,
		Drivers:       []string{"uiautomator2"},
		Plugins:       []string{"inspector"},
	}

	// Check if Appium installation is enabled
	if installAppium := os.Getenv("GBOX_INSTALL_APPIUM"); installAppium != "" {
		cfg.InstallAppium = strings.ToLower(installAppium) == "true" || installAppium == "1"
	}

	// Get drivers list
	if drivers, exists := os.LookupEnv("GBOX_APPIUM_DRIVERS"); exists {
		drivers = strings.TrimSpace(drivers)
		if drivers == "" {
			// Empty string means: ignore and use default
			// Keep the default value (uiautomator2)
		} else if strings.ToLower(drivers) == "none" {
			// Only "none" or "NONE" explicitly clears the list
			cfg.Drivers = []string{}
		} else {
			cfg.Drivers = strings.Split(drivers, ",")
			// Trim spaces
			for i, d := range cfg.Drivers {
				cfg.Drivers[i] = strings.TrimSpace(d)
			}
		}
	}

	// Get plugins list
	if plugins, exists := os.LookupEnv("GBOX_APPIUM_PLUGINS"); exists {
		plugins = strings.TrimSpace(plugins)
		if plugins == "" {
			// Empty string means: ignore and use default
			// Keep the default value (inspector)
		} else if strings.ToLower(plugins) == "none" {
			// Only "none" or "NONE" explicitly clears the list
			cfg.Plugins = []string{}
		} else {
			cfg.Plugins = strings.Split(plugins, ",")
			// Trim spaces
			for i, p := range cfg.Plugins {
				cfg.Plugins[i] = strings.TrimSpace(p)
			}
		}
	}

	return cfg
}

// CheckNodeInstalled checks if Node.js and npm are installed
func CheckNodeInstalled() error {
	// Check node
	if _, err := exec.LookPath("node"); err != nil {
		return fmt.Errorf("node is not installed or not in PATH")
	}

	// Check npm
	if _, err := exec.LookPath("npm"); err != nil {
		return fmt.Errorf("npm is not installed or not in PATH")
	}

	return nil
}

// IsAppiumInstalled checks if Appium is already installed in our working directory
func IsAppiumInstalled(appiumHome string) bool {
	// Only check if appium binary exists in the appium home (not global)
	appiumBinary := filepath.Join(appiumHome, "node_modules", ".bin", "appium")
	if _, err := os.Stat(appiumBinary); err == nil {
		return true
	}

	return false
}

// InstallAppium installs Appium server and its components
func InstallAppium(cfg AppiumConfig) error {
	if !cfg.InstallAppium {
		fmt.Println("ℹ️  Appium installation is disabled by environment variable GBOX_INSTALL_APPIUM")
		return nil
	}

	// Check Node.js installation
	if err := CheckNodeInstalled(); err != nil {
		return fmt.Errorf("cannot install Appium: %v\n\n"+
			"Please install Node.js and npm first:\n"+
			"  • macOS:         brew install node\n"+
			"  • Ubuntu/Debian: sudo apt-get install nodejs npm\n"+
			"  • Windows:       Download from https://nodejs.org/", err)
	}

	deviceProxyHome := config.GetDeviceProxyHome()
	appiumHome := filepath.Join(deviceProxyHome, "appium")

	// Create appium home directory
	if err := os.MkdirAll(appiumHome, 0755); err != nil {
		return fmt.Errorf("failed to create Appium home directory: %v", err)
	}

	debug := os.Getenv("DEBUG") == "true"

	// Check if Appium is already installed
	fmt.Println()
	fmt.Println("📦 Checking Appium server...")
	if IsAppiumInstalled(appiumHome) {
		// Get Appium version
		appiumBinary := filepath.Join(appiumHome, "node_modules", ".bin", "appium")
		version := ""
		if versionCmd := exec.Command(appiumBinary, "-v"); versionCmd != nil {
			versionCmd.Env = append(os.Environ(), "APPIUM_HOME="+appiumHome)
			if versionOutput, err := versionCmd.Output(); err == nil {
				version = strings.TrimSpace(string(versionOutput))
			}
		}

		if version != "" {
			fmt.Printf("✅ Appium server [%s] is already installed\n", version)
		} else {
			fmt.Println("✅ Appium server is already installed")
		}

		return installAppiumComponents(appiumHome, cfg)
	}

	// Appium not installed
	fmt.Println("⚠️  Appium server not found, installing...")

	// Start spinner
	sp := NewUISpinner(debug, "Installing Appium server...")

	// Initialize package.json if it doesn't exist
	packageJSONPath := filepath.Join(appiumHome, "package.json")
	if _, err := os.Stat(packageJSONPath); os.IsNotExist(err) {
		initCmd := exec.Command("npm", "init", "-y")
		initCmd.Dir = appiumHome
		if err := initCmd.Run(); err != nil {
			sp.Stop()
			return fmt.Errorf("failed to initialize npm package: %v", err)
		}
	}

	// Install Appium using npm
	cmd := exec.Command("npm", "install", "appium")
	cmd.Dir = appiumHome
	cmd.Env = append(os.Environ(), "APPIUM_HOME="+appiumHome)

	if debug {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		sp.Fail("Failed to install Appium server")
		return fmt.Errorf("failed to install appium server: %v", err)
	}

	// Get Appium version
	version := ""
	appiumBinary := filepath.Join(appiumHome, "node_modules", ".bin", "appium")
	if versionCmd := exec.Command(appiumBinary, "-v"); versionCmd != nil {
		versionCmd.Env = append(os.Environ(), "APPIUM_HOME="+appiumHome)
		if versionOutput, err := versionCmd.Output(); err == nil {
			version = strings.TrimSpace(string(versionOutput))
		}
	}

	// Print success with version
	if version != "" {
		sp.Success(fmt.Sprintf("Appium server [%s] installed", version))
	} else {
		sp.Success("Appium server installed")
	}

	// Install drivers and plugins
	return installAppiumComponents(appiumHome, cfg)
}

// AppiumDriverInfo represents driver information from Appium JSON output
/*
{
  "uiautomator2": {
    "pkgName": "appium-uiautomator2-driver",
    "version": "4.2.7",
    "installType": "npm",
    "installSpec": "uiautomator2",
    "installPath": "/Users/gbox/.appium/node_modules/appium-uiautomator2-driver",
    "appiumVersion": "^2.4.1 || ^3.0.0-beta.0",
    "automationName": "UiAutomator2",
    "platformNames": [
      "Android"
    ],
    "mainClass": "AndroidUiautomator2Driver",
    "scripts": {
      "reset": "scripts/reset.js"
    },
    "doctor": {
      "checks": [
        "./build/lib/doctor/required-checks.js",
        "./build/lib/doctor/optional-checks.js"
      ]
    },
    "installed": true
  }
}
*/
type AppiumDriverInfo struct {
	PkgName   string `json:"pkgName"`
	Version   string `json:"version"`
	Installed bool   `json:"installed"`
}

// AppiumPluginInfo represents plugin information from Appium JSON output
/*
{
  "inspector": {
    "pkgName": "appium-inspector-plugin",
    "version": "2025.8.2",
    "installType": "npm",
    "installSpec": "inspector",
    "installPath": "/Users/gbox/.appium/node_modules/appium-inspector-plugin",
    "appiumVersion": "^3.0.0-beta.0",
    "mainClass": "AppiumInspectorPlugin",
    "installed": true
  }
}
*/
type AppiumPluginInfo struct {
	PkgName   string `json:"pkgName"`
	Version   string `json:"version"`
	Installed bool   `json:"installed"`
}

// getInstalledDrivers returns a map of installed drivers with their info
func getInstalledDrivers(appiumBinary, appiumHome string) (map[string]AppiumDriverInfo, error) {
	checkCmd := exec.Command(appiumBinary, "driver", "list", "--installed", "--json")
	checkCmd.Dir = appiumHome
	checkCmd.Env = append(os.Environ(), "APPIUM_HOME="+appiumHome)
	output, err := checkCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list drivers: %v", err)
	}

	var drivers map[string]AppiumDriverInfo
	if err := json.Unmarshal(output, &drivers); err != nil {
		return nil, fmt.Errorf("failed to parse driver list: %v", err)
	}

	return drivers, nil
}

// getInstalledPlugins returns a map of installed plugins with their info
func getInstalledPlugins(appiumBinary, appiumHome string) (map[string]AppiumPluginInfo, error) {
	checkCmd := exec.Command(appiumBinary, "plugin", "list", "--installed", "--json")
	checkCmd.Dir = appiumHome
	checkCmd.Env = append(os.Environ(), "APPIUM_HOME="+appiumHome)
	output, err := checkCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list plugins: %v", err)
	}

	var plugins map[string]AppiumPluginInfo
	if err := json.Unmarshal(output, &plugins); err != nil {
		return nil, fmt.Errorf("failed to parse plugin list: %v", err)
	}

	return plugins, nil
}

// installAppiumComponents installs Appium drivers and plugins
func installAppiumComponents(appiumHome string, cfg AppiumConfig) error {
	debug := os.Getenv("DEBUG") == "true"
	appiumBinary := filepath.Join(appiumHome, "node_modules", ".bin", "appium")

	// Check if local appium exists, otherwise try global
	if _, err := os.Stat(appiumBinary); err != nil {
		if globalAppium, err := exec.LookPath("appium"); err == nil {
			appiumBinary = globalAppium
		} else {
			return fmt.Errorf("appium binary not found")
		}
	}

	var installErrors []string

	// ===== Check and install drivers =====
	fmt.Println()
	fmt.Println("📦 Checking Appium drivers...")

	// Get currently installed drivers
	installedDrivers, err := getInstalledDrivers(appiumBinary, appiumHome)
	if err != nil {
		// If we can't get installed drivers list, try to proceed but warn
		fmt.Printf("⚠️ Warning: Failed to get installed drivers list: %v\n", err)
		fmt.Printf("⚠️ Will attempt to install configured drivers anyway.\n")
		installedDrivers = make(map[string]AppiumDriverInfo)
	}

	// Check configured drivers
	if len(cfg.Drivers) == 0 {
		fmt.Println("ℹ️  No drivers configured")
	} else {
		// Check each configured driver
		var toInstall []string
		for _, driver := range cfg.Drivers {
			if driver == "" {
				continue
			}
			if driverInfo, exists := installedDrivers[driver]; exists && driverInfo.Installed {
				fmt.Printf("✅ Driver [%s@%s] is already installed\n", driver, driverInfo.Version)
			} else {
				toInstall = append(toInstall, driver)
			}
		}

		// Install missing drivers
		if len(toInstall) > 0 {
			fmt.Printf("⚠️  Missing drivers: %s, installing...\n", strings.Join(toInstall, ", "))

			for _, driver := range toInstall {
				// Start spinner
				sp := NewUISpinner(debug, fmt.Sprintf("Installing driver [%s]...", driver))

				// Install driver with APPIUM_HOME set
				cmd := exec.Command(appiumBinary, "driver", "install", driver)
				var stderr strings.Builder
				if debug {
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
				} else {
					cmd.Stderr = &stderr
				}
				cmd.Dir = appiumHome
				cmd.Env = append(os.Environ(), "APPIUM_HOME="+appiumHome)

				if err := cmd.Run(); err != nil {
					sp.Fail(fmt.Sprintf("Failed to install driver [%s]", driver))
					errMsg := fmt.Sprintf("failed to install driver %s: %v", driver, err)
					if stderr.String() != "" && debug {
						fmt.Printf("[DEBUG] Error: %s\n", strings.TrimSpace(stderr.String()))
					}
					installErrors = append(installErrors, errMsg)
					continue
				}

				// Get version for success message
				version := ""
				if updatedDrivers, err := getInstalledDrivers(appiumBinary, appiumHome); err == nil {
					if driverInfo, exists := updatedDrivers[driver]; exists {
						version = driverInfo.Version
					}
				}

				// Print success with version
				if version != "" {
					sp.Success(fmt.Sprintf("Driver [%s@%s] installed", driver, version))
				} else {
					sp.Success(fmt.Sprintf("Driver [%s] installed", driver))
				}
			}
		}
	}

	// ===== Check and install plugins =====
	fmt.Println()
	fmt.Println("📦 Checking Appium plugins...")

	// Get currently installed plugins
	installedPlugins, err := getInstalledPlugins(appiumBinary, appiumHome)
	if err != nil {
		// If we can't get installed plugins list, try to proceed but warn
		fmt.Printf("⚠️ Warning: Failed to get installed plugins list: %v\n", err)
		fmt.Printf("⚠️ Will attempt to install configured plugins anyway.\n")
		installedPlugins = make(map[string]AppiumPluginInfo)
	}

	// Check configured plugins
	if len(cfg.Plugins) == 0 {
		fmt.Println("ℹ️  No plugins configured")
	} else {
		// Check each configured plugin
		var toInstall []string
		for _, plugin := range cfg.Plugins {
			if plugin == "" {
				continue
			}
			if pluginInfo, exists := installedPlugins[plugin]; exists && pluginInfo.Installed {
				fmt.Printf("✅ Plugin [%s@%s] is already installed\n", plugin, pluginInfo.Version)
			} else {
				toInstall = append(toInstall, plugin)
			}
		}

		// Install missing plugins
		if len(toInstall) > 0 {
			fmt.Printf("⚠️  Missing plugins: %s, installing...\n", strings.Join(toInstall, ", "))

			for _, plugin := range toInstall {
				// Start spinner
				sp := NewUISpinner(debug, fmt.Sprintf("Installing plugin [%s]...", plugin))

				// Install plugin with APPIUM_HOME set
				cmd := exec.Command(appiumBinary, "plugin", "install", plugin)
				var stderr strings.Builder
				if debug {
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
				} else {
					cmd.Stderr = &stderr
				}
				cmd.Dir = appiumHome
				cmd.Env = append(os.Environ(), "APPIUM_HOME="+appiumHome)

				if err := cmd.Run(); err != nil {
					sp.Fail(fmt.Sprintf("Failed to install plugin [%s]", plugin))
					errMsg := fmt.Sprintf("failed to install plugin %s: %v", plugin, err)
					if stderr.String() != "" && debug {
						fmt.Printf("[DEBUG] Error: %s\n", strings.TrimSpace(stderr.String()))
					}
					installErrors = append(installErrors, errMsg)
					continue
				}

				// Get version for success message
				version := ""
				if updatedPlugins, err := getInstalledPlugins(appiumBinary, appiumHome); err == nil {
					if pluginInfo, exists := updatedPlugins[plugin]; exists {
						version = pluginInfo.Version
					}
				}

				// Print success with version
				if version != "" {
					sp.Success(fmt.Sprintf("Plugin [%s@%s] installed", plugin, version))
				} else {
					sp.Success(fmt.Sprintf("Plugin [%s] installed", plugin))
				}
			}
		}
	}

	// If there were any installation errors, return them
	if len(installErrors) > 0 {
		fmt.Println("\n╔═══════════════════════════════════════╗")
		fmt.Println("║  ❌  Installation Errors Detected     ║")
		fmt.Println("╚═══════════════════════════════════════╝")
		for i, err := range installErrors {
			fmt.Printf("  %d. %s\n", i+1, err)
		}
		fmt.Println()
		return fmt.Errorf("%d component(s) failed to install", len(installErrors))
	}

	return nil
}

// GetAppiumPath returns the path to the Appium binary
func GetAppiumPath() string {
	deviceProxyHome := config.GetDeviceProxyHome()
	appiumHome := filepath.Join(deviceProxyHome, "appium")
	appiumBinary := filepath.Join(appiumHome, "node_modules", ".bin", "appium")

	// Check if local appium exists
	if _, err := os.Stat(appiumBinary); err == nil {
		return appiumBinary
	}

	// Try global appium
	if globalAppium, err := exec.LookPath("appium"); err == nil {
		return globalAppium
	}

	return ""
}
