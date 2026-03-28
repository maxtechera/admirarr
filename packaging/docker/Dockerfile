FROM alpine:3.21

RUN apk add --no-cache \
    ca-certificates \
    docker-cli \
    docker-compose \
    curl \
    bash

COPY admirarr /usr/local/bin/admirarr

RUN chmod +x /usr/local/bin/admirarr

ENTRYPOINT ["/usr/local/bin/admirarr"]
