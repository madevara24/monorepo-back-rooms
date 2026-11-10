# syntax=docker/dockerfile:1
FROM node:22-alpine AS build
WORKDIR /src

ARG PUBLIC_DOKPLOY_URL=""
ENV PUBLIC_DOKPLOY_URL=$PUBLIC_DOKPLOY_URL

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build

FROM nginx:1.27-alpine
COPY --from=build /src/build /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
