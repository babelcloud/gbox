import "dotenv/config";

// Ensure API KEY is available
const apiKey = process.env.GBOX_API_KEY;
if (!apiKey) {
  throw new Error("Please set GBOX_API_KEY in environment variables or .env file");
}

export const config = {
  gboxApiKey: apiKey,
  mode: process.env.MODE?.toLowerCase() || "stdio",
};