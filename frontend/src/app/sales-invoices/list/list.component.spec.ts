import { ListComponent } from './list.component';
import { ApiService, BusinessPartner, SalesInvoice } from '../../api.service';

describe('Invoice billing address history', () => {
  let component: ListComponent;
  const partner: BusinessPartner = {
    id: 1, name: 'Customer', billing_address: 'Current partner location', invoice_currency: 'INR', tax_information: '',
    addresses: [
      { id: 10, address: 'Historical office', is_archived: true },
      { id: 11, address: 'Current office', is_archived: false },
      { id: 12, address: 'Another office', is_archived: false }
    ]
  };
  const invoice: SalesInvoice = { id: 1, invoice_number: '001/2026-2027', financial_year: '2026-2027', business_partner_id: 1, invoice_date: '2026-09-11', currency: 'INR', amount: 100, address_id: 10, billing_address: 'Historical office' };
  beforeEach(() => { component = new ListComponent({} as ApiService); component.businessPartners = [partner]; });
  it('defaults to the first active address when multiple addresses exist', () => {
    component.formInvoice.business_partner_id = 1; component.onBusinessPartnerChange();
    expect(component.activeAddresses.map(a => a.id)).toEqual([11, 12]);
    expect(component.formInvoice.address_id).toBe(11); expect(component.retainedArchivedAddress).toBeUndefined();
  });
  it('retains the original archived address while editing', () => {
    component.editInvoice(invoice); expect(component.retainedArchivedAddress?.id).toBe(10);
    expect(component.formInvoice.address_id).toBe(10);
    component.formInvoice.business_partner_id = 2; component.onBusinessPartnerChange();
    expect(component.formInvoice.address_id).toBeNull(); expect(component.retainedArchivedAddress).toBeUndefined();
  });
  it('prints the invoice address rather than the current partner address', () => {
    const write = jasmine.createSpy('write');
    spyOn(window, 'open').and.returnValue({ document: { write, close: () => {} } } as unknown as Window);
    component.downloadPDF(invoice);
    const html = write.calls.mostRecent().args[0] as string;
    expect(html).toContain('Historical office'); expect(html).not.toContain('Current partner location');
  });
  it('uses partner currency for new lines without changing an existing invoice on edit', () => {
    component.businessPartners = [{ ...partner, invoice_currency: 'USD' }];
    component.formInvoice.business_partner_id = 1;
    component.onBusinessPartnerChange(); component.addLineItem();
    expect(component.formInvoice.currency).toBe('USD');
    expect(component.lineItems[0].gst_percent).toBe(0);
    expect(component.lineItems[0].description).toBe('Architectural Services for CAD Documentation');
    component.editInvoice({ ...invoice, currency: 'EUR', due_in_days: 30 });
    expect(component.formInvoice.currency).toBe('EUR');
    expect(component.paymentDueDays).toBe('30');
  });
  it('calculates due dates from typed day counts across month boundaries', () => {
    component.switchToCreate(); component.formInvoice.invoice_date = '2026-09-25'; component.onDateChange();
    expect(component.paymentDueDays).toBe('10');
    expect(component.formInvoice.due_date).toBe('2026-10-05');
    component.onPaymentDueDaysChange('15');
    expect(component.formInvoice.due_in_days).toBe(15);
    expect(component.formInvoice.due_date).toBe('2026-10-10');
    component.onPaymentDueDaysChange('0');
    expect(component.formInvoice.due_date).toBe('2026-09-25');
  });
  it('clears stale due dates when the day count is invalid', () => {
    component.formInvoice.invoice_date = '2026-09-25';
    for (const value of ['', '-1', '1.5', 'abc', '99999999999999999999']) {
      component.onPaymentDueDaysChange('10'); component.onPaymentDueDaysChange(value);
      expect(component.formInvoice.due_in_days).toBeUndefined();
      expect(component.formInvoice.due_date).toBe('');
    }
  });

});
