# gbox

**gbox** is a self-hostable sandbox for AI Agents to execute commands, surf web and use desktop/mobile.

This project is based on the technology behind [gru.ai](https://gru.ai). It has been tested over 100000 Agent jobs.

As MCP is getting more and more popular, we also implemented a MCP server to make it easy to be directly integrated into MCP client such as Claude Desktop/Cursor.

## Use gbox as a CLI

## Installation

### System Requirements for macOS

- macOS 10.15 or later
- [Homebrew](https://brew.sh)

### Installation Steps

```bash
# Install via Homebrew
brew install babelcloud/gru/gbox

# Login to gbox.ai
gbox login

# Export MCP config and merge into Claude Desktop
gbox mcp export --merge-to claude
```

> Note: Using gbox on other platforms, please check npm package [@gbox.ai/cli](https://www.npmjs.com/package/@gbox.ai/cli) for installation instructions.

### Command Line Usage

Check [gbox CLI Reference](https://docs.gbox.ai/cli) for detailed usage.

## Use gbox as a MCP Server(Login required)

Using Gbox CLI to configure MCP server to your Claude Code/ Cursor:
```bash
# Export MCP config for Cursor
gbox mcp export --merge-to claude

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

## Android MCP Use Cases

### 1. Test android apk

Test [geoquiz](https://github.com/babelcloud/geoquiz) apk:
![Image](https://i.imghippo.com/files/DOop9372TM.jpeg)
https://claude.ai/share/78242bf9-201b-40cc-9af8-7f2cdca36e56

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
