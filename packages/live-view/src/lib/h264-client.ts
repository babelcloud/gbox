// Types
interface H264ClientOptions {
  onConnectionStateChange?: (
    state: "connecting" | "connected" | "disconnected" | "error",
    message?: string
  ) => void;
  onError?: (error: Error) => void;
  onStatsUpdate?: (stats: any) => void;
  enableAudio?: boolean; // 新增：是否启用音频
  audioCodec?: "opus" | "aac"; // 新增：音频编解码器
}

// NAL Unit types
const NALU = {
  SPS: 7, // Sequence Parameter Set
  PPS: 8, // Picture Parameter Set
  IDR: 5, // IDR frame
} as const;

// ProfessionalMSEAudioProcessor - 基于成功的专业MSE+ReadableStream方案
class ProfessionalMSEAudioProcessor {
  private mediaSource: MediaSource | null = null;
  private sourceBuffer: SourceBuffer | null = null;
  private audioElement: HTMLAudioElement | null = null;
  private audioElementError: boolean = false;
  private isStreaming: boolean = false;
  private reader: ReadableStreamDefaultReader<Uint8Array> | null = null;
  private abortController: AbortController | null = null;
  private stats = {
    bytesReceived: 0,
    chunksProcessed: 0,
    bufferedSeconds: 0,
    startTime: 0,
  };

  constructor(private container: HTMLElement) {
    // 构造函数保持简单，实际初始化在 connect 方法中
  }

  // 基于成功测试的专业MSE方案
  async connect(audioUrl: string): Promise<void> {
    console.log("[ProfessionalMSEAudio] Connecting to:", audioUrl);

    // 检查MSE支持
    if (
      !window.MediaSource ||
      !MediaSource.isTypeSupported('audio/webm; codecs="opus"')
    ) {
      throw new Error("浏览器不支持WebM/Opus MSE");
    }

    // 重置状态
    this.isStreaming = true;
    this.stats.startTime = Date.now();
    this.stats.bytesReceived = 0;
    this.stats.chunksProcessed = 0;

    // 创建音频元素
    this.audioElement = document.createElement("audio");
    this.audioElement.controls = false; // 隐藏控件，由视频播放器控制
    this.audioElement.style.display = "none";
    this.container.appendChild(this.audioElement);

    // 添加音频元素错误处理
    this.audioElement.addEventListener("error", (e) => {
      console.error("[ProfessionalMSEAudio] Audio element error:", e);
      console.error("[ProfessionalMSEAudio] Error details:", {
        error: this.audioElement?.error,
        networkState: this.audioElement?.networkState,
        readyState: this.audioElement?.readyState,
      });

      // 标记音频元素有错误，稍后重新创建
      this.audioElementError = true;
    });

    // 创建MediaSource
    this.mediaSource = new MediaSource();
    this.audioElement.src = URL.createObjectURL(this.mediaSource);

    // 等待MediaSource打开
    await new Promise((resolve, reject) => {
      this.mediaSource!.addEventListener("sourceopen", resolve, { once: true });
      this.mediaSource!.addEventListener("error", reject, { once: true });
    });

    console.log("[ProfessionalMSEAudio] MediaSource opened");

    // 创建SourceBuffer
    this.sourceBuffer = this.mediaSource.addSourceBuffer(
      'audio/webm; codecs="opus"'
    );

    // SourceBuffer事件监听
    this.sourceBuffer.addEventListener("updateend", () => {
      // 尝试播放
      if (
        this.audioElement &&
        this.audioElement.readyState >= 3 &&
        this.audioElement.paused
      ) {
        this.audioElement
          .play()
          .then(() => {
            console.log("[ProfessionalMSEAudio] 音频开始播放");
          })
          .catch((e) => {
            console.warn("[ProfessionalMSEAudio] 播放失败:", e.message);
          });
      }
    });

    this.sourceBuffer.addEventListener("error", (e) => {
      console.error("[ProfessionalMSEAudio] SourceBuffer错误:", e);
    });

    // 启动流式获取
    await this.startStreaming(audioUrl);
  }

