FROM oven/bun:1-alpine

WORKDIR /app

COPY package.json bun.lock* ./
RUN bun install --frozen-lockfile || bun install

COPY tsconfig.json ./
COPY src/ ./src/

EXPOSE 9999

CMD ["bun", "run", "src/server.ts"]
