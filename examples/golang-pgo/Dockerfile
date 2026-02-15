FROM golang:1.26.0

WORKDIR /go/src/app
COPY . .

RUN go build main.go

CMD ["./main"]