  private async startStreaming(audioUrl: string): Promise<void> {
    try {
      // 创建AbortController用于取消请求
      this.abortController = new AbortController();

      const response = await fetch(audioUrl, {
        signal: this.abortController.signal,
      });

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }

      if (!response.body) {
        throw new Error("ReadableStream not supported");
      }

      console.log("[ProfessionalMSEAudio] 连接成功，开始接收流数据");

      // 获取ReadableStream reader
      this.reader = response.body.getReader();

      // 流数据处理循环
      while (this.isStreaming) {
        const { done, value } = await this.reader.read();

        if (done) {
          console.log("[ProfessionalMSEAudio] 服务器结束流传输");
          break;
        }

        // 更新统计信息
        this.stats.bytesReceived += value.length;
        this.stats.chunksProcessed++;

        // 检查音频元素状态
        if (
          this.audioElementError ||
          (this.audioElement && this.audioElement.error)
        ) {
          console.warn(
            "[ProfessionalMSEAudio] Audio element has error, skipping chunk"
          );
          continue;
        }

        // 将数据追加到SourceBuffer
        if (
          this.sourceBuffer &&
          !this.sourceBuffer.updating &&
          this.mediaSource &&
          this.mediaSource.readyState === "open" &&
          this.audioElement &&
          !this.audioElementError &&
          !this.audioElement.error
        ) {
          try {
            this.sourceBuffer.appendBuffer(
              value.buffer.slice(
                value.byteOffset,
                value.byteOffset + value.byteLength
              ) as ArrayBuffer
            );

            // 更新缓冲区统计
            if (this.sourceBuffer.buffered.length > 0) {
              this.stats.bufferedSeconds = this.sourceBuffer.buffered.end(0);
            }

            // 每100个块记录一次进度
            if (this.stats.chunksProcessed % 100 === 0) {
              const elapsed = Date.now() - this.stats.startTime;
              const throughput = (
                this.stats.bytesReceived /
                1024 /
                (elapsed / 1000)
              ).toFixed(1);
              console.log(
                `[ProfessionalMSEAudio] 已处理${
                  this.stats.chunksProcessed
                }块, ${Math.round(
                  this.stats.bytesReceived / 1024
                )}KB, ${throughput}KB/s`
              );
            }
          } catch (e) {
            console.error("[ProfessionalMSEAudio] SourceBuffer追加失败:", e);

            // 检查是否是音频元素错误导致的
            if (
              this.audioElementError ||
              (this.audioElement && this.audioElement.error)
            ) {
              console.warn(
                "[ProfessionalMSEAudio] Audio element error detected"
              );
            }

            // 实现错误恢复机制
            await this.retryWithBackoff();
          }
        } else {
          // 如果SourceBuffer正在更新或状态异常，等待一下
          await new Promise((resolve) => setTimeout(resolve, 10));
        }
      }
    } catch (error) {
      if (error instanceof Error && error.name !== "AbortError") {
        console.error("[ProfessionalMSEAudio] 流处理错误:", error);

        // 自动重连机制
        if (this.isStreaming) {
          console.log("[ProfessionalMSEAudio] 5秒后自动重连...");
          setTimeout(() => {
            if (this.isStreaming) {
              this.startStreaming(audioUrl);
            }
          }, 5000);
        }
      }
    }
  }

  // 错误恢复机制
  private async retryWithBackoff(): Promise<void> {
    const delays = [100, 200, 500, 1000]; // 递增退避

    for (const delay of delays) {
      await new Promise((resolve) => setTimeout(resolve, delay));

      // 检查音频元素状态
      if (
        this.audioElementError ||
        (this.audioElement && this.audioElement.error)
      ) {
        console.warn(
          "[ProfessionalMSEAudio] Audio element error during recovery"
        );
      }

      if (
        this.sourceBuffer &&
        !this.sourceBuffer.updating &&
        this.mediaSource &&
        this.mediaSource.readyState === "open" &&
        this.audioElement &&
        !this.audioElementError &&
        !this.audioElement.error
      ) {
        return; // 恢复成功
      }
    }

    throw new Error("SourceBuffer recovery failed");
  }

  // 停止音频流
  disconnect(): void {
    this.isStreaming = false;

    // 取消网络请求
    if (this.abortController) {
      this.abortController.abort();
      this.abortController = null;
    }

    // 关闭reader
    if (this.reader) {
      this.reader.cancel().catch((e) => {
        // 静默处理预期的取消错误，避免控制台污染
        if (e.name !== "AbortError") {
          console.log(
            "[ProfessionalMSEAudio] Reader cancel error (unexpected):",
            e
          );
        }
      });
      this.reader = null;
    }

    // 停止音频
    if (this.audioElement) {
      this.audioElement.pause();
      this.audioElement.remove();
      this.audioElement = null;
    }

    // 关闭MediaSource
    if (this.mediaSource && this.mediaSource.readyState === "open") {
      try {
        this.mediaSource.endOfStream();
      } catch (e) {
        // 静默处理预期的MediaSource关闭错误
        // 这些错误在快速模式切换时是正常的
      }
    }

    // 显示最终统计
    if (this.stats.startTime > 0) {
      const elapsed = Date.now() - this.stats.startTime;
      const avgThroughput = (
        this.stats.bytesReceived /
        1024 /
        (elapsed / 1000)
      ).toFixed(1);
      console.log(
        `[ProfessionalMSEAudio] 音频流已停止 - 总计: ${Math.round(
          this.stats.bytesReceived / 1024
        )}KB, ${avgThroughput}KB/s平均速率`
      );
    }

    // 重置状态
    this.mediaSource = null;
    this.sourceBuffer = null;
  }

  // 手动播放音频（用于用户交互后）
  play(): void {
    if (this.audioElement && this.audioElement.paused) {
      this.audioElement.play().catch((e) => {
        console.warn("[ProfessionalMSEAudio] Manual play failed:", e);
      });
    }
  }

  // 暂停音频
  pause(): void {
    if (this.audioElement && !this.audioElement.paused) {
      this.audioElement.pause();
    }
  }
}

export class H264Client {
  // Android key codes
  static readonly ANDROID_KEYCODES = {
    POWER: 26,
    VOLUME_UP: 24,
    VOLUME_DOWN: 25,
    BACK: 4,
    HOME: 3,
    APP_SWITCH: 187,
    MENU: 82,
  };

