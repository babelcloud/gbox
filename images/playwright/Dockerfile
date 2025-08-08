ARG REGISTRY=babelcloud
ARG TAG=latest # Define ARG for base image tag
ARG PLAYWRIGHT_VERSION=1.54.0 # Define Playwright version ARG

FROM ${REGISTRY}/gbox-python:${TAG}

# Install browser dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    libnss3 \
    libxss1 \
    libasound2 \
    fonts-liberation \
    libatk-bridge2.0-0 \
    libatk1.0-0 \
    libatspi2.0-0 \
    libcups2 \
    libdrm2 \
    libgbm1 \
    libgtk-3-0 \
    libxcomposite1 \
    libxdamage1 \
    libxkbcommon0 \
    libxrandr2 \
    xdg-utils \
    chromium \
    socat \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Install Node.js and npm
# Using ARG for Node version consistency
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y nodejs \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Set Playwright environment variables
ENV PLAYWRIGHT_BROWSERS_PATH=/root/.cache/ms-playwright \
    PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1

# Install Playwright dependencies and browser using ARG
ARG PLAYWRIGHT_VERSION
RUN npx playwright@${PLAYWRIGHT_VERSION} install --with-deps --no-shell chromium

# Copy playwright supervisord config and substitute version placeholder
COPY supervisord.conf /etc/supervisor/conf.d/playwright.conf
ARG PLAYWRIGHT_VERSION
RUN sed -i "s|__PLAYWRIGHT_VERSION__|${PLAYWRIGHT_VERSION}|g" /etc/supervisor/conf.d/playwright.conf

# Expose ports
EXPOSE 3000 7681 9222

# Add health check (relevant for browser stage)
HEALTHCHECK --interval=10s --timeout=5s --start-period=5s --retries=3 \
    CMD curl --fail http://localhost:3000 || exit 1

# Inherit ENTRYPOINT and CMD from base image 