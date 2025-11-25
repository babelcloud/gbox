# @babelcloud/live-view

Live view component for Android device streaming using WebRTC.

## Features

- Real-time Android screen mirroring
- WebRTC-based low-latency streaming
- Touch and control input support
- Android system button controls
- Device list management
- Auto-reconnection support

## Installation

At first, go to [GitHub token settings](https://github.com/settings/tokens) to create a classic token. Make sure you checked `repo` and `read:packages` scopes.

<img width="2222" height="1350" alt="image" src="https://github.com/user-attachments/assets/f556eadf-15ca-4b04-aab1-8618c2c7470d" />

Then, create a `.npmrc` file in the project root directory and add content as below. 

```
@babelcloud:registry=https://npm.pkg.github.com
//npm.pkg.github.com/:_authToken=${YOUR_GITHUB_PERSONAL_ACCESS_TOKEN}
```

Finally you can install it.

```bash
npm install @babelcloud/live-view
# or
pnpm add @babelcloud/live-view
```

## Usage

### As a React Component

```tsx
import { AndroidLiveView } from '@babelcloud/live-view';

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
