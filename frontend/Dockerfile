# Build Angular for production
FROM node:20-alpine

WORKDIR /app

# Accept build configuration as argument (defaults to production)
ARG BUILD_CONFIGURATION=production

# Copy package files and install dependencies
COPY package*.json ./
RUN npm ci

# Copy source code and build with the specified configuration
COPY . .
RUN npm run build -- --configuration ${BUILD_CONFIGURATION}

# Install a simple static file server
RUN npm install -g serve

# Expose port 4200
EXPOSE 4200

# Serve the built Angular app
CMD ["serve", "-s", "dist/frontend/browser", "-l", "4200"]