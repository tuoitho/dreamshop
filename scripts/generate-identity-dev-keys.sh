#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
key_dir="$repo_root/deployments/secrets/identity"
private_path="$key_dir/private.pem"
public_path="$key_dir/public.pem"

if [[ (-e "$private_path") && ! (-e "$public_path") ]] || [[ (! -e "$private_path") && -e "$public_path" ]]; then
  echo "Identity key pair is incomplete; remove the remaining file deliberately and rerun." >&2
  exit 1
fi

if [[ -e "$private_path" && -e "$public_path" ]]; then
  echo "Identity development key pair already exists."
  exit 0
fi

mkdir -p "$key_dir"
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$private_path" >/dev/null 2>&1
openssl pkey -in "$private_path" -pubout -out "$public_path" >/dev/null 2>&1

echo "Created untracked Identity development key pair."
