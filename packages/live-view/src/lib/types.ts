// 连接状态类型
export type ConnectionState =
  | "connecting"
  | "connected"
  | "disconnected"
  | "error";

// 客户端选项
export interface ClientOptions {
  onConnectionStateChange?: (state: ConnectionState, message?: string) => void;
  onError?: (error: Error) => void;
  onStatsUpdate?: (stats: any) => void;
  enableAudio?: boolean;
  audioCodec?: "opus" | "aac";
}

// 连接参数
export interface ConnectionParams {
  deviceSerial: string;
  apiUrl: string;
  wsUrl?: string;
  // 新增：更清晰的参数选项
  controlPath?: string; // 自定义控制路径，如 "/api/devices/{serial}/control"
}

// 控制客户端接口
export interface ControlClient {
  // 连接管理
  connect(deviceSerial: string, apiUrl: string, wsUrl?: string): Promise<void>;
  disconnect(): void;
  isControlConnected(): boolean;

  // 控制事件
  sendKeyEvent(
    keycode: number,
    action: "down" | "up",
    metaState?: number
  ): void;
  sendTouchEvent(
    x: number,
    y: number,
    action: "down" | "up" | "move",
    pressure?: number
  ): void;
  sendControlAction(action: string, params?: any): void;
  sendClipboardSet(text: string, paste?: boolean): void;
  requestKeyframe(): void;

  // 事件处理
  handleMouseEvent(event: MouseEvent, action: "down" | "up" | "move"): void;
  handleTouchEvent(event: TouchEvent, action: "down" | "up" | "move"): void;

  // 状态
  isMouseDragging: boolean;
}

// Android 键码常量
export const ANDROID_KEYCODES = {
  POWER: 26,
  VOLUME_UP: 24,
  VOLUME_DOWN: 25,
  BACK: 4,
  HOME: 3,
  APP_SWITCH: 187,
  MENU: 82,
} as const;

// 设备信息
export interface Device {
  serial: string;
  state: string;
  model?: string;
  connected?: boolean;
  videoWidth?: number;
  videoHeight?: number;
}

// 控制消息
export interface ControlMessage {
  type:
    | "touch"
    | "key"
    | "scroll"
    | "reset_video"
    | "ping"
    | "pong"
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
  id?: string;
  paste?: boolean;
  timestamp?: number;
  data?: Uint8Array;
}

// 信令消息
export interface SignalingMessage {
  type: "offer" | "answer" | "ice-candidate" | "error";
  sdp?: string;
  candidate?: RTCIceCandidate;
  error?: string;
}

// 统计信息
export interface Stats {
  fps?: number;
  resolution?: string;
  latency?: number;
}

// 统计服务类型
export interface StatsServiceOptions {
  updateInterval?: number; // Update interval in milliseconds
  onStatsUpdate?: (stats: Stats) => void;
  enableFPS?: boolean;
  enableResolution?: boolean;
  enableLatency?: boolean;
  enableBandwidth?: boolean;
}

export interface PerformanceMetrics {
  fps?: number;
  resolution?: string;
  latency?: number;
  bandwidth?: number;
  frameDrops?: number;
  bitrate?: number;
  packetLoss?: number;
}

// 视频渲染服务类型
export interface VideoRenderServiceOptions {
  container: HTMLElement;
  onStatsUpdate?: (stats: { resolution?: string; fps?: number }) => void;
  onError?: (error: Error) => void;
  enableStats?: boolean;
  enableResizeObserver?: boolean;
  enableOrientationCheck?: boolean;
  aspectRatioMode?: "contain" | "cover" | "fill" | "scale-down";
  backgroundColor?: string;
}

export interface CanvasDimensions {
  width: number;
  height: number;
}

export interface VideoFrame {
  displayWidth: number;
  displayHeight: number;
  close(): void;
}

// 错误处理服务类型
export interface ErrorHandlingServiceOptions {
  onError?: (error: Error, context: string) => void;
  onRecoverableError?: (error: Error, context: string) => void;
  onFatalError?: (error: Error, context: string) => void;
  enableRetry?: boolean;
  maxRetries?: number;
  retryDelay?: number;
  enableErrorReporting?: boolean;
  enableErrorRecovery?: boolean;
  errorContext?: string;
}

export interface ErrorContext {
  component: string;
  operation: string;
  timestamp: number;
  metadata?: Record<string, any>;
}

export interface ErrorRecoveryStrategy {
  canRecover: (error: Error, context: ErrorContext) => boolean;
  recover: (error: Error, context: ErrorContext) => Promise<void>;
  maxRetries?: number;
  retryDelay?: number;
}

// 基础客户端类型
export interface BaseClientOptions extends ClientOptions {
  container: HTMLElement;
}

export abstract class BaseClient implements ControlClient {
  abstract connect(
    deviceSerial: string,
    apiUrl: string,
    wsUrl?: string
  ): Promise<void>;
  abstract disconnect(): Promise<void>;
  abstract isControlConnected(): boolean;
  abstract sendKeyEvent(
    keycode: number,
    action: "down" | "up",
    metaState?: number
  ): void;
  abstract sendTouchEvent(
    x: number,
    y: number,
    action: "down" | "up" | "move",
    pressure?: number
  ): void;
  abstract sendControlAction(action: string, params?: any): void;
  abstract sendClipboardSet(text: string, paste?: boolean): void;
  abstract requestKeyframe(): void;
  abstract handleMouseEvent(
    event: MouseEvent,
    action: "down" | "up" | "move"
  ): void;
  abstract handleTouchEvent(
    event: TouchEvent,
    action: "down" | "up" | "move"
  ): void;
  abstract isMouseDragging: boolean;
}

// Reconnection service types
export interface ReconnectionOptions {
  maxAttempts?: number;
  baseDelay?: number;
  maxDelay?: number;
  onReconnectAttempt?: (attempt: number, maxAttempts: number) => void;
  onReconnectSuccess?: () => void;
  onReconnectFailure?: (error: Error) => void;
  onMaxAttemptsReached?: () => void;
}

export interface ReconnectionCallbacks {
  onReconnectAttempt?: (attempt: number, maxAttempts: number) => void;
  onReconnectSuccess?: () => void;
  onReconnectFailure?: (error: Error) => void;
  onMaxAttemptsReached?: () => void;
}

// 组件属性
export interface LiveViewProps {
  apiUrl?: string;
  wsUrl?: string;
  mode?: "webrtc" | "h264";
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
