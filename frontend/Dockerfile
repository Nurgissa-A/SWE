FROM node:24-alpine@sha256:06d7adf3e0ef61a13f1d28682726366f97a15aa1888c934d3ecec86c33918b53 AS build
WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts
COPY . .
ARG IDENTITY_API_URL=http://python:8000
ARG EVENTS_API_URL=http://go:8080
ENV IDENTITY_API_URL=$IDENTITY_API_URL EVENTS_API_URL=$EVENTS_API_URL
RUN npm run build
FROM node:24-alpine@sha256:06d7adf3e0ef61a13f1d28682726366f97a15aa1888c934d3ecec86c33918b53
WORKDIR /app
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 PORT=3000 HOSTNAME=0.0.0.0
COPY --from=build --chown=node:node /app/.next/standalone ./
COPY --from=build --chown=node:node /app/.next/static ./.next/static
COPY --from=build --chown=node:node /app/public ./public
USER node
ARG RELEASE_COMMIT=development
LABEL org.opencontainers.image.revision=$RELEASE_COMMIT
EXPOSE 3000
CMD ["node","server.js"]
