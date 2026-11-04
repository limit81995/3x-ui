import { describe, expect, it } from 'vitest';

import { DBInbound } from '@/models/dbinbound';
import { rawInboundToFormValues, formValuesToWirePayload } from '@/lib/xray/inbound-form-adapter';
import { buildClonePayload } from '@/lib/xray/inbound-clone';
import { InboundDbFieldsSchema } from '@/schemas/forms/inbound-form';

describe('inbound port speed limit', () => {
  it.each([0, 100, 10000])('preserves %d Mbps through edit and submit', (speedLimit) => {
    const inbound = new DBInbound({
      port: 443,
      protocol: 'vless',
      speedLimit,
      settings: { clients: [], decryption: 'none' },
      streamSettings: { network: 'tcp', security: 'none' },
    });
    const values = rawInboundToFormValues(inbound);
    expect(values.speedLimit).toBe(speedLimit);
    const payload = formValuesToWirePayload(values);
    expect(payload.speedLimit).toBe(speedLimit);
    expect(JSON.parse(payload.settings)).not.toHaveProperty('speedLimit');
    expect(buildClonePayload(inbound, 8443, null).speedLimit).toBe(speedLimit);
  });

  it('defaults legacy records to unlimited and rejects invalid rates', () => {
    expect(new DBInbound().speedLimit).toBe(0);
    expect(InboundDbFieldsSchema.parse({}).speedLimit).toBe(0);
    for (const speedLimit of [-1, 1.5, 10001, Number.NaN]) {
      expect(InboundDbFieldsSchema.safeParse({ speedLimit }).success).toBe(false);
    }
  });
});
