# GBOX ANDROID MCP SERVER

> MCP server exposing Gbox Android control tools via Model Context Protocol

[![npm version](https://img.shields.io/npm/v/gbox-mcp-android-server.svg)](https://www.npmjs.com/package/gbox-mcp-android-server)
[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## Description

This package provides an MCP (Model Context Protocol) server for controlling Android devices via Gbox tools. It exposes a set of tools and APIs for automation, device management, and integration with the Gbox ecosystem.

## Usage

Copy the following configuration into your Cursor or Claude code MCP config file:

```json
"gbox-android": {
  "command": "npx",
  "args": [
    "-y",
    "@gbox.ai/mcp-android-server"
  ],
  // NOTE: You can omit the 'env' section if you have already configured your Gbox profile locally using the Gbox CLI.
  "env": {
    "GBOX_API_KEY": "gbox_xxxx"
  }
}
```

For instructions on logging in and configuring your profile, please refer to the [Gbox CLI Documentation](https://github.com/babelcloud/gbox).

If you are already logged in, you can obtain your `GBOX_API_KEY` from the Personal tab at [gbox.ai/dashboard](https://gbox.ai/dashboard).

To learn more about **GBOX**, be sure to check out the [official documentation](https://docs.gbox.ai).