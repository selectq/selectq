import { of, Subject, throwError } from 'rxjs';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { AgGridAngular } from 'ag-grid-angular';
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
  it('downloads a PDF for the saved invoice ID', () => {
    const pdf = new Blob(['%PDF-1.7'], { type: 'application/pdf' });
    const download = jasmine.createSpy('download').and.returnValue(of(pdf));
    component = new ListComponent({ downloadSalesInvoicePDF: download } as unknown as ApiService);
    spyOn(URL, 'createObjectURL').and.returnValue('blob:invoice');
    const click = spyOn(HTMLAnchorElement.prototype, 'click');
    component.downloadPDF(invoice);
    expect(download).toHaveBeenCalledWith(1);
    expect(URL.createObjectURL).toHaveBeenCalledWith(pdf);
    expect(click).toHaveBeenCalled();
    expect((click.calls.mostRecent().object as HTMLAnchorElement).download).toBe('Invoice-001-2026-2027.pdf');
  });
  it('shows PDF generation errors', () => {
    component = new ListComponent({ downloadSalesInvoicePDF: () => throwError(() => new Error('failed')) } as unknown as ApiService);
    spyOn(window, 'alert'); component.downloadPDF(invoice);
    expect(window.alert).toHaveBeenCalledWith('Could not generate the invoice PDF. Please try again.');
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

describe('Sales invoice grid return navigation', () => {
  let fixture: ComponentFixture<ListComponent>;
  let component: ListComponent;
  let refreshedInvoices: Subject<SalesInvoice[]>;
  let invoices: SalesInvoice[];

  const settleGrid = async () => {
    fixture.detectChanges();
    await fixture.whenStable();
    await new Promise(resolve => setTimeout(resolve, 50));
    fixture.detectChanges();
  };
  const grid = () => fixture.debugElement.query(By.directive(AgGridAngular)).componentInstance as AgGridAngular;

  beforeEach(async () => {
    invoices = Array.from({ length: 130 }, (_, index) => ({
      id: index + 1, invoice_number: `${String(index + 1).padStart(3, '0')}/2026-2027`,
      financial_year: '2026-2027', business_partner_id: 1,
      invoice_date: '2026-09-11', currency: index < 120 ? 'INR' : 'USD', amount: 100,
      line_items: [{ description: 'Services', quantity: 1, rate: 100, amount: 100, gst_percent: 0, hsn_sac_code: '' }]
    }));
    refreshedInvoices = new Subject<SalesInvoice[]>();
    await TestBed.configureTestingModule({
      imports: [ListComponent],
      providers: [{ provide: ApiService, useValue: {
        getBusinessPartners: () => of([]),
        getSalesInvoices: jasmine.createSpy().and.returnValues(of(invoices), refreshedInvoices),
        getCompanyProfile: () => of(null),
        updateSalesInvoice: () => of({})
      } }]
    }).compileComponents();
    fixture = TestBed.createComponent(ListComponent);
    component = fixture.componentInstance;
    await settleGrid();
  });

  it('restores filters, sort, page size and page after updating and refreshing invoices', async () => {
    const api = grid().api;
    const filters = { currency: { filterType: 'text', type: 'equals', filter: 'INR' } };
    api.setFilterModel(filters);
    api.applyColumnState({ state: [{ colId: 'invoice_number', sort: 'desc' }] });
    api.setGridOption('paginationPageSize', 50);
    api.paginationGoToPage(1);
    await settleGrid();

    component.editInvoice(api.getDisplayedRowAtIndex(50)!.data);
    await settleGrid();
    expect(fixture.nativeElement.querySelector('h3').textContent)
      .toContain(`Edit Sales Invoice: ${component.formInvoice.invoice_number}`);
    component.lineItems[0].description = 'Updated services';
    fixture.nativeElement.querySelector('.form-actions .btn-primary').click();
    await settleGrid();
    expect(component.isLoading).toBeTrue();
    refreshedInvoices.next(invoices.map(inv => inv.id === component.formInvoice.id
      ? { ...component.formInvoice } : inv));
    refreshedInvoices.complete();
    await settleGrid();

    const restored = grid().api;
    expect(component.activeTab).toBe('view');
    expect(restored.getFilterModel()).toEqual(filters);
    expect(restored.paginationGetPageSize()).toBe(50);
    expect(restored.paginationGetCurrentPage()).toBe(1);
    expect(restored.getDisplayedRowCount()).toBe(120);
    expect(restored.getColumnState().find(col => col.colId === 'invoice_number')?.sort).toBe('desc');
    expect(restored.getDisplayedRowAtIndex(50)!.data.line_items[0].description).toBe('Updated services');
  });

  it('restores the current page when editing is cancelled', async () => {
    grid().api.paginationGoToPage(2);
    component.editInvoice(invoices[40]);
    await settleGrid();
    component.cancelForm();
    await settleGrid();
    expect(grid().api.paginationGetCurrentPage()).toBe(2);
  });
});
