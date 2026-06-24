# Sandbox image for Go submissions.
#
# At runtime the executor mounts a tmpfs at /box (the only writable path),
# disables networking, drops all capabilities, and makes the rootfs read-only.
FROM golang:1.26-alpine

RUN adduser -D -u 10001 -s /sbin/nologin sandbox

# The build cache and temp dir live on the per-run tmpfs and are wiped after
# compilation (tmpfs pages count against the run-phase memory limit); the
# rootfs — including /tmp — is read-only at runtime. cgo is disabled:
# submissions are pure Go and the image ships no C toolchain.
ENV GOCACHE=/box/.gocache \
    GOPATH=/box/.gopath \
    GOTMPDIR=/box \
    TMPDIR=/box \
    CGO_ENABLED=0

# Pre-warm a build cache at /opt/gocache: without it, a submission's first
# build compiles the standard library from scratch (~73 s measured at
# 2 CPUs / 512 MB — far beyond any sane compile budget). The compile phase
# copies this cache onto the tmpfs, so user builds only compile the
# submission itself. The warm program imports the packages competitive
# submissions actually use.
RUN mkdir -p /opt/warmsrc && \
    printf 'package main\n\nimport (\n\t"bufio"\n\t"fmt"\n\t"math"\n\t"os"\n\t"sort"\n\t"strconv"\n\t"strings"\n)\n\nfunc main() {\n\tr := bufio.NewReader(os.Stdin)\n\t_ = r\n\tfmt.Println(math.MaxInt64, sort.IntsAreSorted(nil), strconv.Itoa(1), strings.TrimSpace(""))\n}\n' > /opt/warmsrc/main.go && \
    GOCACHE=/opt/gocache GOTMPDIR=/tmp CGO_ENABLED=0 go build -o /tmp/warm /opt/warmsrc/main.go && \
    rm -rf /opt/warmsrc /tmp/warm && \
    chmod -R a+rX /opt/gocache

USER sandbox
WORKDIR /box
