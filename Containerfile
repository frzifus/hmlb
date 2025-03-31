FROM quay.io/fedora/fedora-silverblue:41

# NOTE: Just to test RHEL9 version
# FROM registry.redhat.io/rhel9/rhel-bootc:9.5

COPY ./etc/rpm-ostreed.conf /etc/rpm-ostreed.conf

COPY ./etc/vconsole.conf /etc/vconsole.conf

COPY ./etc/yum.repos.d/tailscale.repo /etc/yum.repos.d/tailscale.repo

# NOTE: init initramfs with de-nodeadkeys
# RUN rpm-ostree initramfs-etc --track=/etc/vconsole.conf

# RUN rpm-ostree override remove \
#     firefox \
#     firefox-langpacks

# https://github.com/fedora-silverblue/issue-tracker/issues/430
RUN mkdir -p /etc/alternatives && mkdir -p /var/lib/alternatives

RUN dnf install -y https://kojipkgs.fedoraproject.org//packages/azure-cli/2.68.0/1.fc42/noarch/azure-cli-2.68.0-1.fc42.noarch.rpm

RUN dnf install -y tailscale

RUN dnf install -y \
    emacs \
    gnome-terminal \
    age \
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

RUN dnf install -y \
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

RUN dnf install -y \
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
    openssl \
    libtool \
    yamllint

RUN dnf install -y \
    @virtualization \
    libvirt \
    libvirt-daemon-kvm \
    qemu-kvm \
    libvirt-daemon \
    libvirt-client \
    virt-manager \
    android-tools \
    fastboot

RUN dnf install -y \
    mesa-libGL-devel \
    mesa-libGLES-devel \
    libXrandr-devel \
    libXcursor-devel \
    libXinerama-devel \
    libXi-devel \
    libXxf86vm-devel \
    alsa-lib-devel \
    pkg-config \
    distrobox
