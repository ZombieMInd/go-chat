FROM golang as build-stage
WORKDIR /app

ENV REDIS_HOST=localhost:6379
ENV REDIS_PASSWORD=e3e5a63c6bb2ae99
ENV EXT_PORT=9090

COPY go.mod /app
COPY go.sum /app
RUN go mod download

ADD src/chat/ /app/chat
COPY src/main.go /app


RUN cd /app && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app

FROM alpine
COPY --from=build-stage /app/app /

COPY fullchain.pem /
COPY privkey.pem /

EXPOSE 8080

CMD ["/app"]