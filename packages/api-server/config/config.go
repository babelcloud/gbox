package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/babelcloud/gbox/packages/api-server/pkg/logger"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var (
	instance *Config
	once     sync.Once
	v        *viper.Viper
	log      = logger.New()
)

// Config represents the application configuration
type Config struct {
	Server  ServerConfig
	File    FileConfig
	Cluster ClusterConfig
	Browser BrowserConfig
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Port int
}

// FileConfig represents file service configuration
type FileConfig struct {
	Home      string `mapstructure:"home"`
	Share     string `mapstructure:"share"`
	HostShare string `mapstructure:"host_share"`
}

// ClusterConfig represents cluster configuration
type ClusterConfig struct {
	Mode                   string        `yaml:"mode"`
	ReclaimStopThreshold   time.Duration `yaml:"reclaimStopThreshold"`
	ReclaimDeleteThreshold time.Duration `yaml:"reclaimDeleteThreshold"`
	Namespace              string        `yaml:"namespace"`
	Docker                 DockerConfig  `yaml:"docker"`
	K8s                    K8sConfig     `yaml:"k8s"`
}

// DockerConfig represents Docker-specific configuration
type DockerConfig struct {
	Host string
}

// K8sConfig represents Kubernetes-specific configuration
type K8sConfig struct {
	Config string
}

// BrowserConfig represents browser service specific configuration
type BrowserConfig struct {
	Host string `yaml:"host"`
}

func init() {
	v = viper.New()

	// Environment variables
	v.AutomaticEnv()
	v.BindEnv("cluster.mode", "CLUSTER_MODE")
	v.BindEnv("cluster.reclaimStopThreshold", "RECLAIM_STOP_THRESHOLD")
	v.BindEnv("cluster.reclaimDeleteThreshold", "RECLAIM_DELETE_THRESHOLD")
	v.BindEnv("server.port", "PORT")
	v.BindEnv("cluster.docker.host", "DOCKER_HOST")
	v.BindEnv("cluster.k8s.cfg", "KUBECONFIG")
	v.BindEnv("file.home", "GBOX_HOME")
	v.BindEnv("file.share", "GBOX_SHARE")
	v.BindEnv("file.host_share", "GBOX_HOST_SHARE")
	v.BindEnv("cluster.namespace", "GBOX_NAMESPACE")
	v.BindEnv("browser.host", "GBOX_BROWSER_HOST")

	// Image environment variables (bound to dynamically generated keys)
	v.BindEnv("gbox.python.img.tag", "PY_IMG_TAG")
	v.BindEnv("gbox.typescript.img.tag", "TS_IMG_TAG")

	// Config file
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	// Look for config in the following paths
	configPaths := []string{
		".",
		"$HOME/.gbox",
		"/etc/gbox",
	}

	for _, path := range configPaths {
		expandedPath := os.ExpandEnv(path)
		v.AddConfigPath(expandedPath)
	}

	// Read config file if it exists
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file was found but another error was produced
			log.Error("Error reading config file: %v", err)
		}
		// Config file not found; ignore error and use defaults
	}
}

// GetInstance returns the singleton Config instance
func GetInstance() *Config {
	once.Do(func() {
		var err error
		instance, err = New()
		if err != nil {
			panic(fmt.Sprintf("failed to create config: %v", err))
		}

		// Debug output
		if os.Getenv("DEBUG") == "true" {
			// Convert config to YAML
			yamlData, err := yaml.Marshal(instance)
			if err != nil {
				log.Error("Failed to marshal config to YAML: %v", err)
			} else {
				log.Debug("Final configuration:\n%s", string(yamlData))
			}
		}
	})
	return instance
}

// initFileConfig initializes file service configuration
func initFileConfig(homeDir string) (FileConfig, error) {
	// Get raw values from config
	fileHome := v.GetString("file.home")
	fileShare := v.GetString("file.share")
	fileHostShare := v.GetString("file.host_share")

	// First expand ${HOME} to actual home directory
	fileHome = os.ExpandEnv(fileHome)
	if fileHome == "" {
		fileHome = filepath.Join(homeDir, ".gbox")
	}

	// Then replace ${file.home} with actual home path
	fileShare = os.ExpandEnv(fileShare)
	if fileShare == "" {
		fileShare = filepath.Join(fileHome, "share")
	} else {
		// Replace ${file.home} in the path
		fileShare = strings.ReplaceAll(fileShare, "${file.home}", fileHome)
	}

	// Finally replace ${file.share} with actual share path
	fileHostShare = os.ExpandEnv(fileHostShare)
	if fileHostShare == "" {
		fileHostShare = fileShare
	} else {
		// Replace ${file.share} in the path
		fileHostShare = strings.ReplaceAll(fileHostShare, "${file.share}", fileShare)
	}

	// Create directories if they don't exist
	if err := os.MkdirAll(fileHome, 0755); err != nil {
		return FileConfig{}, fmt.Errorf("failed to create home directory: %v", err)
	}

	if err := os.MkdirAll(fileShare, 0755); err != nil {
		return FileConfig{}, fmt.Errorf("failed to create share directory: %v", err)
	}

	return FileConfig{
		Home:      fileHome,
		Share:     fileShare,
		HostShare: fileHostShare,
	}, nil
}

// initClusterConfig initializes cluster configuration
func initClusterConfig(homeDir string) (ClusterConfig, error) {
	dockerHost := v.GetString("cluster.docker.host")
	if dockerHost == "" {
		dockerHost = findDockerSocket(homeDir)
	}

	kubeConfig := v.GetString("cluster.k8s.cfg")
	if kubeConfig == "" {
		kubeConfig = findKubeConfig(homeDir)
	}

	return ClusterConfig{
		Mode:                   v.GetString("cluster.mode"),
		ReclaimStopThreshold:   v.GetDuration("cluster.reclaimStopThreshold"),
		ReclaimDeleteThreshold: v.GetDuration("cluster.reclaimDeleteThreshold"),
		Namespace:              v.GetString("cluster.namespace"),
		Docker: DockerConfig{
			Host: dockerHost,
		},
		K8s: K8sConfig{
			Config: kubeConfig,
		},
	}, nil
}

