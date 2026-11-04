import { describe, expect, it } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { TableScroll } from '../components/ui';

describe('TableScroll', () => {
  it('labels each cell with its column and keeps the table roles', () => {
    render(
      <TableScroll>
        <table>
          <caption>Things</caption>
          <thead>
            <tr>
              <th>Name</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>Alpha</td>
              <td>ok</td>
            </tr>
          </tbody>
        </table>
      </TableScroll>,
    );
    const table = screen.getByRole('table', { name: 'Things' });
    const row = within(table).getAllByRole('row')[1];
    const cells = within(row).getAllByRole('cell');
    expect(cells.map((c) => c.getAttribute('data-label'))).toEqual(['Name', 'Status']);
    expect(within(table).getAllByRole('columnheader')).toHaveLength(2);
  });
});
