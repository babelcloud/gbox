package control

import (
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/core"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/protocol"
	"github.com/babelcloud/gbox/packages/cli/internal/device_connect/scrcpy"
	"github.com/babelcloud/gbox/packages/cli/internal/util"
)

// ControlService 控制服务
type ControlService struct {
	// 直接使用 scrcpy 全局管理器获取设备源
}

// NewControlService creates a new control service
func NewControlService() *ControlService {
	return &ControlService{}
}

// HandleTouchEvent 处理触摸事件
func (s *ControlService) HandleTouchEvent(msg map[string]interface{}, deviceSerial string) error {
	action, _ := msg["action"].(string)
	x, _ := msg["x"].(float64)
	y, _ := msg["y"].(float64)
	pressure, _ := msg["pressure"].(float64)
	pointerId, _ := msg["pointerId"].(float64)

	util.GetLogger().Debug("Touch event received", "device", deviceSerial, "action", action, "x", x, "y", y, "pressure", pressure, "pointerId", pointerId)

	// 获取设备的 source
	source := scrcpy.GetSource(deviceSerial)
	if source == nil {
		util.GetLogger().Warn("Device source not found", "device", deviceSerial)
		return nil
	}

	// 获取设备屏幕尺寸（用于坐标转换）
	_, screenWidth, screenHeight := source.GetConnectionInfo()
	if screenWidth == 0 || screenHeight == 0 {
		util.GetLogger().Warn("Unknown screen size, using default", "device", deviceSerial)
		screenWidth, screenHeight = 1080, 1920 // 默认尺寸
	}

	// 创建触摸事件，复用 WebRTC 模式的控制逻辑
	touchEvent := protocol.TouchEvent{
		Action:    action,
		X:         x,
		Y:         y,
		Pressure:  pressure,
		PointerID: int(pointerId),
	}

	// 编码触摸事件（需要屏幕尺寸进行坐标转换）
	data := protocol.EncodeTouchEvent(touchEvent, screenWidth, screenHeight)

	// 创建控制消息
	controlMsg := core.ControlMessage{
		Type: int32(protocol.ControlMsgTypeInjectTouchEvent),
		Data: data,
	}

	// 发送到设备
	if err := source.SendControl(controlMsg); err != nil {
		util.GetLogger().Error("Failed to send touch event", "device", deviceSerial, "error", err)
		return err
	}

	util.GetLogger().Debug("Touch event sent successfully", "device", deviceSerial, "action", action)
	return nil
}

// HandleKeyEvent 处理键盘事件
func (s *ControlService) HandleKeyEvent(msg map[string]interface{}, deviceSerial string) error {
	action, _ := msg["action"].(string)
	keycode, _ := msg["keycode"].(float64)
	metaState, _ := msg["metaState"].(float64)

	util.GetLogger().Debug("Key event", "device", deviceSerial, "action", action, "keycode", keycode, "metaState", metaState)

	// 获取设备的 source
	source := scrcpy.GetSource(deviceSerial)
	if source == nil {
		util.GetLogger().Warn("Device source not found", "device", deviceSerial)
		return nil
	}

	// 创建按键事件，复用 WebRTC 模式的控制逻辑
	keyEvent := protocol.KeyEvent{
		Action:    action,
		Keycode:   int(keycode),
		MetaState: int(metaState),
		Repeat:    0, // H264 模式暂时不支持 repeat
	}

	// 编码按键事件
	data := protocol.EncodeKeyEvent(keyEvent)

	// 创建控制消息
	controlMsg := core.ControlMessage{
		Type: int32(protocol.ControlMsgTypeInjectKeycode),
		Data: data,
	}

	// 发送到设备
	if err := source.SendControl(controlMsg); err != nil {
		util.GetLogger().Error("Failed to send key event", "device", deviceSerial, "error", err)
		return err
	}

	util.GetLogger().Debug("Key event sent successfully", "device", deviceSerial)
	return nil
}