  private container: HTMLElement;
  private canvas: HTMLCanvasElement | null = null;
  private context: CanvasRenderingContext2D | null = null;
  private decoder: VideoDecoder | null = null;
  private abortController: AbortController | null = null;
  private audioProcessor: ProfessionalMSEAudioProcessor | null = null; // 新的专业音频处理器
  private controlWs: WebSocket | null = null; // 控制WebSocket连接
  private opts: H264ClientOptions;
  private buffer: Uint8Array = new Uint8Array(0);
  private spsData: Uint8Array | null = null;
  private ppsData: Uint8Array | null = null;
  private animationFrameId: number | undefined;
  private decodedFrames: Array<{ frame: VideoFrame; timestamp: number }> = [];
  private waitingForKeyframe: boolean = true; // 等待关键帧标志
  private keyframeRequestTimer: number | null = null; // 关键帧请求定时器
  private controlRetryCount: number = 0; // 控制WebSocket重试计数器
  private controlReconnectTimer: number | null = null; // 控制WebSocket重连定时器
  private maxControlRetries: number = 5; // 最大重试次数
  public isMouseDragging: boolean = false; // 鼠标拖拽状态
  private lastConnectParams: {
    deviceSerial: string;
    apiUrl: string;
    wsUrl?: string;
  } | null = null; // 保存连接参数用于重连

  constructor(container: HTMLElement, opts: H264ClientOptions = {}) {
    this.container = container;
    this.opts = {
      enableAudio: true, // 默认启用音频
      audioCodec: "opus", // 默认使用 OPUS
      ...opts,
    };
    this.initializeWebCodecs();
  }

  // Initialize WebCodecs decoder
  private initializeWebCodecs(): void {
    console.log("[H264Client] Initializing WebCodecs decoder...");

    // Check if WebCodecs is supported
    if (typeof VideoDecoder !== "function") {
      console.error("[H264Client] WebCodecs not supported");
      this.opts.onError?.(new Error("WebCodecs not supported"));
      return;
    }

    try {
      // Create canvas for rendering
      this.canvas = document.createElement("canvas");
      this.canvas.style.width = "100%";
      this.canvas.style.height = "100%";
      this.canvas.style.display = "block";
      this.container.appendChild(this.canvas);

      // Get 2D context
      const context = this.canvas.getContext("2d");
      if (!context) {
        throw new Error("Failed to get 2d context from canvas");
      }
      this.context = context;

      // Create VideoDecoder
      this.decoder = new VideoDecoder({
        output: (frame) => this.onFrameDecoded(frame),
        error: (error: DOMException) => {
          console.error("[H264Client] VideoDecoder error:", error);
          this.opts.onError?.(
            new Error(`VideoDecoder error: ${error.message}`)
          );
        },
      });

      console.log("[H264Client] WebCodecs decoder initialized successfully");
    } catch (error) {
      console.error("[H264Client] WebCodecs initialization failed:", error);
      this.opts.onError?.(new Error("WebCodecs initialization failed"));
    }
  }

  // Connect to H.264 AVC format stream
  public async connect(
    deviceSerial: string,
    apiUrl: string = "/api",
    wsUrl?: string
  ): Promise<void> {
    const url = `${apiUrl}/stream/video/${deviceSerial}?mode=h264&format=avc`;
    console.log("[H264Client] Connecting to H.264 AVC stream:", url);

    // Reinitialize WebCodecs if decoder is not ready (e.g., after disconnect)
    if (!this.decoder) {
      console.log(
        "[H264Client] Decoder not ready, reinitializing WebCodecs..."
      );
      this.initializeWebCodecs();
    }

    if (!this.decoder) {
      throw new Error("WebCodecs decoder not ready");
    }

    // Notify connecting state
    this.opts.onConnectionStateChange?.(
      "connecting",
      "Connecting to H.264 stream..."
    );

    try {
      // 保存连接参数用于重连
      this.lastConnectParams = { deviceSerial, apiUrl, wsUrl };

      await this.startHTTP(url);

      // 先连接控制WebSocket（优先级更高）
      console.log("[H264Client] About to connect control WebSocket...");
      try {
        await this.connectControl(deviceSerial, apiUrl, wsUrl);
        console.log("[H264Client] Control connection completed successfully");
      } catch (error) {
        console.warn(
          "[H264Client] Control connection failed, but continuing with video:",
          error
        );
      }

      // 连接音频（如果启用）
      if (this.opts.enableAudio) {
        console.log("[H264Client] About to connect audio...");
        try {
          await this.connectAudio(deviceSerial, apiUrl);
          console.log("[H264Client] Audio connection completed");
        } catch (error) {
          console.warn(
            "[H264Client] Audio connection failed, but continuing:",
            error
          );
        }
      } else {
        console.log("[H264Client] Audio disabled, skipping audio connection");
      }

      // 启动关键帧请求
      this.requestKeyframe();

      // Notify connected state
      this.opts.onConnectionStateChange?.(
        "connected",
        "H.264 stream connected"
      );
    } catch (error) {
      console.error("[H264Client] Connection failed:", error);
      this.opts.onConnectionStateChange?.("error", "H.264 connection failed");
      this.opts.onError?.(error as Error);
      throw error;
    }
  }

