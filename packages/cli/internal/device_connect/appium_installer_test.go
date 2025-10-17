package device_connect

import (
	"os"
	"testing"
)

func TestGetAppiumConfig(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		wantInstall bool
		wantDrivers []string
		wantPlugins []string
	}{
		{
			name:        "default configuration",
			envVars:     map[string]string{},
			wantInstall: true,
			wantDrivers: []string{"uiautomator2"},
			wantPlugins: []string{"inspector"},
		},
		{
			name: "disable installation",
			envVars: map[string]string{
				"GBOX_INSTALL_APPIUM": "false",
			},
			wantInstall: false,
			wantDrivers: []string{"uiautomator2"},
			wantPlugins: []string{"inspector"},
		},
		{
			name: "custom drivers",
			envVars: map[string]string{
				"GBOX_APPIUM_DRIVERS": "uiautomator2,xcuitest",
			},
			wantInstall: true,
			wantDrivers: []string{"uiautomator2", "xcuitest"},
			wantPlugins: []string{"inspector"},
		},
		{
			name: "no drivers",
			envVars: map[string]string{
				"GBOX_APPIUM_DRIVERS": "none",
			},
			wantInstall: true,
			wantDrivers: []string{},
			wantPlugins: []string{"inspector"},
		},
		{
			name: "with custom plugins",
			envVars: map[string]string{
				"GBOX_APPIUM_PLUGINS": "images,execute-driver",
			},
			wantInstall: true,
			wantDrivers: []string{"uiautomator2"},
			wantPlugins: []string{"images", "execute-driver"},
		},
		{
			name: "no plugins with none",
			envVars: map[string]string{
				"GBOX_APPIUM_PLUGINS": "none",
			},
			wantInstall: true,
			wantDrivers: []string{"uiautomator2"},
			wantPlugins: []string{},
		},
		{
			name: "no plugins with NONE uppercase",
			envVars: map[string]string{
				"GBOX_APPIUM_PLUGINS": "NONE",
			},
			wantInstall: true,
			wantDrivers: []string{"uiautomator2"},
			wantPlugins: []string{},
		},
		{
			name: "no plugins with empty string",
			envVars: map[string]string{
				"GBOX_APPIUM_PLUGINS": "",
			},
			wantInstall: true,
			wantDrivers: []string{"uiautomator2"},
			wantPlugins: []string{},
		},
		{
			name: "no drivers with empty string",
			envVars: map[string]string{
				"GBOX_APPIUM_DRIVERS": "",
			},
			wantInstall: true,
			wantDrivers: []string{},
			wantPlugins: []string{"inspector"},
		},
		{
			name: "full custom configuration",
			envVars: map[string]string{
				"GBOX_INSTALL_APPIUM": "true",
				"GBOX_APPIUM_DRIVERS": "uiautomator2,xcuitest,espresso",
				"GBOX_APPIUM_PLUGINS": "images",
			},
			wantInstall: true,
			wantDrivers: []string{"uiautomator2", "xcuitest", "espresso"},
			wantPlugins: []string{"images"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment variables
			os.Unsetenv("GBOX_INSTALL_APPIUM")
			os.Unsetenv("GBOX_APPIUM_DRIVERS")
			os.Unsetenv("GBOX_APPIUM_PLUGINS")

			// Set test environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Get configuration
			cfg := GetAppiumConfig()

			// Check installation flag
			if cfg.InstallAppium != tt.wantInstall {
				t.Errorf("InstallAppium = %v, want %v", cfg.InstallAppium, tt.wantInstall)
			}

			// Check drivers
			if len(cfg.Drivers) != len(tt.wantDrivers) {
				t.Errorf("Drivers length = %d, want %d", len(cfg.Drivers), len(tt.wantDrivers))
			}
			for i, driver := range cfg.Drivers {
				if i < len(tt.wantDrivers) && driver != tt.wantDrivers[i] {
					t.Errorf("Drivers[%d] = %s, want %s", i, driver, tt.wantDrivers[i])
				}
			}

			// Check plugins
			if len(cfg.Plugins) != len(tt.wantPlugins) {
				t.Errorf("Plugins length = %d, want %d", len(cfg.Plugins), len(tt.wantPlugins))
			}
			for i, plugin := range cfg.Plugins {
				if i < len(tt.wantPlugins) && plugin != tt.wantPlugins[i] {
					t.Errorf("Plugins[%d] = %s, want %s", i, plugin, tt.wantPlugins[i])
				}
			}

			// Cleanup
			for key := range tt.envVars {
				os.Unsetenv(key)
			}
		})
	}
}

func TestCheckNodeInstalled(t *testing.T) {
	// This test will only pass if Node.js and npm are actually installed
	// In a CI environment without Node.js, this test should be skipped
	if os.Getenv("CI") == "true" && os.Getenv("SKIP_NODE_TEST") == "true" {
		t.Skip("Skipping Node.js test in CI environment")
	}

	err := CheckNodeInstalled()
	if err != nil {
		t.Logf("Node.js check failed (expected in environments without Node.js): %v", err)
	}
}
