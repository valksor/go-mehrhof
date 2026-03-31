# Tauri Updater Signing Setup

The desktop app uses Tauri's built-in updater to deliver automatic updates. Updates must be cryptographically signed so the app can verify authenticity before applying them.

## Prerequisites

- Rust toolchain installed
- Tauri CLI: `cargo install tauri-cli --version "^2"`
- GitHub repo admin access (for adding secrets)

## Steps

### 1. Generate the signing keypair

```bash
cargo tauri signer generate -w ~/.tauri/kvelmo.key
```

This prompts for a password. You can leave it empty (press Enter) or set one — if you set a password, you'll need it in step 3.

The command outputs:

- **Private key** saved to `~/.tauri/kvelmo.key`
- **Public key** printed to stdout (starts with `dW50cnVzdGVk...` base64 string)

Copy the public key string.

### 2. Set the public key in tauri.conf.json

Edit `web/src-tauri/tauri.conf.json` and replace the empty `pubkey` field:

```json
"plugins": {
  "updater": {
    "endpoints": [
      "https://github.com/valksor/kvelmo/releases/latest/download/latest.json"
    ],
    "pubkey": "PASTE_PUBLIC_KEY_HERE"
  }
}
```

Commit this change.

### 3. Add GitHub secrets

Go to **Settings → Secrets and variables → Actions** in the GitHub repo and add:

| Secret | Value |
|--------|-------|
| `TAURI_SIGNING_PRIVATE_KEY` | Contents of `~/.tauri/kvelmo.key` |
| `TAURI_SIGNING_PRIVATE_KEY_PASSWORD` | The password from step 1 (empty string if none) |

The release workflow (`.github/workflows/release.yml`) already passes these to `cargo tauri build` via environment variables.

### 4. Generate the update manifest

Each release needs a `latest.json` file uploaded as a release asset. This is the file the updater checks.

Format:

```json
{
  "version": "1.2.3",
  "notes": "Release notes here",
  "pub_date": "2026-03-31T12:00:00Z",
  "platforms": {
    "darwin-aarch64": {
      "signature": "SIGNATURE_FROM_BUILD_OUTPUT",
      "url": "https://github.com/valksor/kvelmo/releases/download/v1.2.3/kvelmo_1.2.3_aarch64.app.tar.gz"
    },
    "darwin-x86_64": {
      "signature": "SIGNATURE_FROM_BUILD_OUTPUT",
      "url": "https://github.com/valksor/kvelmo/releases/download/v1.2.3/kvelmo_1.2.3_x64.app.tar.gz"
    },
    "linux-x86_64": {
      "signature": "SIGNATURE_FROM_BUILD_OUTPUT",
      "url": "https://github.com/valksor/kvelmo/releases/download/v1.2.3/kvelmo_1.2.3_amd64.AppImage.tar.gz"
    },
    "windows-x86_64": {
      "signature": "SIGNATURE_FROM_BUILD_OUTPUT",
      "url": "https://github.com/valksor/kvelmo/releases/download/v1.2.3/kvelmo_1.2.3_x64-setup.nsis.zip"
    }
  }
}
```

When `TAURI_SIGNING_PRIVATE_KEY` is set, `cargo tauri build` produces `.sig` files alongside each bundle. The signature content goes into the `signature` field above.

This manifest generation should be automated in the release workflow as a post-build step. Until then, it can be created manually per release.

## Verification

After setup, verify the flow:

1. Build locally: `TAURI_SIGNING_PRIVATE_KEY=$(cat ~/.tauri/kvelmo.key) make desktop-build`
2. Check that `.sig` files are produced alongside the bundles in `web/src-tauri/target/release/bundle/`
3. On next release, the desktop app should detect the update and verify the signature before installing

## Security notes

- The private key (`~/.tauri/kvelmo.key`) should never be committed to the repo
- Add `*.key` to `.gitignore` if not already present
- The public key is safe to commit — it's used for verification only
- Tauri uses Ed25519 signatures (same algorithm family as minisign, but different format)
