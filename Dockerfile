FROM golang:1.21.1

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

RUN mkdir -p /var/lib/appdata/ && chmod -R 777 /var/lib/appdata/

COPY . .

# Compila la aplicación Go
RUN go build -o leonlib ./cmd/webapp

CMD /app/leonlib


