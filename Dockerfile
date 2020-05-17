FROM node:12-alpine AS build

WORKDIR /build

COPY package*.json ./
RUN npm ci

COPY resume.json .
RUN npm run build

FROM nginx:alpine

COPY --from=build /build/index.html /usr/share/nginx/html/index.html