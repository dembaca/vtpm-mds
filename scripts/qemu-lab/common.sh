#!/usr/bin/env bash
# Shared helpers for the QEMU lab scripts. Source it; it has no side effects.

LAB_INSTALL_HINT="scripts/qemu-lab/cloud-install.sh"

# require_tools TOOL...: fail naming every missing tool, before any work is done.
require_tools() {
  local missing=() tool
  for tool in "$@"; do
    command -v "$tool" >/dev/null 2>&1 || missing+=("$tool")
  done
  if ((${#missing[@]})); then
    echo "ERROR: missing tool(s): ${missing[*]}" >&2
    echo "Run ${LAB_INSTALL_HINT} to install what the lab needs. Nothing was changed." >&2
    return 1
  fi
}

# require_one_of TOOL...: at least one of the alternatives must exist.
require_one_of() {
  local tool
  for tool in "$@"; do
    command -v "$tool" >/dev/null 2>&1 && return 0
  done
  echo "ERROR: none of these tools found: $*" >&2
  echo "Run ${LAB_INSTALL_HINT} to install what the lab needs. Nothing was changed." >&2
  return 1
}

# discover_ovmf: set OVMF_CODE and OVMF_VARS_TEMPLATE. Explicit values of either
# variable win; whatever is unset is taken from the first directory below that
# holds a readable code+vars pair. On failure every path tried is reported.
OVMF_SEARCH_DIRS=(/usr/share/OVMF /usr/share/pve-edk2-firmware /usr/share/edk2/ovmf)

discover_ovmf() {
  local dir pair code vars tried=()
  if [[ -z "${OVMF_CODE:-}" || -z "${OVMF_VARS_TEMPLATE:-}" ]]; then
    for dir in "${OVMF_SEARCH_DIRS[@]}"; do
      for pair in "OVMF_CODE_4M.fd:OVMF_VARS_4M.fd" "OVMF_CODE.fd:OVMF_VARS.fd"; do
        code="${dir}/${pair%%:*}"
        vars="${dir}/${pair##*:}"
        tried+=("$code" "$vars")
        if [[ -r "$code" && -r "$vars" ]]; then
          OVMF_CODE="${OVMF_CODE:-$code}"
          OVMF_VARS_TEMPLATE="${OVMF_VARS_TEMPLATE:-$vars}"
          break 2
        fi
      done
    done
  fi
  if [[ -z "${OVMF_CODE:-}" || -z "${OVMF_VARS_TEMPLATE:-}" ]]; then
    {
      echo "ERROR: no UEFI firmware (OVMF) found. Tried:"
      printf '  %s\n' "${tried[@]}"
      echo "Install 'ovmf', or set OVMF_CODE and OVMF_VARS_TEMPLATE."
    } >&2
    return 1
  fi
  local f
  for f in "$OVMF_CODE" "$OVMF_VARS_TEMPLATE"; do
    if [[ ! -r "$f" ]]; then
      echo "ERROR: firmware file not readable: $f" >&2
      return 1
    fi
  done
  export OVMF_CODE OVMF_VARS_TEMPLATE
}

# ensure_no_conflicting_imds: refuse when HOST_IP / IMDS_IP already sit on a
# non-lab interface, or a route for the lab prefix leaves via another device.
# Uses MDS_LAB_BRIDGE / MDS_LAB_HOST_IP / MDS_LAB_IMDS_IP / MDS_LAB_PREFIX
# (same defaults as setup-host.sh). Moved here so start-reconcile-lab-host.sh
# can share the guard without claiming the bridge itself.
ensure_no_conflicting_imds() {
  local bridge="${MDS_LAB_BRIDGE:-br-imds}"
  local host_ip="${MDS_LAB_HOST_IP:-169.254.169.1}"
  local imds_ip="${MDS_LAB_IMDS_IP:-169.254.169.254}"
  local prefix="${MDS_LAB_PREFIX:-16}"
  local conflict="" ip on via
  for ip in "$host_ip" "$imds_ip"; do
    on="$(ip -o -4 addr show | awk -v a="$ip" '$4 ~ "^"a"/" {print $2}' \
          | grep -vx "$bridge" | head -1 || true)"
    [[ -n "$on" ]] && conflict="${conflict}  ${ip} is already on ${on}"$'\n'
  done
  via="$(ip -o -4 route show "${host_ip%.*.*}.0.0/${prefix}" 2>/dev/null \
         | awk '{for(i=1;i<NF;i++) if($i=="dev") print $(i+1)}' \
         | grep -vx "$bridge" | head -1 || true)"
  [[ -n "$via" ]] && conflict="${conflict}  a route for the lab prefix already goes via ${via}"$'\n'

  if [[ -n "$conflict" ]]; then
    cat >&2 <<EOF
ERROR: this host already runs a metadata setup that the QEMU lab would collide
with:
${conflict}
The lab is for the cloud lab host. On a Proxmox host, test against a real guest
and the installed package instead — see scripts/qemu-lab/README.md.

Nothing was changed. To run the lab here anyway, point it elsewhere with
MDS_LAB_BRIDGE, MDS_LAB_HOST_IP, MDS_LAB_IMDS_IP and MDS_LAB_PREFIX, and note
that the guest cloud-init config and devid-enroll's default URL carry the
addresses too.
EOF
    return 1
  fi
}

# refuse_non_lab_host: abort before gen-lab-pki / SPIRE bind can touch a
# production Proxmox host or an existing site DevID CA. Callers that already
# set MDS_LAB_ETC_DIR to a scratch directory pass the CA check.
refuse_non_lab_host() {
  if [[ -e /etc/pve ]]; then
    echo "ERROR: /etc/pve present — refuse QEMU lab helpers on a Proxmox host." >&2
    echo "Nothing was changed." >&2
    return 1
  fi
  local pkg_etc="/etc/vtpm-mds"
  if [[ -z "${MDS_LAB_ETC_DIR:-}" && -f "${pkg_etc}/devid-ca.pem" ]]; then
    echo "ERROR: ${pkg_etc}/devid-ca.pem exists and MDS_LAB_ETC_DIR is unset." >&2
    echo "Set MDS_LAB_ETC_DIR to a scratch directory so gen-lab-pki does not" >&2
    echo "overwrite a site DevID CA. Nothing was changed." >&2
    return 1
  fi
  ensure_no_conflicting_imds
}
