import { of, throwError } from 'rxjs';
import { TransactionsComponent } from './transactions.component';
import { ApiService, BankTransaction, SalesInvoice } from '../api.service';
import { ActivatedRoute, Router } from '@angular/router';

describe('Transaction invoice allocation', () => {
  let component: TransactionsComponent;
  let api: jasmine.SpyObj<ApiService>;
  beforeEach(() => {
    api = jasmine.createSpyObj('ApiService', ['getSalesInvoices', 'updateTransaction', 'getTransactions']);
    api.getSalesInvoices.and.returnValue(of([]));
    api.getTransactions.and.returnValue(of([]));
    component = new TransactionsComponent(api, {} as ActivatedRoute, {} as Router);
  });
  it('suggests open invoices for the chosen partner and retains existing closed links', () => {
    component.selected = { business_partner_id: 1 } as BankTransaction;
    component.allocations = [{ sales_invoice_id: 3, amount: 20 }];
    component.invoices = [
      { id: 1, business_partner_id: 1 },
      { id: 2, business_partner_id: 2 },
      { id: 3, business_partner_id: 1, is_closed: true },
      { id: 4, business_partner_id: 1, is_closed: true },
      { id: 5, business_partner_id: 1, is_settled: true }
    ] as SalesInvoice[];
    expect(component.availableInvoices.map(i => i.id)).toEqual([1, 3]);
  });
  it('edits a copy so cancelling preserves saved allocations', () => {
    const txn = { allocations: [{ sales_invoice_id: 1, amount: 10 }] } as BankTransaction;
    component.openAllocation(txn);
    component.setAllocation(1, 20);
    expect(txn.allocations![0].amount).toBe(10);
  });
  it('keeps the form and backend validation error when saving fails', () => {
    component.selected = { id: 1, business_partner_id: 1 } as BankTransaction;
    api.updateTransaction.and.returnValue(throwError(() => ({ error: 'Allocation exceeds outstanding amount' })));
    component.saveAllocations();
    expect(component.selected).not.toBeNull();
    expect(component.allocationError).toContain('outstanding');
    expect(component.saving).toBeFalse();
  });
});
