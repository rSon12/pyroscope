FROM golang:1.25.3

WORKDIR /go/src/app
COPY . .
RUN go build main.go
CMD ["./main"]
