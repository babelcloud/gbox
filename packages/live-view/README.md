# @gbox.ai/live-view

Live view component for Android device streaming using WebRTC.

## Features

- Real-time Android screen mirroring
- WebRTC-based low-latency streaming
- Touch and control input support
- Android system button controls
- Device list management
- Auto-reconnection support

## Installation

```bash
npm install @gbox.ai/live-view
# or
pnpm add @gbox.ai/live-view
```

## Usage

### As a React Component

```tsx
import { AndroidLiveView } from '@gbox.ai/live-view';

function App() {
  return (
    <AndroidLiveView
      apiUrl="/api"
      wsUrl="ws://localhost:8080/ws"
      showControls={true}
      showDeviceList={true}
      showAndroidControls={true}
      onConnect={(device) => console.log('Connected to', device)}
      onDisconnect={() => console.log('Disconnected')}
      onError={(error) => console.error('Error:', error)}
    />
  );
}
```

### Props

- `apiUrl`: API endpoint URL (default: `/api`)
- `wsUrl`: WebSocket URL for WebRTC signaling (default: `ws://localhost:8080/ws`)
- `deviceSerial`: Auto-connect to specific device
- `autoConnect`: Auto-connect when device is available
- `showControls`: Show video controls and stats
- `showDeviceList`: Show device list sidebar
- `showAndroidControls`: Show Android control buttons
- `onConnect`: Callback when device connects
- `onDisconnect`: Callback when device disconnects
- `onError`: Error handler callback
- `className`: Additional CSS class name

## Development

```bash
# Install dependencies
pnpm install

# Run development server
pnpm dev

# Build component library
pnpm build:component

# Build static site
pnpm build:static

# Build both
pnpm build
```

## Publishing

This package is configured to publish to GitHub Packages registry.

```bash
# Login to GitHub registry
npm login --registry=https://npm.pkg.github.com

# Publish
npm publish
```

## License

Apache-2.0
