# Hooks Documentation

## Overview

The hooks are now split into specialized, focused modules for better maintainability:

## Specialized Hooks

### `useClipboardHandler`
Handles clipboard operations (copy/paste).

```typescript
const { handleClipboardPaste, handleClipboardCopy } = useClipboardHandler({
  client,
  enabled: true,
  isConnected: true,
  onError: (error) => console.error(error),
});
```

### `useControlHandler`
Handles control actions (power, volume, back, home, etc.).

```typescript
const { handleControlAction, handleIMESwitch } = useControlHandler({
  client,
  enabled: true,
  isConnected: true,
});
```

### `useKeyboardHandler`
Handles keyboard events and key mapping.

```typescript
const { handleKeyDown, handleKeyUp } = useKeyboardHandler({
  client,
  enabled: true,
  keyboardCaptureEnabled: true,
  isConnected: true,
  onClipboardPaste: clipboardHandler.handleClipboardPaste,
  onClipboardCopy: clipboardHandler.handleClipboardCopy,
});
```

### `useWheelHandler`
Handles wheel/scroll events.

```typescript
const { handleWheel } = useWheelHandler({
  client,
  enabled: true,
  isConnected: true,
});
```

### `useMouseHandler`
Handles mouse events and drag state.

```typescript
const { handleMouseDown, handleMouseUp, handleMouseMove, handleMouseLeave, isMouseDragging } = useMouseHandler({
  client,
  enabled: true,
  isConnected: true,
});
```

### `useClickHandler`
Handles click events (single, double, triple click).

```typescript
const { handleClick } = useClickHandler({
  client,
  enabled: true,
  isConnected: true,
});
```

### `useTouchHandler`
Handles touch events.

```typescript
const { handleTouchStart, handleTouchEnd, handleTouchMove } = useTouchHandler({
  client,
  enabled: true,
  isConnected: true,
});
```

## Usage Example

Here's how to use the specialized hooks in a React component:

```typescript
import { 
  useClipboardHandler, 
  useControlHandler, 
  useKeyboardHandler, 
  useWheelHandler, 
  useMouseHandler, 
  useClickHandler,
  useTouchHandler
} from '../hooks';

function MyComponent() {
  const clipboardHandler = useClipboardHandler({ client, enabled, isConnected, onError });
  const controlHandler = useControlHandler({ client, enabled, isConnected });
  const keyboardHandler = useKeyboardHandler({ 
    client, 
    enabled, 
    keyboardCaptureEnabled, 
    isConnected,
    onClipboardPaste: clipboardHandler.handleClipboardPaste,
    onClipboardCopy: clipboardHandler.handleClipboardCopy,
  });
  const wheelHandler = useWheelHandler({ client, enabled, isConnected });
  const mouseHandler = useMouseHandler({ client, enabled, isConnected });
  const clickHandler = useClickHandler({ client, enabled, isConnected });
  const touchHandler = useTouchHandler({ client, enabled, isConnected });

  return (
    <video
      onKeyDown={keyboardHandler.handleKeyDown}
      onKeyUp={keyboardHandler.handleKeyUp}
      onMouseDown={mouseHandler.handleMouseDown}
      onMouseUp={mouseHandler.handleMouseUp}
      onMouseMove={mouseHandler.handleMouseMove}
      onMouseLeave={mouseHandler.handleMouseLeave}
      onTouchStart={touchHandler.handleTouchStart}
      onTouchEnd={touchHandler.handleTouchEnd}
      onTouchMove={touchHandler.handleTouchMove}
      onWheel={wheelHandler.handleWheel}
      onClick={clickHandler.handleClick}
    />
  );
}
```

## Benefits

1. **Modularity**: Each hook has a single responsibility
2. **Reusability**: Hooks can be used independently
3. **Maintainability**: Easier to debug and modify specific functionality
4. **Testability**: Each hook can be tested in isolation
5. **Composability**: Hooks can be combined in different ways
