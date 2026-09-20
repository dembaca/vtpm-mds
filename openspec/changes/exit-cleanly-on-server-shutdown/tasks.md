# Tasks

## 1. Reproduce the defect

- [x] 1.1 Run the current binary in the foreground on an unprivileged listen address, send it `SIGTERM`, and verify the observed exit status is 1 — record the value, this is the baseline the change has to move

      Ran the pre-fix binary (`go build` of main.go before the change, from this branch's parent commit) with a temp config
      `mds.listen_addr: 127.0.0.1:18080` in `/tmp/.../scratchpad/exit-clean-repro/config.yaml`. Sent `SIGTERM` after the
      listener was up. Observed exit status: **1**. Log output:

          2026/09/20 18:03:38 vtpm-mds version dev
          2026/09/20 18:03:38 Starting server on 127.0.0.1:18080
          2026/09/20 18:03:48 Shutting down server...
          2026/09/20 18:03:48 Server error: http: Server closed

      Confirms the race: `log.Fatalf("Server error: %v", err)` in the listener goroutine won before `Shutdown`'s own
      "Server stopped gracefully" line could be logged, and the process exited 1 on a requested stop.

## 2. Stop treating a requested shutdown as a failure

- [x] 2.1 In `main.go`, make the listener goroutine return without ending the process when `errors.Is(err, http.ErrServerClosed)`, keeping `log.Fatalf` for every other error, and verify `go build ./...` succeeds

      Implemented in `main.go`: the listener goroutine now checks `errors.Is(err, http.ErrServerClosed)`, logs a line and
      returns (letting the main goroutine's `Shutdown` path own the exit) instead of calling `log.Fatalf`; every other
      error still calls `log.Fatalf`. `go build ./...` output: (no output) — build succeeded.

- [x] 2.2 Repeat the task 1.1 foreground run and verify the exit status is now 0 and that `Server stopped gracefully` is logged before the process exits

      Same config, rebuilt binary. Exit status: **0**. Log output:

          2026/09/20 18:04:24 vtpm-mds version dev
          2026/09/20 18:04:24 Starting server on 127.0.0.1:18080
          2026/09/20 18:04:33 Shutting down server...
          2026/09/20 18:04:33 Listener stopped: http: Server closed
          2026/09/20 18:04:33 Server stopped gracefully

- [x] 2.3 Verify a genuine listener error still fails: start the daemon with a `listen_addr` that cannot be bound and verify it logs the error and exits non-zero

      Used `mds.listen_addr: 127.0.0.1:80` (privileged port, agent user has no CAP_NET_BIND_SERVICE). Output:

          2026/09/20 18:04:51 Starting server on 127.0.0.1:80
          2026/09/20 18:04:51 Server error: listen tcp 127.0.0.1:80: bind: permission denied

      Exit status: **1**, confirmed via `$?` immediately after the foreground run.

## 3. Cover it with a test

- [x] 3.1 Add a test in `internal/server` that starts a server on an ephemeral address, calls `Shutdown`, and verifies `Start` returned an error satisfying `errors.Is(err, http.ErrServerClosed)`, and verify the test passes

      Added `internal/server/server_test.go` (`TestStartReturnsErrServerClosedOnShutdown`). `go test ./internal/server/...
      -run TestStartReturnsErrServerClosedOnShutdown -v` output:

          === RUN   TestStartReturnsErrServerClosedOnShutdown
          --- PASS: TestStartReturnsErrServerClosedOnShutdown (0.10s)
          PASS
          ok  	github.com/dembaca/vtpm-mds/internal/server	0.110s

- [x] 3.2 Run `go vet ./...` and `go test ./...` and verify both pass, reporting the output

      `go vet ./...`: no output, exit 0.

      `go test ./...`:

          ?   	github.com/dembaca/vtpm-mds	[no test files]
          ok  	github.com/dembaca/vtpm-mds/attest	0.023s
          ?   	github.com/dembaca/vtpm-mds/cmd/devid-enroll	[no test files]
          ok  	github.com/dembaca/vtpm-mds/identity	0.013s
          ok  	github.com/dembaca/vtpm-mds/imds	0.081s
          ok  	github.com/dembaca/vtpm-mds/internal/config	0.025s
          ok  	github.com/dembaca/vtpm-mds/internal/devid	11.349s
          ok  	github.com/dembaca/vtpm-mds/internal/inventory	0.010s
          ?   	github.com/dembaca/vtpm-mds/internal/proxmox	[no test files]
          ok  	github.com/dembaca/vtpm-mds/internal/server	0.108s

## 4. Verify on the real unit

- [ ] 4.1 Build a git-stamped package with `make deb-local` and verify `dpkg-deb -f` reports the expected version for the tree under test

      Not run: needs root on the lab host; maintainer-run.

- [ ] 4.2 Install it with `dpkg -i --force-confold` and verify `vtpm-mds -version` matches the package version, so the running daemon is known to be this tree — needs root on the lab host, so record it as maintainer-run if it cannot be executed here

      Not run: needs root on the lab host; maintainer-run.

- [ ] 4.3 Verify `systemctl stop vtpm-mds` leaves `inactive (dead)` with result `success` and that `systemctl is-failed` reports `inactive` — needs root, record who ran it

      Not run: needs root on the lab host; maintainer-run.

- [ ] 4.4 Verify `systemctl restart vtpm-mds` ends active and `journalctl -u vtpm-mds` shows no `exit-code` result for the restart — needs root, record who ran it

      Not run: needs root on the lab host; maintainer-run.
