FROM scratch

ADD bin/yggdrasil yggdrasil

ENTRYPOINT ["/yggdrasil"]
