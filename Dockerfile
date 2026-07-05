FROM golang:1.26-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /agent-pr-police ./cmd/agent-pr-police

FROM gcr.io/distroless/static-debian12
COPY --from=build /agent-pr-police /agent-pr-police
ENTRYPOINT ["/agent-pr-police"]
