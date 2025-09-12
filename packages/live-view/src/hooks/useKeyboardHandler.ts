import { useCallback } from 'react';
import { WebRTCClient } from '../lib/webrtc-client';

interface UseKeyboardHandlerProps {
  clientRef: React.RefObject<WebRTCClient | null>;
  isConnected: boolean;
  keyboardCaptureEnabled: boolean;
  onSmartPaste: () => void;
  onSmartCopy: () => void;
}

export const useKeyboardHandler = ({
  clientRef,
  isConnected,
  keyboardCaptureEnabled,
  onSmartPaste,
  onSmartCopy,
}: UseKeyboardHandlerProps) => {
  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    console.log('[Keyboard] Key down:', e.key, 'code:', e.code, 'keyCode:', e.keyCode, 'isConnected:', isConnected, 'captureEnabled:', keyboardCaptureEnabled);
    
    // Handle smart clipboard sync (when keyboard capture is enabled)
    if (keyboardCaptureEnabled && isConnected) {
      const isMac = navigator.platform.toUpperCase().indexOf('MAC') >= 0;
      const isCtrlOrCmd = isMac ? e.metaKey : e.ctrlKey;
      
      // Handle Cmd/Ctrl key combinations
      if (isCtrlOrCmd) {
        if (e.key.toLowerCase() === 'v') {
          // Cmd+V or Ctrl+V - paste to device
          console.log('[Clipboard] Smart paste triggered by Cmd+V/Ctrl+V');
          e.preventDefault();
          e.stopPropagation();
          onSmartPaste();
          return;
        }
        
        if (e.key.toLowerCase() === 'c') {
          // Cmd+C or Ctrl+C - copy from device
          console.log('[Clipboard] Smart copy triggered by Cmd+C/Ctrl+C');
          e.preventDefault();
          e.stopPropagation();
          onSmartCopy();
          return;
        }
        
        if (e.key.toLowerCase() === 'a') {
          // Cmd+A or Ctrl+A - select all
          console.log('[Keyboard] Select all triggered by Cmd+A/Ctrl+A');
          e.preventDefault();
          e.stopPropagation();
          
          // Send Ctrl+A combination to device with proper timing
          if (clientRef.current) {
            const META_CTRL_ON = 0x1000; // Android meta state for Ctrl key
            
            // Send Ctrl down first
            clientRef.current.sendKeyEvent(113, 'down', META_CTRL_ON); // KEYCODE_CTRL_LEFT
            
            // Small delay to ensure Ctrl is registered
            setTimeout(() => {
              if (clientRef.current) {
                // Send A down with Ctrl meta state
                clientRef.current.sendKeyEvent(29, 'down', META_CTRL_ON);  // KEYCODE_A
                
                // Small delay before releasing A
                setTimeout(() => {
                  if (clientRef.current) {
                    // Send A up with Ctrl meta state
                    clientRef.current.sendKeyEvent(29, 'up', META_CTRL_ON);    // KEYCODE_A
                    
                    // Small delay before releasing Ctrl
                    setTimeout(() => {
                      if (clientRef.current) {
                        // Send Ctrl up
                        clientRef.current.sendKeyEvent(113, 'up', 0);   // KEYCODE_CTRL_LEFT
                      }
                    }, 10);
                  }
                }, 10);
              }
            }, 10);
          }
          return;
        }
        
        // Prevent Cmd/Ctrl key from being sent to device when used in combinations
        if (e.key === 'Meta' || e.key === 'Control') {
          console.log('[Keyboard] Preventing Cmd/Ctrl key from being sent to device');
          e.preventDefault();
          e.stopPropagation();
          return;
        }
      }
    }
    
    if (!clientRef.current || !isConnected || !keyboardCaptureEnabled) return;
    
    // Map keyboard codes to Android keycodes (using e.code like ws-scrcpy)
    const keyMap: { [code: string]: number } = {
      // Functional keys
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
      
      // Arrow keys
      'ArrowUp': 19,
      'ArrowDown': 20,
      'ArrowLeft': 21,
      'ArrowRight': 22,
      
      // Navigation keys
      'Home': 122,
      'End': 123,
      'PageUp': 92,
      'PageDown': 93,
      'Insert': 124,
      
      // Letters
      'KeyA': 29, 'KeyB': 30, 'KeyC': 31, 'KeyD': 32, 'KeyE': 33,
      'KeyF': 34, 'KeyG': 35, 'KeyH': 36, 'KeyI': 37, 'KeyJ': 38,
      'KeyK': 39, 'KeyL': 40, 'KeyM': 41, 'KeyN': 42, 'KeyO': 43,
      'KeyP': 44, 'KeyQ': 45, 'KeyR': 46, 'KeyS': 47, 'KeyT': 48,
      'KeyU': 49, 'KeyV': 50, 'KeyW': 51, 'KeyX': 52, 'KeyY': 53,
      'KeyZ': 54,
      
      // Numbers
      'Digit0': 7, 'Digit1': 8, 'Digit2': 9, 'Digit3': 10, 'Digit4': 11,
      'Digit5': 12, 'Digit6': 13, 'Digit7': 14, 'Digit8': 15, 'Digit9': 16,
      
      // Symbols
      'Period': 56, 'Comma': 55, 'Slash': 76, 'Semicolon': 74, 'Quote': 75,
      'BracketLeft': 71, 'BracketRight': 72, 'Backslash': 73, 'Minus': 69, 'Equal': 70,
      'Backquote': 68,
      
      // Function keys
      'F1': 131, 'F2': 132, 'F3': 133, 'F4': 134, 'F5': 135, 'F6': 136,
      'F7': 137, 'F8': 138, 'F9': 139, 'F10': 140, 'F11': 141, 'F12': 142,
      
      // Input method keys (for triggering IME)
      'Lang1': 204,        // Language switch (most common for IME)
      'Lang2': 204,        // Alternative language switch
      'Convert': 214,      // Convert (Japanese IME)
      'NonConvert': 213,   // Non-convert (Japanese IME)
      'KanaMode': 218,     // Kana mode (Japanese IME)
    };
    
    const keycode = keyMap[e.code];
    
    if (keycode) {
      // Calculate meta state for modifier keys
      let metaState = 0;
      if (e.shiftKey) metaState |= 0x0001; // META_SHIFT_ON
      if (e.ctrlKey) metaState |= 0x1000;  // META_CTRL_ON
      if (e.altKey) metaState |= 0x0002;   // META_ALT_ON
      if (e.metaKey) metaState |= 0x10000; // META_META_ON
      
      console.log('[Keyboard] Sending key down:', keycode, 'metaState:', metaState, 'shiftKey:', e.shiftKey);
      e.preventDefault();
      e.stopPropagation();
      clientRef.current.sendKeyEvent(keycode, 'down', metaState);
    } else {
      console.log('[Keyboard] No keycode found for key:', e.key);
    }
  }, [isConnected, keyboardCaptureEnabled, onSmartPaste, onSmartCopy, clientRef]);

  const handleKeyUp = useCallback((e: React.KeyboardEvent) => {
    console.log('[Keyboard] Key up:', e.key, 'code:', e.code, 'isConnected:', isConnected, 'captureEnabled:', keyboardCaptureEnabled);
    
    // Handle smart clipboard sync (when keyboard capture is enabled)
    if (keyboardCaptureEnabled && isConnected) {
      // Prevent Cmd/Ctrl key from being sent to device when used in combinations
      if (e.key === 'Meta' || e.key === 'Control') {
        console.log('[Keyboard] Preventing Cmd/Ctrl keyup from being sent to device');
        e.preventDefault();
        e.stopPropagation();
        return;
      }
    }
    
    if (!clientRef.current || !isConnected || !keyboardCaptureEnabled) return;
    
    // Map keyboard codes to Android keycodes (using e.code like ws-scrcpy)
    const keyMap: { [code: string]: number } = {
      // Functional keys
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
      
      // Arrow keys
      'ArrowUp': 19,
      'ArrowDown': 20,
      'ArrowLeft': 21,
      'ArrowRight': 22,
      
      // Navigation keys
      'Home': 122,
      'End': 123,
      'PageUp': 92,
      'PageDown': 93,
      'Insert': 124,
      
      // Letters
      'KeyA': 29, 'KeyB': 30, 'KeyC': 31, 'KeyD': 32, 'KeyE': 33,
      'KeyF': 34, 'KeyG': 35, 'KeyH': 36, 'KeyI': 37, 'KeyJ': 38,
      'KeyK': 39, 'KeyL': 40, 'KeyM': 41, 'KeyN': 42, 'KeyO': 43,
      'KeyP': 44, 'KeyQ': 45, 'KeyR': 46, 'KeyS': 47, 'KeyT': 48,
      'KeyU': 49, 'KeyV': 50, 'KeyW': 51, 'KeyX': 52, 'KeyY': 53,
      'KeyZ': 54,
      
      // Numbers
      'Digit0': 7, 'Digit1': 8, 'Digit2': 9, 'Digit3': 10, 'Digit4': 11,
      'Digit5': 12, 'Digit6': 13, 'Digit7': 14, 'Digit8': 15, 'Digit9': 16,
      
      // Symbols
      'Period': 56, 'Comma': 55, 'Slash': 76, 'Semicolon': 74, 'Quote': 75,
      'BracketLeft': 71, 'BracketRight': 72, 'Backslash': 73, 'Minus': 69, 'Equal': 70,
      'Backquote': 68,
      
      // Function keys
      'F1': 131, 'F2': 132, 'F3': 133, 'F4': 134, 'F5': 135, 'F6': 136,
      'F7': 137, 'F8': 138, 'F9': 139, 'F10': 140, 'F11': 141, 'F12': 142,
      
      // Input method keys (for triggering IME)
      'Lang1': 204,        // Language switch (most common for IME)
      'Lang2': 204,        // Alternative language switch
      'Convert': 214,      // Convert (Japanese IME)
      'NonConvert': 213,   // Non-convert (Japanese IME)
      'KanaMode': 218,     // Kana mode (Japanese IME)
    };
    
    const keycode = keyMap[e.code];
    
    if (keycode) {
      // Calculate meta state for modifier keys
      let metaState = 0;
      if (e.shiftKey) metaState |= 0x0001; // META_SHIFT_ON
      if (e.ctrlKey) metaState |= 0x1000;  // META_CTRL_ON
      if (e.altKey) metaState |= 0x0002;   // META_ALT_ON
      if (e.metaKey) metaState |= 0x10000; // META_META_ON
      
      console.log('[Keyboard] Sending key up:', keycode, 'metaState:', metaState, 'shiftKey:', e.shiftKey);
      e.preventDefault();
      e.stopPropagation();
      clientRef.current.sendKeyEvent(keycode, 'up', metaState);
    } else {
      console.log('[Keyboard] No keycode found for key:', e.key);
    }
  }, [isConnected, keyboardCaptureEnabled, clientRef]);

  return {
    handleKeyDown,
    handleKeyUp,
  };
};
