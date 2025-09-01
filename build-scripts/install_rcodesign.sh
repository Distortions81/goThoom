# nuke Ubuntu's old Rust if you installed it
sudo apt remove -y rustc cargo

# install rustup + latest stable toolchain
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
source "$HOME/.cargo/env"
rustup default stable
rustc --version
cargo --version

# smartcard (YubiKey) support needs these libs first
sudo apt update
sudo apt install -y libpcsclite-dev pcscd

# now install rcodesign
cargo install apple-codesign
# or with smartcard support:
cargo install --features smartcard apple-codesign