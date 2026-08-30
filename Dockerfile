# Escalight is pure Go (the SQLite driver is CGO-free), so the runtime image
# needs no libc/sqlite shared libraries - distroless static is enough.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/escalight .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/escalight /usr/local/bin/escalight
VOLUME ["/data"]
ENV ESCALIGHT_DB_PATH=/data/escalight.db
ENV ESCALIGHT_ADDR=:8080
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/escalight", "serve"]
