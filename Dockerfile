FROM registry.gitlab.com/ulrichschreiner/base/debian:buster-slim

RUN apt -y update && \
    apt -y install ca-certificates curl tzdata && \
    rm -rf /var/lib/apt/lists/* && \
    useradd --create-home --user-group --shell /bin/bash --home-dir /work --uid 10001 s3sync && \
    mkdir /work/.mc && chown s3sync:s3sync /work/.mc && \
    curl https://dl.min.io/client/mc/release/linux-amd64/mc >/usr/local/bin/mc && \
    chmod +x /usr/local/bin/mc

RUN mkdir /work/mc && ln -s /work/mc/config.json /work/.mc/config.json

COPY bin/s3syncer /usr/local/bin/s3syncer
WORKDIR /work
EXPOSE 9999

USER s3sync
ENTRYPOINT ["/usr/local/bin/s3syncer","-listen","0.0.0.0:9999"]