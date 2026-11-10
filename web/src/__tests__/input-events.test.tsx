import { fireEvent, render, screen } from '@testing-library/react';
import { expect, it } from 'vitest';
import InputEvents from '../components/InputEvents';
import { mockApi } from '../test/mock';
it('shows quarantine and requests the next exclusive cursor',async()=>{
 const calls=mockApi({'GET /api/v1/inputs/orders/events':{events:[{sequence:7,status:'quarantined',reason:'batch violates data contract',received_at:'2026-11-10T01:00:00Z',document:{at:'2026-11-10T00:00:00Z'}}],next:7,oldest:5,truncated:true}});
 render(<InputEvents channel="orders" />);
 expect(await screen.findByText('quarantined')).toBeTruthy();
 expect(screen.getByText(/Earlier events have expired/)).toBeTruthy();
 fireEvent.click(screen.getByRole('button',{name:'Next page'}));
 expect(await screen.findByText('batch violates data contract')).toBeTruthy();
 expect(calls[0].method).toBe('GET');
});
