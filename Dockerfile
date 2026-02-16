# Build stage for frontend
FROM node:18-alpine3.20 AS frontend-builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

# Build stage for backend
FROM golang:1.24-alpine3.21 AS backend-builder
WORKDIR /app/backend
RUN apk add --no-cache git
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server

# Final stage
FROM alpine:3.21
RUN apk --no-cache add ca-certificates
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /home/appuser

# Copy backend binary
COPY --from=backend-builder /app/backend/server .
COPY --from=backend-builder /app/backend/configs ./configs

# Copy frontend build
COPY --from=frontend-builder /app/dist ./public

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Run the server
CMD ["./server"]
