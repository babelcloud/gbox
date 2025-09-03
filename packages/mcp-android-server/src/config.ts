import "dotenv/config";

export const config = {
  mode: process.env.MODE?.toLowerCase() || "stdio",
};
