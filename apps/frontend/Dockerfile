FROM node:22-alpine AS builder
WORKDIR /app

COPY package*.json ./
RUN npm ci
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
