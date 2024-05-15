FROM quay.io/fedora/fedora-silverblue:40

COPY ./etc/rpm-ostreed.conf /etc/rpm-ostreed.conf

COPY ./etc/vconsole.conf /etc/vconsole.conf

# NOTE: init initramfs with de-nodeadkeys
# RUN rpm-ostree initramfs-etc --track=/etc/vconsole.conf

RUN rpm-ostree override remove \
    firefox \
    firefox-langpacks

# https://github.com/fedora-silverblue/issue-tracker/issues/430
RUN mkdir -p /etc/alternatives && mkdir -p /var/lib/alternatives

RUN rpm-ostree install \
    emacs \
    blueman \
    zsh \
    make \
    ripgrep \
    fd-find \
    pam_yubico \
    yubikey-personalization-gui \
    strace \
    nmap \
    bat \
    eza \
    fzf \
    restic \
    htop \
    tldr \
    podman-compose \
    cloc \
    vim \
    tig \
    perf \
    podman-docker \
    nextcloud-client \
    krb5-workstation \
    keepassxc \
    gnome-shell-extension-forge \
    gnome-shell-extension-appindicator

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
    kernel-headers \
    lld \
    gcc \
    glibc-static \
    libbpf-devel \
    libstdc++-static \
    libvterm \
    binutils \
    cmake \
    ImageMagick \
    wireshark \
    libtool

RUN rpm-ostree install -y \
    libvirt \
    libvirt-daemon-kvm \
    qemu-kvm \
    fastboot

COPY ./etc/yum.repos.d/_copr:copr.fedorainfracloud.org:packit:os-observability-redhat-opentelemetry-collector-63.repo /etc/yum.repos.d/_copr:copr.fedorainfracloud.org:packit:os-observability-redhat-opentelemetry-collector-63.repo

RUN rpm-ostree install -y opentelemetry-collector-0.99.0-1.20240514165826437302.pr63.7.g984bf2a.fc40.x86_64
