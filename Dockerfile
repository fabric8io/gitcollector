FROM scratch

ADD ./build/gitcollector-linux-amd64 /bin/gitcollector

ENTRYPOINT ["/bin/gitcollector", "version"]