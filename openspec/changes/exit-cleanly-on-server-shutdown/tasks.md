# Tasks

## 1. Reproduce the defect

- [ ] 1.1 Run the current binary in the foreground on an unprivileged listen address, send it `SIGTERM`, and verify the observed exit status is 1 — record the value, this is the baseline the change has to move

## 2. Stop treating a requested shutdown as a failure

- [ ] 2.1 In `main.go`, make the listener goroutine return without ending the process when `errors.Is(err, http.ErrServerClosed)`, keeping `log.Fatalf` for every other error, and verify `go build ./...` succeeds
- [ ] 2.2 Repeat the task 1.1 foreground run and verify the exit status is now 0 and that `Server stopped gracefully` is logged before the process exits
- [ ] 2.3 Verify a genuine listener error still fails: start the daemon with a `listen_addr` that cannot be bound and verify it logs the error and exits non-zero

## 3. Cover it with a test

- [ ] 3.1 Add a test in `internal/server` that starts a server on an ephemeral address, calls `Shutdown`, and verifies `Start` returned an error satisfying `errors.Is(err, http.ErrServerClosed)`, and verify the test passes
- [ ] 3.2 Run `go vet ./...` and `go test ./...` and verify both pass, reporting the output

## 4. Verify on the real unit

- [ ] 4.1 Build a git-stamped package with `make deb-local` and verify `dpkg-deb -f` reports the expected version for the tree under test
- [ ] 4.2 Install it with `dpkg -i --force-confold` and verify `vtpm-mds -version` matches the package version, so the running daemon is known to be this tree — needs root on the lab host, so record it as maintainer-run if it cannot be executed here
- [ ] 4.3 Verify `systemctl stop vtpm-mds` leaves `inactive (dead)` with result `success` and that `systemctl is-failed` reports `inactive` — needs root, record who ran it
- [ ] 4.4 Verify `systemctl restart vtpm-mds` ends active and `journalctl -u vtpm-mds` shows no `exit-code` result for the restart — needs root, record who ran it
