import { of, throwError, Subject } from 'rxjs';
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

describe('Link existing purchase invoice', () => {
 it('links an available invoice without resaving its details', () => {
  const api = jasmine.createSpyObj('ApiService', ['linkPurchaseInvoice', 'savePurchaseInvoice']);
  api.linkPurchaseInvoice.and.returnValue(of({}));
  const c = new PurchaseInvoicesComponent(api, {} as ActivatedRoute);
  c.sourceTransactionId = 12; c.selectedInvoiceId = 5;
  c.invoices = [{ id: 5, bank_transaction_id: null, party_name: 'Supplier', external_url: 'https://example.com/invoice' }, { id: 6, bank_transaction_id: 8 }] as PurchaseInvoice[];
  expect(c.unlinkedInvoices.map(i => i.id)).toEqual([5]);
  c.linkExisting();
  expect(api.linkPurchaseInvoice).toHaveBeenCalledWith(5, 12);
  expect(api.savePurchaseInvoice).not.toHaveBeenCalled();
  expect(c.invoices[0].bank_transaction_id).toBe(12);
  expect(c.invoices[0].external_url).toBe('https://example.com/invoice');
  expect(c.sourceTransactionId).toBeNull(); c.ngOnDestroy();
 });
});

describe('Purchase GSTIN autofill', () => {
 it('normalizes GSTIN and fills saved party details', () => {
  const api = jasmine.createSpyObj('ApiService', ['getPurchaseParty']);
  api.getPurchaseParty.and.returnValue(of({gstin:'27ABCDE1234F1Z5',party_name:'Supplier',party_address:'City'}));
  const c = new PurchaseInvoicesComponent(api, {} as ActivatedRoute); c.add();
  c.gstinChanged('27abcde1234f1z5');
  expect(api.getPurchaseParty).toHaveBeenCalledWith('27ABCDE1234F1Z5');
  expect(c.draft?.party_name).toBe('Supplier'); expect(c.draft?.party_address).toBe('City');
  c.ngOnDestroy();
 });
 it('preserves manual edits made while a lookup is pending', () => {
  const api = jasmine.createSpyObj('ApiService', ['getPurchaseParty']);
  const response = new Subject<any>(); api.getPurchaseParty.and.returnValue(response);
  const c = new PurchaseInvoicesComponent(api, {} as ActivatedRoute); c.add();
  c.gstinChanged('27ABCDE1234F1Z5'); c.draft!.party_name = 'Manual';
  response.next({party_name:'Cached',party_address:'City'});
  expect(c.draft?.party_name).toBe('Manual');
  c.gstinChanged('27ABCDE1234F1Z5'); c.add();
  response.next({party_name:'Old response',party_address:'City'});
  expect(c.draft?.party_name).toBe(''); c.ngOnDestroy();
 });
});

describe('Purchase invoice PDF upload', () => {
 it('opens an extracted draft and retains the PDF for saving', () => {
  const api = jasmine.createSpyObj('ApiService', ['parsePurchaseInvoice', 'savePurchaseInvoice']);
  api.parsePurchaseInvoice.and.returnValue(of({ invoice_number: 'DPO2721818165521', invoice_date: '2026-08-06', party_name: 'HDFC Bank Ltd', party_address: 'Branch', party_gstin: '06AAACH2702H1Z4', total_amount: 2297.45 }));
  const c = new PurchaseInvoicesComponent(api, {} as ActivatedRoute); c.sourceTransactionId = 12;
  const file = new File(['%PDF-1.4'], 'invoice.pdf', {type:'application/pdf'});
  c.parseInvoice({ target: { files: [file], value: 'invoice.pdf' } } as unknown as Event);
  expect(c.draft?.invoice_number).toBe('DPO2721818165521');
  expect(c.draft?.total_amount).toBe(2297.45); expect(c.draft?.bank_transaction_id).toBe(12);
  expect(c.file).toBe(file); expect(api.savePurchaseInvoice).not.toHaveBeenCalled(); c.ngOnDestroy();
 });
 it('preserves the draft when extraction fails', () => {
  const api = jasmine.createSpyObj('ApiService', ['parsePurchaseInvoice']);
  api.parsePurchaseInvoice.and.returnValue(throwError(() => ({ error: 'Unsupported invoice' })));
  const c = new PurchaseInvoicesComponent(api, {} as ActivatedRoute); c.add(); c.draft!.party_name = 'Manual';
  const file = new File(['%PDF-1.4'], 'invoice.pdf');
  c.parseInvoice({target:{files:[file],value:''}} as unknown as Event);
  expect(c.draft?.party_name).toBe('Manual'); expect(c.error).toBe('Unsupported invoice'); expect(c.parsing).toBeFalse(); c.ngOnDestroy();
 });
});
