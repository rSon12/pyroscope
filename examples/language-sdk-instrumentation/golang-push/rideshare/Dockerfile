FROM golang:1.25rc3

WORKDIR /go/src/app
COPY . .
RUN go build main.go
CMD ["./main"]
