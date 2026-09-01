import test from 'node:test';
import assert from 'node:assert/strict';
import { PrinterProtocol, PrinterTransport, registerProtocolBackend, registerTransportAdapter, encodeForPrinter, sendToPrinter } from '../src/index.js';

test('one protocol backend can be used with multiple transports', async () => {
  const protocol = 'test-zpl'; const sent = [];
  registerProtocolBackend({ protocol, encode: d => new TextEncoder().encode(`^XA^FD${d.text}^XZ`) });
  registerTransportAdapter({ transport: 'test-usb', send: (bytes, c) => sent.push(['usb', bytes, c.endpoint]) });
  registerTransportAdapter({ transport: 'test-tcp', send: (bytes, c) => sent.push(['tcp', bytes, c.endpoint]) });
  const bytes = encodeForPrinter({ text: 'hello' }, { protocol, transport: 'test-usb' });
  await sendToPrinter(bytes, { protocol, transport: 'test-usb', endpoint: 'usb:1' });
  await sendToPrinter(bytes, { protocol, transport: 'test-tcp', endpoint: '10.0.0.8:9100' });
  assert.equal(new TextDecoder().decode(sent[0][1]), '^XA^FDhello^XZ');
  assert.deepEqual(sent.map(x => [x[0], x[2]]), [['usb', 'usb:1'], ['tcp', '10.0.0.8:9100']]);
});

test('the public catalog identifies distinct protocol and transport values', () => {
  assert.notEqual(PrinterProtocol.ESC_POS, PrinterTransport.BLUETOOTH_CLASSIC);
  assert.equal(PrinterProtocol.ZPL, 'zpl');
  assert.equal(PrinterTransport.WINDOWS_SPOOLER, 'windows-spooler');
});
