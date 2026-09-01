# Go application architecture

The initial Go implementation keeps UI and printing concerns independent so that
Fyne can serve Windows and Android without becoming the source of printer logic.

```text
Fyne UI -> application workflows -> document model -> renderer -> protocol backend -> transport
                                                    \-> platform integration
```

* `internal/ui/fyneapp` owns only the application shell and future editor
  presentation.
* `internal/document` owns durable, platform-independent printable objects and
  physical document coordinates.
* `internal/render` owns contracts for previews, raster output, protocol
  support, and future export. It does not import Fyne.
* `internal/printer` owns protocol, connection, backend, and transport contracts.
  Protocol backends encode; transports send encoded payloads.
* `internal/platform` owns the narrow seam for Windows and Android discovery,
  permissions, and native integrations.
* `internal/config` owns versioned user preferences, while `internal/apperr`
  adds user-safe operation context to errors.

No protocol, printer command encoder, device discovery, Bluetooth implementation,
USB implementation, or editor canvas is present yet. Those will be added behind
the contracts above only after target hardware and capability profiles are
identified.
