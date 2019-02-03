FROM golang:1.11-alpine

WORKDIR /go/src/incursion-discord
COPY . .

RUN go get -d -v ./...
RUN go install -v ./...

CMD ["incursion-discord"]