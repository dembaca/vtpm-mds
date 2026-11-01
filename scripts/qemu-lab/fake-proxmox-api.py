#!/usr/bin/env python3
"""Minimal HTTPS Proxmox API stub for reconcile lab verification.

Serves:
  GET /api2/json/access/permissions
  GET /api2/json/cluster/resources?type=vm

VM listing is read from --inventory (JSON file) on every request so the lab
can mutate it between reconciler intervals. Token is not checked; the file
only needs the shape ``user@pve!tokenid=secret``.
"""
from __future__ import annotations

import argparse
import json
import ssl
import subprocess
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
from urllib.parse import parse_qs, urlparse


PERMISSIONS = {
    "data": {
        "/vms": {"VM.Audit": 1},
        "/pool": {"Pool.Audit": 1},
    }
}


def make_self_signed(cert_path: Path, key_path: Path) -> None:
    # IP SAN required: the reconciler verifies TLS against api_url's host.
    check_call = subprocess.check_call
    check_call(
        [
            "openssl",
            "req",
            "-x509",
            "-newkey",
            "rsa:2048",
            "-sha256",
            "-days",
            "7",
            "-nodes",
            "-subj",
            "/CN=fake-proxmox-lab",
            "-addext",
            "subjectAltName=IP:127.0.0.1,DNS:localhost",
            "-keyout",
            str(key_path),
            "-out",
            str(cert_path),
        ],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )


class Handler(BaseHTTPRequestHandler):
    inventory_path: Path = Path("/dev/null")

    def log_message(self, fmt: str, *args) -> None:  # quieter lab logs
        sys.stderr.write("%s - %s\n" % (self.address_string(), fmt % args))

    def _json(self, code: int, body: object) -> None:
        data = json.dumps(body).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self) -> None:  # noqa: N802
        u = urlparse(self.path)
        if u.path == "/api2/json/access/permissions":
            self._json(200, PERMISSIONS)
            return
        if u.path == "/api2/json/cluster/resources":
            qs = parse_qs(u.query)
            # Intentionally empty when type≠vm: the reconciler only asks for
            # qemu guests; other types are not part of this stub.
            if qs.get("type", ["vm"])[0] != "vm":
                self._json(200, {"data": []})
                return
            try:
                data = json.loads(self.inventory_path.read_text())
            except (OSError, json.JSONDecodeError) as exc:
                self._json(500, {"errors": {"inventory": str(exc)}})
                return
            if not isinstance(data, dict) or "data" not in data:
                self._json(500, {"errors": {"inventory": "need {\"data\":[...]}"}})
                return
            self._json(200, data)
            return
        self._json(404, {"errors": {"path": "not found"}})


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument(
        "--listen",
        default="127.0.0.1:8443",
        help="IPv4 host:port to bind (IPv4 only)",
    )
    ap.add_argument("--inventory", required=True, type=Path)
    ap.add_argument("--cert", type=Path, help="TLS cert PEM (created if missing)")
    ap.add_argument("--key", type=Path, help="TLS key PEM (created if missing)")
    ap.add_argument("--tls-dir", type=Path, default=Path("/var/lib/mds-lab/fake-pve"))
    args = ap.parse_args()

    args.tls_dir.mkdir(parents=True, exist_ok=True)
    cert = args.cert or (args.tls_dir / "cert.pem")
    key = args.key or (args.tls_dir / "key.pem")
    if not cert.is_file() or not key.is_file():
        make_self_signed(cert, key)

    host, _, port_s = args.listen.partition(":")
    port = int(port_s or "8443")
    Handler.inventory_path = args.inventory

    ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    ctx.load_cert_chain(certfile=str(cert), keyfile=str(key))
    httpd = HTTPServer((host, port), Handler)
    httpd.socket = ctx.wrap_socket(httpd.socket, server_side=True)
    print(f"fake-proxmox listening https://{host}:{port} inventory={args.inventory}", flush=True)
    print(f"ca_file={cert}", flush=True)
    httpd.serve_forever()


if __name__ == "__main__":
    main()
