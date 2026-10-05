FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

FROM alpine:3.22
COPY --from=build /server /server
ENV DB_PATH=/data/freshket.db
VOLUME /data
EXPOSE 8080
CMD ["/server"]
