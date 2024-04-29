FROM quay.io/fedora/fedora-silverblue:40

COPY ./etc/rpm-ostreed.conf /etc/rpm-ostreed.conf

COPY ./etc/vconsole.conf /etc/vconsole.conf

# NOTE: init initramfs with de-nodeadkeys
# RUN rpm-ostree initramfs-etc --track=/etc/vconsole.conf

RUN rpm-ostree override remove \
    firefox \
    firefox-langpacks

RUN rpm-ostree install \
    libvterm \
    emacs \
    libtool \
    blueman \
    zsh \
    gcc \
    make \
    ripgrep \
    fd-find \
    lld \
    pam_yubico \
    yubikey-personalization-gui \
    strace \
    nmap \
    bat \
    eza \
    restic \
    kernel-headers \
    tldr \
    podman-compose \
    cloc \
    gcc \
    libbpf-devel \
    perf \
    podman-docker \
    nextcloud-client \
    gnome-shell-extension-forge \
    gnome-shell-extension-appindicator \
    glibc-static \
    libstdc++-static \
    binutils

RUN touch /etc/containers/nodocker

RUN rpm-ostree install \
    sway \
    axel \
    pavucontrol \
    grim \
    rofi \
    swaylock \
    NetworkManager-tui \
    waybar \
    swappy \
    mako

RUN rpm-ostree install \
    binutils \
    cmake \
    ImageMagick \
    wireshark \
    krb5-workstation \
    keepassxc

RUN rpm-ostree install -y \
    libvirt \
    libvirt-daemon-kvm \
    qemu-kvm

