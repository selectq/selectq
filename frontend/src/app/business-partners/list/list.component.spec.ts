import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FormBuilder } from '@angular/forms';
import { ActivatedRoute, convertToParamMap, Router } from '@angular/router';
import { of } from 'rxjs';
import { ApiService, BusinessPartner } from '../../api.service';
import { ListComponent } from './list.component';
import { FormComponent } from '../form/form.component';

describe('Optional business partner contacts', () => {
  let fixture: ComponentFixture<ListComponent>;
  let component: ListComponent;
  let api: jasmine.SpyObj<ApiService>;
  const partner: BusinessPartner = { id: 1, name: 'No-contact partner', billing_address: '', invoice_currency: 'INR', tax_information: '', contacts: [] };
  beforeEach(async () => {
    api = jasmine.createSpyObj('ApiService', ['getBusinessPartners', 'getBusinessPartner', 'createBusinessPartner', 'updateBusinessPartner']);
    api.getBusinessPartners.and.returnValue(of([partner])); api.getBusinessPartner.and.returnValue(of(partner));
    api.createBusinessPartner.and.returnValue(of(partner)); api.updateBusinessPartner.and.returnValue(of(partner));
    await TestBed.configureTestingModule({ imports: [ListComponent], providers: [{ provide: ApiService, useValue: api }] }).compileComponents();
    fixture = TestBed.createComponent(ListComponent); component = fixture.componentInstance; fixture.detectChanges();
  });
  it('enables Save for a new partner without contacts and submits an empty list', () => {
    component.addNew(); component.bpForm.patchValue({ name: partner.name }); fixture.detectChanges();
    expect(component.contacts.length).toBe(0); expect(component.bpForm.valid).toBeTrue();
    expect(fixture.nativeElement.querySelector('button[type="submit"]').disabled).toBeFalse();
    expect(fixture.nativeElement.textContent).toContain('Contacts (optional)');
    component.onSubmit(); expect(api.createBusinessPartner.calls.mostRecent().args[0].contacts).toEqual([]);
  });
  it('edits an existing partner with no contacts without creating a blank row', () => {
    component.selectedPartner = partner; component.editPartner();
    expect(component.contacts.length).toBe(0); expect(component.bpForm.valid).toBeTrue();
    component.onSubmit(); expect(api.updateBusinessPartner.calls.mostRecent().args[1].contacts).toEqual([]);
  });
  it('allows removing the last contact and marks the next added contact primary', () => {
    component.addNew(); component.bpForm.patchValue({ name: partner.name }); component.addContact(); fixture.detectChanges();
    expect(component.bpForm.invalid).toBeTrue();
    const remove = fixture.nativeElement.querySelector('button[title="Remove Contact"]'); expect(remove).not.toBeNull(); remove.click();
    expect(component.contacts.length).toBe(0); expect(component.bpForm.valid).toBeTrue();
    component.addContact(); expect(component.contacts.at(0).get('is_primary')!.value).toBeTrue();
  });
  it('preserves existing named contacts and requires a name only when a contact is added', () => {
    component.selectedPartner = { ...partner, contacts: [{ name: 'Contact', email: '', phone: '', is_primary: true }] };
    component.editPartner(); expect(component.bpForm.valid).toBeTrue(); expect(component.contacts.length).toBe(1);
    component.addContact(); expect(component.bpForm.invalid).toBeTrue();
    component.contacts.at(1).patchValue({ name: 'Second contact' }); expect(component.bpForm.valid).toBeTrue();
    component.removeContact(0); expect(component.contacts.at(0).get('is_primary')!.value).toBeTrue();
  });
  it('also supports contact-free creation in the standalone form', () => {
    const router = jasmine.createSpyObj<Router>('Router', ['navigate']);
    const form = new FormComponent(new FormBuilder(), api, router, {} as ActivatedRoute);
    form.bpForm.patchValue({ name: partner.name }); expect(form.bpForm.valid).toBeTrue();
    form.addContact(); form.removeContact(0); expect(form.contacts.length).toBe(0);
    form.onSubmit(); expect(api.createBusinessPartner.calls.mostRecent().args[0].contacts).toEqual([]); form.ngOnDestroy();
  });
  it('also preserves empty contacts on standalone edit', () => {
    const router = jasmine.createSpyObj<Router>('Router', ['navigate']);
    const route = { paramMap: of(convertToParamMap({ id: '1' })) } as ActivatedRoute;
    const form = new FormComponent(new FormBuilder(), api, router, route); form.ngOnInit();
    expect(form.contacts.length).toBe(0); expect(form.bpForm.valid).toBeTrue();
    form.onSubmit(); expect(api.updateBusinessPartner.calls.mostRecent().args[1].contacts).toEqual([]); form.ngOnDestroy();
  });
});
