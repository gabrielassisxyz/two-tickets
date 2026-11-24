# Production image for the `api` process: a static Go binary on distroless
# static, running as non-root. The build disables cgo (SQLite is pure Go), so
# the binary needs no libc at run time and the image holds no shell.
FROM golang:1.26.11-bookworm@sha256:dc40a13b61304647ed7e5d581d246b86413dfc29d63c6d018310feb93cd98fc0 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 go build -trimpath -o /two-tickets ./cmd/two-tickets

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab

COPY --from=builder /two-tickets /usr/local/bin/two-tickets
# The image's own unprivileged user: the server only reads its files and the
# mounted data volume, so it has no reason to run as root.
USER nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/two-tickets"]