  // 连接控制WebSocket
  private async connectControl(
    deviceSerial: string,
    apiUrl: string,
    wsUrl?: string
  ): Promise<void> {
    console.log("[H264Client] Starting control WebSocket connection...");
    console.log(
      "[H264Client] Device:",
      deviceSerial,
      "API URL:",
      apiUrl,
      "WS URL:",
      wsUrl
    );

    try {
      // 构建控制WebSocket URL - 使用与WebRTCClient相同的逻辑
      let controlWsUrl;
      if (wsUrl) {
        // 使用提供的wsUrl构建控制WebSocket URL
        const baseUrl = wsUrl.replace(/\/ws$/, ""); // Remove /ws suffix if present
        controlWsUrl = `${baseUrl}/api/stream/control/${deviceSerial}`.replace(
          /^http/,
          "ws"
        );
      } else if (apiUrl.startsWith("http")) {
        // 如果apiUrl是完整URL
        controlWsUrl = `${apiUrl}/stream/control/${deviceSerial}`.replace(
          /^http/,
          "ws"
        );
      } else {
        // 如果apiUrl是相对路径，构建完整URL
        const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
        const host = window.location.hostname;
        let port = window.location.port;
        if (port === "3000" || port === "") {
          port = "8080"; // 默认后端端口
        }
        controlWsUrl = `${protocol}//${host}:${port}${apiUrl}/stream/control/${deviceSerial}`;
      }

      console.log(`[H264Client] Control WebSocket URL: ${controlWsUrl}`);

      // 创建WebSocket连接
      console.log("[H264Client] Creating WebSocket connection...");
      try {
        this.controlWs = new WebSocket(controlWsUrl);
        console.log("[H264Client] WebSocket object created successfully");
      } catch (wsError) {
        console.error("[H264Client] Failed to create WebSocket:", wsError);
        throw wsError;
      }

      // 设置WebSocket事件处理器
      this.controlWs.onopen = () => {
        console.log("[H264Client] Control WebSocket connected successfully");
        console.log("[H264Client] WebSocket URL:", controlWsUrl);
        console.log(
          "[H264Client] WebSocket ready state:",
          this.controlWs?.readyState
        );
        // 连接成功后，重置重试计数器
        this.controlRetryCount = 0;
      };

      this.controlWs.onmessage = (event) => {
        console.log("[H264Client] Control WebSocket message:", event.data);
      };

      this.controlWs.onerror = (error) => {
        console.error("[H264Client] Control WebSocket error:", error);
      };

      this.controlWs.onclose = (event) => {
        console.log(
          "[H264Client] Control WebSocket closed:",
          event.code,
          event.reason
        );
        if (event.code !== 1000) {
          console.warn(
            "[H264Client] Control WebSocket closed unexpectedly:",
            event.code,
            event.reason
          );
          // 尝试重连控制WebSocket
          this.scheduleControlReconnect(deviceSerial, apiUrl, wsUrl);
        }
        this.controlWs = null;
      };

      // 等待连接建立
      await new Promise<void>((resolve, reject) => {
        const timeout = setTimeout(() => {
          console.log("[H264Client] Control WebSocket connection timeout");
          reject(new Error("Control WebSocket connection timeout"));
        }, 10000); // 增加超时时间到10秒

        const originalOnOpen = this.controlWs!.onopen;
        const originalOnError = this.controlWs!.onerror;

        this.controlWs!.onopen = () => {
          clearTimeout(timeout);
          console.log("[H264Client] Control WebSocket connected successfully");
          // 恢复原始处理器
          this.controlWs!.onopen = originalOnOpen;
          this.controlWs!.onerror = originalOnError;
          resolve();
        };

        this.controlWs!.onerror = (error) => {
          clearTimeout(timeout);
          console.error(
            "[H264Client] Control WebSocket connection error:",
            error
          );
          // 恢复原始处理器
          this.controlWs!.onopen = originalOnOpen;
          this.controlWs!.onerror = originalOnError;
          reject(new Error("Control WebSocket connection failed"));
        };
      });
    } catch (error) {
      console.error("[H264Client] Control WebSocket connection failed:", error);
      // 清理失败的WebSocket连接
      if (this.controlWs) {
        this.controlWs.close();
        this.controlWs = null;
      }
      // 尝试重连控制WebSocket
      this.scheduleControlReconnect(deviceSerial, apiUrl, wsUrl);
      // 抛出错误让上层知道连接失败
      throw error;
    }
  }

  // 安排控制WebSocket重连
  private scheduleControlReconnect(
    deviceSerial?: string,
    apiUrl?: string,
    wsUrl?: string
  ): void {
    if (this.controlRetryCount >= this.maxControlRetries) {
      console.log(
        "[H264Client] Control WebSocket max retries reached, giving up"
      );
      return;
    }

    // 使用保存的连接参数或传入的参数
    const params = this.lastConnectParams || {
      deviceSerial: deviceSerial!,
      apiUrl: apiUrl!,
      wsUrl,
    };
    if (!params.deviceSerial || !params.apiUrl) {
      console.error(
        "[H264Client] Cannot reconnect control WebSocket - missing connection parameters"
      );
      return;
    }

    this.controlRetryCount++;
    const delay = Math.min(
      1000 * Math.pow(2, this.controlRetryCount - 1),
      10000
    ); // 指数退避，最大10秒

    console.log(
      `[H264Client] Scheduling control WebSocket reconnect in ${delay}ms (attempt ${this.controlRetryCount}/${this.maxControlRetries})`
    );

    this.controlReconnectTimer = window.setTimeout(() => {
      console.log(
        `[H264Client] Attempting control WebSocket reconnect (attempt ${this.controlRetryCount})`
      );
      this.connectControl(
        params.deviceSerial,
        params.apiUrl,
        params.wsUrl
      ).catch((error) => {
        console.error(
          "[H264Client] Control WebSocket reconnect failed:",
          error
        );
      });
    }, delay);
  }

