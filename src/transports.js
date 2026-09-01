import { PrinterTransport } from './protocols.js';
const registry = new Map();

/** A transport sends bytes supplied by a protocol backend; it never formats printer commands. */
export function registerTransportAdapter(adapter) {
  if (!adapter || typeof adapter.transport !== 'string' || typeof adapter.send !== 'function') {
    throw new TypeError('A transport adapter requires a transport id and send(bytes, connection) function.');
  }
  if (registry.has(adapter.transport)) throw new Error(`Transport adapter already registered: ${adapter.transport}`);
  registry.set(adapter.transport, Object.freeze({ ...adapter }));
}
export function getTransportAdapter(transport) { return registry.get(transport); }

/** Complete the pipeline without coupling a protocol to a transport. */
export async function sendToPrinter(bytes, connection) {
  if (!connection || typeof connection.transport !== 'string') throw new TypeError('connection.transport is required.');
  const adapter = getTransportAdapter(connection.transport);
  if (!adapter) throw new Error(`No transport adapter registered for transport: ${connection.transport}`);
  if (!(bytes instanceof Uint8Array)) throw new TypeError('Protocol backends must return Uint8Array raw bytes.');
  return adapter.send(bytes, connection);
}

export { PrinterTransport };
