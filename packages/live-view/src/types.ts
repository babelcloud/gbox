export interface Device {
  serial: string;
  state: string;
  model?: string;
  connected?: boolean;
  videoWidth?: number;
  videoHeight?: number;
}

export interface LiveViewProps {
  apiUrl?: string;
  wsUrl?: string;
  autoConnect?: boolean;
  showControls?: boolean;
  showDeviceList?: boolean;
  onConnect?: (device: Device) => void;
  onDisconnect?: () => void;
  onError?: (error: Error) => void;
  className?: string;
}

export interface AndroidLiveViewProps extends LiveViewProps {
  deviceSerial?: string;
  showAndroidControls?: boolean;
}

export interface ControlMessage {
  type:
    | "touch"
    | "key"
    | "scroll"
    | "reset_video"
    | "ping"
    | "clipboard_set"
    | "clipboard_get"
    | number;
  action?: string;
  x?: number;
  y?: number;
  keycode?: number;
  metaState?: number;
  pressure?: number;
  pointerId?: number;
  hScroll?: number;
  vScroll?: number;
  text?: string;
  paste?: boolean;
  id?: string;
  timestamp?: number;
  data?: Uint8Array;
}

export interface SignalingMessage {
  type: "offer" | "answer" | "ice-candidate" | "error";
  sdp?: string;
  candidate?: RTCIceCandidate;
  error?: string;
}

export interface Stats {
  fps?: number;
  resolution?: string;
  latency?: number;
}
