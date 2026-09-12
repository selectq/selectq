import { of, throwError } from 'rxjs';
import { ActivatedRoute, convertToParamMap } from '@angular/router';
import { PurchaseInvoicesComponent } from './purchase-invoices.component';
import { ApiService, BankTransaction, PurchaseInvoice } from '../api.service';

describe('Purchase invoice register', () => {
 let api: jasmine.SpyObj<ApiService>;
 beforeEach(() => {
  api = jasmine.createSpyObj('ApiService', ['getPurchaseInvoices', 'getTransactions', 'savePurchaseInvoice']);
  api.getPurchaseInvoices.and.returnValue(of([]));
 });
 function component(params: Record<string, string> = {}) {
  return new PurchaseInvoicesComponent(api, { snapshot: { queryParamMap: convertToParamMap(params) } } as ActivatedRoute);
 }
 it('prefills a linked withdrawal and submits its upload with the invoice', () => {
  api.getTransactions.and.returnValue(of([{ id: 12, date: '12/09/26', withdrawal_amt: 118, account_head: 'Software/Services', narration: 'Supplier payment' } as BankTransaction]));
  const c = component({ transaction_id: '12', import_id: '3' }); c.ngOnInit();
  expect(c.draft?.bank_transaction_id).toBe(12); expect(c.draft?.total_amount).toBe(118);
  expect(c.draft?.invoice_date).toBe('2026-09-12'); expect(c.source).toBe('Supplier payment');
  c.draft!.party_name = 'Supplier'; c.draft!.party_address = 'City';
  const invoice = { ...c.draft! }; const file = new File(['%PDF-1.4'], 'invoice.pdf', { type: 'application/pdf' }); c.file = file;
  api.savePurchaseInvoice.and.returnValue(of({ ...invoice, id: 1 })); c.save();
  expect(api.savePurchaseInvoice).toHaveBeenCalledWith(invoice, file);
  expect(c.draft).toBeNull(); expect(c.message).toContain('saved'); c.ngOnDestroy();
 });
 it('opens an existing linked invoice and retains edits after save failure', () => {
  const invoice: PurchaseInvoice = { id: 5, bank_transaction_id: 12, invoice_number: '', invoice_date: '2026-09-12', party_name: 'Supplier', party_address: 'City', party_gstin: '', total_amount: 118, file_name: 'invoice.pdf' };
  api.getPurchaseInvoices.and.returnValue(of([invoice]));
  const c = component({ id: '5' }); c.ngOnInit();
  expect(api.getTransactions).not.toHaveBeenCalled(); expect(c.draft?.id).toBe(5);
  c.draft!.party_name = 'Updated'; expect(invoice.party_name).toBe('Supplier');
  api.savePurchaseInvoice.and.returnValue(throwError(() => ({error: 'Save failed'}))); c.save();
  expect(c.draft?.party_name).toBe('Updated'); expect(c.error).toBe('Save failed'); expect(c.saving).toBeFalse(); c.ngOnDestroy();
 });
 it('rejects personal withdrawals even when opened through a direct URL', () => {
  api.getTransactions.and.returnValue(of([{ id: 12, withdrawal_amt: 100, account_head: 'To Personal' } as BankTransaction]));
  const c = component({transaction_id: '12', import_id: '3'}); c.ngOnInit();
  expect(c.draft).toBeNull(); expect(c.error).toContain('not eligible'); c.ngOnDestroy();
 });
});
