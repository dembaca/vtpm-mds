# Tasks

## 1. Reproduce the defect

- [ ] 1.1 Add a test that drives `GET /latest/meta-data/local-ipv4` with `RemoteAddr` set to `[fe80::1]:5000` and verify it currently returns `200` with the body `[fe80` — record that output as the baseline
- [ ] 1.2 Verify the same value reaches the identity document by asserting `privateIp` is `[fe80` for that peer

## 2. Parse the peer address correctly

- [ ] 2.1 Add a peer-address helper using `net.SplitHostPort` with a bare-host fallback and `net.ParseIP`, and verify unit tests cover `10.0.0.5:1234`, `[fe80::1]:5000`, `[::ffff:10.0.0.5]:5000` and a bare `10.0.0.5`
- [ ] 2.2 Rewrite `getLocalIP` in `imds/handlers.go` on top of that helper and verify the existing `10.0.0.5` scenario still returns `200` with the body `10.0.0.5`
- [ ] 2.3 Rewrite the peer-address half of `getClientIP` in `imds/handlers.go` and the copy in `identity/handlers.go` to use the same helper, and verify no textual `strings.Split(..., ":")` on a remote address remains, with `grep -n 'RemoteAddr' imds identity`

## 3. Serve the corrected values

- [ ] 3.1 Make `HandleLocalIPv4` respond `404` with an empty body and `Content-Type: text/plain` when the peer has no IPv4 address, and verify the task 1.1 test now expects and gets `404`
- [ ] 3.2 Make the identity document emit `privateIp` as the empty string for such a peer, and verify the document still carries exactly its ten keys
- [ ] 3.3 Verify an IPv4-mapped peer `[::ffff:10.0.0.5]:5000` is served `10.0.0.5` on both `local-ipv4` and `privateIp`

## 4. Confirm nothing else moved

- [ ] 4.1 Verify the metadata index still lists `local-ipv4` and that `public-ipv4` still answers `404`, so the index and the neighbouring key are untouched
- [ ] 4.2 Run `go vet ./...` and `go test ./...` and verify both pass, reporting the output