  // 连接专业MSE音频流
  private async connectAudio(
    deviceSerial: string,
    apiUrl: string
  ): Promise<void> {
    console.log("[H264Client] Connecting professional MSE audio...");

    try {
      // 创建专业MSE音频处理器
      this.audioProcessor = new ProfessionalMSEAudioProcessor(this.container);

      // 使用MSE优化的WebM端点 (基于Pion WebRTC专业实现)
      const audioUrl = `${apiUrl}/stream/audio/${deviceSerial}?codec=opus&format=webm&mse=true`;

      // 连接音频流
      console.log("[H264Client] Calling audioProcessor.connect...");

      // 添加超时机制防止音频连接卡住
      const audioTimeout = new Promise<void>((_, reject) => {
        setTimeout(() => reject(new Error("Audio connection timeout")), 10000);
      });

      await Promise.race([this.audioProcessor.connect(audioUrl), audioTimeout]);

      console.log("[H264Client] audioProcessor.connect returned successfully");

      console.log("[H264Client] Professional MSE audio connected successfully");
    } catch (error) {
      console.error(
        "[H264Client] Professional MSE audio connection failed:",
        error
      );
      // 不抛出错误，让视频继续工作
    }
  }

  // 启用音频播放（用于用户交互后）
  public async enableAudio(): Promise<void> {
    if (this.audioProcessor) {
      this.audioProcessor.play();
      console.log("[H264Client] Professional MSE audio enabled");
    }
  }

  // 手动播放音频（用于用户交互后）
  public playAudio(): void {
    this.enableAudio();
  }

  // 暂停音频
  public pauseAudio(): void {
    if (this.audioProcessor) {
      this.audioProcessor.pause();
    }
  }

  private async startHTTP(url: string): Promise<void> {
    this.abortController = new AbortController();
    const response = await fetch(url, {
      signal: this.abortController.signal,
    });

    if (!response.ok) {
      throw new Error(`HTTP error: ${response.status}`);
    }

    const reader = response.body?.getReader();
    if (!reader) {
      throw new Error("No response body reader available");
    }

    // Process stream data in async function
    (async () => {
      try {
        for (;;) {
          const { done, value } = await reader.read();
          if (done) break;

          if (value && value.length) {
            // Append new data to buffer
            const newBuffer = new Uint8Array(this.buffer.length + value.length);
            newBuffer.set(this.buffer);
            newBuffer.set(value, this.buffer.length);
            this.buffer = newBuffer;

            // Process NAL units from AVC format stream
            const { processedNals, remainingBuffer } = this.parseAVC(
              this.buffer
            );
            this.buffer = remainingBuffer;

            // Process each NAL unit
            for (const nalData of processedNals) {
              this.processNALUnit(nalData);
            }
          }
        }
      } catch (error) {
        // Only log error if it's not an abort error (which is expected when disconnecting)
        if (error instanceof Error && error.name !== "AbortError") {
          console.error("[H264Client] Stream processing error:", error);
        }
      }
    })();
  }

  // Parse AVC format NAL units (length-prefixed)
  private parseAVC(data: Uint8Array): {
    processedNals: Uint8Array[];
    remainingBuffer: Uint8Array;
  } {
    const processedNals: Uint8Array[] = [];
    let offset = 0;

    while (offset < data.length) {
      // Need at least 4 bytes for length prefix
      if (offset + 4 > data.length) {
        break;
      }

      // Read length prefix (big-endian)
      const length =
        (data[offset] << 24) |
        (data[offset + 1] << 16) |
        (data[offset + 2] << 8) |
        data[offset + 3];

      offset += 4;

      // Check if we have enough data for the NAL unit
      if (offset + length > data.length) {
        // Not enough data, put back the length prefix
        offset -= 4;
        break;
      }

      // Extract NAL unit
      const nalData = data.slice(offset, offset + length);
      processedNals.push(nalData);
      offset += length;
    }

    // Return remaining buffer
    const remainingBuffer = data.slice(offset);
    return { processedNals, remainingBuffer };
  }

  private processNALUnit(nalData: Uint8Array): void {
    if (nalData.length === 0) return;

    const nalType = nalData[0] & 0x1f;

    // Handle SPS
    if (nalType === NALU.SPS) {
      this.spsData = nalData;
      this.tryConfigureDecoder();
      return;
    }

    // Handle PPS
    if (nalType === NALU.PPS) {
      this.ppsData = nalData;
      this.tryConfigureDecoder();
      return;
    }

    // Only decode if we have SPS and PPS
    if (!this.spsData || !this.ppsData) {
      return;
    }

    // Decode frame
    this.decodeFrame(nalData);
  }

