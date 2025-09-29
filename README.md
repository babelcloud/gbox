# GBOX

![GBOX Animation](https://github.com/user-attachments/assets/50a6ebb4-d432-4364-b651-1738855a4b1f)

**GBOX** provides environments for AI Agents to operate computer and mobile devices.

![GBOX Introduction](https://github.com/user-attachments/assets/eded50bd-4498-4bca-85f8-fb3ec272e032)

*Mobile Scenario:*
Your agents can use GBOX to develop/test android apps, or run apps on the Android to complete various tasks(mobile automation).

*Desktop Scenario:*
Your agents can use GBOX to operate desktop apps such as browser, terminal, VSCode, etc(desktop automation).

*MCP:* 
You can also plug GBOX MCP to any Agent you like, such as Cursor, Claude Code. These agents will instantly get the ability to operate computer and mobile devices.

## Installation

### System Requirements

- MacOS 
  - Version: 10.15 or later
  - [Homebrew](https://brew.sh)

> Note: Using gbox on other platforms, please check npm package [@gbox.ai/cli](https://www.npmjs.com/package/@gbox.ai/cli) for installation instructions. You can also login to [GBOX.AI](https://gbox.ai) to use web-based dashboard.

### Installation Steps

```bash
# Install via Homebrew (on MacOS)
brew install gbox
# Login to gbox.ai
gbox login

# Export MCP config and merge into Claude Code/Cursor
gbox mcp export --merge-to claude-code
gbox mcp export --merge-to cursor
```

### Command Line Usage

Check [GBOX CLI Reference](https://docs.gbox.ai/cli) for detailed usage.

### SDK Usage

Check [GBOX SDK Reference](https://docs.gbox.ai/sdk) for detailed usage.

## Use GBOX as a MCP Server(Login required)

Using GBOX CLI to configure MCP server to your Claude Code/Cursor:
```bash
# Export MCP config for Cursor
gbox mcp export --merge-to cursor

# Export MCP config for Claude Code
gbox mcp export --merge-to claude-code --scope project

```

Or copy paste the following content into your Claude Code/Cursor MCP config:
```json
{
  "mcpServers": {
    "gbox-android": {
      "command": "npx",
      "args": [
        "-y",
        "@gbox.ai/mcp-android-server@latest"
      ]
    }
  }
}
```
> Note:
> - Currently, GBOX MCP can only control Android environments.
> - If you need Cursor/Claude Code to control your local Android device, please check [Register Local Device](https://docs.gbox.ai/cli/register-local-device)

## Android MCP Use Cases

| Use Case | Demo |
|----------|------|
| Claude Code Develop/Test Android App | [![Claude Code Develop/Test Android App](https://img.youtube.com/vi/IzlZFsqC4CY/maxresdefault.jpg)](https://www.youtube.com/watch?v=IzlZFsqC4CY) |
| Claude Code Compare Prices on eCommerce Apps | [![Claude Code Compare Prices on eCommerce Apps](https://img.youtube.com/vi/Op3ZSVg-qg8/maxresdefault.jpg)](https://www.youtube.com/watch?v=Op3ZSVg-qg8) |


## Environments
Currently, GBOX supports the following environments:
- Android
- Linux Desktop/Browser

### Android Environment
There are three types of Android environments, you can choose based on your needs:

**1. Cloud Virtual Device:** 

Login to [GBOX.AI](https://gbox.ai) to get a cloud virtual device. Best for testing and development.

**2. Cloud Physical Device:** 

Login to [GBOX.AI](https://gbox.ai) to get a cloud physical device. Cloud physical device is a real Android phone that you can use for production scenarios.

**3. Local Physical Device:** 

Use your own physical device [Register Local Device](https://docs.gbox.ai/cli/register-local-device). Your local device can be any Android device that have Developer Mode enabled. Best for production scenarios and personal use.

### Linux Desktop/Browser Environment

Login to [GBOX.AI](https://gbox.ai) to get a Linux desktop/browser environment. Best for testing and development.

## Develop gbox

### Prerequisites

- Go 1.21 or later
- Make
- pnpm (via corepack)
- Node.js 16.13 or later

### Build

```bash
# Build all components
make build

# Create distribution package
make dist
```

### Running Services

```bash
# MCP Server
cd packages/mcp-server && pnpm dev

# MCP Inspector
cd packages/mcp-server && pnpm inspect
```

### Contributing

We welcome contributions! Please feel free to submit a Pull Request. For major changes, please open an issue first to discuss what you would like to change.

1. Fork the repository
2. Create your feature branch (`git checkout -b username/feature-name`)
3. Commit your changes (`git commit -m 'Add some feature'`)
4. Push to the branch (`git push origin username/feature-name`)
5. Open a Pull Request

### Things to Know about Dev and Debug Locally

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
