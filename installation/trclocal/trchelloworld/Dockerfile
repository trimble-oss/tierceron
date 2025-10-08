FROM alpine:3.16.9@sha256:452e7292acee0ee16c332324d7de05fa2c99f9994ecc9f0779c602916a672ae4

ADD bin/trchelloworld /usr/local/hello/trchelloworld

RUN GRPC_HEALTH_PROBE_VERSION=v0.4.13 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64

RUN chmod 755 /bin/grpc_health_probe

# Commented to let deployment descriptors handle.
#ADD config.yml /usr/local/hello/config.yml
#ADD hello.crt /usr/local/hello/hello.crt
#ADD hellokey.key /usr/local/hello/hellokey.key

RUN chmod 755 /usr/local/hello/trchelloworld
RUN  apk add shadow

ARG USERNAME=hello
ARG USER_UID=1234
ARG USER_GID=$USER_UID
RUN groupadd --gid $USER_GID $USERNAME && useradd --uid $USER_UID --gid $USER_GID -m $USERNAME

RUN chown -R $USERNAME:$USERNAME /usr/local/hello

USER $USERNAME:$USERNAME

CMD /usr/local/hello/trchelloworld