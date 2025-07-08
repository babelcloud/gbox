# gbox

**gbox** is a self-hostable sandbox for AI Agents to execute commands, surf web and use desktop/mobile. See "Features" section for details.

This project is based on the technology behind [gru.ai](https://gru.ai). It has been tested over 100000 Agent jobs.

As MCP is getting more and more popular, we also implemented a MCP server to make it easy to be directly integrated into MCP client such as Claude Desktop/Cursor.

## Features
1. Terminal
   - Execute any linux command
   - Execute python scripts directly
   - Share session across invokes <em>[under-development]</em>
2. File
   - Read/Write/List files in sandbox
   - Edit files <em>[under-development]</em>
   - Search files <em>[under-development]</em>
3. Browser
   - Provide cdp url for browser control <em>[under-development]</em>
   - Download from any url <em>[under-development]</em>
   - Human take over <em>[under-development]</em>
4. Computer-Using Agent for Android
   - Natural language task execution via ADB client
   - Physical Android devices supported only
5. HTTP Server
   - Start http service on any folder on demand <em>[under-development]</em>
6. SDKs
   - Typescript SDK Installing using `npm install gbox-sdk`, see [Gbox SDK Reference](https://docs.gbox.cloud/api-reference) for details.
7. MCP
   - Standard MCP support
   - Android device automation (APK management, AI action, key press, screenshot capture, live view)
   - Integrate Claude Code & Claude Desktop & Cursor

## Use gbox as a CLI

## Installation

### System Requirements

- macOS 10.15 or later
- [Docker Desktop for Mac](https://docs.docker.com/desktop/setup/install/mac-install/)
- [Homebrew](https://brew.sh)

> Note: Support for other platforms (Linux, Windows) is coming soon.

### Installation Steps

```bash
# Install via Homebrew
brew tap babelcloud/gru && brew install gbox

# Initialize environment
gbox setup

# Export MCP config and merge into Claude Desktop
gbox mcp export --merge-to claude
# or gbox mcp export --merge-to cursor

# Restart Claude Desktop
```

### Update Steps

```bash
# Update gbox to the latest version
brew update && brew upgrade gbox

# Update the environment
gbox setup

# Export and merge latest MCP config into Claude Desktop
gbox mcp export --merge-to claude
# or gbox mcp export --merge-to cursor

# Restart Claude Desktop
```

### Command Line Usage (Under Development)

The project provides a command-line tool `gbox` for managing sandbox containers:

```bash
# Login to gbox.cloud
gbox login

# Profile management
gbox profile add --key <key> --name <name>                  # add profile
gbox profile list                                           # list all profiles
gbox profile use [index]                                    # switch current profile (interactive if index omitted)
gbox profile delete <index>                                 # delete profile
gbox profile current                                        # show current profile

# Local environment
gbox setup                                                  # initialize local runtime environment (alias: cluster setup)
gbox cleanup                                                # clean up local runtime environment (alias: cluster cleanup)

# Container (Box) management
gbox box create --image python:3.9 --env DEBUG=true -- python -c "print('hello')"  # create a box
gbox box list                                               # list boxes
gbox box terminate <box-id>                                 # terminate box
gbox box exec <box-id> -- ls /                              # execute command inside box
gbox box cp <box-id>:<container-path> <local-path>          # file copy
gbox box inspect <box-id>                                   # inspect box

# Android CUA (requires OPENAI_API_KEY)
gbox cua android "Open Uber and order a ride to CUHK"

# MCP configuration
gbox mcp export                                             # export config only
gbox mcp export --merge-to claude                           # integrate linux mcp server into Claude Desktop
gbox mcp export --merge-to cursor                           # integrate linux mcp server into Cursor
gbox mcp export --merge-to claude-code --type android       # integrate android mcp server into Claude Code
gbox mcp export --dry-run                                   # preview merge result
```

## Computer-Using Agent on Android

### 📋 Prerequisites

1. An Android device connected via USB or ADB over TCP/IP
2. ADB (Android Debug Bridge) installed and configured
3. API key for OpenAI with access to the computer-use-preview model

### 🔧 Setting up ADB

ADB (Android Debug Bridge) is required for gbox to communicate with your Android device:

1. **Install ADB**:
   - **Windows**: Download [Android SDK Platform Tools](https://developer.android.com/studio/releases/platform-tools) and extract the ZIP file
   - **macOS**: `brew install android-platform-tools`
   - **Linux**: `sudo apt install adb` (Ubuntu/Debian) or `sudo pacman -S android-tools` (Arch)

2. **Add ADB to your PATH**:
   - **Windows**: Add the path to the extracted platform-tools folder to your system's PATH environment variable
   - **macOS/Linux**: Add the following to your ~/.bashrc or ~/.zshrc:
     ```bash
     export PATH=$PATH:/path/to/platform-tools
     ```

3. **Verify ADB installation**:
   ```bash
   adb version
   ```

4. **Enable USB debugging on your Android device**:
   - Go to **Settings → About phone**
   - Tap **Build number** 7 times to enable Developer options
   - Go to **Settings → System → Developer options** (location may vary by device)
   - Enable **USB debugging**

5. **Connect Android device via USB and confirm ADB connected**
   ```bash
   adb devices -l
   List of devices attached
   897X0691U       Pixel_3 device
   ```
6. **Set the OPENAI_API_KEY environment variable and enjoy gbox Android CUA**
   ```bash
   export OPENAI_API_KEY=YOUR_KEY
   gbox cua android "Open Uber and book a ride to The Chinese University of Hong Kong now."
   ```
   
   The video demonstration for this task can be found in the first use case below under "Use Cases" section.


### Use Cases
#### 1. Call an Uber

**Prompt**: Open Uber and book a ride to The Chinese University of Hong Kong now.

<video src="https://github.com/user-attachments/assets/804c3bf4-8269-4d3b-88b0-52ad3b74aec2" width="720" height="1448"></video>

#### 2. Use DoorDash/Foodpanda to order food delivery.
<img src="https://i.imghippo.com/files/hYdz1413YaE.jpg" width="40%" height="40%">

## Linux MCP Use Cases

Your AI client such as Claude Desktop can use gbox MCP to deliver better results, such as

### 1. Generating Diagrams

Generate diagrams of Tesla stock prices:
![Image](https://i.imghippo.com/files/njBB6977VQQ.png)
https://claude.ai/share/34de8ca3-4e04-441b-9e79-5875fa9fc97a

### 2. Generating PDFs

Generate PDF of latest AI news:
![Image](https://i.imghippo.com/files/oMF9723LA.png)
https://claude.ai/share/84600933-dcf2-44be-a2fd-7f49540db57a

### 3. Analyzing and Calculation

Analyze and compare Nvidia/Tesla market cap:
![Image](https://i.imghippo.com/files/FE2710WR.png)
https://claude.ai/share/70c335b7-9fff-4ee7-8459-e6b7462d8994

### 5. Execute Arbitrary Tasks

Download youtube video:
![Image](https://i.imghippo.com/files/TI9396Rjg.png)
https://claude.ai/share/c2ab6bcb-7032-489f-87d5-cc38f72c2ca9

## Android MCP Use Cases

### 1. Test android apk

Test [geoquiz](https://github.com/babelcloud/geoquiz) apk:
![Image](https://i.imghippo.com/files/DOop9372TM.jpeg)
https://claude.ai/share/78242bf9-201b-40cc-9af8-7f2cdca36e56

## Develop gbox

### Prerequisites

- Go 1.21 or later
- Docker Desktop
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
# API Server
make -C packages/api-server dev

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

#### How to run gbox in dev env instead of the system installed one

1. Stop the installed gbox by `gbox cleanup`. It will stop the api server so that you can run the api server in dev env.
2. Execute `make api-dev` in project root.
3. Execute `./gbox box list`, this is the command run from your dev env.

#### How to connect MCP client such as Claude Desktop to the MCP server in dev env

1. Execute `make mcp-dev` in project root.
2. Execute `./gbox mcp export --merge-to claude`

#### How to open MCP inspect

1. Execute `make mcp-inspect` in project root.
2. Click the link returned in terminal.

#### How to build and use image in dev env

1. Execute `make build-image-python` in project root to build Python image, or `make build-images` to build all images.
2. Change the image name as needed (e.g., `make build-image-typescript` for TypeScript image).
3. You may need to delete current sandboxes to make the new image effective `./gbox box delete --all`

#### Why MCP client still get the old MCP content?

1. After you change MCP configuration such as tool definitions, you need to run `make build` to update the `dist/index.js` file.
2. You may also need to execute `./gbox mcp export --merge-to claude`

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
