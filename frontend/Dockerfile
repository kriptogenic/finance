FROM node:26-bookworm-slim AS build

WORKDIR /app/frontend

# Install deps against a cacheable layer.
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY specs/ /app/specs/
COPY frontend/ ./

ARG VITE_API_URL=/api
ENV VITE_API_URL=${VITE_API_URL}
RUN npm run build

FROM nginx:1.27-alpine
COPY frontend/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/frontend/dist /usr/share/nginx/html
EXPOSE 80
