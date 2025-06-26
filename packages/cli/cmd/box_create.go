package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	// 内部 SDK 客户端
	sdk "github.com/babelcloud/gbox-sdk-go"
	model "github.com/babelcloud/gbox/packages/api-server/pkg/box"
	gboxclient "github.com/babelcloud/gbox/packages/cli/internal/gboxsdk"
	"github.com/spf13/cobra"
)

type BoxCreateOptions struct {
	OutputFormat    string
	Image           string
	Env             []string
	Labels          []string
	WorkingDir      string
	Command         []string
	ImagePullSecret string
	Volumes         []string
}

type BoxCreateResponse struct {
	ID string `json:"id"`
}

func parseKeyValuePairs(pairs []string, pairType string) (map[string]string, error) {
	if len(pairs) == 0 {
		return nil, nil
	}

	result := make(map[string]string)
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		} else {
			return nil, fmt.Errorf("invalid %s format: %s (must be KEY=VALUE)", pairType, pair)
		}
	}
	return result, nil
}

// parseVolumes parses volume mount strings in the format "source:target[:ro][:propagation]"
func parseVolumes(volumes []string) ([]model.VolumeMount, error) {
	if len(volumes) == 0 {
		return nil, nil
	}

	result := make([]model.VolumeMount, 0, len(volumes))
	for _, volume := range volumes {
		parts := strings.Split(volume, ":")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid volume format: %s (must be source:target[:ro][:propagation])", volume)
		}

		mount := model.VolumeMount{
			Source: parts[0],
			Target: parts[1],
		}

		// Parse optional flags
		for i := 2; i < len(parts); i++ {
			switch parts[i] {
			case "ro":
				mount.ReadOnly = true
			case "private", "rprivate", "shared", "rshared", "slave", "rslave":
				mount.Propagation = parts[i]
			default:
				return nil, fmt.Errorf("invalid volume option: %s", parts[i])
			}
		}

		result = append(result, mount)
	}

	return result, nil
}

func NewBoxCreateCommand() *cobra.Command {
	opts := &BoxCreateOptions{}

	cmd := &cobra.Command{
		Use:   "create [flags] -- [command] [args...]",
		Short: "Create a new box",
		Long: `Create a new box with various options for image, environment, and commands.

You can specify box configurations through various flags, including which container image to use,
setting environment variables, adding labels, and specifying a working directory.

Command arguments can be specified directly in the command line or added after the '--' separator.`,
		Example: `  gbox box create --image python:3.9 -- python3 -c 'print("Hello")'
  gbox box create --env PATH=/usr/local/bin:/usr/bin:/bin -w /app -- node server.js
  gbox box create --label project=myapp --label env=prod -- python3 server.py
  gbox box create --volumes /host/path:/container/path:ro:rprivate --image python:3.9`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(opts, args)
		},
		DisableFlagsInUseLine: true,
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.OutputFormat, "output", "o", "text", "Output format (json or text)")
	flags.StringVarP(&opts.Image, "image", "i", "", "Container image to use")
	flags.StringArrayVar(&opts.Env, "env", []string{}, "Environment variables in KEY=VALUE format")
	flags.StringArrayVarP(&opts.Labels, "label", "l", []string{}, "Custom labels in KEY=VALUE format")
	flags.StringVarP(&opts.WorkingDir, "work-dir", "w", "", "Working directory")
	flags.StringArrayVarP(&opts.Volumes, "volume", "v", nil, "Bind mount a volume (source:target[:ro][:propagation])")

	cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json", "text"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func runCreate(opts *BoxCreateOptions, args []string) error {
	// 创建 SDK 客户端
	client, err := gboxclient.NewClientFromProfile()
	if err != nil {
		return fmt.Errorf("failed to initialize gbox client: %v", err)
	}

	// 解析环境变量
	envMap, err := parseKeyValuePairs(opts.Env, "environment variable")
	if err != nil {
		return err
	}

	// 解析标签
	labelMap, err := parseKeyValuePairs(opts.Labels, "label")
	if err != nil {
		return err
	}

	// 解析卷挂载
	volumes, err := parseVolumes(opts.Volumes)
	if err != nil {
		return err
	}

	// 构建命令和参数
	var cmd string
	var cmdArgs []string
	if len(opts.Command) > 0 {
		cmd = opts.Command[0]
		if len(opts.Command) > 1 {
			cmdArgs = opts.Command[1:]
		}
	} else if len(args) > 0 {
		cmd = args[0]
		if len(args) > 1 {
			cmdArgs = args[1:]
		}
	}

	// 构建 SDK 参数
	createParams := sdk.V1BoxNewLinuxParams{
		CreateLinuxBox: sdk.CreateLinuxBoxParam{
			Wait: sdk.Bool(true), // 等待操作完成
			Config: sdk.CreateBoxConfigParam{
				Envs:   envMap,
				Labels: labelMap,
			},
		},
	}

	// 如果有工作目录，添加到标签中（因为 SDK 配置中没有直接的工作目录字段）
	if opts.WorkingDir != "" {
		if createParams.CreateLinuxBox.Config.Labels == nil {
			createParams.CreateLinuxBox.Config.Labels = make(map[string]interface{})
		}
		createParams.CreateLinuxBox.Config.Labels.(map[string]interface{})["working_dir"] = opts.WorkingDir
	}

	// 如果有命令，添加到标签中
	if cmd != "" {
		if createParams.CreateLinuxBox.Config.Labels == nil {
			createParams.CreateLinuxBox.Config.Labels = make(map[string]interface{})
		}
		createParams.CreateLinuxBox.Config.Labels.(map[string]interface{})["cmd"] = cmd
		if len(cmdArgs) > 0 {
			createParams.CreateLinuxBox.Config.Labels.(map[string]interface{})["args"] = cmdArgs
		}
	}

	// 如果有镜像，添加到标签中
	if opts.Image != "" {
		if createParams.CreateLinuxBox.Config.Labels == nil {
			createParams.CreateLinuxBox.Config.Labels = make(map[string]interface{})
		}
		createParams.CreateLinuxBox.Config.Labels.(map[string]interface{})["image"] = opts.Image
	}

	// 如果有卷挂载，添加到标签中
	if len(volumes) > 0 {
		if createParams.CreateLinuxBox.Config.Labels == nil {
			createParams.CreateLinuxBox.Config.Labels = make(map[string]interface{})
		}
		createParams.CreateLinuxBox.Config.Labels.(map[string]interface{})["volumes"] = volumes
	}

	// 调试输出
	if os.Getenv("DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "Request params:\n")
		requestJSON, _ := json.MarshalIndent(createParams, "", "  ")
		fmt.Fprintln(os.Stderr, string(requestJSON))
	}

	// 调用 SDK
	ctx := context.Background()
	box, err := client.V1.Boxes.NewLinux(ctx, createParams)
	if err != nil {
		return fmt.Errorf("failed to create box: %v", err)
	}

	// 输出结果
	if opts.OutputFormat == "json" {
		boxJSON, _ := json.MarshalIndent(box, "", "  ")
		fmt.Println(string(boxJSON))
	} else {
		fmt.Printf("Box created with ID \"%s\"\n", box.ID)
	}

	return nil
}
