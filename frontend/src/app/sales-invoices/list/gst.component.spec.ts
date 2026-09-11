import { EMPTY } from 'rxjs';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ApiService } from '../../api.service';
import { ListComponent } from './list.component';

describe('Invoice GST fields', () => {
  let component: ListComponent;
  let fixture: ComponentFixture<ListComponent>;
  beforeEach(async () => {
    await TestBed.configureTestingModule({ imports: [ListComponent], providers: [{ provide: ApiService, useValue: { getBusinessPartners: () => EMPTY, getCompanyProfile: () => EMPTY } }] }).compileComponents();
    fixture = TestBed.createComponent(ListComponent); component = fixture.componentInstance;
    component.companyProfile = { company_name: 'Seller', address: '', gstin: '29AABCU9603R1ZP', pan: '', email: '', phone: '' };
    component.businessPartners = [{ id: 1, name: 'Buyer', billing_address: '', invoice_currency: 'INR', tax_information: '', addresses: [
      { id: 1, address: 'Same state', gstin: '29ABCDE1234F1Z5', is_archived: false },
      { id: 2, address: 'Another state', gstin: '27ABCDE1234F1Z5', is_archived: false },
      { id: 3, address: 'No registration', gstin: '', is_archived: false }
    ] }];
    component.switchToCreate(); component.formInvoice.business_partner_id = 1; component.onBusinessPartnerChange();
    component.addLineItem(); component.lineItems[0].rate = 100; component.recalcLineAmount(component.lineItems[0]); fixture.detectChanges();
  });
  function field(tax: string): HTMLInputElement { return fixture.nativeElement.querySelector(`[aria-label="${tax} percent for line 1"]`); }
  it('shows 9/9 and disables IGST, then switches to 18% IGST for another address', () => {
    expect(field('CGST').value).toBe('9'); expect(field('SGST').value).toBe('9'); expect(field('IGST').value).toBe('0');
    expect(field('IGST').disabled).toBeTrue(); expect(field('CGST').disabled).toBeFalse();
    expect(component.totalCGST).toBe(9); expect(component.totalSGST).toBe(9);
    component.formInvoice.address_id = 2; fixture.detectChanges();
    expect(field('IGST').value).toBe('18'); expect(field('IGST').disabled).toBeFalse();
    expect(field('CGST').value).toBe('0'); expect(field('CGST').disabled).toBeTrue(); expect(field('SGST').disabled).toBeTrue();
    expect(component.totalIGST).toBe(18); expect(component.grandTotal).toBe(118);
  });
  it('uses the configured same-state split when buyer GSTIN is absent', () => {
    component.formInvoice.address_id = 3; fixture.detectChanges();
    expect(component.gstTreatment).toBe('intrastate'); expect(field('IGST').disabled).toBeTrue();
    expect(fixture.nativeElement.textContent).toContain('fallback');
  });
  it('does not show GST fields on a non-INR invoice', () => {
    component.formInvoice.currency = 'USD'; component.onCurrencyChange(); fixture.detectChanges();
    expect(field('IGST')).toBeNull(); expect(component.totalGst).toBe(0);
  });
  it('prints saved GSTINs and tax split after the company profile changes', () => {
    const write = jasmine.createSpy('write'); spyOn(window, 'open').and.returnValue({ document: { write, close: () => {} } } as unknown as Window);
    component.downloadPDF({ ...component.formInvoice, line_items: component.lineItems, seller_gstin: '27AABCU9603R1ZP', buyer_gstin: '29ABCDE1234F1Z5', gst_treatment: 'interstate' });
    const html = write.calls.mostRecent().args[0] as string;
    expect(html).toContain('27AABCU9603R1ZP'); expect(html).not.toContain('29AABCU9603R1ZP');
    expect(html).toContain('<td class="label">IGST</td>'); expect(html).not.toContain('<td class="label">CGST</td>');
  });
});
