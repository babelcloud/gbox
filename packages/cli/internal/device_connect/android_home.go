package device_connect

import (
	"os"
	"path/filepath"
	"runtime"
)

// DetectAndroidHome returns the Android SDK root directory (ANDROID_HOME) if found.
// It checks the ANDROID_HOME/ANDROID_SDK_ROOT env vars first, then platform-specific
// default locations. Returns (path, true) only when the path exists and looks like
// an SDK (e.g. contains platform-tools or platforms).
func DetectAndroidHome() (string, bool) {
	// 1. Explicit environment variables
	for _, key := range []string{"ANDROID_HOME", "ANDROID_SDK_ROOT"} {
		if path := os.Getenv(key); path != "" {
			path = filepath.Clean(path)
			if isAndroidSDKRoot(path) {
				return path, true
			}
		}
	}

	// 2. Platform-specific default locations
	var candidates []string
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "darwin":
		// Android Studio default
		candidates = append(candidates, filepath.Join(home, "Library", "Android", "sdk"))
		// Homebrew (Intel: /usr/local, Apple Silicon: /opt/homebrew)
		candidates = append(candidates, "/usr/local/opt/android-sdk", "/opt/homebrew/opt/android-sdk")
		// Homebrew Cellar (versioned path)
		if cellar := filepath.Join("/usr/local", "Cellar", "android-sdk"); home != "" {
			// We can't list dir easily without extra imports; try common version
			candidates = append(candidates, cellar)
		}
		candidates = append(candidates, filepath.Join("/opt/homebrew", "Cellar", "android-sdk"))
	case "linux":
		candidates = append(candidates,
			filepath.Join(home, "Android", "Sdk"),
			filepath.Join(home, "android-sdk"),
			"/usr/lib/android-sdk",
		)
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			candidates = append(candidates, filepath.Join(localAppData, "Android", "Sdk"))
		}
		candidates = append(candidates,
			"C:\\Android\\Sdk",
			filepath.Join(home, "AppData", "Local", "Android", "Sdk"),
		)
	default:
		candidates = append(candidates,
			filepath.Join(home, "Android", "Sdk"),
			filepath.Join(home, "android-sdk"),
		)
	}

	for _, p := range candidates {
		if p == "" {
			continue
		}
		if isAndroidSDKRoot(p) {
			return p, true
		}
	}

	return "", false
}

// isAndroidSDKRoot returns true if dir exists and looks like an Android SDK root
// (contains platform-tools or platforms directory).
func isAndroidSDKRoot(dir string) bool {
	if dir == "" {
		return false
	}
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		return false
	}
	// Typical SDK has platform-tools and/or platforms
	platformTools := filepath.Join(dir, "platform-tools")
	platforms := filepath.Join(dir, "platforms")
	if _, err := os.Stat(platformTools); err == nil {
		return true
	}
	if _, err := os.Stat(platforms); err == nil {
		return true
	}
	return false
}
