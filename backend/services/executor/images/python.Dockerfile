# Sandbox image for Python submissions.
#
# At runtime the executor mounts a tmpfs at /box (the only writable path),
# disables networking, drops all capabilities, and makes the rootfs read-only.
FROM python:3.12-alpine

RUN adduser -D -u 10001 -s /sbin/nologin sandbox

# Never buffer program output: verdict judging reads exact stdout.
# TMPDIR points at the writable tmpfs; the rootfs (including /tmp) is
# read-only at runtime.
ENV PYTHONUNBUFFERED=1 \
    TMPDIR=/box

USER sandbox
WORKDIR /box
