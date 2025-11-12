import { ControlClient, ANDROID_KEYCODES } from '../types';

export class ControlService {
  private client: ControlClient | null = null;

  setClient(client: ControlClient | null) {
    this.client = client;
  }

  // 键盘事件处理
  handleKeyEvent(e: React.KeyboardEvent, action: "down" | "up") {
    if (!this.client) return;
    
    const keycode = this.getKeycodeFromEvent(e);
    if (keycode) {
      this.client.sendKeyEvent(keycode, action, this.getMetaState(e));
    }
  }

  // 鼠标事件处理
  handleMouseEvent(e: React.MouseEvent, action: "down" | "up" | "move") {
    if (!this.client) return;
    this.client.handleMouseEvent(e.nativeEvent, action);
  }

  // 触摸事件处理
  handleTouchEvent(e: React.TouchEvent, action: "down" | "up" | "move") {
    if (!this.client) return;
    this.client.handleTouchEvent(e.nativeEvent, action);
  }

  // 滚轮事件处理
  handleWheelEvent(e: WheelEvent, videoElement: HTMLVideoElement | HTMLCanvasElement) {
    if (!this.client) return;
    
    e.preventDefault();
    e.stopPropagation();
    
    const rect = videoElement.getBoundingClientRect();
    const x = (e.clientX - rect.left) / rect.width;
    const y = (e.clientY - rect.top) / rect.height;
    
    let hScroll = -e.deltaX;
    let vScroll = -e.deltaY;
    
    const scaleFactor = 0.5;
    hScroll *= scaleFactor;
    vScroll *= scaleFactor;
    
    hScroll = Math.max(-16, Math.min(16, hScroll));
    vScroll = Math.max(-16, Math.min(16, vScroll));
    
    if (hScroll !== 0 || vScroll !== 0) {
      if (x >= 0 && x <= 1 && y >= 0 && y <= 1) {
        this.client.sendControlAction("scroll", {
          x, y, hScroll, vScroll, timestamp: Date.now()
        });
      }
    }
  }

  // 剪切板处理
  async handleClipboardPaste() {
    if (!this.client) return;
    try {
      const text = await navigator.clipboard.readText();
      if (text) {
        this.client.sendClipboardSet(text, true);
      }
    } catch (error) {
      console.error('[ControlService] Clipboard paste failed:', error);
    }
  }

  async handleClipboardCopy() {
    if (!this.client) return;
    // 实现剪切板复制逻辑
    try {
      this.client.sendControlAction("clipboard_get", {});
    } catch (error) {
      console.error('[ControlService] Clipboard copy failed:', error);
    }
  }

  // 控制按钮处理
  handleControlAction(action: string) {
    if (!this.client) return;
    const keycode = ANDROID_KEYCODES[action.toUpperCase() as keyof typeof ANDROID_KEYCODES];
    if (keycode) {
      this.client.sendKeyEvent(keycode, "down");
      setTimeout(() => {
        this.client?.sendKeyEvent(keycode, "up");
      }, 100);
    }
  }

  // IME 切换
  handleIMESwitch() {
    if (!this.client) return;
    this.client.sendKeyEvent(204, "down");
    setTimeout(() => {
      this.client?.sendKeyEvent(204, "up");
    }, 50);
  }

  private getKeycodeFromEvent(e: React.KeyboardEvent): number | null {
    const keyMap: { [code: string]: number } = {
      'Enter': 66,
      'Backspace': 67,
      'Delete': 112,
      'Escape': 111,
      'Tab': 61,
      'Space': 62,
      'CapsLock': 115,
      'ShiftLeft': 59,
      'ShiftRight': 60,
      'ControlLeft': 113,
      'ControlRight': 114,
      'AltLeft': 57,
      'AltRight': 58,
      'MetaLeft': 117,
      'MetaRight': 118,
      'ArrowUp': 19,
      'ArrowDown': 20,
      'ArrowLeft': 21,
      'ArrowRight': 22,
      'Home': 122,
      'End': 123,
      'PageUp': 92,
      'PageDown': 93,
      'Insert': 124,
      'KeyA': 29, 'KeyB': 30, 'KeyC': 31, 'KeyD': 32, 'KeyE': 33,
      'KeyF': 34, 'KeyG': 35, 'KeyH': 36, 'KeyI': 37, 'KeyJ': 38,
      'KeyK': 39, 'KeyL': 40, 'KeyM': 41, 'KeyN': 42, 'KeyO': 43,
      'KeyP': 44, 'KeyQ': 45, 'KeyR': 46, 'KeyS': 47, 'KeyT': 48,
      'KeyU': 49, 'KeyV': 50, 'KeyW': 51, 'KeyX': 52, 'KeyY': 53,
      'KeyZ': 54,
      'Digit0': 7, 'Digit1': 8, 'Digit2': 9, 'Digit3': 10, 'Digit4': 11,
      'Digit5': 12, 'Digit6': 13, 'Digit7': 14, 'Digit8': 15, 'Digit9': 16,
      'Period': 56,
      'Comma': 55,
      'Semicolon': 74,
      'Quote': 75,
      'BracketLeft': 71,
      'BracketRight': 72,
      'Backslash': 73,
      'Slash': 76,
      'Equal': 70,
      'Minus': 69,
      'Backquote': 68,
    };
    return keyMap[e.code] || null;
  }

  private getMetaState(e: React.KeyboardEvent): number {
    let metaState = 0;
    if (e.shiftKey) metaState |= 0x1;
    if (e.ctrlKey) metaState |= 0x1000;
    if (e.altKey) metaState |= 0x2;
    if (e.metaKey) metaState |= 0x10000;
    return metaState;
  }
}
