FROM node:24-bookworm-slim AS build

WORKDIR /src
COPY package.json pnpm-workspace.yaml ./
COPY apps/admin/package.json apps/admin/package.json
RUN corepack enable && pnpm install --filter @seoblog/admin... --frozen-lockfile=false
COPY apps/admin apps/admin
RUN pnpm --filter @seoblog/admin build

FROM node:24-bookworm-slim

WORKDIR /app
COPY --from=build --chown=node:node /src/apps/admin/.output /app/.output
USER node
EXPOSE 3000
CMD ["node", ".output/server/index.mjs"]
