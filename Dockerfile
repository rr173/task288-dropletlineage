FROM docker.m.daocloud.io/library/golang:1.26.3-bookworm AS build

WORKDIR /app
ENV GOPROXY=https://goproxy.cn,direct GOSUMDB=sum.golang.google.cn GOTOOLCHAIN=local
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/task288 ./cmd/task288

FROM docker.m.daocloud.io/library/alpine:3.20
COPY --from=build /app/task288 /app/task288
ENTRYPOINT ["/app/task288"]
CMD ["--smoke-test"]
