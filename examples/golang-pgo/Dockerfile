FROM golang:1.25.4

WORKDIR /go/src/app
COPY . .

RUN go build main.go

CMD ["./main"]
