# PrintCat

PrintCat is a cross-platform thermal-print application foundation written in Go,
with [Fyne](https://fyne.io/) as its GUI framework. Windows is the first desktop
target and Android is a planned first-class standalone application target.

## Go application foundation

The initial application shell lives in `cmd/printcat`. Its packages deliberately
separate the Fyne UI, application configuration/errors, printer-independent
document model, reusable renderer contract, printer protocol backends,
transports, and platform integrations. The application currently presents only
the shell needed for a future editor; it does not discover or print to hardware.

```sh
go test ./...
go build ./cmd/printcat
go run ./cmd/printcat
```

## Protocol/transport reference

The existing JavaScript prototype remains in `src/` as a small reference for the
protocol/transport split while the production Go architecture is established. A
protocol backend creates bytes for a command language; a transport sends those
already-created bytes. Neither is coupled to the other.

Read [the protocol research and support plan](docs/printer-protocol-research.md) before enabling a printer family.

```js
const bytes = encodeForPrinter(receipt, {
  protocol: PrinterProtocol.ZPL,
  transport: PrinterTransport.TCP,
});
await sendToPrinter(bytes, { protocol: PrinterProtocol.ZPL, transport: PrinterTransport.TCP, endpoint: '192.0.2.10:9100' });
```

The example requires a registered ZPL backend and TCP transport adapter; they are intentionally platform-specific integrations rather than bundled mock implementations.
