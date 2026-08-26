FROM docker.m.daocloud.io/library/golang:1.26.3-bookworm AS build

WORKDIR /app
ENV GOPROXY=https://goproxy.cn,direct GOSUMDB=sum.golang.google.cn GOTOOLCHAIN=local
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/inkorder ./cmd/task275

FROM docker.m.daocloud.io/library/alpine:3.20
COPY --from=build /app/inkorder /app/inkorder
ENTRYPOINT ["/app/inkorder"]
CMD ["--smoke-test"]
