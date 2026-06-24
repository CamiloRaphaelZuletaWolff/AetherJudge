# Sandbox image for C++ submissions.
#
# Contains only the compiler toolchain and an unprivileged user. At runtime
# the executor mounts a tmpfs at /box (the only writable path), disables
# networking, drops all capabilities, and makes the rootfs read-only.
FROM alpine:3.22

RUN apk add --no-cache g++ && \
    adduser -D -u 10001 -s /sbin/nologin sandbox

# Precompile <bits/stdc++.h> with the exact flags the executor compiles with
# (see internal/lang). Without this, including the standard competitive-
# programming header costs ~21 s of parse time per submission (measured);
# g++ picks up the adjacent .gch automatically when flags match.
RUN hdr="$(find /usr/include -name stdc++.h -path '*bits*' | head -n 1)" && \
    g++ -O2 -std=c++20 -x c++-header "$hdr" -o "$hdr.gch"

# The compiler writes intermediates to TMPDIR; the rootfs — including /tmp —
# is read-only at runtime, so point it at the writable tmpfs.
ENV TMPDIR=/box

USER sandbox
WORKDIR /box
