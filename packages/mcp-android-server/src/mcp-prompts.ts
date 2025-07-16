const MCP_ANDROID_TESTING_PROMPT = {
  "name": "gbox_apk_testing_guide",
  "description": "Step-by-step guide to test an Android APK using Gbox, with parameters for APK path and Box ID.",
  "prompt_template": "Follow these steps to test an Android APK using Gbox:\n\n" +
    "1. Immediately after creating or starting the Android box with ID `{box_id}`, open its Live View by calling the `open_live_view` tool.\n" +
    "2. Install your APK using the `install_apk` tool with the absolute path:\n" +
    "   `{apk_path}`\n" +
    "   You can add `open=true` to launch the app automatically after installation.\n" +
    "3. Wait for the APK to finish installing before performing any interactions.\n" +
    "4. If multiple boxes are active, ensure you only interact with box `{box_id}` to avoid conflicts.\n\n" +
    "To perform UI actions, use the `ui_action` tool with natural language commands such as:\n" +
    "- Tap the email input field\n" +
    "- Tap the submit button\n" +
    "- Tap the plus button in the upper right corner\n" +
    "- Fill the search field with text: `gbox ai`\n" +
    "- Press the back button\n" +
    "- Double-click the video\n\n" +
    "This procedure ensures stable, visible, and accurate UI testing in Gbox. Keep Live View open during the entire test session.",
  "parameters": {
    "apk_path": {
      "type": "string",
      "description": "Absolute path to the APK file to be installed"
    },
    "box_id": {
      "type": "string",
      "description": "The unique ID of the Android box instance under test"
    }
  },
  "type": "instruction"
}
