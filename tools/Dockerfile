FROM node:24-alpine
WORKDIR /tools
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts
USER node
ENTRYPOINT ["./node_modules/.bin/prism"]
CMD ["mock", "-h", "0.0.0.0", "/contracts/openapi.yaml"]
