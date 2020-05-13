FROM node:8-alpine AS build

WORKDIR /build

COPY package*.json /build/
RUN npm ci

COPY resume.json .
RUN npm run build

FROM nginx:alpine

COPY --from=build /build/dist/index.html /usr/share/nginx/html/index.html