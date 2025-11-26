# Test Fixtures for Integration Tests

This directory contains captured output from BSD ports Makefiles used for platform-agnostic testing of the pkg library.

## Purpose

The pkg library needs to query port Makefiles using `make -V` commands, which requires:
- BSD `make` (or `bmake`)
- A valid ports tree (FreeBSD `/usr/ports` or DragonFly `/usr/dports`)
- Port Makefiles with BSD-specific variables

To enable development and testing on Linux, we capture real Makefile output as fixtures that can be used in tests on any platform.

## Fixture Format

Each fixture file in `fixtures/` contains the raw output from running:

```bash
make -C /usr/ports/{category}/{port} \
  -V PKGNAME \
  -V PKGVERSION \
  -V PKGFILE \
  -V FETCH_DEPENDS \
  -V EXTRACT_DEPENDS \
  -V PATCH_DEPENDS \
  -V BUILD_DEPENDS \
  -V LIB_DEPENDS \
  -V RUN_DEPENDS \
  -V IGNORE
```

The output is 10 lines (one per variable), in the order listed above. Empty lines indicate the variable is empty/unset.

### Example: `editors__vim.txt`

```
vim-9.0.1234
9.0.1234
vim-9.0.1234.pkg

devel/gettext-runtime:patch
devel/gettext-runtime:patch
devel/gmake:build devel/gettext-tools:build
/usr/local/lib/libintl.so:devel/gettext-runtime
shells/bash:run

```

### Naming Convention

Fixtures are named using the pattern: `{category}__{port}.txt`
- `editors__vim.txt` - editors/vim
- `devel__git.txt` - devel/git
- `lang__python39.txt` - lang/python39
- `editors__vim@python39.txt` - editors/vim with python39 flavor

The double underscore (`__`) separator allows safe parsing of category/port names.

## Updating Fixtures

### On FreeBSD/DragonFly

Run the provided script from the project root:

```bash
./scripts/capture-fixtures.sh
```

This will regenerate all fixtures from your local ports tree.

### Manual Capture

To capture a single port manually:

```bash
cd /usr/ports/editors/vim  # or /usr/dports on DragonFly
make -V PKGNAME -V PKGVERSION -V PKGFILE \
     -V FETCH_DEPENDS -V EXTRACT_DEPENDS -V PATCH_DEPENDS \
     -V BUILD_DEPENDS -V LIB_DEPENDS -V RUN_DEPENDS -V IGNORE \
  > /path/to/go-synth/pkg/testdata/fixtures/editors__vim.txt
```

### For Flavored Ports

Add `FLAVOR=flavorname` before the `-V` options:

```bash
cd /usr/ports/editors/vim
make FLAVOR=python39 -V PKGNAME -V PKGVERSION ... \
  > /path/to/go-synth/pkg/testdata/fixtures/editors__vim@python39.txt
```

## Using Fixtures in Tests

Tests use the `testFixtureQuerier` to load fixtures instead of calling `make`:

```go
func TestSomething(t *testing.T) {
    // Configure to use fixtures
    oldQuerier := portsQuerier
    portsQuerier = newTestFixtureQuerier(map[string]string{
        "editors/vim":   "testdata/fixtures/editors__vim.txt",
        "devel/git":     "testdata/fixtures/devel__git.txt",
    })
    defer func() { portsQuerier = oldQuerier }()
    
    // Now ParsePortList will use fixtures
    cfg := &config.Config{DPortsPath: "/usr/ports"}
    packages, err := ParsePortList([]string{"editors/vim"}, cfg, ...)
    // ...
}
```

## Fixture Maintenance

- **Frequency**: Update when port Makefile format changes or when adding tests for new ports
- **Version**: Fixtures represent a snapshot in time; exact version numbers may differ
- **Coverage**: Add fixtures for ports commonly used in tests (vim, git, python, gmake, etc.)

## Available Fixtures

