import React from 'react';
import ReactDOM from 'react-dom/client';
import { AndroidLiveView } from './components/AndroidLiveView';
import './main.css';

// Get configuration from environment or URL parameters
const params = new URLSearchParams(window.location.search);
const apiUrl = params.get('api') || (import.meta as any).env?.VITE_API_URL || '/api';
const wsUrl = params.get('ws') || (import.meta as any).env?.VITE_WS_URL || `ws://${window.location.host}/ws`;
const mode = params.get('mode') || 'h264'; // Default to H.264 mode

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <AndroidLiveView
      apiUrl={apiUrl}
      wsUrl={wsUrl}
      mode={mode as "webrtc" | "h264"}
      autoConnect={false}
      showControls={true}
      showDeviceList={true}
      showAndroidControls={true}
    />
  </React.StrictMode>
);