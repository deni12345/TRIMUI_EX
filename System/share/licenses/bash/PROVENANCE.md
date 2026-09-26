# GNU Bash for AArch64

Package: Debian `bash-static_5.1-2+deb11u1_arm64.deb`.
Downloaded from https://deb.debian.org/debian/pool/main/b/bash/ over HTTPS.
Package SHA-256: `df79bca8223cba174e338dfb59d3ccfd2110c85673a12d88cf6d56593aa13f11`.
`System/bin/bash.real` SHA-256: `5dc3de6983f3ed54837b60d70ed73e9aa9849692d0f91a8e15446068b522bdfc`.

`System/bin/bash` is a shell wrapper that initializes `SHELL` before invoking
the binary. This avoids a reproduced startup crash in the stock menu environment,
which lacks `SHELL`; the internal `/bin/bash` entry point uses the same safeguard.

This is the unmodified `/bin/bash-static` from that package: static AArch64 ELF,
minimum kernel 3.7.0. It does not replace stock BusyBox or `/bin/sh`.
The exact upstream source, Debian patches/build rules and `.dsc` are in
`sources/bash/` at the repository root and in the GitHub source archive for this
tag. Copyright notices and GPLv3 are adjacent to this file.
