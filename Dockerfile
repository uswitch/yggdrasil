FROM scratch

COPY --chmod=755 bin/yggdrasil-linux-amd64 yggdrasil

ENTRYPOINT ["/yggdrasil"]
