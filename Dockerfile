
#build stage
FROM golang:stretch AS builder
ENV GO111MODULE=on
WORKDIR /go/src
COPY . .
RUN go get -v -d .
RUN go build -o gitlab-source-link-proxy .

#final stage
FROM ubuntu:latest
# RUN apk --no-cache add ca-certificates
COPY --from=builder /go/src /app
CMD ["/app/gitlab-source-link-proxy"]
LABEL Name=gitlab-source-link-proxy Version=0.0.1
EXPOSE 7000
