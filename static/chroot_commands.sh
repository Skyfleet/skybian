#!/bin/bash
set -e
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

# function to log messages as info
function info() {
    printf '\033[0;32m[ Info ]\033[0m %s\n' "${1}"
}

# Chroot extra commands. This allow us to pass some extra commands
# inside the chroot, for example to install/remove additional pkgs
# or execute some bash commands

# change root password to the default of: skybian
info "Setting default root password to 'skybian'..."
printf "skybian\nskybian\n" | passwd root

# by default update the es_US locales
info "Re-generating the locales info for en_US.UTF-8..."
locale-gen en_US.UTF-8

# apt-get commands (install/remove/purge)
# modify and un-comment
info "Updating your system via APT"
export DEBIAN_FRONTEND=noninteractive
set +e
apt-get -y update && apt-get -y install software-properties-common
set -e
#add repository signing key
apt-key add /root/KEY.asc
#sync package database
#apt -y update
#Install the package
dpkg --force-overwrite -i /tmp/*.deb
#Synth said he wants golang in the images ; installing golang
apt -y install golang
#apt-get -y install [your_pkgs_here]
#apt-get -y remove --purge [your_pkgs_here]
# keep this on the very end of this block
info "Cleaning the APT cache to make a smaller image"
apt-get clean

# forge a time on the system to avoid fs dates are in the future
info "Setting the chroot clock to now to avoid bugs with the date..."
/sbin/fake-hwclock save force

# Mkdir
mkdir -p /etc/skywire-hypervisor || 0
mkdir -p /var/skywire-hypervisor || 0

# Enable systemd units.
info "Enabling systemd units..."
systemctl enable skybian-firstrun.service || exit 1

#reset /etc/apt/sources.list which was changed outsidethe chroot
mv /etc/apt/sources.list.save /etc/apt/sources.list
mv /etc/apt/sources.list.d/armbian.list.save /etc/apt/sources.list.d/armbian.list

#TESTING REPOSITORY ; SHOULD BE CHANGED TO ONE IN SKYCOINPROJECT
add-apt-repository 'deb http://skyfleet.github.io/sky-update sid main'
