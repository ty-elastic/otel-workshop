FROM golang:1.21.4-alpine AS builder
WORKDIR /usr/src/app/

COPY . ./
RUN go build -o /go/bin/catalog

# -----------------------------------------------------------------------------

FROM alpine

WORKDIR /usr/src/app/

COPY --from=builder /go/bin/catalog ./

EXPOSE 9000
ENTRYPOINT [ "./catalog" ]
