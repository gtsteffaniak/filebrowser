# Archive test fixtures

## RAR (`*.rar`)

From the [libarchive](https://github.com/libarchive/libarchive) test suite
(`libarchive/test/test_read_format_*.rar.uu`, uudecoded), used under libarchive's
BSD 2-clause license. RAR archives can only be created with the proprietary `rar` tool.

| File | Source fixture |
| --- | --- |
| `rar.rar` | `test_read_format_rar.rar.uu` (RAR4: files, dir, empty dir, symlink) |
| `rar4_encrypted.rar` | `test_read_format_rar4_encrypted.rar.uu` |
| `rar5_encrypted.rar` | `test_read_format_rar5_encrypted.rar.uu` |
| `rar5_encrypted_filenames.rar` | `test_read_format_rar5_encrypted_filenames.rar.uu` |
| `rar5_hardlink.rar` | `test_read_format_rar5_hardlink.rar.uu` |
| `rar5_symlink.rar` | `test_read_format_rar5_symlink.rar.uu` |
| `rar5_solid.rar` | `test_read_format_rar5_solid.rar.uu` |
| `rar5_multiarchive_solid.part0[1-4].rar` | `test_read_format_rar5_multiarchive_solid.part0[1-4].rar.uu` |

In the `*_encrypted.rar` fixtures, `a.txt` and `c.txt` are not encrypted, `b.txt` uses the
password `password`, and `d.txt` uses `password2`. In `rar5_encrypted_filenames.rar`, every
file and the headers use `password`.

## 7z (`*.7z`)

Created with 7-Zip 26.00 from small text files (`dir/sub/file.txt`, `top.txt`, `link.txt -> top.txt`):

| File | Command |
| --- | --- |
| `basic.7z` | `7z a -snl basic.7z dir top.txt link.txt` |
| `encrypted.7z` | `7z a -ppassword encrypted.7z dir top.txt` |
| `encrypted_headers.7z` | `7z a -ppassword -mhe=on encrypted_headers.7z dir top.txt` |
| `encrypted_store.7z` | `7z a -ppassword -mx0 encrypted_store.7z top.txt` |
| `traversal.7z` | `7z a -mhc=off -mx0` of `xx/evil.txt`, then the name patched to `../evil.txt` and the header CRCs recomputed |