### Basic Dependencies
| Fixture | Port | Purpose |
|---------|------|---------|
| `devel__gmake.txt` | devel/gmake | Build tool dependency |
| `devel__pkgconf.txt` | devel/pkgconf | Configuration helper |
| `devel__gettext-runtime.txt` | devel/gettext-runtime | Internationalization runtime |
| `devel__gettext-tools.txt` | devel/gettext-tools | Internationalization build tools |
| `devel__libffi.txt` | devel/libffi | Foreign function interface library |
| `devel__libiconv.txt` | devel/libiconv | Character encoding conversion |

### Network Libraries
| Fixture | Port | Purpose |
|---------|------|---------|
| `ftp__curl.txt` | ftp/curl | HTTP/FTP client library |
| `textproc__expat.txt` | textproc/expat | XML parser |
| `security__ca_root_nss.txt` | security/ca_root_nss | SSL certificate bundle |
| `dns__libidn2.txt` | dns/libidn2 | Internationalized domain names |

### Language Runtimes
| Fixture | Port | Purpose |
|---------|------|---------|
| `lang__python39.txt` | lang/python39 | Python 3.9 runtime |
| `lang__perl5.txt` | lang/perl5 | Perl 5 runtime |
| `lang__ruby31.txt` | lang/ruby31 | Ruby 3.1 runtime |

### Basic Applications
| Fixture | Port | Purpose |
|---------|------|---------|
| `editors__vim.txt` | editors/vim | Text editor (moderate deps) |
| `editors__vim@python39.txt` | editors/vim@python39 | Vim with Python flavor |
| `devel__git.txt` | devel/git | Version control (many deps) |
| `shells__bash.txt` | shells/bash | Bash shell |

### Complex Applications (Deep Dependencies)
| Fixture | Port | Purpose |
|---------|------|---------|
| `www__firefox.txt` | www/firefox | Web browser (100+ deps) |
| `www__chromium.txt` | www/chromium | Web browser (200+ deps) |
| `multimedia__ffmpeg.txt` | multimedia/ffmpeg | Media encoder (50+ deps) |
| `multimedia__gstreamer1.txt` | multimedia/gstreamer1 | Media framework |

### X11 and Graphics
| Fixture | Port | Purpose |
|---------|------|---------|
| `x11__xorg-server.txt` | x11/xorg-server | X11 display server |
| `x11__xorg-libs.txt` | x11/xorg-libs | X11 libraries meta-port |
| `x11__libX11.txt` | x11/libX11 | Core X11 library |
| `x11__libxcb.txt` | x11/libxcb | X C Binding |
| `graphics__mesa-libs.txt` | graphics/mesa-libs | OpenGL implementation |
| `graphics__cairo.txt` | graphics/cairo | 2D graphics library |

### Desktop Environments
| Fixture | Port | Purpose |
|---------|------|---------|
| `x11-wm__i3.txt` | x11-wm/i3 | Tiling window manager |
| `x11__gnome-shell.txt` | x11/gnome-shell | GNOME desktop |

### Meta Ports
| Fixture | Port | Purpose |
|---------|------|---------|
| `x11__xorg.txt` | x11/xorg | Complete X11 system |
| `x11__gnome.txt` or `x11__meta-gnome.txt` | x11/gnome | GNOME desktop meta-port |
| `x11__kde5.txt` | x11/kde5 | KDE5 desktop meta-port |

**Note**: Complex fixtures are optional but recommended for thorough testing. They test deep dependency resolution, large graph handling, and real-world scenarios.

## Testing on Linux vs BSD

### Linux (Development)
- Tests use fixtures automatically
- No ports tree required
- Fast iteration cycle
- Coverage: ~90% of code paths

### BSD (Integration)
- Can use real ports tree or fixtures
- Tests with build tag `//go:build freebsd || dragonfly`
- Validates actual port parsing
- Coverage: 100% including platform-specific code

## See Also

- `TESTING.md` - Overall testing strategy
- `PHASE_1_DEVELOPER_GUIDE.md` - Using the pkg library
- `scripts/capture-fixtures.sh` - Fixture generation script
