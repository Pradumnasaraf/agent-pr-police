# A hardened Dockerfile that passes copilot-pr-guard's container checks.
FROM node:20.11.1-alpine@sha256:c0a3badbd8a0a760de903e00cedbca94588e609299820557e72cba2a53dbaa2c AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --omit=dev
COPY ./src ./src

FROM node:20.11.1-alpine@sha256:c0a3badbd8a0a760de903e00cedbca94588e609299820557e72cba2a53dbaa2c
WORKDIR /app
RUN addgroup -S app && adduser -S app -G app
COPY --from=build /app /app
USER app
ENTRYPOINT ["node", "src/server.js"]
