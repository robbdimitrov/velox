FROM node:22-alpine AS builder
WORKDIR /app
ARG PUBLIC_GATEWAY_BASE_URL=http://localhost:8081
ENV PUBLIC_GATEWAY_BASE_URL=${PUBLIC_GATEWAY_BASE_URL}
COPY package*.json ./
RUN npm install
COPY . .
RUN npm run build

FROM node:22-alpine AS runner
WORKDIR /app
RUN addgroup -S velox && adduser -S -G velox velox
COPY --from=builder --chown=velox:velox /app/package*.json ./
COPY --from=builder --chown=velox:velox /app/build ./build
USER velox
ENV NODE_ENV=production
ENV HOST=0.0.0.0
ENV PORT=8080
EXPOSE 8080
CMD ["node", "build"]
