package sandbox

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

// execSpec describes one process to run inside an existing container.
type execSpec struct {
	cmd         []string
	stdin       string
	timeout     time.Duration
	outputLimit int64 // bytes captured per stream
}

// execResult is the observed outcome of one exec.
type execResult struct {
	exitCode int
	timedOut bool
	stdout   string
	stderr   string
	duration time.Duration
}

// drainGrace bounds how long we wait for the attach stream to close after
// killing the container on a timeout.
const drainGrace = 5 * time.Second

// exec runs spec.cmd inside the container, feeding spec.stdin and capturing
// capped stdout/stderr. On wall-clock timeout the whole container is killed
// (the submission is over either way) and timedOut is set.
func (e *Engine) exec(ctx context.Context, containerID string, spec execSpec) (execResult, error) {
	execCtx, cancel := context.WithTimeout(ctx, spec.timeout)
	defer cancel()

	created, err := e.cli.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		Cmd:          spec.cmd,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return execResult{}, fmt.Errorf("sandbox: create exec %v: %w", spec.cmd[0], err)
	}

	attach, err := e.cli.ContainerExecAttach(execCtx, created.ID, container.ExecStartOptions{})
	if err != nil {
		return execResult{}, fmt.Errorf("sandbox: attach exec %v: %w", spec.cmd[0], err)
	}
	defer attach.Close()

	start := time.Now()

	// Feed stdin and signal EOF. Write errors are expected when the process
	// exits without reading everything (or is killed) — log only.
	go func() {
		if spec.stdin != "" {
			if _, err := io.Copy(attach.Conn, strings.NewReader(spec.stdin)); err != nil {
				e.log.Debug("write exec stdin", "error", err)
			}
		}
		if err := attach.CloseWrite(); err != nil {
			e.log.Debug("close exec stdin", "error", err)
		}
	}()

	stdout := newCappedBuffer(spec.outputLimit)
	stderr := newCappedBuffer(spec.outputLimit)

	copyDone := make(chan error, 1)
	go func() {
		_, copyErr := stdcopy.StdCopy(stdout, stderr, attach.Reader)
		copyDone <- copyErr
	}()

	timedOut := false
	select {
	case copyErr := <-copyDone:
		if copyErr != nil {
			return execResult{}, fmt.Errorf("sandbox: read exec output: %w", copyErr)
		}
	case <-execCtx.Done():
		timedOut = true
		e.killContainer(containerID)
		// Closing the attach unblocks StdCopy even if the daemon is slow to
		// tear the stream down after the kill.
		select {
		case <-copyDone:
		case <-time.After(drainGrace):
			attach.Close()
			<-copyDone
		}
	}
	duration := time.Since(start)

	// Fresh context: the exec context is likely expired by now.
	inspectCtx, inspectCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer inspectCancel()

	inspect, err := e.cli.ContainerExecInspect(inspectCtx, created.ID)
	if err != nil {
		if timedOut {
			// The kill can tear state down before inspect lands; the verdict
			// is TLE regardless of exit code.
			return execResult{exitCode: -1, timedOut: true, stdout: stdout.String(), stderr: stderr.String(), duration: duration}, nil
		}
		return execResult{}, fmt.Errorf("sandbox: inspect exec %v: %w", spec.cmd[0], err)
	}

	return execResult{
		exitCode: inspect.ExitCode,
		timedOut: timedOut,
		stdout:   stdout.String(),
		stderr:   stderr.String(),
		duration: duration,
	}, nil
}

// cappedBuffer keeps at most limit bytes and silently discards the rest,
// always reporting full writes so the producing stream keeps draining —
// an output-flooding submission must not block or balloon the executor.
type cappedBuffer struct {
	buf       strings.Builder
	limit     int64
	truncated bool
}

func newCappedBuffer(limit int64) *cappedBuffer {
	return &cappedBuffer{limit: limit}
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	remaining := b.limit - int64(b.buf.Len())
	switch {
	case remaining <= 0:
		b.truncated = true
	case int64(len(p)) > remaining:
		b.buf.Write(p[:remaining])
		b.truncated = true
	default:
		b.buf.Write(p)
	}
	return len(p), nil
}

func (b *cappedBuffer) String() string { return b.buf.String() }