// findDockerSocket finds the Docker socket path
func findDockerSocket(homeDir string) string {
	// Try user's home directory socket first
	userSocket := filepath.Join(homeDir, ".docker", "run", "docker.sock")
	if _, err := os.Stat(userSocket); err == nil {
		return fmt.Sprintf("unix://%s", userSocket)
	}

	// If user socket doesn't exist, try /var/run/docker.sock
	systemSocket := "/var/run/docker.sock"
	if _, err := os.Stat(systemSocket); err == nil {
		return fmt.Sprintf("unix://%s", systemSocket)
	}

	return "unix:///var/run/docker.sock" // Default fallback
}

// findKubeConfig finds the Kubernetes config path
func findKubeConfig(homeDir string) string {
	// Try user's home directory
	userConfig := filepath.Join(homeDir, ".kube", "config")
	if _, err := os.Stat(userConfig); err == nil {
		return userConfig
	}

	// If user config doesn't exist, try /etc/kubernetes/admin.conf
	systemConfig := "/etc/kubernetes/admin.conf"
	if _, err := os.Stat(systemConfig); err == nil {
		return systemConfig
	}

	return filepath.Join(homeDir, ".kube", "config") // Default fallback
}

// New creates a new configuration instance
func New() (*Config, error) {
	// Initialize default values
	cfg := &Config{
		Server: ServerConfig{
			Port: 28080,
		},
		File: FileConfig{
			Home:      filepath.Join(os.Getenv("HOME"), ".gbox"),
			Share:     filepath.Join(os.Getenv("HOME"), ".gbox", "share"), // Default based on container's HOME
			HostShare: filepath.Join(os.Getenv("HOME"), ".gbox", "share"), // Default based on container's HOME
		},
		Cluster: ClusterConfig{
			Mode:                   "docker",
			ReclaimStopThreshold:   30 * time.Minute,
			ReclaimDeleteThreshold: 24 * time.Hour,
			Namespace:              "gbox-boxes",
			Docker: DockerConfig{
				Host: findDockerSocket(os.Getenv("HOME")),
			},
			K8s: K8sConfig{
				Config: findKubeConfig(os.Getenv("HOME")),
			},
		},
		Browser: BrowserConfig{
			Host: "localhost",
		},
	}

	// Load configuration from viper
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %v", err)
	}

	// Resolve paths (Original logic restored)
	cfg.File.Home = os.ExpandEnv(cfg.File.Home)
	cfg.File.Share = os.ExpandEnv(cfg.File.Share)
	cfg.File.HostShare = os.ExpandEnv(cfg.File.HostShare)

	// Create directories if they don't exist
	if err := os.MkdirAll(cfg.File.Home, 0755); err != nil {
		return nil, fmt.Errorf("failed to create home directory '%s': %v", cfg.File.Home, err)
	}
	if err := os.MkdirAll(cfg.File.Share, 0755); err != nil {
		return nil, fmt.Errorf("failed to create share directory '%s': %v", cfg.File.Share, err)
	}

	// Note: findDockerSocket/findKubeConfig are called during default initialization.
	// If Viper unmarshals non-empty values for Docker.Host or K8s.Config, those will be used.

	return cfg, nil
}

func CheckImageTag(imgName string) string {
	// 1. If the image name already contains a tag, return it as is.
	if strings.Contains(imgName, ":") {
		log.Info("Image name '%s' already contains a tag. Using original name.", imgName)
		return imgName
	}

	log.Debug("Checking image tag for '%s'", imgName)
	// 2. Extract repo name and base name once.
	repoName := ""
	baseName := imgName
	if strings.Contains(imgName, "/") {
		parts := strings.Split(imgName, "/")
		if len(parts) > 1 {
			// Assuming format like "repo/image" or "registry/repo/image"
			repoName = strings.Join(parts[:len(parts)-1], "/") + "/"
			baseName = parts[len(parts)-1]
		} // else: malformed name like "/imagename", treat imgName as baseName
	}
	log.Debug("Repo name: '%s', Base name: '%s'", repoName, baseName)

	// 3. Generate Viper key: replace '-' with '.' and append '.img.tag'.
	vipKey := strings.ReplaceAll(baseName, "-", ".") + ".img.tag"
	// Ensure key is lowercase if needed, although BindEnv is case-insensitive by default
	vipKey = strings.ToLower(vipKey)
	log.Debug("Dynamically generated key: %s", vipKey)

	// 4. Check if the dynamically generated key is set in Viper (maps to env var).
	if v.IsSet(vipKey) {
		tag := v.GetString(vipKey)
		if tag != "" {
			// Use the extracted repoName and baseName
			resolvedImage := repoName + baseName + ":" + tag
			log.Info("Resolved image name '%s' to '%s' using dynamically generated key '%s'", imgName, resolvedImage, vipKey)
			return resolvedImage // Return the fully formed image name with tag
		} else {
			log.Warn("Dynamically generated key '%s' for image '%s' is set but empty.", vipKey, imgName)
		}
	} else {
		log.Debug("Dynamically generated key '%s' for image '%s' is not set.", vipKey, imgName)
	}

	// 5. If not resolved via env var, return the original untagged name..
	return imgName
}

func GetPythonImageTag() string {
	return v.GetString("gbox.python.img.tag")
}
