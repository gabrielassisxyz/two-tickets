# Production image for the `web` rendering process. It serves the Next.js
# standalone server built from `web/`; the API lives in a separate image.
# The base line below is upstream Seerr's own runtime base, kept byte for
# byte so the two images share the same Node and libc.
FROM node:22.23.5-alpine3.23@sha256:6868d4695b947ee120bffaa463fecd68cde370d7bfec92955e7c8ac576b4f475 AS base

FROM base AS deps
RUN apk add --no-cache libc6-compat python3 make g++
WORKDIR /app
COPY package.json pnpm-lock.yaml .npmrc .node-version ./
RUN corepack enable && corepack pnpm install --frozen-lockfile

FROM base AS builder
RUN apk add --no-cache libc6-compat python3 make g++
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN corepack enable && corepack pnpm run build \
  && cp -r .next/static .next/standalone/.next/static \
  && cp -r public .next/standalone/public

FROM base AS runner
RUN apk add --no-cache libc6-compat
WORKDIR /app
ENV NODE_ENV=production
COPY --from=builder /app/.next/standalone ./.next/standalone
# The image's own unprivileged user: the server only reads its files and
# writes nothing, so it has no reason to run as root.
USER node
EXPOSE 3000
ENV PORT=3000
ENV HOSTNAME=0.0.0.0
CMD ["node", ".next/standalone/server.js"]
