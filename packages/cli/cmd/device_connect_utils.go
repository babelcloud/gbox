package cmd

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fatih/color"
)

// printDeveloperModeHint prints the developer mode hint with dim formatting
func printDeveloperModeHint() {
	color.New(color.Faint).Println("If you can not see your devices here, make sure you have turned on the developer mode on your Android device. For more details, see https://docs.gbox.ai/cli")
}

func checkAdbInstalled() bool {
	_, err := exec.LookPath("adb")
	return err == nil
}

func checkFrpcInstalled() bool {
	_, err := exec.LookPath("frpc")
	return err == nil
}

func printAdbInstallationHint() {
	const (
		ansiRed    = "\033[31m"
		ansiYellow = "\033[33m"
		ansiBold   = "\033[1m"
		ansiReset  = "\033[0m"
	)

	fmt.Println()
	fmt.Printf("%s%s⚠️  IMPORTANT: Android Debug Bridge (ADB) Required%s\n", ansiRed, ansiBold, ansiReset)
	fmt.Printf("%s%s================================================%s\n", ansiYellow, ansiBold, ansiReset)
	fmt.Printf("%sTo use the device-connect feature, you need to install ADB tools first:%s\n", ansiYellow, ansiReset)
	fmt.Println()
	fmt.Printf("%s📱 Installation Methods:%s\n", ansiBold, ansiReset)
	fmt.Printf("  • macOS: brew install android-platform-tools\n")
	fmt.Printf("  • Ubuntu/Debian: sudo apt-get install android-tools-adb\n")
	fmt.Printf("  • Windows: Download Android SDK Platform Tools\n")
	fmt.Println()
	fmt.Printf("%s🔗 After installation, ensure:%s\n", ansiBold, ansiReset)
	fmt.Printf("  1. Enable Developer Options and USB Debugging on your Android device\n")
	fmt.Printf("  2. Connect device via USB or start an emulator\n")
	fmt.Printf("  3. Run 'adb devices' to confirm device recognition\n")
	fmt.Println()
	fmt.Printf("%s%s================================================%s\n", ansiYellow, ansiBold, ansiReset)
	fmt.Println()
}

func printFrpcInstallationHint() {
	const (
		ansiRed    = "\033[31m"
		ansiYellow = "\033[33m"
		ansiBold   = "\033[1m"
		ansiReset  = "\033[0m"
	)

	fmt.Println()
	fmt.Printf("%s%s⚠️  IMPORTANT: FRP Client (frpc) Required%s\n", ansiRed, ansiBold, ansiReset)
	fmt.Printf("%s%s==============================================%s\n", ansiYellow, ansiBold, ansiReset)
	fmt.Printf("%sTo use the device-connect feature, you need to install frpc (FRP Client) first:%s\n", ansiYellow, ansiReset)
	fmt.Println()
	fmt.Printf("%s🌐 Installation Methods:%s\n", ansiBold, ansiReset)
	fmt.Printf("  • macOS: brew install frpc\n")
	fmt.Printf("  • Ubuntu/Debian: Download from https://github.com/fatedier/frp/releases\n")
	fmt.Printf("  • Windows: Download from https://github.com/fatedier/frp/releases\n")
	fmt.Println()
	fmt.Printf("%s📥 Manual Installation:%s\n", ansiBold, ansiReset)
	fmt.Printf("  1. Download frpc binary for your platform from GitHub releases\n")
	fmt.Printf("  2. Extract and place frpc in your PATH or current directory\n")
	fmt.Printf("  3. Ensure frpc is executable: chmod +x frpc\n")
	fmt.Println()
	fmt.Printf("%s🔗 After installation, ensure:%s\n", ansiBold, ansiReset)
	fmt.Printf("  1. frpc is in your PATH or current directory\n")
	fmt.Printf("  2. Run 'frpc version' to confirm installation\n")
	fmt.Println()
	fmt.Printf("%s%s==============================================%s\n", ansiYellow, ansiBold, ansiReset)
	fmt.Println()
}

