FROM scratch

ADD bin/yggdrasil-linux-amd64 yggdrasil

ENTRYPOINT ["/yggdrasil"]
