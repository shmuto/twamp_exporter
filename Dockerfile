ARG OS="linux"
ARG ARCH="amd64"

FROM golang:1.19-alpine as build
RUN apk update
RUN mkdir /build
COPY . /build
WORKDIR /build
RUN GOOS=${OS} GOARCH=${ARCH} go build -o twamp_exporter -v main.go

FROM alpine:3.16.3
RUN apk add --no-cache --update bash 

COPY --from=build /build/twamp_exporter /bin/twamp_exporter
COPY ./config.yaml /usr/share/twamp_exporter/config.yaml

EXPOSE 9861

ENTRYPOINT [ "/bin/twamp_exporter"]
CMD ["-config.file=/usr/share/twamp_exporter/config.yaml"]