// HandleScrollEvent 处理滚动事件
func (s *ControlService) HandleScrollEvent(msg map[string]interface{}, deviceSerial string) error {
	x, _ := msg["x"].(float64)
	y, _ := msg["y"].(float64)
	hScroll, _ := msg["hScroll"].(float64)
	vScroll, _ := msg["vScroll"].(float64)

	util.GetLogger().Debug("Scroll event", "device", deviceSerial, "x", x, "y", y, "hScroll", hScroll, "vScroll", vScroll)

	// 获取设备的 source
	source := scrcpy.GetSource(deviceSerial)
	if source == nil {
		util.GetLogger().Warn("Device source not found", "device", deviceSerial)
		return nil
	}

	// 获取设备屏幕尺寸（用于坐标转换）
	_, screenWidth, screenHeight := source.GetConnectionInfo()
	if screenWidth == 0 || screenHeight == 0 {
		util.GetLogger().Warn("Unknown screen size, using default", "device", deviceSerial)
		screenWidth, screenHeight = 1080, 1920 // 默认尺寸
	}

	// 创建滚动事件，复用 WebRTC 模式的控制逻辑
	scrollEvent := protocol.ScrollEvent{
		X:       x,
		Y:       y,
		HScroll: hScroll,
		VScroll: vScroll,
	}

	// 编码滚动事件（需要屏幕尺寸进行坐标转换）
	data := protocol.EncodeScrollEvent(scrollEvent, screenWidth, screenHeight)

	// 创建控制消息
	controlMsg := core.ControlMessage{
		Type: int32(protocol.ControlMsgTypeInjectScrollEvent),
		Data: data,
	}

	// 发送到设备
	if err := source.SendControl(controlMsg); err != nil {
		util.GetLogger().Error("Failed to send scroll event", "device", deviceSerial, "error", err)
		return err
	}

	util.GetLogger().Debug("Scroll event sent successfully", "device", deviceSerial)
	return nil
}

// HandleClipboardEvent 处理剪贴板事件
func (s *ControlService) HandleClipboardEvent(msg map[string]interface{}, deviceSerial string) error {
	text, _ := msg["text"].(string)
	paste, _ := msg["paste"].(bool)

	util.GetLogger().Debug("Clipboard event", "device", deviceSerial, "text_length", len(text), "paste", paste)

	// TODO: Forward clipboard event to bridge manager
	// This will be implemented when bridge integration is ready
	util.GetLogger().Warn("Clipboard event received but bridge integration not yet implemented")

	return nil
}

// HandleVideoResetEvent 处理视频重置事件
func (s *ControlService) HandleVideoResetEvent(msg map[string]interface{}, deviceSerial string) error {
	util.GetLogger().Debug("Reset video event", "device", deviceSerial)

	// 获取设备的 source
	source := scrcpy.GetSource(deviceSerial)
	if source == nil {
		util.GetLogger().Warn("Device source not found", "device", deviceSerial)
		return nil
	}

	// 请求关键帧，复用 WebRTC 模式的控制逻辑
	// 创建一个空的控制消息，类型为重置视频
	controlMsg := core.ControlMessage{
		Type: int32(protocol.ControlMsgTypeResetVideo),
		Data: []byte{}, // 视频重置不需要额外数据
	}

	// 发送到设备
	if err := source.SendControl(controlMsg); err != nil {
		util.GetLogger().Error("Failed to send video reset event", "device", deviceSerial, "error", err)
		return err
	}

	util.GetLogger().Debug("Video reset event sent successfully", "device", deviceSerial)
	return nil
}

// HandleWebRTCEvent 处理 WebRTC 事件
func (s *ControlService) HandleWebRTCEvent(msg map[string]interface{}, deviceSerial string) error {
	msgType, _ := msg["type"].(string)

	util.GetLogger().Debug("WebRTC event", "device", deviceSerial, "type", msgType)

	// TODO: Forward WebRTC event to WebRTC handler
	// This will be implemented when WebRTC integration is ready
	util.GetLogger().Warn("WebRTC event received but WebRTC integration not yet implemented")

	return nil
}

// 全局控制服务实例
var controlService *ControlService

// GetControlService 获取控制服务实例
func GetControlService() *ControlService {
	return controlService
}

// SetControlService 设置控制服务实例
func SetControlService() {
	controlService = NewControlService()
}
