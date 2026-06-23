# Desktop Image

Bootc/ostree image based on Fedora Silverblue 44, built and signed via Tekton.

## Setup

### Option A: Fresh Install

1. Install [Fedora Silverblue](https://fedoraproject.org/silverblue/) on the machine.

2. Rebase to the desktop image (unverified, since the policy isn't installed yet):

   ```sh
   rpm-ostree rebase ostree-unverified-registry:ghcr.io/frzifus/desktop:latest
   systemctl reboot
   ```

3. Switch to verified upgrades (the reboot above installed the cosign public
   key and `policy.json`):

   ```sh
   rpm-ostree rebase ostree-image-signed:docker://ghcr.io/frzifus/desktop:latest
   systemctl reboot
   ```

### Option B: Migration from an Existing Unverified Build

If you are already running the desktop image via
`ostree-unverified-registry`, skip step 1 and go straight to step 2:

2. Rebase to the desktop image (unverified) to pick up the latest image
   containing the signature policy and cosign public key:

   ```sh
   rpm-ostree rebase ostree-unverified-registry:ghcr.io/frzifus/desktop:latest
   systemctl reboot
   ```

3. Switch to verified upgrades:

   ```sh
   rpm-ostree rebase ostree-image-signed:docker://ghcr.io/frzifus/desktop:latest
   systemctl reboot
   ```

---

After step 3, `rpm-ostree upgrade` will reject any image that isn't signed
with the matching cosign key.

> **Note:** `bootc upgrade` does not enforce `policy.json` yet
> ([bootc#528](https://github.com/bootc-dev/bootc/issues/528)).
> Use `rpm-ostree upgrade` for verified upgrades.

## Verifying Manually

```sh
cosign verify --key /etc/pki/containers/cosign.pub ghcr.io/frzifus/desktop:latest
```

## Key Setup (One-Time)

Generate the key pair and populate the placeholders in this repo:

```sh
cosign generate-key-pair
# Replace images/desktop/etc/pki/containers/cosign.pub
# Populate & SOPS-encrypt clusters/homelab/apps/cicd-builds/secret-cosign-key.yaml
```

> **Important:** The Tekton tasks pin cosign v2.4.1. Cosign v3 changed the
> bundle format which breaks rpm-ostree signature verification
> ([rpm-ostree#5509](https://github.com/coreos/rpm-ostree/issues/5509)).
> Do not upgrade the signing step past v2 until this is resolved.
