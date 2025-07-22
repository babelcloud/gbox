// Create Android Box
export {
  CREATE_ANDROID_BOX_TOOL,
  CREATE_ANDROID_BOX_DESCRIPTION,
  createAndroidBoxParamsSchema,
  handleCreateAndroidBox,
} from "./create-android-box.js";

// List Boxes
export {
  LIST_BOXES_TOOL,
  LIST_BOXES_DESCRIPTION,
  listBoxesParamsSchema,
  handleListBoxes,
} from "./list-boxes.js";

// Get Box
export {
  GET_BOX_TOOL,
  GET_BOX_DESCRIPTION,
  getBoxParamsSchema,
  handleGetBox,
} from "./get-box.js";

// Screenshot
export {
  GET_SCREENSHOT_TOOL,
  GET_SCREENSHOT_DESCRIPTION,
  getScreenshotParamsSchema,
  handleGetScreenshot,
} from "./screenshot.js";

// UI Action
export {
  UI_ACTION_TOOL,
  UI_ACTION_DESCRIPTION,
  uiActionParamsSchema,
  handleUiAction,
} from "./ui-action.js";

// APK Management
export {
  INSTALL_APK_TOOL,
  INSTALL_APK_DESCRIPTION,
  installApkParamsSchema,
  handleInstallApk,
  UNINSTALL_APK_TOOL,
  UNINSTALL_APK_DESCRIPTION,
  uninstallApkParamsSchema,
  handleUninstallApk,
  OPEN_APP_TOOL,
  OPEN_APP_DESCRIPTION,
  openAppParamsSchema,
  handleOpenApp,
  CLOSE_APP_TOOL,
  CLOSE_APP_DESCRIPTION,
  closeAppParamsSchema,
  handleCloseApp,
} from "./apk-management.js";

// Press Key
export {
  PRESS_KEY_TOOL,
  PRESS_KEY_DESCRIPTION,
  pressKeyParamsSchema,
  handlePressKey,
} from "./press-key.js";

// Type Text
export {
  TYPE_TEXT_TOOL,
  TYPE_TEXT_DESCRIPTION,
  typeTextParamsSchema,
  handleTypeText,
} from "./type-text.js";

// Wait
export {
  WAIT_TOOL,
  WAIT_TOOL_DESCRIPTION,
  waitParamsSchema,
  handleWait,
} from "./wait.js";

// Logcat
export {
  LOGCAT_TOOL,
  LOGCAT_DESCRIPTION,
  logcatParamsSchema,
  handleLogcat,
} from "./logcat.js";

// Adb Shell
export {
  ADB_SHELL_TOOL,
  ADB_SHELL_DESCRIPTION,
  adbShellParamsSchema,
  handleAdbShell,
} from "./adb-shell.js";
