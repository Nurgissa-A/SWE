FROM node:24-alpine@sha256:06d7adf3e0ef61a13f1d28682726366f97a15aa1888c934d3ecec86c33918b53
WORKDIR /tools
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts
USER node
ENTRYPOINT ["./node_modules/.bin/prism"]
CMD ["mock", "-h", "0.0.0.0", "/contracts/openapi.yaml"]