// getMacOSSerialNumber gets macOS serial number using system_profiler
func getMacOSSerialNumber() (string, error) {
	cmd := exec.Command("system_profiler", "SPHardwareDataType")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Parse output to find Serial Number
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Serial Number") {
			// Extract serial number from line like "      Serial Number (system): H96D2Q2LR3"
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				serialNo := strings.TrimSpace(parts[1])
				if serialNo != "" {
					return serialNo, nil
				}
			}
		}
	}
	return "", fmt.Errorf("serial number not found")
}

// getMacOSVersion gets macOS version and returns release name with version (e.g., "Sequoia 15.6.1")
func getMacOSVersion() (string, error) {
	cmd := exec.Command("sw_vers", "-productVersion")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(string(output))

	// Get release name from system file
	releaseName := getMacOSReleaseName()
	if releaseName != "" {
		return fmt.Sprintf("%s %s", releaseName, version), nil
	}
	return version, nil
}

// getMacOSReleaseName gets macOS release name from system file
func getMacOSReleaseName() string {
	// Use awk to extract release name from system license file
	// Command: awk '/SOFTWARE LICENSE AGREEMENT FOR macOS/' '/System/Library/CoreServices/Setup Assistant.app/Contents/Resources/en.lproj/OSXSoftwareLicense.rtf' | awk -F 'macOS ' '{print $NF}' | awk '{print substr($0, 0, length($0)-1)}'
	licenseFile := "/System/Library/CoreServices/Setup Assistant.app/Contents/Resources/en.lproj/OSXSoftwareLicense.rtf"

	// Check if file exists
	if _, err := os.Stat(licenseFile); os.IsNotExist(err) {
		return ""
	}

	// Run the awk command to extract release name
	cmd := exec.Command("sh", "-c", fmt.Sprintf("awk '/SOFTWARE LICENSE AGREEMENT FOR macOS/' '%s' | awk -F 'macOS ' '{print $NF}' | awk '{print substr($0, 0, length($0)-1)}'", licenseFile))
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	releaseName := strings.TrimSpace(string(output))
	if releaseName != "" {
		return releaseName
	}
	return ""
}

// getLinuxVersion gets Linux distribution version
func getLinuxVersion() (string, error) {
	// Try /etc/os-release first
	if _, err := os.Stat("/etc/os-release"); err == nil {
		cmd := exec.Command("sh", "-c", "source /etc/os-release && echo $PRETTY_NAME")
		output, err := cmd.Output()
		if err == nil {
			version := strings.TrimSpace(string(output))
			if version != "" {
				return version, nil
			}
		}
	}
	return "Linux", nil
}

// getWindowsVersion gets Windows version
func getWindowsVersion() (string, error) {
	cmd := exec.Command("cmd", "/c", "ver")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(string(output))
	// Clean up output (remove "Microsoft Windows [" prefix and "]" suffix if present)
	version = strings.TrimPrefix(version, "Microsoft Windows [")
	version = strings.TrimSuffix(version, "]")
	return strings.TrimSpace(version), nil
}

