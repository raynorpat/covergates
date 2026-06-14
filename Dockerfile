FROM golang:alpine AS build
RUN apk --update add musl-dev
RUN apk --update add util-linux-dev
RUN apk --update add gcc g++

WORKDIR /go/src/github.com/covergates/covergates

RUN go env -w GOPROXY=https://goproxy.cn,direct
COPY go.mod ./
COPY go.sum ./
RUN go mod download

FROM build as server-build
RUN apk --update add nodejs npm
COPY web/package.json ./web/package.json
COPY web/package-lock.json ./web/package-lock.json
RUN cd web && npm install
RUN node --version && npm --version
RUN go env -w GOBIN=/bin
RUN go install github.com/bradrydzewski/togo@latest
COPY web ./web
RUN go generate ./web
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -v -o covergates ./cmd/server

FROM alpine as server
COPY --from=server-build /go/src/github.com/covergates/covergates/covergates /covergates
ENTRYPOINT [ "/covergates" ]
