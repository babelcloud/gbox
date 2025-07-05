# Use the official Node.js runtime as the parent image
# LTS version (e.g., Node 20) is recommended
FROM node:20-alpine AS base

# Set the working directory in the container
WORKDIR /app

# Install pnpm globally
RUN npm install -g pnpm

# Copy package.json and pnpm-lock.yaml
COPY package.json pnpm-lock.yaml ./

# Install dependencies using pnpm. --frozen-lockfile ensures reproducibility.
RUN pnpm install --frozen-lockfile

# Copy the rest of the application code
# Note: This copies all files; .dockerignore will exclude unnecessary files later
COPY . .

# Build TypeScript code to JavaScript
RUN pnpm run build

# Use a smaller base image for the final stage
FROM node:20-slim AS production

WORKDIR /app

# Copy only necessary files from the build stage
COPY --from=base /app/package.json /app/pnpm-lock.yaml ./
COPY --from=base /app/node_modules ./node_modules
COPY --from=base /app/dist ./dist
COPY --from=base /app/.env.defaults ./

EXPOSE 28091

# Define the command to run the application
# Use the start script from package.json
CMD ["node", "./dist/index.js"] 