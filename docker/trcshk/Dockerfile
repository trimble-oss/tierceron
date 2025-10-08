FROM debian:buster-slim@sha256:4620251e86cea652ef2746c8abef2fcc3ea54abb5f014bfbf841159d805cd1fd

WORKDIR /usr/local/trcshk

RUN  apt update \
  && apt install -y wget

RUN GRPC_HEALTH_PROBE_VERSION=v0.4.13 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64

RUN  apt remove -y wget

RUN chmod 755 /bin/grpc_health_probe

ADD bin/trcshk /usr/local/trcshk/trcshk

RUN chmod 755 /usr/local/trcshk/trcshk

ARG USERNAME=trcshk
ARG USER_UID=1575
ARG USER_GID=$USER_UID
RUN groupadd --gid $USER_GID $USERNAME && useradd --uid $USER_UID --gid $USER_GID -m $USERNAME

VOLUME /usr/local/trcshk/data
VOLUME /usr/local/trcshk/plugins
RUN chown -R $USERNAME:$USERNAME /usr/local/trcshk

USER $USERNAME:$USERNAME

CMD /usr/local/trcshk/trcshk