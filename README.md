# GBOX

**GBOX** provides environments for AI Agents to operate computer and mobile devices.

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

> Note: Using gbox on other platforms, please check npm package [@gbox.ai/cli](https://www.npmjs.com/package/@gbox.ai/cli) for installation instructions.

### Installation Steps

```bash
# Install via Homebrew (on macOS)
brew install gbox
# Login to gbox.ai
gbox login

# Export MCP config and merge into Claude Code/ Cursor
gbox mcp export --merge-to claude-code
gbox mcp export --merge-to cursor
```

### Command Line Usage

Check [GBOX CLI Reference](https://docs.gbox.ai/cli) for detailed usage.

## Use GBOX as a MCP Server(Login required)

Using GBOX CLI to configure MCP server to your Claude Code/ Cursor:
```bash
# Export MCP config for Cursor
gbox mcp export --merge-to cursor

# Export MCP config for Claude Code
gbox mcp export --merge-to claude-code --scope project

```

Or copy paste the following content into your Claude Code/ Cursor MCP config:
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
> Note: Currently, GBOX MCP is only available for Android.

## Android MCP Use Cases

### 1. Claude Code Develop/Test Android App

[![Claude Code Develop/Test Android App](https://img.youtube.com/vi/qFrPXKK9RW0/maxresdefault.jpg)](https://www.youtube.com/watch?v=qFrPXKK9RW0)


### 2. Claude Code Compare Prices on eCommerce Apps

[![Claude Code Compare Prices on eCommerce Apps](https://img.youtube.com/vi/-2vzBaIU3hQ/maxresdefault.jpg)](https://www.youtube.com/watch?v=-2vzBaIU3hQ)

## Environments
Currently, GBOX supports the following environments:
- Android
- Linux Desktop/Browser

### Android Environment
There are three types of Android environments, you can choose based on your needs:

**1. Cloud Virtual Device:** 

Login to GBOX.AI to get a cloud virtual device. Best for testing and development.

**2. Cloud Physical Device:** 

Login to GBOX.AI to get a cloud physical device. Cloud physical device is a real Android phone that you can use for production scenarios.

**3. Local Physical Device:** 

Use your own physical device [How to use](https://docs.gbox.ai/cli/android-local-device). Your local device can be any Android device that have Developer Mode enabled. Best for production scenarios and personal use.

### Linux Desktop/Browser Environment

Login to GBOX.AI to get a Linux desktop/browser environment. Best for testing and development.

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
