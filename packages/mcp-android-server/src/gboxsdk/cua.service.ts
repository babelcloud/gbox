import axios from "axios";
import { config } from "../config.js";

export type Coordinates = {
  x: number;
  y: number;
}[];

export async function getCUACoordinates(
  instruction: string,
  screenshotUri: string,
): Promise<Coordinates> {
  const baseUrl = (
    process.env.AGENT_BASE_URL ||
    process.env.BACKEND_BASE_URL ||
    "https://playground.gbox.ai"
  ).replace(/\/$/, "");
  const url = `${baseUrl}/api/agent/call-uitars`;

  // try 3 times
  for (let i = 0; i < 3; i++) {
    try {
      const response = await axios.post(url, {
        instruction,
        image: screenshotUri,
        gboxApiKey: config.gboxApiKey,
      });

      const message: string | undefined = response?.data?.message;
      const success: boolean = response?.data?.success;
      if (success && message) {
        return extractCoordinates(message) ?? [];
      }
    } catch (error) {
      console.error("[ERROR] getCoordinates failed:", error);
      return [];
    }
  }
  return [];
}

/** Extract numeric coordinates from model output of the form "(x,y)" or "x,y" or "(x,y)(x,y)" */
function extractCoordinates(text: string): { x: number; y: number }[] | null {
  if (!text) return null;
  const matches = Array.from(text.matchAll(/\(?\s*(\d+)\s*,\s*(\d+)\s*\)?/g));
  if (!matches.length) return null;
  return matches.map((m) => ({ x: Number(m[1]), y: Number(m[2]) }));
}
