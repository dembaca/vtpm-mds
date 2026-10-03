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
