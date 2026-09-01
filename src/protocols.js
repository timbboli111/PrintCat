/** Protocol identity is deliberately independent from the connection used to carry it. */
export const PrinterProtocol = Object.freeze({
  ESC_POS: 'esc-pos', ESC_X: 'esc-x', STAR_PRNT: 'star-prnt', STAR_RASTER: 'star-raster',
  ZPL: 'zpl', EPL: 'epl', CPCL: 'cpcl', TSPL: 'tspl', DPL: 'dpl', SBPL: 'sbpl', IPL: 'ipl',
  ESC_P: 'esc-p', ESC_P2: 'esc-p2', RASTER_GENERIC: 'raster-generic', VENDOR_SPECIFIC: 'vendor-specific',
});

export const PrinterTransport = Object.freeze({
  BLUETOOTH_CLASSIC: 'bluetooth-classic', BLE: 'ble', USB: 'usb', SERIAL: 'serial',
  TCP: 'tcp', WINDOWS_SPOOLER: 'windows-spooler', ANDROID_PRINT_FRAMEWORK: 'android-print-framework', OTHER: 'other',
});

/** @typedef {{ protocol: string, transport: string, endpoint?: string, options?: Readonly<Record<string, unknown>> }} PrinterConnection */

const registry = new Map();

/**
 * Register a command encoder. An encoder turns a PrintCat document into the raw
 * bytes accepted by one protocol; it does not open sockets, pair Bluetooth, or
 * invoke an operating-system print UI.
 */
export function registerProtocolBackend(backend) {
  if (!backend || typeof backend.protocol !== 'string' || typeof backend.encode !== 'function') {
    throw new TypeError('A backend requires a protocol id and encode(document, options) function.');
  }
  if (registry.has(backend.protocol)) throw new Error(`Protocol backend already registered: ${backend.protocol}`);
  registry.set(backend.protocol, Object.freeze({ ...backend }));
}

export function getProtocolBackend(protocol) { return registry.get(protocol); }
export function supportedProtocols() { return [...registry.keys()]; }

/** Encode only. The caller selects and owns the transport adapter separately. */
export function encodeForPrinter(document, connection) {
  if (!connection || typeof connection.protocol !== 'string') throw new TypeError('connection.protocol is required.');
  const backend = getProtocolBackend(connection.protocol);
  if (!backend) throw new Error(`No backend registered for protocol: ${connection.protocol}`);
  return backend.encode(document, connection.options ?? {});
}