// runAsRoot executes a command with root privileges if needed
func runAsRoot(name string, args ...string) error {
	// Check if already running as root (Unix-like systems)
	if runtime.GOOS != "windows" {
		cmd := exec.Command("id", "-u")
		output, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(output)) == "0" {
			// Already root, run directly
			cmd := exec.Command(name, args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
	}

	// Check if sudo is available
	if _, err := exec.LookPath("sudo"); err == nil {
		// Use sudo
		fullArgs := append([]string{name}, args...)
		cmd := exec.Command("sudo", fullArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// No sudo available, try running directly
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// installADB attempts to install ADB using the system package manager
func installADB() error {
	if _, err := exec.LookPath("brew"); err == nil {
		// macOS with Homebrew
		cmd := exec.Command("brew", "install", "android-platform-tools")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	if _, err := exec.LookPath("apt-get"); err == nil {
		// Debian/Ubuntu
		return runAsRoot("apt-get", "install", "-y", "android-tools-adb")
	}

	if _, err := exec.LookPath("yum"); err == nil {
		// RHEL/CentOS
		return runAsRoot("yum", "install", "-y", "android-tools")
	}

	return fmt.Errorf("unable to detect package manager")
}

// installFrpc attempts to install frpc using the system package manager or GitHub releases
func installFrpc() error {
	// Try Homebrew first on macOS
	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("brew"); err == nil {
			cmd := exec.Command("brew", "install", "frpc")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err == nil {
				return nil
			}
			// If brew fails, fall through to GitHub installation
		}
	}

	// Download from GitHub releases for all platforms
	return installFrpcFromGitHub()
}

// installFrpcFromGitHub downloads and installs frpc from GitHub releases
func installFrpcFromGitHub() error {
	// Get latest frpc version from GitHub API
	resp, err := http.Get("https://api.github.com/repos/fatedier/frp/releases/latest")
	if err != nil {
		return fmt.Errorf("failed to fetch frpc version: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch frpc version: HTTP %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to parse release info: %v", err)
	}

	// Remove 'v' prefix from version
	frpcVersion := strings.TrimPrefix(release.TagName, "v")
	if frpcVersion == "" {
		return fmt.Errorf("invalid version tag: %s", release.TagName)
	}

	// Detect OS and architecture
	osType := runtime.GOOS
	archType := runtime.GOARCH

	// Map architecture names to frp naming convention
	switch archType {
	case "amd64":
		// Keep as is
	case "arm64":
		// Keep as is
	case "arm":
		// Keep as is
	default:
		return fmt.Errorf("unsupported architecture: %s", archType)
	}

	// Construct download URL
	downloadURL := fmt.Sprintf(
		"https://github.com/fatedier/frp/releases/download/v%s/frp_%s_%s_%s.tar.gz",
		frpcVersion, frpcVersion, osType, archType,
	)

	// Create temporary directory for frpc binary only
	tempDir, err := os.MkdirTemp("", "frpc-install-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Download and stream-extract in one pass
	resp, err = http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download frpc: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download frpc: HTTP %d", resp.StatusCode)
	}

	// Create gzip reader directly from HTTP response body (no intermediate file)
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %v", err)
	}
	defer gzr.Close()

	// Create tar reader from gzip stream
	tr := tar.NewReader(gzr)

	// Extract frpc binary directly from stream
	var frpcBinaryPath string
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %v", err)
		}

		// Look for frpc binary
		if filepath.Base(header.Name) == "frpc" && header.Typeflag == tar.TypeReg {
			frpcBinaryPath = filepath.Join(tempDir, "frpc")

			// Create file with proper permissions
			outFile, err := os.OpenFile(frpcBinaryPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return fmt.Errorf("failed to create frpc binary: %v", err)
			}

			// Stream-copy from tar to file
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to extract frpc binary: %v", err)
			}

			if err := outFile.Close(); err != nil {
				return fmt.Errorf("failed to close frpc binary: %v", err)
			}

			// Found and extracted, stop processing archive
			break
		}
	}

	if frpcBinaryPath == "" {
		return fmt.Errorf("frpc binary not found in archive")
	}

	// Install to system location
	installPath := "/usr/local/bin/frpc"
	if err := installBinaryWithSudo(frpcBinaryPath, installPath); err != nil {
		return fmt.Errorf("failed to install frpc to %s: %v", installPath, err)
	}

	return nil
}

// installBinaryWithSudo installs a binary to the system location, using sudo if necessary
func installBinaryWithSudo(src, dst string) error {
	// Try direct copy first (works if we have write permission)
	if err := copyBinaryFile(src, dst); err == nil {
		return nil
	}

	// If direct copy fails, use install command with runAsRoot (Unix-like systems)
	if runtime.GOOS != "windows" {
		if err := runAsRoot("install", "-m", "755", src, dst); err != nil {
			return fmt.Errorf("install with elevated privileges failed: %v", err)
		}
		return nil
	}

	return fmt.Errorf("permission denied and elevated privileges not available")
}

// copyBinaryFile copies a binary file with executable permissions
func copyBinaryFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source: %v", err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("failed to create destination: %v", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy: %v", err)
	}

	return nil
}
