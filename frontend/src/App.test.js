import { render, screen } from '@testing-library/react';
import App from './App';

test('renders demo title', () => {
  render(<App />);
  const title = screen.getByText(/Omnichannel Inventory - Demo UI/i);
  expect(title).toBeInTheDocument();
});
