import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { AgGridAngular } from 'ag-grid-angular';
import { convertToParamMap } from '@angular/router';
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

// Exercise the template event binding and reveal the rendered allocation form.
describe('Transaction allocation grid interaction', () => {
  it('opens and reveals the clicked receipt, including an empty invoice cell', async () => {
    const api = jasmine.createSpyObj('ApiService', ['getSalesInvoices', 'getTransactions', 'getBusinessPartners']);
    const txn = { id: 8, invoice_number: '', narration: 'Receipt', deposit_amt: 100, allocations: [] } as unknown as BankTransaction;
    api.getTransactions.and.returnValue(of([txn]));
    api.getSalesInvoices.and.returnValue(of([]));
    api.getBusinessPartners.and.returnValue(of([]));
    await TestBed.configureTestingModule({ imports: [TransactionsComponent], providers: [
      { provide: ApiService, useValue: api },
      { provide: ActivatedRoute, useValue: { paramMap: of(convertToParamMap({ id: '3' })) } },
      { provide: Router, useValue: {} }
    ] }).compileComponents();
    const fixture = TestBed.createComponent(TransactionsComponent);
    const scroll = spyOn(HTMLElement.prototype, 'scrollIntoView');
    fixture.detectChanges();
    const grid = fixture.debugElement.query(By.directive(AgGridAngular));
    grid.triggerEventHandler('cellClicked', { colDef: { field: 'narration' }, data: txn });
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('.allocation-panel')).toBeNull();
    grid.triggerEventHandler('cellClicked', { colDef: { field: 'invoice_number' }, data: txn });
    fixture.detectChanges();
    const panel = fixture.nativeElement.querySelector('.allocation-panel') as HTMLElement;
    expect(panel).not.toBeNull();
    expect(panel.textContent).toContain('Receipt');
    expect(api.getSalesInvoices).toHaveBeenCalledTimes(1);
    expect(scroll).toHaveBeenCalledWith({ block: 'start', behavior: 'smooth' });
    expect(fixture.componentInstance.selected?.id).toBe(8);
    fixture.componentInstance.saving = true;
    grid.triggerEventHandler('cellClicked', { colDef: { field: 'invoice_number' }, data: { ...txn, id: 9 } });
    expect(fixture.componentInstance.selected?.id).toBe(8);
    fixture.destroy();
  });
});
