import { createAndroidLiveView } from "./index";

// 用于测试
async function main() {
  const div = document.createElement("div");
  div.id = "video-container";
  div.style = "width: 100vw; height: 100vh;";
  document.body.appendChild(div);

  createAndroidLiveView(div, {
    onConnectionStateChange: (state, message) => {
      console.log("连接状态变化:", state, message);
    },
    onError: (error) => {
      console.error("发生错误:", error);
    },
    onStatsUpdate: (stats) => {
      console.log("统计信息:", stats);
    },
    connectaParams: {
      deviceSerial: "68afd15",
      apiUrl: "http://localhost:3000/api",
      wsUrl: "ws://localhost:3000",
    }
  })
}

main();