  private tryConfigureDecoder(): void {
    if (!this.spsData || !this.ppsData || !this.decoder) {
      return;
    }

    try {
      const description = this.createAVCDescription(this.spsData, this.ppsData);

      const config: VideoDecoderConfig = {
        codec: "avc1.42E01E", // H.264 Baseline Profile
        optimizeForLatency: true,
        description,
        hardwareAcceleration: "prefer-hardware" as HardwareAcceleration,
      };

      this.decoder.configure(config);

      // 配置后需要等待关键帧
      this.waitingForKeyframe = true;
    } catch (error) {
      console.error("[H264Client] Decoder configuration failed:", error);
    }
  }

  private createAVCDescription(sps: Uint8Array, pps: Uint8Array): ArrayBuffer {
    // Create AVC Configuration Record
    const configLength = 11 + sps.length + pps.length;
    const config = new Uint8Array(configLength);
    let offset = 0;

    // AVC Configuration Record header
    config[offset++] = 0x01; // configurationVersion
    config[offset++] = sps[1]; // AVCProfileIndication
    config[offset++] = sps[2]; // profile_compatibility
    config[offset++] = sps[3]; // AVCLevelIndication
    config[offset++] = 0xff; // lengthSizeMinusOne (3) + reserved bits

    // SPS
    config[offset++] = 0xe1; // numOfSequenceParameterSets (1) + reserved bits
    config[offset++] = (sps.length >> 8) & 0xff; // sequenceParameterSetLength (high)
    config[offset++] = sps.length & 0xff; // sequenceParameterSetLength (low)
    config.set(sps, offset);
    offset += sps.length;

    // PPS
    config[offset++] = 0x01; // numOfPictureParameterSets
    config[offset++] = (pps.length >> 8) & 0xff; // pictureParameterSetLength (high)
    config[offset++] = pps.length & 0xff; // pictureParameterSetLength (low)
    config.set(pps, offset);

    return config.buffer;
  }

  private decodeFrame(nalData: Uint8Array): void {
    if (!this.decoder || this.decoder.state !== "configured") {
      return;
    }

    const nalType = nalData[0] & 0x1f;
    const isIDR = nalType === NALU.IDR;

    // 如果正在等待关键帧，只处理IDR帧
    if (this.waitingForKeyframe) {
      if (!isIDR) {
        // 跳过非关键帧，继续等待
        return;
      } else {
        // 收到关键帧，停止等待
        this.waitingForKeyframe = false;
        console.log("[H264Client] Received keyframe, starting video decode");
        // 清除关键帧请求定时器
        if (this.keyframeRequestTimer) {
          clearInterval(this.keyframeRequestTimer);
          this.keyframeRequestTimer = null;
        }
      }
    }

    try {
      // Convert NAL unit to AVC format (add length prefix)
      const avcData = this.convertNALToAVC(nalData);

      // Use performance.now() for better timing accuracy
      const timestamp = performance.now() * 1000; // Convert to microseconds

      // Create EncodedVideoChunk
      const chunk = new EncodedVideoChunk({
        type: isIDR ? "key" : "delta",
        timestamp: timestamp,
        data: avcData,
      });

      // Decode the chunk
      this.decoder.decode(chunk);
    } catch (error) {
      console.error("[H264Client] Failed to decode frame:", error);

      // If decode fails due to keyframe requirement, request keyframe
      if (
        error instanceof Error &&
        error.message.includes("key frame is required")
      ) {
        console.log(
          "[H264Client] Decoder requires keyframe, requesting from server"
        );
        this.waitingForKeyframe = true;
        this.requestKeyframe();
      }

      // If decode fails, try to recreate decoder
      if (this.decoder && this.decoder.state !== "configured") {
        this.recreateDecoder();
      }
    }
  }

  private convertNALToAVC(nalUnit: Uint8Array): ArrayBuffer {
    const lengthPrefix = new Uint8Array(4);
    const view = new DataView(lengthPrefix.buffer);
    view.setUint32(0, nalUnit.length, false); // Big-endian

    const avcData = new Uint8Array(4 + nalUnit.length);
    avcData.set(lengthPrefix, 0);
    avcData.set(nalUnit, 4);

    return avcData.buffer;
  }

  private recreateDecoder(): void {
    // Close existing decoder if it exists
    if (this.decoder && this.decoder.state === "configured") {
      this.decoder.close();
    }

    // Reset keyframe waiting state
    this.waitingForKeyframe = true;

    // Create new decoder
    this.decoder = new VideoDecoder({
      output: (frame) => this.onFrameDecoded(frame),
      error: (error: DOMException) => {
        console.error("[H264Client] VideoDecoder error:", error);
        this.opts.onError?.(new Error(`VideoDecoder error: ${error.message}`));
      },
    });

    // Reconfigure with existing SPS/PPS data
    if (this.spsData && this.ppsData) {
      this.tryConfigureDecoder();
    }
  }

  // 请求关键帧
  public requestKeyframe(): void {
    console.log("[H264Client] Requesting keyframe from server");

    // 清除现有的定时器
    if (this.keyframeRequestTimer) {
      clearInterval(this.keyframeRequestTimer);
    }

    // 立即请求一次关键帧
    this.sendKeyframeRequest();

    // 设置定时器，每2秒请求一次，直到收到关键帧
    this.keyframeRequestTimer = window.setInterval(() => {
      if (this.waitingForKeyframe) {
        console.log(
          "[H264Client] Still waiting for keyframe, requesting again"
        );
        this.sendKeyframeRequest();
      } else {
        // 收到关键帧后清除定时器
        if (this.keyframeRequestTimer) {
          clearInterval(this.keyframeRequestTimer);
          this.keyframeRequestTimer = null;
        }
      }
    }, 2000);
  }

