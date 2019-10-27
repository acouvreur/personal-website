FROM node:12-alpine AS build

WORKDIR /build

RUN npm install -g resume-cli
COPY resume.json .
RUN resume export -t elegant index.html

FROM nginx:alpine
COPY --from=build /build/index.html /usr/share/nginx/html/index.html