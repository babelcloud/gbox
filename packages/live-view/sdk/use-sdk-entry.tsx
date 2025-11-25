import ReactDOM from "react-dom/client";
import AndroidLiveviewComponent from "./index";

const rootElement = document.getElementById("root");
if (!rootElement) {
  throw new Error("Root element not found");
}

ReactDOM.createRoot(rootElement).render(
  <AndroidLiveviewComponent
    connectParams={{
      deviceSerial: "68afd15",
      apiUrl: "http://localhost:29888/api",
      wsUrl: "ws://localhost:29888",
    }}
  />
);