  // 发送关键帧请求到服务器（这里需要根据你的协议实现）
  private sendKeyframeRequest(): void {
    // 这里应该实现向服务器发送关键帧请求的逻辑
    // 例如通过WebSocket或HTTP请求通知服务器生成关键帧
    console.log(
      "[H264Client] Keyframe request sent (placeholder - implement based on your protocol)"
    );
  }

  // 发送按键事件
  public sendKeyEvent(
    keycode: number,
    action: "down" | "up",
    metaState: number = 0
  ): void {
    console.log("[H264Client] Sending key event:", {
      keycode,
      action,
      metaState,
    });

    if (!this.controlWs || this.controlWs.readyState !== WebSocket.OPEN) {
      console.warn(
        "[H264Client] Control WebSocket not connected, cannot send key event"
      );
      // 尝试重连控制WebSocket（如果还有重试次数）
      if (
        this.controlRetryCount < this.maxControlRetries &&
        this.lastConnectParams
      ) {
        console.log(
          "[H264Client] Attempting to reconnect control WebSocket for key event"
        );
        this.scheduleControlReconnect();
      } else {
        console.log(
          "[H264Client] Cannot reconnect control WebSocket - no retries left or missing connection params"
        );
      }
      return;
    }

    const message = {
      type: "key",
      action,
      keycode,
      metaState,
    };

    try {
      this.controlWs.send(JSON.stringify(message));
      console.log("[H264Client] Key event sent successfully");
    } catch (error) {
      console.error("[H264Client] Failed to send key event:", error);
      // 发送失败时，标记连接可能有问题
      if (this.controlWs) {
        this.controlWs.close();
        this.controlWs = null;
      }
      // 尝试重连
      if (this.lastConnectParams) {
        this.scheduleControlReconnect();
      }
    }
  }

  // 发送触摸事件
  public sendTouchEvent(
    x: number,
    y: number,
    action: "down" | "up" | "move",
    pressure: number = 1.0
  ): void {
    console.log("[H264Client] Sending touch event:", {
      x,
      y,
      action,
      pressure,
    });

    if (!this.controlWs || this.controlWs.readyState !== WebSocket.OPEN) {
      console.warn(
        "[H264Client] Control WebSocket not connected, cannot send touch event"
      );
      // 尝试重连控制WebSocket（如果还有重试次数）
      if (
        this.controlRetryCount < this.maxControlRetries &&
        this.lastConnectParams
      ) {
        console.log(
          "[H264Client] Attempting to reconnect control WebSocket for touch event"
        );
        this.scheduleControlReconnect();
      }
      return;
    }

    const message = {
      type: "touch",
      action,
      x,
      y,
      pressure: action === "down" || action === "move" ? pressure : 0,
      pointerId: 0,
    };

    try {
      this.controlWs.send(JSON.stringify(message));
    } catch (error) {
      console.error("[H264Client] Failed to send touch event:", error);
      // 发送失败时，标记连接可能有问题
      if (this.controlWs) {
        this.controlWs.close();
        this.controlWs = null;
      }
      // 尝试重连
      if (this.lastConnectParams) {
        this.scheduleControlReconnect();
      }
    }
  }

  // 处理鼠标事件 - 与WebRTC客户端保持一致的接口
  public handleMouseEvent(
    event: MouseEvent,
    action: "down" | "up" | "move"
  ): void {
    // 使用 canvas 或 container 元素
    const targetElement = this.canvas || this.container;
    if (!targetElement) {
      console.warn("[H264Client] No target element available for mouse event");
      return;
    }

    // 检查控制连接状态
    if (!this.isControlConnected()) {
      console.warn(
        "[H264Client] Control WebSocket not connected, cannot send mouse event"
      );
      return;
    }

    // 只处理左键鼠标事件（模拟触摸）
    if ((action === "down" || action === "up") && event.button !== 0) {
      console.log(
        `[H264Client] Ignoring non-left mouse button: ${event.button}`
      );
      return;
    }

    // 更新拖拽状态
    if (action === "down") {
      this.isMouseDragging = true;
      event.preventDefault(); // 防止拖拽时选择文本
    } else if (action === "up") {
      this.isMouseDragging = false;
    } else if (action === "move" && !this.isMouseDragging) {
      // 只在拖拽时发送移动事件（模拟触摸拖拽）
      return;
    }

    const rect = targetElement.getBoundingClientRect();
    const x = (event.clientX - rect.left) / rect.width;
    const y = (event.clientY - rect.top) / rect.height;

    // 确保坐标在有效范围内
    const clampedX = Math.max(0, Math.min(1, x));
    const clampedY = Math.max(0, Math.min(1, y));

    console.log(
      `[H264Client] Mouse ${action} at (${clampedX.toFixed(
        3
      )}, ${clampedY.toFixed(3)})`
    );

    // 使用现有的sendTouchEvent方法
    this.sendTouchEvent(
      clampedX,
      clampedY,
      action,
      action === "down" || (action === "move" && this.isMouseDragging)
        ? 1.0
        : 0.0
    );
  }

