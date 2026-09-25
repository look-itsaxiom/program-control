import { render, screen } from '@testing-library/react';
import { expect, it } from 'vitest';
import App from '../App';
import { phaseGate } from './helpers';

const describe = phaseGate(0);

describe('phase 0 scaffold', () => {
  it('renders the placeholder', () => {
    render(<App />);
    expect(screen.getByTestId('placeholder')).toBeInTheDocument();
  });

  it('runs under the test env mode', () => {
    expect(import.meta.env.MODE).toBe('test');
  });
});
