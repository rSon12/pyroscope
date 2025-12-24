FROM golang:1.26rc1

WORKDIR /go/src/app
COPY . .
RUN go build main.go
CMD ["./main"]
