import React from 'react';
import ReactDOM from 'react-dom/client';
import { AndroidLiveView } from './components/AndroidLiveView';
import './main.css';

// Get configuration from environment or URL parameters
const params = new URLSearchParams(window.location.search);
const importMeta = import.meta as { env?: { VITE_API_URL?: string; VITE_WS_URL?: string } };
const apiUrl = params.get('api') || importMeta.env?.VITE_API_URL || '/api';
const wsUrl = params.get('ws') || importMeta.env?.VITE_WS_URL || `ws://${window.location.host}`;
const mode = params.get('mode') || 'separated'; // Default to separated mode

const rootElement = document.getElementById('root');
if (!rootElement) {
  throw new Error('Root element not found');
}

ReactDOM.createRoot(rootElement).render(
  <React.StrictMode>
    <AndroidLiveView
      apiUrl={apiUrl}
      wsUrl={wsUrl}
      mode={mode as "webrtc" | "separated" | "muxed"}
      autoConnect={false}
      showControls={true}
      showDeviceList={true}
      showAndroidControls={true}
    />
  </React.StrictMode>
);