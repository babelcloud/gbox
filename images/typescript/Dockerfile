ARG NODE_VERSION=22.18

FROM node:${NODE_VERSION}-bookworm

# Install tini for proper process management
RUN apt update && apt install -y --no-install-recommends tini && apt clean && rm -rf /var/lib/apt/lists/*

WORKDIR /root

# Install global TypeScript and other common tools
RUN npm install -g typescript tsx @types/node

ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["sleep", "infinity"]