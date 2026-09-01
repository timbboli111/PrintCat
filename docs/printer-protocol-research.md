# Printer command-language research and PrintCat support plan

## Scope and terminology

A **protocol** is the printer's command language/byte format (for example, ZPL). A **transport** moves those bytes (for example, USB or RFCOMM Bluetooth Classic). They are independently selectable: `ZPL + TCP`, `ESC/POS + USB`, and `ESC/POS + Windows raw spooler` are all valid combinations. A BLE connection is *not* automatically a printer protocol: the model's GATT service, framing, MTU handling, and SDK/manual decide whether it can carry a raw stream.

“Bluetooth”, “BLE”, “USB”, and “Android direct” below mean *commonly possible on supported models*, not guaranteed. Windows drivers/spoolers are often available but are not required for raw interfaces; an OS print framework/driver may instead render a document and can prevent language-specific controls. Public/vendor documentation does not grant trademark, SDK, font, barcode, or patent rights; ship only independently implemented encoders and follow each vendor's licence.

## Comparison

|Language / family|Typical use and examples|Raw / transports / Windows / Android|Documentation, ownership, PrintCat decision|
|---|---|---|---|
|ESC/POS|Receipt/POS, kiosk, mobile; Epson TM, Bixolon, Citizen and many compatibles|Raw bytes; Classic BT, USB, serial, TCP common; BLE model-specific; driver optional; direct Android common|Vendor-documented, Epson-originated/proprietary family with many compatible implementations. **MVP, medium**: capability-profiled encoder, not “universal ESC/POS”.|
|ESC/X (Epson variants)|Epson POS/receipt|Raw; USB/serial/TCP, drivers; BT/BLE only model-specific; Android SDK/direct on supported models|Vendor docs/SDK; proprietary. **Future, high**: distinct backend; do not route it through ESC/POS blindly.|
|StarPRNT / Star Line Mode|Star receipt/POS/kiosk; TSP, mC-Print, SM series|Raw/SDK command stream; Classic BT, BLE, USB, TCP; Windows driver optional; Android SDK/direct|Star documented SDK ecosystem, proprietary. **MVP, medium**.|
|Star Raster|Star receipt/POS models using raster commands|Raw raster stream; Classic BT/USB/TCP typical, BLE model-specific; Android SDK/direct|Star/vendor documentation; proprietary. **Future, high** because rendering/raster constraints differ.|
|ZPL|Label, industrial and mobile labels; Zebra ZD/ZT/ZQ and compatible devices|Raw ASCII/binary; USB/serial/TCP common, BT/BLE model-specific; Windows driver optional; Android direct/SDK|Zebra public programming guides; proprietary language. **MVP, medium**; validate media, DPI, fonts and firmware.|
|EPL|Legacy Eltron/Zebra label/mobile|Raw; USB/serial/TCP and Classic BT on models; driver optional; Android direct possible|Vendor docs; proprietary. **Future, medium**; legacy compatibility only.|
|CPCL|Mobile receipt/label; Zebra QLn/ZQ/iMZ families|Raw; Classic BT, BLE/USB/TCP by model; driver optional; Android direct/SDK|Zebra public guides; proprietary. **MVP, medium** for mobile templates.|
|TSPL / TSPL2 / compatible|Desktop/industrial label; TSC and compatible manufacturers|Raw; USB/serial/TCP and Classic BT common; BLE by model; driver optional; Android direct possible|TSC manuals; proprietary/vendor-compatible dialects. **Future, medium**; model capability matrix required.|
|DPL|Datamax-O'Neil/Honeywell industrial labels|Raw; USB/serial/TCP; wireless model-specific; driver optional; Android direct if port/SDK exposed|Vendor manuals; proprietary. **Future, high**.|
|SBPL|SATO industrial/desktop labels|Raw; USB/serial/TCP; BT/BLE model-specific; driver optional; Android direct if supported|Vendor manuals; proprietary. **Future, high**.|
|IPL|Intermec/Honeywell industrial/mobile labels|Raw; USB/serial/TCP; Classic BT by model; driver optional; Android direct if exposed|Vendor manuals; proprietary. **Future, high**.|
|ESC/P|Dot-matrix and legacy Epson-compatible printers (not a thermal default)|Raw; serial/USB; Windows drivers common; Android model-dependent|Epson documented family, proprietary origin. **Experimental, high**.|
|ESC/P2|Epson legacy/photo/office variants (not a thermal default)|Raw; USB/Windows typical; Android model-dependent|Vendor docs; proprietary. **Experimental, high**.|
|Generic raster/image streams|Low-cost mobile/receipt and vendor devices|Usually raw bytes, but framing is device-specific; Classic BT/BLE/USB/serial vary; driver/direct Android vary|Often undocumented/proprietary. **Experimental, high**: needs captured specs and per-device adapters.|
|Vendor BLE/BT protocols and driver printing|Portable/vendor-specific units, including SDK-only products|May use GATT chunks, RFCOMM framing, SDK APIs, or driver/Android framework—not necessarily raw|Usually proprietary/licensed. **Experimental, high** unless an official SDK permits implementation; isolate as a vendor backend/transport pair.|

### Manufacturer sources consulted

* Epson’s [ESC/POS reference library](https://download4.epson.biz/sec_pubs/pos/reference_en/escpos/) and [ESC/P/ESC-P2 command reference](https://download4.epson.biz/sec_pubs/ink/escp2/). Epson documentation differentiates POS and legacy printer families.
* Star Micronics’ [CloudPRNT and StarPRNT resources](https://www.starmicronics.com/support/), including model-specific SDK/manual distribution.
* Zebra’s [ZPL programming guide](https://www.zebra.com/us/en/support-downloads/printer-software/by-requester/zpl.html) and [CPCL programming guide](https://www.zebra.com/us/en/support-downloads/printer-software/by-requester/cpcl.html).
* TSC’s [TSPL/TSPL2 programming manual download](https://www.tscprinters.com/EN/support/downloads).
* Honeywell’s [printer language resources](https://automation.honeywell.com/us/en/support/productivity/printer-media/printer-software) and SATO’s [technical support/download portal](https://www.sato-global.com/support/).

Network access to these sources was denied in the build environment, so URLs and conclusions should be revalidated against the exact model/firmware manual before activating a backend.

## Architecture implemented

`PrinterConnection` carries a `protocol`, a separate `transport`, an optional endpoint, and options. A **protocol backend** implements `encode(document, options) -> Uint8Array`. A **transport adapter** implements `send(bytes, connection)`. The orchestration sequence is:

```text
PrintCat document -> selected protocol backend -> Uint8Array -> selected transport adapter -> printer
```

Backends neither connect nor pair devices. Transports neither generate ESC/POS/ZPL/etc. Registering a backend once therefore permits it to be sent through multiple adapters, while each adapter can carry multiple protocol outputs. The catalog is planning metadata only; discovery must identify model, firmware, language emulation, page width/DPI, cutter/status capability, and advertised interfaces before choosing it.

## MVP implementation order

1. Add device profiles plus ESC/POS, StarPRNT, ZPL and CPCL encoders with golden byte tests and model capability gates.
2. Add Classic Bluetooth, USB, TCP and Windows raw-spooler adapters as independent modules. Add BLE only per documented GATT service, flow-control and chunking rules.
3. Add TSPL/EPL and vendor-certified dialects after fixtures/printers are available; add DPL/SBPL/IPL and generic raster only with device-specific test hardware.
4. Treat OS driver and Android print framework as document-rendering fallback transports: no promise of raw command fidelity or status querying.