  // 处理触摸事件 - 与WebRTC客户端保持一致的接口
  public handleTouchEvent(
    event: TouchEvent,
    action: "down" | "up" | "move"
  ): void {
    // 使用 canvas 或 container 元素
    const targetElement = this.canvas || this.container;
    if (!targetElement) {
      console.warn("[H264Client] No target element available for touch event");
      return;
    }

    const rect = targetElement.getBoundingClientRect();
    const touch = event.touches[0] || event.changedTouches[0];

    if (!touch) {
      console.warn("[H264Client] No touch point available");
      return;
    }

    const x = (touch.clientX - rect.left) / rect.width;
    const y = (touch.clientY - rect.top) / rect.height;

    // 确保坐标在有效范围内
    const clampedX = Math.max(0, Math.min(1, x));
    const clampedY = Math.max(0, Math.min(1, y));

    console.log(
      `[H264Client] Touch ${action} at (${clampedX.toFixed(
        3
      )}, ${clampedY.toFixed(3)})`
    );

    // 使用现有的sendTouchEvent方法
    this.sendTouchEvent(
      clampedX,
      clampedY,
      action,
      action === "down" || action === "move" ? 1.0 : 0.0
    );
  }

  // 检查控制WebSocket连接状态
  public isControlConnected(): boolean {
    const isConnected = !!(
      this.controlWs && this.controlWs.readyState === WebSocket.OPEN
    );
    console.log("[H264Client] Control WebSocket status:", {
      ws: !!this.controlWs,
      readyState: this.controlWs?.readyState,
      isConnected,
      retryCount: this.controlRetryCount,
      maxRetries: this.maxControlRetries,
    });
    return isConnected;
  }

  // 发送控制动作
  public sendControlAction(action: string, params?: any): void {
    console.log("[H264Client] Sending control action:", { action, params });

    if (!this.controlWs || this.controlWs.readyState !== WebSocket.OPEN) {
      console.warn(
        "[H264Client] Control WebSocket not connected, cannot send control action"
      );
      // 尝试重连控制WebSocket（如果还有重试次数）
      if (
        this.controlRetryCount < this.maxControlRetries &&
        this.lastConnectParams
      ) {
        console.log(
          "[H264Client] Attempting to reconnect control WebSocket for control action"
        );
        this.scheduleControlReconnect();
      }
      return;
    }

    const message = {
      type: "control",
      action,
      params,
    };

    try {
      this.controlWs.send(JSON.stringify(message));
      console.log("[H264Client] Control action sent successfully");
    } catch (error) {
      console.error("[H264Client] Failed to send control action:", error);
      // 发送失败时，标记连接可能有问题
      if (this.controlWs) {
        this.controlWs.close();
        this.controlWs = null;
      }
      // 尝试重连
      if (this.lastConnectParams) {
        this.scheduleControlReconnect();
      }
    }
  }

  private onFrameDecoded(frame: VideoFrame): void {
    if (!this.context || !this.canvas) return;

    try {
      // Update canvas size to match frame
      if (
        this.canvas.width !== frame.displayWidth ||
        this.canvas.height !== frame.displayHeight
      ) {
        this.canvas.width = frame.displayWidth;
        this.canvas.height = frame.displayHeight;
      }

      // Draw frame to canvas
      this.context.drawImage(frame, 0, 0);
    } catch (error) {
      console.error("[H264Client] Failed to render frame:", error);
    } finally {
      frame.close();
    }
  }

  public disconnect(): void {
    console.log("[H264Client] Disconnecting and cleaning up resources...");

    // Notify disconnecting state
    this.opts.onConnectionStateChange?.(
      "disconnected",
      "H.264 stream disconnected"
    );

    // Cancel HTTP request first
    if (this.abortController) {
      this.abortController.abort();
      this.abortController = null;
    }

    // 清理专业MSE音频处理器
    if (this.audioProcessor) {
      this.audioProcessor.disconnect();
      this.audioProcessor = null;
    }

    // 清理控制WebSocket连接
    if (this.controlWs) {
      this.controlWs.close();
      this.controlWs = null;
    }

    // 清理控制WebSocket重连定时器
    if (this.controlReconnectTimer) {
      clearTimeout(this.controlReconnectTimer);
      this.controlReconnectTimer = null;
    }

    // 重置重试计数器和连接参数
    this.controlRetryCount = 0;
    this.lastConnectParams = null;

    // 清理关键帧请求定时器
    if (this.keyframeRequestTimer) {
      clearInterval(this.keyframeRequestTimer);
      this.keyframeRequestTimer = null;
    }

    // Close decoder
    if (this.decoder) {
      this.decoder.close();
      this.decoder = null;
    }

    // Clear animation frame
    if (this.animationFrameId) {
      cancelAnimationFrame(this.animationFrameId);
      this.animationFrameId = undefined;
    }

    // Close all pending frames
    for (const { frame } of this.decodedFrames) {
      frame.close();
    }
    this.decodedFrames = [];

    // Clear canvas
    if (this.canvas && this.canvas.parentNode) {
      this.canvas.parentNode.removeChild(this.canvas);
      this.canvas = null;
    }

    this.context = null;
    this.buffer = new Uint8Array(0);
    this.spsData = null;
    this.ppsData = null;

    console.log("[H264Client] Disconnect completed");
  }

  public cleanup(): void {
    this.disconnect();
  }
}
