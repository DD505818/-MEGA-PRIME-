FROM node:20.18.1-alpine3.20
WORKDIR /app
COPY package*.json ./
RUN npm install --omit=dev --no-audit --no-fund \
    && addgroup -S -g 65532 omega \
    && adduser -S -u 65532 -G omega omega
COPY --chown=65532:65532 . .
USER 65532:65532
EXPOSE 3001
CMD ["node", "index.js"]
