FROM golang:1.24rc2

WORKDIR /go/src/app
COPY . .
RUN go build main.go
CMD ["./main"]
