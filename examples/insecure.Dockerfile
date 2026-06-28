# An intentionally insecure Dockerfile used to demonstrate copilot-pr-guard.
FROM node:latest

# Hardcoded secret baked into the image.
ENV API_KEY=sk_live_5f8a1c2b3d4e

# Remote script piped straight into a shell.
RUN curl -fsSL https://example.com/install.sh | sh

# Needs root for "convenience".
RUN sudo apt-get update

# Pull a remote archive with ADD instead of a verified download.
ADD https://example.com/app.tar.gz /app/

# Copy the entire build context, secrets and all.
COPY . .

# No USER directive: the container runs as root.
CMD ["node", "server.js"]
