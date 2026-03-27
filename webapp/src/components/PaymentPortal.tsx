import { Panel } from './Panel';

export function PaymentPortal() {
  return (
    <Panel title="Payment Portal" subtitle="Treasury settlement">
      <form className="payment-form" onSubmit={(e) => e.preventDefault()}>
        <label>
          Counterparty
          <input placeholder="Prime Broker A" />
        </label>
        <label>
          Amount (USD)
          <input type="number" placeholder="250000" />
        </label>
        <label>
          Method
          <select defaultValue="wire">
            <option value="wire">Bank Wire</option>
            <option value="usdc">USDC Transfer</option>
            <option value="ach">ACH</option>
          </select>
        </label>
        <button className="primary-btn" type="submit">Authorize Payment</button>
      </form>
    </Panel>
  );
}
