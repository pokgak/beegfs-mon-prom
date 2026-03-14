FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /beegfs-mon-prom .

FROM alpine:3.21
RUN apk add --no-cache beegfs-ctl || true
COPY --from=build /beegfs-mon-prom /usr/local/bin/beegfs-mon-prom
EXPOSE 9100
ENTRYPOINT ["beegfs-mon-prom"]
