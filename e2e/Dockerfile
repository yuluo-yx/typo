FROM golang:1.25.8-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends zsh ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY . .

CMD ["go", "test", "./e2e", "-v"]
